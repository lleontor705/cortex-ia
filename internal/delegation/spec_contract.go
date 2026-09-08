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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type ContractPin struct {
	Transport string `json:"transport"`
	Project   string `json:"project"`
	Locator   string `json:"locator"`
	SHA256    string `json:"sha256"`
}

type SDDContract struct {
	Version        int           `json:"version"`
	Workflow       string        `json:"workflow"`
	ChangeID       string        `json:"change_id"`
	SpecPlane      string        `json:"spec_plane"`
	Pins           []ContractPin `json:"pins"`
	RequirementIDs []string      `json:"requirement_ids"`
}

type ReviewBinding struct {
	FingerprintVersion int          `json:"fingerprint_version"`
	Contract           *SDDContract `json:"contract"`
	DefinitionSHA256   string       `json:"definition_sha256"`
	ChangeSHA256       string       `json:"change_sha256"`
}

var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var requirementPattern = regexp.MustCompile(`^REQ[-A-Za-z0-9_]{1,95}$`)

func encodeContract(contract *SDDContract) (string, error) {
	if contract == nil {
		return "", nil
	}
	if contract.Version != 1 || (contract.Workflow != "sdd-lite" && contract.Workflow != "sdd-full") ||
		(contract.SpecPlane != "cortex" && contract.SpecPlane != "openspec" && contract.SpecPlane != "hybrid") ||
		!regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`).MatchString(contract.ChangeID) ||
		len(contract.Pins) == 0 || len(contract.Pins) > 32 || len(contract.RequirementIDs) == 0 || len(contract.RequirementIDs) > 128 {
		return "", errors.New("invalid bounded SDD contract identity, pins, or requirement IDs")
	}
	normalized := *contract
	normalized.Pins = append([]ContractPin(nil), contract.Pins...)
	normalized.RequirementIDs = append([]string(nil), contract.RequirementIDs...)
	seen := map[string]bool{}
	for _, id := range normalized.RequirementIDs {
		if !requirementPattern.MatchString(id) || seen[id] {
			return "", errors.New("invalid or duplicate SDD requirement ID")
		}
		seen[id] = true
	}
	seen = map[string]bool{}
	for _, pin := range normalized.Pins {
		if !digestPattern.MatchString(pin.SHA256) || strings.TrimSpace(pin.Project) == "" || len(pin.Project) > 512 || strings.TrimSpace(pin.Locator) == "" || len(pin.Locator) > 1024 || strings.ContainsRune(pin.Locator, 0) {
			return "", errors.New("invalid SDD contract pin")
		}
		switch pin.Transport {
		case "workspace_file", "local_cortex_cli", "cortex_mcp":
		default:
			return "", errors.New("unsupported contract pin transport")
		}
		if contract.SpecPlane == "openspec" && pin.Transport != "workspace_file" || contract.SpecPlane == "cortex" && pin.Transport == "workspace_file" {
			return "", errors.New("pin transport does not match specification plane")
		}
		key := pin.Transport + "\x00" + pin.Project + "\x00" + pin.Locator
		if seen[key] {
			return "", errors.New("duplicate SDD contract pin")
		}
		seen[key] = true
	}
	sort.Strings(normalized.RequirementIDs)
	sort.Slice(normalized.Pins, func(i, j int) bool {
		a, _ := json.Marshal(normalized.Pins[i])
		b, _ := json.Marshal(normalized.Pins[j])
		return string(a) < string(b)
	})
	data, err := json.Marshal(normalized)
	if len(data) > 32768 {
		return "", errors.New("SDD contract exceeds 32 KiB")
	}
	return string(data), err
}

func decodeContract(raw string) (*SDDContract, error) {
	if raw == "" {
		return nil, nil
	}
	var contract SDDContract
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("trailing contract JSON")
	}
	if _, err := encodeContract(&contract); err != nil {
		return nil, err
	}
	return &contract, nil
}

// ReadSDDContract reads only an explicitly supplied bounded transfer file.
func ReadSDDContract(path string) (*SDDContract, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, 32769))
	if err != nil {
		return nil, err
	}
	if len(data) > 32768 {
		return nil, errors.New("SDD contract exceeds 32 KiB")
	}
	return decodeContract(string(data))
}

func hashJSON(value any) string {
	data, _ := json.Marshal(value)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// fingerprintFile rejects symlinks, including parent components, and bounds reads.
// It does not sandbox other processes or close the check-to-write race.
func fingerprintFile(workspace, relative string) (string, error) {
	clean, err := canonicalLeasePath(relative)
	if err != nil {
		return "", err
	}
	current := workspace
	for _, component := range strings.Split(filepath.ToSlash(clean), "/") {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			return "absent", nil
		}
		if statErr != nil {
			return "", statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("fingerprint paths must not contain symlinks")
		}
	}
	f, err := os.Open(current)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("fingerprint requires explicit regular files")
	}
	if info.Size() > 16*1024*1024 {
		return "", errors.New("fingerprint file exceeds 16 MiB")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, 16*1024*1024+1))
	if err != nil {
		return "", err
	}
	if n > 16*1024*1024 {
		return "", errors.New("fingerprint file exceeds 16 MiB")
	}
	return "file:" + hex.EncodeToString(h.Sum(nil)), nil
}

func currentReviewBinding(ctx context.Context, conn *sql.Conn, id string) (string, error) {
	return currentReviewBindingAt(ctx, conn, id, "", "")
}
func currentReviewBindingAt(ctx context.Context, conn *sql.Conn, id, sourcePath, destinationPath string) (string, error) {
	var workspace, board, title, objective, acceptance, verification, filesJSON, contractJSON string
	err := conn.QueryRowContext(ctx, `SELECT i.workspace,i.board_id,i.title,d.objective,d.acceptance_criteria,d.verification,d.allowed_files_json,d.contract_json FROM work_items i JOIN work_definitions d ON d.item_id=i.id WHERE i.id=?`, id).Scan(&workspace, &board, &title, &objective, &acceptance, &verification, &filesJSON, &contractJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	contract, err := decodeContract(contractJSON)
	if err != nil {
		return "", err
	}
	if contract == nil {
		return "", nil
	}
	contractJSON, err = encodeContract(contract)
	if err != nil {
		return "", err
	}
	var files []string
	if err = json.Unmarshal([]byte(filesJSON), &files); err != nil {
		return "", err
	}
	if len(files) == 0 || len(files) > 128 || objective == "" || acceptance == "" || verification == "" {
		return "", errors.New("SDD review requires complete task definition and writable file scope")
	}
	workspace, err = CanonicalWorkspace(workspace)
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	type fileDigest struct {
		Path   string `json:"path"`
		Digest string `json:"digest"`
	}
	entries := make([]fileDigest, 0, len(files))
	for _, file := range files {
		mapped, e := relocateContractPath(workspace, file, sourcePath, destinationPath)
		if e != nil {
			return "", e
		}
		digest, e := fingerprintFile(workspace, mapped)
		if e != nil {
			return "", fmt.Errorf("fingerprint %s: %w", file, e)
		}
		entries = append(entries, fileDigest{file, digest})
	}
	if err := verifyWorkspacePins(workspace, contract, sourcePath, destinationPath); err != nil {
		return "", err
	}
	if err := verifyLocalCortexPins(ctx, contract); err != nil {
		return "", err
	}
	dependencies, err := workDependencyIDs(ctx, conn, id)
	if err != nil {
		return "", err
	}
	sort.Strings(dependencies)
	definitionDigest := hashJSON([]any{id, board, workspace, title, objective, acceptance, verification, files, dependencies, contractJSON})
	data, err := json.Marshal(ReviewBinding{FingerprintVersion: 1, Contract: contract, DefinitionSHA256: definitionDigest, ChangeSHA256: hashJSON(entries)})
	return string(data), err
}

type ArchiveBinding struct {
	BoardID   string   `json:"board_id"`
	Workspace string   `json:"workspace"`
	ChangeID  string   `json:"change_id"`
	Workflow  string   `json:"workflow"`
	SpecPlane string   `json:"spec_plane"`
	TaskIDs   []string `json:"task_ids"`
	SHA256    string   `json:"sha256"`
}

func (s *Store) ValidateArchiveBoard(ctx context.Context, boardID, workspace, changeID, workflow, plane string) (ArchiveBinding, error) {
	var result ArchiveBinding
	err := s.immediate(ctx, func(conn *sql.Conn) error {
		var err error
		result, err = s.validateArchiveBoard(ctx, conn, boardID, workspace, changeID, workflow, plane)
		return err
	})
	return result, err
}

func (s *Store) validateArchiveBoard(ctx context.Context, conn *sql.Conn, boardID, workspace, changeID, workflow, plane string) (ArchiveBinding, error) {
	return s.validateArchiveBoardAt(ctx, conn, boardID, workspace, changeID, workflow, plane, "", "")
}
func (s *Store) validateArchiveBoardAt(ctx context.Context, conn *sql.Conn, boardID, workspace, changeID, workflow, plane, sourcePath, destinationPath string) (ArchiveBinding, error) {
	result := ArchiveBinding{BoardID: boardID, ChangeID: changeID, Workflow: workflow, SpecPlane: plane}
	canonical, err := CanonicalWorkspace(workspace)
	if err != nil {
		return result, err
	}
	result.Workspace = canonical
	rows, err := conn.QueryContext(ctx, `SELECT i.id,i.workspace,i.status,d.contract_json FROM work_items i LEFT JOIN work_definitions d ON d.item_id=i.id WHERE i.board_id=? ORDER BY i.id`, boardID)
	if err != nil {
		return result, err
	}
	type task struct{ id, workspace, status, contract string }
	var tasks []task
	for rows.Next() {
		var t task
		var raw sql.NullString
		if err = rows.Scan(&t.id, &t.workspace, &t.status, &raw); err != nil {
			_ = rows.Close()
			return result, err
		}
		t.contract = raw.String
		tasks = append(tasks, t)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return result, err
	}
	otherRows, err := conn.QueryContext(ctx, `SELECT i.workspace,d.contract_json FROM work_items i JOIN work_definitions d ON d.item_id=i.id WHERE i.board_id<>? AND d.contract_json<>''`, boardID)
	if err != nil {
		return result, err
	}
	for otherRows.Next() {
		var otherWorkspace, raw string
		if err = otherRows.Scan(&otherWorkspace, &raw); err != nil {
			_ = otherRows.Close()
			return result, err
		}
		other, e := decodeContract(raw)
		if e != nil {
			_ = otherRows.Close()
			return result, e
		}
		if other != nil && otherWorkspace == canonical && other.ChangeID == changeID {
			_ = otherRows.Close()
			return result, errors.New("change has tasks outside the archive board")
		}
	}
	err = otherRows.Err()
	_ = otherRows.Close()
	if err != nil {
		return result, err
	}
	var bindings []string
	for _, t := range tasks {
		contract, e := decodeContract(t.contract)
		if e != nil {
			return result, e
		}
		if contract == nil {
			return result, errors.New("archive board contains an unbound legacy/direct task")
		}
		if contract.ChangeID != changeID || contract.Workflow != workflow || contract.SpecPlane != plane || t.workspace != canonical {
			return result, errors.New("archive board contract or workspace mismatch")
		}
		var replacementCount int
		if e = conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_decomposition_steps WHERE parent_id=?`, t.id).Scan(&replacementCount); e != nil {
			return result, e
		}
		if replacementCount > 0 && t.status == string(WorkBlocked) {
			continue
		}
		if t.status != string(WorkDone) {
			return result, fmt.Errorf("archive task %s is not done", t.id)
		}
		current, e := currentReviewBindingAt(ctx, conn, t.id, sourcePath, destinationPath)
		if e != nil {
			return result, e
		}
		var approved string
		if e = conn.QueryRowContext(ctx, `SELECT binding_json FROM work_approvals WHERE item_id=? AND verdict='PASS' ORDER BY id DESC LIMIT 1`, t.id).Scan(&approved); e != nil {
			return result, errors.New("archive requires durable PASS binding")
		}
		if approved == "" || approved != current {
			return result, fmt.Errorf("archive task %s changed after approval; fresh review required", t.id)
		}
		result.TaskIDs = append(result.TaskIDs, t.id)
		bindings = append(bindings, current)
	}
	if len(result.TaskIDs) == 0 {
		return result, errors.New("archive requires at least one approved bound task")
	}
	result.SHA256 = hashJSON([]any{result.BoardID, result.Workspace, result.ChangeID, result.Workflow, result.SpecPlane, result.TaskIDs, bindings})
	return result, nil
}

func verifyWorkspacePins(workspace string, contract *SDDContract, sourcePath, destinationPath string) error {
	for _, pin := range contract.Pins {
		if pin.Transport != "workspace_file" {
			continue
		}
		pinProject, err := CanonicalWorkspace(pin.Project)
		if err != nil || pinProject != workspace {
			return errors.New("workspace pin project mismatch")
		}
		locator, err := relocateContractPath(workspace, pin.Locator, sourcePath, destinationPath)
		if err != nil {
			return err
		}
		digest, err := fingerprintFile(workspace, locator)
		if err != nil {
			return err
		}
		if digest != "file:"+pin.SHA256 {
			return errors.New("workspace specification pin changed; fresh contract review required")
		}
	}
	return nil
}

// RefreshWorkReview explicitly reconciles an already-approved SDD task after later
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
		var contractJSON string
		if err := conn.QueryRowContext(ctx, `SELECT contract_json FROM work_definitions WHERE item_id=?`, id).Scan(&contractJSON); err != nil {
			return err
		}
		contract, err := decodeContract(contractJSON)
		if err != nil {
			return err
		}
		if err := requireOpenSDDChange(ctx, conn, board, contract); err != nil {
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
		var owner, priorBinding string
		var attempt int64
		if err := conn.QueryRowContext(ctx, `SELECT implementation_owner,binding_json,attempt FROM work_approvals WHERE item_id=? AND verdict='PASS' ORDER BY id DESC LIMIT 1`, id).Scan(&owner, &priorBinding, &attempt); err != nil {
			return err
		}
		if owner == "" || priorBinding == "" {
			return errors.New("review refresh requires a prior typed SDD approval with implementation identity")
		}
		binding, err := currentReviewBinding(ctx, conn, id)
		if err != nil {
			return err
		}
		if binding == "" {
			return errors.New("review refresh requires an SDD contract")
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

func relocateContractPath(workspace, locator, sourcePath, destinationPath string) (string, error) {
	clean, err := canonicalLeasePath(locator)
	if err != nil {
		return "", err
	}
	if sourcePath == "" {
		return clean, nil
	}
	absolute := filepath.Join(workspace, filepath.FromSlash(clean))
	relative, err := filepath.Rel(sourcePath, absolute)
	if err != nil {
		return "", err
	}
	if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.Rel(workspace, filepath.Join(destinationPath, relative))
	}
	return clean, nil
}

type contractExportBuffer struct{ bytes.Buffer }

func (b *contractExportBuffer) Write(data []byte) (int, error) {
	if len(data) > 8*1024*1024-b.Len() {
		return 0, errors.New("contract export exceeds 8 MiB")
	}
	return b.Buffer.Write(data)
}

// Local store reads are bounded and explicit, not atomic with SQLite task state.
// Remote MCP pins require the controller's independent provider verification.
func verifyLocalCortexPins(ctx context.Context, contract *SDDContract) error {
	groups := map[string][]ContractPin{}
	for _, pin := range contract.Pins {
		if pin.Transport == "local_cortex_cli" {
			groups[pin.Project] = append(groups[pin.Project], pin)
		}
	}
	if len(groups) == 0 {
		return nil
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for project, pins := range groups {
		var output contractExportBuffer
		command := exec.CommandContext(bounded, "cortex", "export", "--project", project)
		command.Stdout = &output
		command.Stderr = io.Discard
		if err := command.Run(); err != nil {
			return errors.New("local Cortex pin export failed, timed out, or exceeded 8 MiB")
		}
		if !utf8.Valid(output.Bytes()) {
			return errors.New("local Cortex export is not valid UTF-8")
		}
		var observations []struct {
			ID      json.Number `json:"id"`
			Project string      `json:"project"`
			Content string      `json:"content"`
		}
		if err := json.Unmarshal(output.Bytes(), &observations); err != nil {
			return errors.New("invalid structured local Cortex export")
		}
		for _, pin := range pins {
			found := 0
			for _, observation := range observations {
				if observation.ID.String() != pin.Locator {
					continue
				}
				found++
				if observation.Project != project || len(observation.Content) > 1024*1024 {
					return errors.New("local Cortex pin project or content bound mismatch")
				}
				digest := sha256.Sum256([]byte(observation.Content))
				if hex.EncodeToString(digest[:]) != pin.SHA256 {
					return errors.New("local Cortex contract pin changed; fresh contract required")
				}
			}
			if found != 1 {
				return errors.New("local Cortex contract observation missing or ambiguous")
			}
		}
	}
	return nil
}

// Called inside the owning authority transaction; a completed change is immutable.
func requireOpenSDDChange(ctx context.Context, conn *sql.Conn, boardID string, contract *SDDContract) error {
	if contract == nil {
		return nil
	}
	var closed int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM change_archives WHERE board_id=? AND change_id=? AND status='complete'`, boardID, contract.ChangeID).Scan(&closed); err != nil {
		return err
	}
	if closed != 0 {
		return errors.New("SDD change is archived; use a new change and board instead of reopening it")
	}
	return nil
}
