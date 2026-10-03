package delegation

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// nonGitWorkspace builds an operational container root that is not inside any git work
// tree — the workspace shape behind the in_review fingerprint failures (#81, #108).
func nonGitWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	workspace := t.TempDir()
	for rel, content := range files {
		fpWrite(t, workspace, rel, content)
	}
	if workTree, err := insideGitWorkTree(workspace); err != nil {
		t.Fatalf("probe workspace git state: %v", err)
	} else if workTree {
		t.Fatalf("fixture %s must sit outside a git work tree", workspace)
	}
	return workspace
}

func TestListTreeFilesFallsBackOutsideGitWorkTree(t *testing.T) {
	workspace := nonGitWorkspace(t, map[string]string{
		"pkg/a.txt":        "alpha",
		"pkg/nested/b.txt": "beta",
		"other/c.txt":      "gamma",
		".git/config":      "fake metadata",
	})

	if _, err := listTrackedFiles(workspace, "pkg"); err == nil {
		t.Fatal("git listing must fail in a workspace that is not a git work tree")
	}
	files, err := listTreeFiles(workspace, "pkg")
	if err != nil {
		t.Fatalf("directory entry outside a work tree failed: %v", err)
	}
	want := []string{"pkg/a.txt", "pkg/nested/b.txt"}
	if strings.Join(files, ",") != strings.Join(want, ",") {
		t.Fatalf("walked files = %v, want %v", files, want)
	}

	globbed, err := listTreeFiles(workspace, "pkg/**")
	if err != nil {
		t.Fatalf("glob entry outside a work tree failed: %v", err)
	}
	if strings.Join(globbed, ",") != strings.Join(want, ",") {
		t.Fatalf("walked glob files = %v, want %v", globbed, want)
	}

	fpWrite(t, workspace, "pkg/untracked.txt", "delta")
	grown, err := listTreeFiles(workspace, "pkg")
	if err != nil {
		t.Fatalf("relist after write: %v", err)
	}
	if len(grown) != len(want)+1 {
		t.Fatalf("walk must cover new files without an index, got %v", grown)
	}

	missing, err := listTreeFiles(workspace, "absent-dir/**")
	if err != nil {
		t.Fatalf("pattern that matches nothing must not fail: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("pattern that matches nothing = %v, want empty", missing)
	}
}

func TestWalkedTreePatternMirrorsGitPathspecCoverage(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"docs*b.md", "docs/b.md", true},
		{"*.md", "docs/b.md", true},
		{"pkg*", "other/pkg/e.txt", false},
		{"pkg*", "pkg/nested/d.txt", true},
		{"pkg/**", "pkg/nested/d.txt", true},
		{"**/*.go", "main.go", true},
		{"pk?/nested", "pkg/nested/d.txt", false},
		{"pk?/nested", "pkg/nested", true},
	}
	for _, test := range cases {
		matcher, err := walkedTreePattern(test.pattern)
		if err != nil {
			t.Fatalf("compile %q: %v", test.pattern, err)
		}
		if got := matcher.MatchString(test.path); got != test.want {
			t.Errorf("pattern %q against %q = %v, want %v", test.pattern, test.path, got, test.want)
		}
	}
}

func TestFingerprintDirectoryEntryWithoutGitWorkspace(t *testing.T) {
	workspace := nonGitWorkspace(t, map[string]string{
		"pkg/a.txt":        "alpha",
		"pkg/nested/b.txt": "beta",
		"other/c.txt":      "gamma",
	})

	first, err := fingerprintFile(workspace, "pkg")
	if err != nil {
		t.Fatalf("non-git directory fingerprint failed: %v", err)
	}
	if !strings.HasPrefix(first, "tree:") {
		t.Fatalf("digest = %q, want a tree digest", first)
	}
	second, err := fingerprintFile(workspace, "pkg")
	if err != nil {
		t.Fatalf("second non-git directory fingerprint failed: %v", err)
	}
	if first != second {
		t.Fatalf("digest is not stable: %q vs %q", first, second)
	}

	fpWrite(t, workspace, "pkg/a.txt", "alpha-modified")
	changed, err := fingerprintFile(workspace, "pkg")
	if err != nil {
		t.Fatalf("modified non-git directory fingerprint failed: %v", err)
	}
	if changed == first {
		t.Fatal("digest did not change after a file changed")
	}
}

func TestTransitionToInReviewWithoutGitWorkspace(t *testing.T) {
	ctx := context.Background()
	t.Setenv("CORTEX_IA_HOME", t.TempDir())
	workspace := nonGitWorkspace(t, map[string]string{
		"pkg/a.txt":        "alpha",
		"pkg/nested/b.txt": "beta",
	})

	store, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.CreateBoard(ctx, "nongit-board", "Non-Git Board", ""); err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateWorkInBoardWithDefinition(ctx, "nongit-board", "nongit-task", "Non-Git Task", nil, WorkDefinition{
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
		t.Fatalf("in_review transition must survive a workspace without git: %v", err)
	}
	fp, err := store.ComputeWorkFingerprint(ctx, item.ID)
	if err != nil {
		t.Fatalf("compute fingerprint: %v", err)
	}
	if len(fp.Files) != 1 || !strings.HasPrefix(fp.Files[0].Digest, "tree:") {
		t.Fatalf("fingerprint did not record a tree digest: %+v", fp.Files)
	}
	if _, err := store.ApproveWork(ctx, item.ID, "independent-reviewer", "PASS", "verified", 0); err != nil {
		t.Fatalf("approve task: %v", err)
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
		t.Fatal("current fingerprints must still match the stored approval")
	}
}
