package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
)

// TargetKind describes the only target classes a journal is allowed to own.
type TargetKind uint8

const (
	TargetFile TargetKind = iota + 1
	TargetDirectory
)

// Presence deliberately distinguishes missing paths from zero-byte files.
type Presence string

const (
	PresenceAbsent      Presence = "absent"
	PresenceRegularFile Presence = "regular-file"
	PresenceDirectory   Presence = "directory"
)

// JournalState is persisted so a failed restore remains safely retryable.
type JournalState string

const (
	JournalPrepared  JournalState = "prepared"
	JournalApplying  JournalState = "applying"
	JournalCommitted JournalState = "committed"
	JournalFailed    JournalState = "failed"
	JournalRestoring JournalState = "restoring"
	JournalRestored  JournalState = "restored"
)

var ErrJournalConflict = errors.New("install journal restore conflict")

type journalCheckpointFile interface {
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

var (
	openJournalCheckpoint = func(path string, flag int, perm fs.FileMode) (journalCheckpointFile, error) {
		file, err := os.OpenFile(path, flag, perm)
		return file, err
	}
	removeJournalCheckpoint = os.Remove
	renameJournalCheckpoint = os.Rename
)

// ManagedTarget must be declared before the journal can capture a preimage.
// Path is relative to the trusted journal root.
type ManagedTarget struct {
	Path  string
	Kind  TargetKind
	Owner string
}

// PathPreimage is enough to reproduce a target exactly without treating an
// empty file as an absent target.
type PathPreimage struct {
	Path         string      `json:"path"`
	Presence     Presence    `json:"presence"`
	Mode         fs.FileMode `json:"mode"`
	Size         int64       `json:"size"`
	SHA256       string      `json:"sha256"`
	SnapshotPath string      `json:"snapshot_path,omitempty"`
}

// MutationOutcome is the post-write compare-and-swap evidence. It is recorded
// only after a writer's atomic mutation has reached a terminal state.
type MutationOutcome struct {
	Path        string      `json:"path"`
	Presence    Presence    `json:"presence"`
	Mode        fs.FileMode `json:"mode"`
	Size        int64       `json:"size"`
	SHA256      string      `json:"sha256"`
	CreatedDirs []string    `json:"created_dirs,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// InstallJournal is the outer receipt for install writers. Workflow receipts
// remain typed child receipts and are attached rather than translated.
type InstallJournal struct {
	SchemaVersion    string               `json:"schema_version"`
	ID               string               `json:"id"`
	State            JournalState         `json:"state"`
	Entries          []PathPreimage       `json:"entries"`
	Targets          []ManagedTarget      `json:"targets"`
	Outcomes         []MutationOutcome    `json:"outcomes"`
	WorkflowReceipts []sddinstall.Receipt `json:"workflow_receipts,omitempty"`
	CreatedDirs      []string             `json:"created_dirs,omitempty"`
	PrimaryError     string               `json:"primary_error,omitempty"`
	TargetRoot       string               `json:"target_root"`
	CheckpointPath   string               `json:"-"`
}

// BeginInstallJournal captures and durably checkpoints every declared target
// before any managed post-backup write may begin.
func BeginInstallJournal(targetRoot, checkpointRoot string, targets []ManagedTarget) (*InstallJournal, error) {
	root, err := filepath.Abs(targetRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve journal target root: %w", err)
	}
	if len(targets) == 0 {
		return nil, errors.New("install journal requires declared targets")
	}
	if checkpointRoot == "" {
		return nil, errors.New("install journal checkpoint root is required")
	}
	checkpointRoot, err = filepath.Abs(checkpointRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve journal checkpoint root: %w", err)
	}
	if err := os.MkdirAll(checkpointRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create journal checkpoint root: %w", err)
	}
	id := "journal-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	dir := filepath.Join(checkpointRoot, id)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create journal checkpoint directory: %w", err)
	}
	journal := &InstallJournal{SchemaVersion: "1.0.0", ID: id, State: JournalPrepared, TargetRoot: root, CheckpointPath: filepath.Join(dir, "journal.json")}
	seen := make(map[string]TargetKind, len(targets))
	for _, target := range targets {
		path, err := journalPath(root, target.Path)
		if err != nil {
			return nil, err
		}
		if target.Kind != TargetFile && target.Kind != TargetDirectory {
			return nil, fmt.Errorf("journal target %q has unknown kind", target.Path)
		}
		if old, duplicate := seen[target.Path]; duplicate {
			if old != target.Kind {
				return nil, fmt.Errorf("journal target %q is declared with incompatible kinds", target.Path)
			}
			return nil, fmt.Errorf("journal target %q is declared more than once", target.Path)
		}
		seen[target.Path] = target.Kind
		journal.Targets = append(journal.Targets, target)
		entry, err := capturePreimage(path, target.Path, target.Kind, filepath.Join(dir, "snapshots"))
		if err != nil {
			return nil, err
		}
		journal.Entries = append(journal.Entries, entry)
	}
	sort.Slice(journal.Entries, func(i, k int) bool { return journal.Entries[i].Path < journal.Entries[k].Path })
	if err := journal.checkpoint(); err != nil {
		return nil, err
	}
	return journal, nil
}

// AttachWorkflowReceipt composes a child receipt without altering its typed
// recovery contract.
func (j *InstallJournal) AttachWorkflowReceipt(receipt sddinstall.Receipt) error {
	if j == nil {
		return errors.New("nil install journal")
	}
	j.WorkflowReceipts = append(j.WorkflowReceipts, receipt)
	return j.checkpoint()
}

// Record checkpoints one completed writer outcome. The outcome is first
// validated against the declared target set so undeclared writes fail closed.
func (j *InstallJournal) Record(outcome MutationOutcome) error {
	if j == nil {
		return errors.New("nil install journal")
	}
	if _, err := journalPath(j.TargetRoot, outcome.Path); err != nil {
		return err
	}
	if !j.declared(outcome.Path) {
		return fmt.Errorf("journal outcome %q was not declared", outcome.Path)
	}
	if kind, ok := j.targetKind(outcome.Path); ok && ((kind == TargetFile && outcome.Presence == PresenceDirectory) || (kind == TargetDirectory && outcome.Presence == PresenceRegularFile)) {
		return fmt.Errorf("journal outcome %q has incompatible target type", outcome.Path)
	}
	if err := validateImage(outcome.Path, outcome.Presence, outcome.Mode, outcome.Size, outcome.SHA256); err != nil {
		return err
	}
	actualPath, err := journalPath(j.TargetRoot, outcome.Path)
	if err != nil {
		return err
	}
	actual, err := inspectPath(actualPath, outcome.Path)
	if err != nil {
		return err
	}
	if !sameImage(actual, outcome) {
		return fmt.Errorf("journal outcome %q does not match the current postimage", outcome.Path)
	}
	for i := range outcome.CreatedDirs {
		if _, err := journalPath(j.TargetRoot, outcome.CreatedDirs[i]); err != nil {
			return fmt.Errorf("journal created directory: %w", err)
		}
	}
	for i := range j.Outcomes {
		if j.Outcomes[i].Path == outcome.Path {
			j.Outcomes[i] = outcome
			j.State = JournalApplying
			return j.checkpoint()
		}
	}
	j.Outcomes = append(j.Outcomes, outcome)
	j.CreatedDirs = append(j.CreatedDirs, outcome.CreatedDirs...)
	j.CreatedDirs = uniqueSorted(j.CreatedDirs)
	j.State = JournalApplying
	return j.checkpoint()
}

func (j *InstallJournal) Commit() error {
	if j == nil {
		return errors.New("nil install journal")
	}
	j.State = JournalCommitted
	return j.checkpoint()
}

// RestoreAndVerify first confirms all outcomes still own their recorded
// postimage. Thus one conflict prevents every inverse mutation in this set.
func (j *InstallJournal) RestoreAndVerify() error {
	if j == nil {
		return errors.New("nil install journal")
	}
	if err := j.preflightInverse(); err != nil {
		j.State = JournalFailed
		j.PrimaryError = err.Error()
		_ = j.checkpoint()
		return err
	}
	j.State = JournalRestoring
	if err := j.checkpoint(); err != nil {
		return err
	}
	for i := len(j.Entries) - 1; i >= 0; i-- {
		if err := j.restore(j.Entries[i]); err != nil {
			j.State = JournalFailed
			j.PrimaryError = err.Error()
			_ = j.checkpoint()
			return err
		}
	}
	for i := len(j.CreatedDirs) - 1; i >= 0; i-- {
		path, err := journalPath(j.TargetRoot, j.CreatedDirs[i])
		if err != nil {
			return err
		}
		if children, readErr := os.ReadDir(path); readErr == nil && len(children) != 0 {
			continue // a user sibling makes the transaction-created directory shared.
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			j.State, j.PrimaryError = JournalFailed, fmt.Sprintf("remove created directory %q: %v", j.CreatedDirs[i], err)
			_ = j.checkpoint()
			return errors.New(j.PrimaryError)
		}
	}
	for _, entry := range j.Entries {
		if err := j.verify(entry); err != nil {
			j.State, j.PrimaryError = JournalFailed, err.Error()
			_ = j.checkpoint()
			return err
		}
	}
	j.State, j.PrimaryError = JournalRestored, ""
	return j.checkpoint()
}

func (j *InstallJournal) preflightInverse() error {
	for _, entry := range j.Entries {
		outcome, recorded := j.outcome(entry.Path)
		if !recorded {
			continue // already restored or never written
		}
		path, err := journalPath(j.TargetRoot, entry.Path)
		if err != nil {
			return err
		}
		current, err := inspectPath(path, entry.Path)
		if err != nil {
			return err
		}
		if sameImage(current, MutationOutcome{Path: entry.Path, Presence: entry.Presence, Mode: entry.Mode, Size: entry.Size, SHA256: entry.SHA256}) {
			continue // a previous restore completed this target; retry is a no-op.
		}
		if !sameImage(current, outcome) {
			return fmt.Errorf("%w: target %q no longer matches journaled postimage", ErrJournalConflict, entry.Path)
		}
	}
	return nil
}

func (j *InstallJournal) restore(entry PathPreimage) error {
	path, err := journalPath(j.TargetRoot, entry.Path)
	if err != nil {
		return err
	}
	switch entry.Presence {
	case PresenceAbsent:
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove initially absent target %q: %w", entry.Path, err)
		}
	case PresenceDirectory:
		if err := os.MkdirAll(path, entry.Mode.Perm()); err != nil {
			return fmt.Errorf("restore directory %q: %w", entry.Path, err)
		}
		if err := os.Chmod(path, entry.Mode.Perm()); err != nil {
			return fmt.Errorf("restore directory mode %q: %w", entry.Path, err)
		}
	case PresenceRegularFile:
		content, err := os.ReadFile(entry.SnapshotPath)
		if err != nil {
			return fmt.Errorf("read journal snapshot for %q: %w", entry.Path, err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("restore parent for %q: %w", entry.Path, err)
		}
		temporary := path + ".cortex-ia-restore.tmp"
		if err := os.WriteFile(temporary, content, entry.Mode.Perm()); err != nil {
			return fmt.Errorf("write restore temporary %q: %w", entry.Path, err)
		}
		if err := os.Chmod(temporary, entry.Mode.Perm()); err != nil {
			_ = os.Remove(temporary)
			return fmt.Errorf("set restore mode %q: %w", entry.Path, err)
		}
		if err := os.Rename(temporary, path); err != nil {
			_ = os.Remove(temporary)
			return fmt.Errorf("replace restored target %q: %w", entry.Path, err)
		}
	default:
		return fmt.Errorf("restore target %q has unknown presence", entry.Path)
	}
	return nil
}

func (j *InstallJournal) verify(entry PathPreimage) error {
	path, err := journalPath(j.TargetRoot, entry.Path)
	if err != nil {
		return err
	}
	actual, err := inspectPath(path, entry.Path)
	if err != nil {
		return err
	}
	if !sameImage(actual, MutationOutcome{Path: entry.Path, Presence: entry.Presence, Mode: entry.Mode, Size: entry.Size, SHA256: entry.SHA256}) {
		return fmt.Errorf("journal restore verification failed for %q", entry.Path)
	}
	return nil
}

func (j *InstallJournal) declared(path string) bool {
	for _, entry := range j.Entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}

func (j *InstallJournal) targetKind(path string) (TargetKind, bool) {
	for _, target := range j.Targets {
		if target.Path == path {
			return target.Kind, true
		}
	}
	return 0, false
}

// LoadInstallJournal opens retained recovery evidence for a safe retry. The
// checkpoint location is supplied by the caller; it is never derived from an
// untrusted journal ID.
func LoadInstallJournal(checkpointPath string) (*InstallJournal, error) {
	encoded, err := os.ReadFile(checkpointPath)
	if err != nil {
		return nil, fmt.Errorf("read install journal checkpoint: %w", err)
	}
	var journal InstallJournal
	if err := json.Unmarshal(encoded, &journal); err != nil {
		return nil, fmt.Errorf("decode install journal checkpoint: %w", err)
	}
	if journal.SchemaVersion != "1.0.0" || journal.ID == "" || journal.TargetRoot == "" {
		return nil, errors.New("invalid install journal checkpoint")
	}
	root, err := filepath.Abs(journal.TargetRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve journal target root: %w", err)
	}
	journal.TargetRoot, journal.CheckpointPath = root, checkpointPath
	for _, entry := range journal.Entries {
		if _, err := journalPath(root, entry.Path); err != nil {
			return nil, err
		}
	}
	return &journal, nil
}

func (j *InstallJournal) outcome(path string) (MutationOutcome, bool) {
	for _, outcome := range j.Outcomes {
		if outcome.Path == path {
			return outcome, true
		}
	}
	return MutationOutcome{}, false
}

func (j *InstallJournal) checkpoint() error {
	if j.CheckpointPath == "" {
		return errors.New("install journal has no checkpoint path")
	}
	encoded, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal install journal: %w", err)
	}
	encoded = append(encoded, '\n')
	temporary := j.CheckpointPath + ".tmp"
	f, err := openJournalCheckpoint(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open journal checkpoint: %w", err)
	}
	_, checkpointErr := f.Write(encoded)
	if checkpointErr == nil {
		checkpointErr = f.Sync()
	}
	if closeErr := f.Close(); checkpointErr == nil {
		checkpointErr = closeErr
	}
	if checkpointErr != nil {
		_ = removeJournalCheckpoint(temporary)
		return fmt.Errorf("write journal checkpoint: %w", checkpointErr)
	}
	if err := renameJournalCheckpoint(temporary, j.CheckpointPath); err != nil {
		_ = removeJournalCheckpoint(temporary)
		return fmt.Errorf("commit journal checkpoint: %w", err)
	}
	return nil
}

func capturePreimage(fullPath, relative string, kind TargetKind, snapshots string) (PathPreimage, error) {
	image, err := inspectPath(fullPath, relative)
	if err != nil {
		return PathPreimage{}, err
	}
	if kind == TargetFile && image.Presence == PresenceDirectory || kind == TargetDirectory && image.Presence == PresenceRegularFile {
		return PathPreimage{}, fmt.Errorf("journal target %q has incompatible existing type", relative)
	}
	entry := PathPreimage{Path: relative, Presence: image.Presence, Mode: image.Mode, Size: image.Size, SHA256: image.SHA256}
	if image.Presence != PresenceRegularFile {
		return entry, nil
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return PathPreimage{}, fmt.Errorf("read journal target %q: %w", relative, err)
	}
	if err := os.MkdirAll(snapshots, 0o700); err != nil {
		return PathPreimage{}, fmt.Errorf("create journal snapshots: %w", err)
	}
	// Path fingerprints are deterministic but content writes remain separate so
	// every zero-byte file has a durable, inspectable snapshot.
	pathHash := sha256.Sum256([]byte(relative))
	entry.SnapshotPath = filepath.Join(snapshots, hex.EncodeToString(pathHash[:])+".snapshot")
	if err := os.WriteFile(entry.SnapshotPath, content, 0o600); err != nil {
		return PathPreimage{}, fmt.Errorf("write journal snapshot for %q: %w", relative, err)
	}
	return entry, nil
}

func inspectPath(path, display string) (MutationOutcome, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return MutationOutcome{Path: display, Presence: PresenceAbsent}, nil
	}
	if err != nil {
		return MutationOutcome{}, fmt.Errorf("inspect journal target %q: %w", display, err)
	}
	if info.IsDir() {
		return MutationOutcome{Path: display, Presence: PresenceDirectory, Mode: info.Mode().Perm()}, nil
	}
	if !info.Mode().IsRegular() {
		return MutationOutcome{}, fmt.Errorf("journal target %q is not a regular file or directory", display)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return MutationOutcome{}, fmt.Errorf("read journal target %q: %w", display, err)
	}
	return MutationOutcome{Path: display, Presence: PresenceRegularFile, Mode: info.Mode().Perm(), Size: int64(len(content)), SHA256: journalSHA256(content)}, nil
}

func validateImage(path string, presence Presence, mode fs.FileMode, size int64, hash string) error {
	switch presence {
	case PresenceAbsent:
		if size != 0 || hash != "" {
			return fmt.Errorf("journal absent outcome %q carries file metadata", path)
		}
	case PresenceDirectory:
		if size != 0 || hash != "" {
			return fmt.Errorf("journal directory outcome %q carries file metadata", path)
		}
	case PresenceRegularFile:
		if size < 0 || len(hash) != sha256.Size*2 {
			return fmt.Errorf("journal file outcome %q lacks size or SHA-256", path)
		}
	default:
		return fmt.Errorf("journal target %q has unknown presence", path)
	}
	_ = mode
	return nil
}

func sameImage(actual MutationOutcome, expected MutationOutcome) bool {
	return actual.Presence == expected.Presence && actual.Mode.Perm() == expected.Mode.Perm() && actual.Size == expected.Size && actual.SHA256 == expected.SHA256
}

func journalPath(root, path string) (string, error) {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", fmt.Errorf("journal target %q must be a relative path", path)
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("journal target %q escapes its root", path)
	}
	full := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("journal target %q escapes its root", path)
	}
	if err := rejectJournalSymlinkComponents(root, full); err != nil {
		return "", err
	}
	return full, nil
}

func rejectJournalSymlinkComponents(root, target string) error {
	root = filepath.Clean(root)
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect journal root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("journal target root is a symlink/reparse point")
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			break
		}
		if err != nil {
			return fmt.Errorf("inspect journal target %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("journal target %q traverses a symlink/reparse point", target)
		}
	}
	return nil
}

func journalSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func uniqueSorted(values []string) []string {
	sort.Strings(values)
	return compactStrings(values)
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	n := 1
	for i := 1; i < len(values); i++ {
		if values[i] != values[n-1] {
			values[n] = values[i]
			n++
		}
	}
	return values[:n]
}
