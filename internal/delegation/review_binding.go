package delegation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

type ReviewBinding struct {
	FingerprintVersion int    `json:"fingerprint_version"`
	DefinitionSHA256   string `json:"definition_sha256"`
	ChangeSHA256       string `json:"change_sha256"`
}

func hashJSON(value any) string {
	data, _ := json.Marshal(value)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// fingerprintFile classifies one allowed-files entry. Existing regular files keep the
// streaming content digest; existing directories and glob patterns are digested from the
// git-tracked regular files they cover — or, in a workspace that is not inside a git work
// tree, from a bounded filesystem walk (see listTreeFiles); a missing path keeps the
// literal "absent" sentinel. Tracking-aware digests keep the binding bounded — an ignored dependency cache
// with no tracked files collapses to one stable value — while still detecting every change
// to tracked content. It rejects symlinks, including parent components, and the result is
// fixed-width, so the binding persisted in binding_json stays far below its 64 KiB column
// cap no matter how large a tree is. It does not sandbox other processes or close the
// check-to-write race.
func fingerprintFile(workspace, relative string) (string, error) {
	clean, err := canonicalLeasePath(relative)
	if err != nil {
		return "", err
	}
	info, statErr := lstatChain(workspace, clean)
	if statErr != nil {
		if isGlobPattern(clean) {
			return fingerprintTrackedTree(workspace, clean)
		}
		if errors.Is(statErr, os.ErrNotExist) {
			return "absent", nil
		}
		return "", statErr
	}
	if info.IsDir() {
		return fingerprintTrackedTree(workspace, clean)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("fingerprint requires explicit regular files or directories")
	}
	return hashRegularFile(workspace, clean)
}

// hashRegularFile streams one regular file's content digest. The name is used as-is so
// a tracked path keeps the case git recorded, even where canonicalLeasePath folds case.
func hashRegularFile(workspace, name string) (string, error) {
	f, err := os.Open(filepath.Join(workspace, filepath.FromSlash(name)))
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "file:" + hex.EncodeToString(h.Sum(nil)), nil
}

// lstatChain walks every component and rejects symlinks. A missing component surfaces the
// raw os.ErrNotExist so the caller can map it to the "absent" sentinel.
func lstatChain(workspace, clean string) (os.FileInfo, error) {
	current := workspace
	var last os.FileInfo
	for _, component := range strings.Split(filepath.ToSlash(clean), "/") {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("fingerprint paths must not contain symlinks")
		}
		last = info
	}
	return last, nil
}

// isGlobPattern reports whether an entry is a path pattern rather than a literal path.
// Only "*" and "?" are treated as glob metacharacters; both are illegal in Windows file
// names, which is what let literal Lstat break glob entries on that platform.
func isGlobPattern(value string) bool {
	return strings.ContainsAny(value, "*?")
}

// fingerprintTrackedTree digests the regular files covered by a directory or glob entry:
// the git-tracked set inside a work tree, or the bounded filesystem walk that
// listTreeFiles falls back to otherwise. Only tracked files are read, so ignored build
// output and dependency caches cannot make the binding unbounded, and a tree with no
// tracked files hashes to the stable empty-set digest instead of failing.
func fingerprintTrackedTree(workspace, pattern string) (string, error) {
	tracked, err := listTreeFiles(workspace, pattern)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, name := range tracked {
		info, statErr := os.Lstat(filepath.Join(workspace, filepath.FromSlash(name)))
		if statErr == nil && !info.Mode().IsRegular() {
			continue // symlinks and submodule gitlinks are not regular files
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		digest := "absent" // tracked in the index but deleted from the worktree
		if statErr == nil {
			if _, err := lstatChain(workspace, name); err != nil {
				return "", err
			}
			if digest, err = hashRegularFile(workspace, name); err != nil {
				return "", err
			}
		}
		if _, err := fmt.Fprintf(h, "%s\x00%s\x00", name, digest); err != nil {
			return "", err
		}
	}
	return "tree:" + hex.EncodeToString(h.Sum(nil)), nil
}

// trackedFilesTimeout bounds the git query that backs a directory or glob fingerprint.
const trackedFilesTimeout = 30 * time.Second

// caseInsensitiveLeasePaths mirrors canonicalLeasePath, which folds case on these
// platforms. Git pathspecs stay case-sensitive regardless of core.ignorecase, so the
// folded pattern needs the icase magic to match mixed-case tracked paths.
var caseInsensitiveLeasePaths = runtime.GOOS == "windows" || runtime.GOOS == "darwin"

// treePathspec builds the git pathspec for a canonical directory or glob entry. Without
// icase the pattern passes through unchanged, so all-lowercase trees resolve exactly as
// they always have.
func treePathspec(pattern string, icase bool) string {
	if icase {
		return ":(icase)" + pattern
	}
	return pattern
}

// listTrackedFiles returns the sorted workspace-relative paths of git-tracked files that
// match a directory entry or glob pattern.
func listTrackedFiles(workspace, pattern string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), trackedFilesTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "-C", workspace, "ls-files", "-z", "--", treePathspec(pattern, caseInsensitiveLeasePaths))
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("list git-tracked files for %s: %w: %s", pattern, err, strings.TrimSpace(stderr.String()))
	}
	fields := bytes.Split(stdout.Bytes(), []byte{0})
	files := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) != 0 {
			files = append(files, filepath.ToSlash(string(field)))
		}
	}
	sort.Strings(files)
	return files, nil
}

// listTreeFiles resolves the files covered by a directory or glob entry. Inside a git
// work tree that is the git-tracked set. A workspace that is not inside a git work tree —
// an operational container root whose children are the actual repositories — falls back
// to a bounded filesystem walk so review binding and the in_review transition keep
// working there; every other git failure still fails closed with the original git error.
func listTreeFiles(workspace, pattern string) ([]string, error) {
	tracked, err := listTrackedFiles(workspace, pattern)
	if err == nil {
		return tracked, nil
	}
	workTree, probeErr := insideGitWorkTree(workspace)
	if probeErr != nil {
		return nil, err
	}
	if workTree {
		return nil, err
	}
	return listWalkedTreeFiles(workspace, pattern)
}

// insideGitWorkTree reports whether dir sits inside a git work tree. A definitive "no"
// from git is a normal answer; only an unavailable git binary or a timed-out probe
// returns an error, so callers keep failing closed on those instead of degrading.
func insideGitWorkTree(dir string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), trackedFilesTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	var stdout bytes.Buffer
	command.Stdout = &stdout
	err := command.Run()
	if err == nil {
		return strings.TrimSpace(stdout.String()) == "true", nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, err
}

// maxWalkedTreeFiles bounds the non-git fingerprint walk so dependency caches or build
// output under a container root cannot make review binding unbounded. Exceeding it fails
// closed with a diagnosable message instead of silently binding a partial tree.
const maxWalkedTreeFiles = 8192

// listWalkedTreeFiles digests a directory or glob entry by walking the filesystem when
// there is no git index to read. It keeps the guarantees the git-backed listing provides
// where it can: sorted unique workspace-relative paths, regular files only (symlinks are
// never followed), repository metadata excluded so routine commits are not content drift,
// and a hard file cap. Without an index it cannot tell tracked from untracked content, so
// untracked files under the entry are covered too — a stricter binding, never a looser one.
func listWalkedTreeFiles(workspace, pattern string) ([]string, error) {
	root := filepath.Join(workspace, filepath.FromSlash(walkLiteralPrefix(pattern)))
	info, statErr := os.Lstat(root)
	if statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return nil, nil // a pattern that matches nothing digests as the empty set
		}
		return nil, statErr
	}
	if !info.IsDir() {
		return nil, nil // only a literal entry can land here, and files have no children
	}
	var matcher *regexp.Regexp
	if isGlobPattern(pattern) {
		compiled, err := walkedTreePattern(pattern)
		if err != nil {
			return nil, err
		}
		matcher = compiled
	}
	var files []string
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if strings.EqualFold(entry.Name(), ".git") {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}
		clean, err := canonicalLeasePath(filepath.ToSlash(relative))
		if err != nil {
			return fmt.Errorf("fingerprint %s: %w", pattern, err)
		}
		if matcher != nil {
			if !matcher.MatchString(clean) {
				return nil
			}
		} else if clean != pattern && !strings.HasPrefix(clean, pattern+"/") {
			return nil
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !entryInfo.Mode().IsRegular() {
			return nil
		}
		if len(files) >= maxWalkedTreeFiles {
			return fmt.Errorf("directory fingerprint %s covers more than %d files in a workspace that is not a git work tree; narrow allowed_files or initialize git", pattern, maxWalkedTreeFiles)
		}
		files = append(files, clean)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(files)
	return files, nil
}

// walkLiteralPrefix returns the leading path components of pattern that contain no glob
// metacharacters: the directory the non-git walk can start from. Matching files are always
// anchored under that prefix, so starting the walk there never misses a match.
func walkLiteralPrefix(pattern string) string {
	components := strings.Split(pattern, "/")
	for i, component := range components {
		if isGlobPattern(component) {
			return strings.Join(components[:i], "/")
		}
	}
	return pattern
}

// walkedTreePattern compiles a glob entry into an anchored expression that mirrors git
// pathspec coverage: "*" and "**" run across separators, "?" matches any single character
// (separators included), and a leading "**/" also matches at the root. Glob patterns are
// never extended to the children of a directory they match — git does not do that either
// (only literal directory entries carry the prefix rule, handled by the caller).
func walkedTreePattern(pattern string) (*regexp.Regexp, error) {
	runes := []rune(pattern)
	var expression strings.Builder
	expression.WriteString(`(?s)^`)
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '*':
			if i+1 < len(runes) && runes[i+1] == '*' {
				i++
				if i+1 < len(runes) && runes[i+1] == '/' {
					i++
					expression.WriteString(`(?:.*/)?`)
					continue
				}
				expression.WriteString(`.*`)
				continue
			}
			expression.WriteString(`.*`)
		case '?':
			expression.WriteString(`.`)
		default:
			expression.WriteString(regexp.QuoteMeta(string(runes[i])))
		}
	}
	expression.WriteString(`$`)
	return regexp.Compile(expression.String())
}

// currentReviewBinding returns the canonical JSON binding of a task's definition and
// allowed-file fingerprints, or "" when the task does not exist.
func currentReviewBinding(ctx context.Context, conn *sql.Conn, id string) (string, error) {
	var workspace, board, title, objective, acceptance, verification, filesJSON string
	err := conn.QueryRowContext(ctx, `SELECT i.workspace,i.board_id,i.title,d.objective,d.acceptance_criteria,d.verification,d.allowed_files_json FROM work_items i JOIN work_definitions d ON d.item_id=i.id WHERE i.id=?`, id).Scan(&workspace, &board, &title, &objective, &acceptance, &verification, &filesJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var files []string
	if err = json.Unmarshal([]byte(filesJSON), &files); err != nil {
		return "", err
	}
	if len(files) > 128 {
		return "", errors.New("review requires a bounded file scope")
	}
	canonical, err := CanonicalWorkspace(workspace)
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	entries := make([]WorkFileDigest, 0, len(files))
	for _, file := range files {
		digest, e := fingerprintFile(canonical, file)
		if e != nil {
			return "", fmt.Errorf("fingerprint %s: %w", file, e)
		}
		entries = append(entries, WorkFileDigest{Path: file, Digest: digest})
	}
	dependencies, err := workDependencyIDs(ctx, conn, id)
	if err != nil {
		return "", err
	}
	sort.Strings(dependencies)
	definitionDigest := hashJSON([]any{id, board, canonical, title, objective, acceptance, verification, files, dependencies, ""})
	data, err := json.Marshal(ReviewBinding{FingerprintVersion: 1, DefinitionSHA256: definitionDigest, ChangeSHA256: hashJSON(entries)})
	return string(data), err
}

// sameReviewBinding compares stored and current bindings by their fingerprint fields so
// a binding persisted by an earlier build still matches an identical task fingerprint.
func sameReviewBinding(approved, current string) bool {
	if approved == current {
		return true
	}
	if approved == "" || current == "" {
		return false
	}
	var a, c ReviewBinding
	if json.Unmarshal([]byte(approved), &a) != nil || json.Unmarshal([]byte(current), &c) != nil {
		return false
	}
	return a == c
}

// RefreshWorkReview explicitly reconciles an already-approved task after later
// changes. It grants no write authority and preserves every historical approval.
func (s *Store) RefreshWorkReview(ctx context.Context, id string, expectedRevision int64) (WorkItem, error) {
	if expectedRevision <= 0 {
		return WorkItem{}, errors.New("review refresh requires an explicit positive revision")
	}
	err := s.immediate(ctx, func(conn *sql.Conn) error {
		var status, board string
		var revision int64
		if err := conn.QueryRowContext(ctx, `SELECT status,revision,board_id FROM work_items WHERE id=?`, id).Scan(&status, &revision, &board); err != nil {
			return err
		}
		if status != string(WorkDone) || revision != expectedRevision {
			return fmt.Errorf("%w: review refresh requires done at the observed revision", ErrWorkConflict)
		}
		var active int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_items WHERE board_id=? AND status IN('in_progress','in_review')`, board).Scan(&active); err != nil {
			return err
		}
		if active != 0 {
			return errors.New("finish active board writes and reviews before refreshing review")
		}
		var owner string
		var attempt int64
		if err := conn.QueryRowContext(ctx, `SELECT implementation_owner,attempt FROM work_approvals WHERE item_id=? AND verdict='PASS' ORDER BY id DESC LIMIT 1`, id).Scan(&owner, &attempt); err != nil {
			return err
		}
		binding, err := currentReviewBinding(ctx, conn, id)
		if err != nil {
			return err
		}
		reviewID, err := newID()
		if err != nil {
			return err
		}
		now := s.timestamp()
		if _, err = conn.ExecContext(ctx, `INSERT INTO work_reviews(item_id,review_id,attempt,implementation_owner,review_revision,created_at,binding_json) VALUES(?,?,?,?,?,?,?)`, id, reviewID, attempt, owner, revision+1, now, binding); err != nil {
			return err
		}
		if _, err = conn.ExecContext(ctx, `UPDATE work_items SET status='in_review',revision=revision+1,updated_at=? WHERE id=? AND revision=?`, now, id, revision); err != nil {
			return err
		}
		return s.addWorkEvent(ctx, conn, id, "review_refreshed", string(WorkDone), string(WorkInReview), "fresh independent review required")
	})
	if err != nil {
		return WorkItem{}, err
	}
	return s.GetWork(ctx, id)
}

// ComputeWorkFingerprint calculates the current authoritative fingerprints for a task's
// definition and allowed files in the workspace, and compares them against any prior PASS approval.
func (s *Store) ComputeWorkFingerprint(ctx context.Context, id string) (WorkFingerprint, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return WorkFingerprint{}, errors.New("task id is required")
	}
	var fp WorkFingerprint
	fp.TaskID = id

	var workspace, board, title, objective, acceptance, verification, filesJSON string
	err := s.db.QueryRowContext(ctx, `SELECT i.workspace,i.board_id,i.title,d.objective,d.acceptance_criteria,d.verification,d.allowed_files_json FROM work_items i JOIN work_definitions d ON d.item_id=i.id WHERE i.id=?`, id).Scan(&workspace, &board, &title, &objective, &acceptance, &verification, &filesJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkFingerprint{}, ErrWorkNotFound
	}
	if err != nil {
		return WorkFingerprint{}, err
	}
	workspace, err = CanonicalWorkspace(workspace)
	if err != nil {
		return WorkFingerprint{}, err
	}
	fp.Workspace = workspace
	fp.BoardID = board

	var files []string
	if err := json.Unmarshal([]byte(filesJSON), &files); err != nil {
		return WorkFingerprint{}, fmt.Errorf("decode allowed files: %w", err)
	}
	sort.Strings(files)
	entries := make([]WorkFileDigest, 0, len(files))
	for _, file := range files {
		clean, err := canonicalLeasePath(file)
		if err != nil {
			return WorkFingerprint{}, err
		}
		digest, err := fingerprintFile(workspace, clean)
		if err != nil {
			return WorkFingerprint{}, fmt.Errorf("fingerprint %s: %w", file, err)
		}
		entries = append(entries, WorkFileDigest{Path: file, Digest: digest})
	}
	fp.Files = entries
	fp.ChangeSHA256 = hashJSON(entries)

	var dependencies []string
	deps, err := s.db.QueryContext(ctx, `SELECT depends_on FROM work_dependencies WHERE item_id=? ORDER BY depends_on`, id)
	if err == nil {
		for deps.Next() {
			var dep string
			if err := deps.Scan(&dep); err == nil {
				dependencies = append(dependencies, dep)
			}
		}
		_ = deps.Close()
	}
	sort.Strings(dependencies)

	fp.DefinitionSHA256 = hashJSON([]any{id, board, workspace, title, objective, acceptance, verification, files, dependencies, ""})

	var approvedBindingJSON string
	err = s.db.QueryRowContext(ctx, `SELECT binding_json FROM work_approvals WHERE item_id=? AND verdict='PASS' ORDER BY id DESC LIMIT 1`, id).Scan(&approvedBindingJSON)
	if err == nil && approvedBindingJSON != "" {
		var approved ReviewBinding
		if err := json.Unmarshal([]byte(approvedBindingJSON), &approved); err == nil {
			fp.ApprovedBinding = &approved
			matches := (approved.ChangeSHA256 == fp.ChangeSHA256 && approved.DefinitionSHA256 == fp.DefinitionSHA256)
			fp.MatchesApproved = &matches
		}
	}
	return fp, nil
}
