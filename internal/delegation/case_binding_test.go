package delegation

import (
	"runtime"
	"testing"
)

// withCaseInsensitiveLeasePaths pins the platform case mode for one test and restores it
// on cleanup. The package variable is shared, so these tests must not run in parallel.
func withCaseInsensitiveLeasePaths(t *testing.T, on bool) {
	t.Helper()
	previous := caseInsensitiveLeasePaths
	caseInsensitiveLeasePaths = on
	t.Cleanup(func() { caseInsensitiveLeasePaths = previous })
}

func TestTreePathspecAddsIcaseOnlyWhenRequested(t *testing.T) {
	if got := treePathspec("internal/foo", false); got != "internal/foo" {
		t.Fatalf("case-sensitive pathspec = %q, want unchanged pattern", got)
	}
	if got := treePathspec("internal/foo", true); got != ":(icase)internal/foo" {
		t.Fatalf("icase pathspec = %q, want :(icase) magic prefix", got)
	}
	if got := treePathspec("docs/*.md", true); got != ":(icase)docs/*.md" {
		t.Fatalf("icase glob pathspec = %q", got)
	}
}

func TestMixedCaseTrackedFilesMatchFoldedDirectoryEntry(t *testing.T) {
	workspace := fpTrackedWorkspace(t, map[string]string{"Internal/Foo/A.go": "alpha"})

	withCaseInsensitiveLeasePaths(t, false)
	plain, err := listTrackedFiles(workspace, "internal/foo")
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != 0 {
		t.Fatalf("case-sensitive pathspec matched %v, want none", plain)
	}

	withCaseInsensitiveLeasePaths(t, true)
	folded, err := listTrackedFiles(workspace, "internal/foo")
	if err != nil {
		t.Fatal(err)
	}
	if len(folded) != 1 || folded[0] != "Internal/Foo/A.go" {
		t.Fatalf("icase pathspec matched %v, want [Internal/Foo/A.go]", folded)
	}
}

// All-lowercase trees must digest identically in both modes, so approvals recorded before
// the fix keep matching.
func TestLowercaseTreeDigestUnchangedByCaseMode(t *testing.T) {
	workspace := fpTrackedWorkspace(t, map[string]string{
		"pkg/a.txt":        "alpha",
		"pkg/nested/b.txt": "beta",
	})
	withCaseInsensitiveLeasePaths(t, false)
	plain, err := fingerprintTrackedTree(workspace, "pkg")
	if err != nil {
		t.Fatal(err)
	}
	withCaseInsensitiveLeasePaths(t, true)
	folded, err := fingerprintTrackedTree(workspace, "pkg")
	if err != nil {
		t.Fatal(err)
	}
	if plain != folded {
		t.Fatalf("lowercase tree digest changed with case mode: %s != %s", plain, folded)
	}
}

func TestMixedCaseTrackedTreeDigestDetectsEdit(t *testing.T) {
	workspace := fpTrackedWorkspace(t, map[string]string{"Internal/Foo/A.go": "alpha"})
	withCaseInsensitiveLeasePaths(t, true)
	before, err := fingerprintTrackedTree(workspace, "internal/foo")
	if err != nil {
		t.Fatal(err)
	}
	fpWrite(t, workspace, "Internal/Foo/A.go", "edited")
	after, err := fingerprintTrackedTree(workspace, "internal/foo")
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("edit to a mixed-case tracked file did not change the directory digest")
	}
}

// Real case-folding filesystems only: the folded lease path must reach the mixed-case
// tracked directory through fingerprintFile, the same route the review binding takes.
func TestFoldedEntryFingerprintsMixedCaseDirectoryOnFoldingPlatforms(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("end-to-end case folding applies only on darwin and windows")
	}
	workspace := fpTrackedWorkspace(t, map[string]string{"Internal/Foo/A.go": "alpha"})
	before, err := fingerprintFile(workspace, "Internal/Foo")
	if err != nil {
		t.Fatal(err)
	}
	fpWrite(t, workspace, "Internal/Foo/A.go", "edited")
	after, err := fingerprintFile(workspace, "Internal/Foo")
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("edit between review and PASS did not change the folded directory digest")
	}
}
