package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fpTrackedWorkspace commits the given files into a fresh temporary Git repository so
// git-tracked fingerprint paths have a real index to read.
func fpTrackedWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	workspace := t.TempDir()
	for rel, content := range files {
		fpWrite(t, workspace, rel, content)
	}
	fpGit(t, workspace, "init")
	fpGit(t, workspace, "config", "user.email", "test@test.com")
	fpGit(t, workspace, "config", "user.name", "Test")
	fpGit(t, workspace, "add", ".")
	fpGit(t, workspace, "commit", "-m", "init")
	return workspace
}

func fpWrite(t *testing.T, workspace, rel, content string) {
	t.Helper()
	abs := filepath.Join(workspace, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fpGit(t *testing.T, workspace string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", workspace}, args...)...)
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

func TestFingerprintFileDirectoryEntryIsStableAndContentSensitive(t *testing.T) {
	workspace := fpTrackedWorkspace(t, map[string]string{
		"pkg/a.txt":        "alpha",
		"pkg/nested/b.txt": "beta",
		"other/c.txt":      "gamma",
	})

	first, err := fingerprintFile(workspace, "pkg")
	if err != nil {
		t.Fatalf("directory fingerprint failed: %v", err)
	}
	if !strings.HasPrefix(first, "tree:") {
		t.Fatalf("directory digest = %q, want a tree digest", first)
	}

	second, err := fingerprintFile(workspace, "pkg")
	if err != nil {
		t.Fatalf("second directory fingerprint failed: %v", err)
	}
	if first != second {
		t.Fatalf("directory digest is not stable: %q vs %q", first, second)
	}

	fpWrite(t, workspace, "pkg/a.txt", "alpha-modified")
	changed, err := fingerprintFile(workspace, "pkg")
	if err != nil {
		t.Fatalf("modified directory fingerprint failed: %v", err)
	}
	if changed == first {
		t.Fatal("directory digest did not change after a tracked file changed")
	}
}

func TestFingerprintFileUntrackedDirectoryUsesEmptySetDigest(t *testing.T) {
	workspace := fpTrackedWorkspace(t, map[string]string{"tracked.txt": "x"})
	fpWrite(t, workspace, "node_modules/dep/index.js", "module.exports = 1")

	empty := "tree:" + hex.EncodeToString(sha256.New().Sum(nil))
	first, err := fingerprintFile(workspace, "node_modules")
	if err != nil {
		t.Fatalf("untracked directory fingerprint failed: %v", err)
	}
	if first != empty {
		t.Fatalf("untracked directory digest = %q, want empty-set digest %q", first, empty)
	}

	fpWrite(t, workspace, "node_modules/dep/index.js", "module.exports = 2")
	second, err := fingerprintFile(workspace, "node_modules")
	if err != nil {
		t.Fatalf("second untracked directory fingerprint failed: %v", err)
	}
	if second != empty {
		t.Fatalf("untracked content leaked into the digest: %q vs %q", second, empty)
	}
}

func TestFingerprintFileGlobEntryExpandsTrackedFiles(t *testing.T) {
	workspace := fpTrackedWorkspace(t, map[string]string{
		"pkg/a.txt":        "alpha",
		"pkg/nested/b.txt": "beta",
	})

	first, err := fingerprintFile(workspace, "pkg/**")
	if err != nil {
		t.Fatalf("glob fingerprint failed: %v", err)
	}
	if !strings.HasPrefix(first, "tree:") {
		t.Fatalf("glob digest = %q, want a tree digest", first)
	}

	fpWrite(t, workspace, "pkg/nested/b.txt", "beta-modified")
	changed, err := fingerprintFile(workspace, "pkg/**")
	if err != nil {
		t.Fatalf("second glob fingerprint failed: %v", err)
	}
	if changed == first {
		t.Fatal("glob digest did not change after a matched tracked file changed")
	}
}

func TestDirectApprovalBindingAcceptsDirectoryAllowedFiles(t *testing.T) {
	ctx := context.Background()
	t.Setenv("CORTEX_IA_HOME", t.TempDir())
	workspace := fpTrackedWorkspace(t, map[string]string{
		"pkg/a.txt":        "alpha",
		"pkg/nested/b.txt": "beta",
	})

	store, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.CreateBoard(ctx, "dir-board", "Directory Board", ""); err != nil {
		t.Fatal(err)
	}

	item, err := store.CreateWorkInBoardWithDefinition(ctx, "dir-board", "dir-task", "Directory Task", nil, WorkDefinition{
		Project: workspace, Objective: "objective", Acceptance: "acceptance", Verification: "verification",
		AllowedFiles: []string{"pkg"},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	claim, err := store.ClaimWork(ctx, item.ID, "impl-owner", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := store.TransitionWork(ctx, item.ID, claim.Token, claim.Revision, WorkInReview); err != nil {
		t.Fatalf("transition in_review with a directory entry failed: %v", err)
	}

	fp, err := store.ComputeWorkFingerprint(ctx, item.ID)
	if err != nil {
		t.Fatalf("work fingerprint with a directory entry failed: %v", err)
	}
	if len(fp.Files) != 1 || !strings.HasPrefix(fp.Files[0].Digest, "tree:") {
		t.Fatalf("fingerprint did not record a tree digest: %+v", fp.Files)
	}

	if _, err := store.ApproveWork(ctx, item.ID, "independent-reviewer", "PASS", "verified", 0); err != nil {
		t.Fatalf("approve task with a directory entry failed: %v", err)
	}
	done, err := store.GetWork(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != WorkDone {
		t.Fatalf("task status = %s, want %s", done.Status, WorkDone)
	}

	matched, err := store.ComputeWorkFingerprint(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if matched.MatchesApproved == nil || !*matched.MatchesApproved {
		t.Fatal("current directory fingerprints must still match the stored approval")
	}
}
