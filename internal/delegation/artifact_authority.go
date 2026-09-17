package delegation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ArtifactAuthority travels only over the controller's private stdin channel.
// Session identity alone is not proof: both current claim and file tokens bind it.
type ArtifactAuthority struct {
	Project    string `json:"project"`
	SessionID  string `json:"session_id"`
	Role       string `json:"role"`
	TaskID     string `json:"task_id"`
	Path       string `json:"path"`
	ClaimToken string `json:"claim_token"`
	LeaseToken string `json:"lease_token"`
}

type ArtifactWriteGuard struct {
	store      *Store
	authority  ArtifactAuthority
	standalone bool
}

// OpenArtifactWriteGuard preserves intentional standalone CLI use explicitly.
// Controller calls must provide authenticated stdin, never standalone mode.
func OpenArtifactWriteGuard(dbPath string, input io.Reader, standalone bool) (*ArtifactWriteGuard, error) {
	if standalone {
		if input != nil {
			return nil, errors.New("standalone output cannot include controller authority")
		}
		return &ArtifactWriteGuard{standalone: true}, nil
	}
	if input == nil {
		return nil, errors.New("artifact output requires controller authority or explicit --standalone")
	}
	data, err := io.ReadAll(io.LimitReader(input, 8193))
	if err != nil || len(data) > 8192 {
		return nil, errors.New("invalid bounded artifact authority")
	}
	var authority ArtifactAuthority
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&authority); err != nil {
		return nil, errors.New("invalid artifact authority envelope")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("invalid artifact authority envelope")
	}
	if authority.Role != "implement" || authority.SessionID == "" || authority.Project == "" || authority.TaskID == "" || len(authority.ClaimToken) == 0 || len(authority.ClaimToken) > 128 || len(authority.LeaseToken) == 0 || len(authority.LeaseToken) > 128 {
		return nil, errors.New("artifact output requires live implement authority")
	}
	if err := (ConversationOwnership{OpenCodeSessionID: authority.SessionID, OpenCodeRootSessionID: authority.SessionID}).Validate(); err != nil {
		return nil, err
	}
	store, err := OpenStoreReadOnly(dbPath)
	if err != nil {
		return nil, errors.New("artifact authority store unavailable")
	}
	return &ArtifactWriteGuard{store: store, authority: authority}, nil
}

func (g *ArtifactWriteGuard) Close() error {
	if g.store != nil {
		return g.store.Close()
	}
	return nil
}

func artifactRelativePath(project, target string) (string, error) {
	if strings.ContainsRune(target, 0) || strings.ContainsRune(project, 0) {
		return "", errors.New("invalid artifact path")
	}
	root, err := filepath.Abs(project)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	relative, err := filepath.Rel(root, filepath.Clean(target))
	if err != nil {
		return "", errors.New("artifact output is outside workspace")
	}
	canonical, err := canonicalLeasePath(relative)
	if err != nil {
		return "", err
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", errors.New("artifact workspace unavailable")
	}
	// Reject every link component, including existing final targets and junctions.
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		physical = filepath.Join(physical, component)
		info, err := os.Lstat(physical)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("artifact output cannot traverse a symlink")
		}
		resolved, err := filepath.EvalSymlinks(physical)
		if err != nil || !SameWorkspace(resolved, physical) {
			return "", errors.New("artifact physical path cannot be verified")
		}
	}
	return canonical, nil
}

// Check must run at admission and immediately before each mutation/renderer launch.
// This does not make SQLite verification and filesystem writes one transaction.
func (g *ArtifactWriteGuard) Check(ctx context.Context, target string) error {
	if g.standalone {
		return nil
	}
	path, err := artifactRelativePath(g.authority.Project, target)
	if err != nil {
		return err
	}
	boundPath, err := canonicalLeasePath(g.authority.Path)
	if err != nil || path != boundPath {
		return errors.New("artifact target does not match reserved path")
	}
	var workspace string
	err = g.store.db.QueryRowContext(ctx, `SELECT w.workspace FROM work_items w JOIN work_claims c ON c.item_id=w.id JOIN work_leases l ON l.item_id=w.id WHERE w.id=? AND w.status='in_progress' AND c.owner=? AND c.token_hash=? AND c.expires_at>? AND l.path=? AND l.token_hash=? AND l.expires_at>?`, g.authority.TaskID, "opencode-session:"+g.authority.SessionID, tokenHash(g.authority.ClaimToken), g.store.timestamp(), path, tokenHash(g.authority.LeaseToken), g.store.timestamp()).Scan(&workspace)
	if err != nil || !SameWorkspace(workspace, g.authority.Project) {
		return errors.New("artifact output requires a live session-owned claim and file lease")
	}
	var external int
	// Match Store.Create's workspace fence, including paused external workers and
	// cancellation/cleanup uncertainty whose status alone does not prove termination.
	err = g.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM delegation_jobs j WHERE lower(replace(j.workspace,'\','/'))=lower(replace(?,'\','/')) AND (j.status IN('accepted','starting','running','blocked') OR ((j.status='lost' OR j.error_code IN('CANCEL_REQUESTED','TERMINATION_UNCONFIRMED')) AND NOT (j.status='lost' AND `+reconciledJobSQL+`)))`, workspace).Scan(&external)
	if err != nil || external != 0 {
		return fmt.Errorf("artifact output blocked by external execution or unresolved authority")
	}
	return nil
}
