package delegation

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// leaseScopeTask opens a fresh store with one claimed task whose allowed_files are the
// given entries, and returns the store with the live claim token.
func leaseScopeTask(t *testing.T, allowed []string) (*Store, string) {
	t.Helper()
	ctx := context.Background()
	s, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.CreateWorkInBoardWithDefinition(ctx, DefaultBoardID, "scoped", "scoped", nil, WorkDefinition{
		Project: t.TempDir(), AllowedFiles: allowed,
	}); err != nil {
		t.Fatal(err)
	}
	claim, err := s.ClaimWork(ctx, "scoped", "owner", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return s, claim.Token
}

func TestLeaseRefusedOutsideAllowedFiles(t *testing.T) {
	ctx := context.Background()
	s, token := leaseScopeTask(t, []string{"internal/foo/a.go"})
	for _, path := range []string{"go.mod", "internal/foo/b.go", "internal/foobar/x.go", "internal/foo/a.go.bak"} {
		_, err := s.ReserveWorkLease(ctx, "scoped", token, path, time.Minute)
		if err == nil {
			t.Fatalf("lease %s outside allowed_files was accepted", path)
		}
		if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "widen allowed_files") {
			t.Fatalf("lease %s error does not name the path and the fix: %v", path, err)
		}
	}
	if _, err := s.ReserveWorkLease(ctx, "scoped", token, "internal/foo/a.go", time.Minute); err != nil {
		t.Fatalf("exact file entry must lease: %v", err)
	}
}

func TestLeaseAllowedByDirectoryAndGlobEntries(t *testing.T) {
	ctx := context.Background()
	s, token := leaseScopeTask(t, []string{"internal/foo", "docs/*.md", "**/*.go"})
	for _, path := range []string{"internal/foo/nested/b.go", "docs/readme.md", "docs/sub/x.md", "cmd/main.go"} {
		if _, err := s.ReserveWorkLease(ctx, "scoped", token, path, time.Minute); err != nil {
			t.Fatalf("lease %s inside allowed_files: %v", path, err)
		}
	}
	for _, path := range []string{"docs/readme.txt", "internal/food/x.md"} {
		if _, err := s.ReserveWorkLease(ctx, "scoped", token, path, time.Minute); err == nil {
			t.Fatalf("lease %s outside allowed_files was accepted", path)
		}
	}
}

func TestLeaseRefusedWhenAllowedFilesEmpty(t *testing.T) {
	ctx := context.Background()
	s, token := leaseScopeTask(t, nil)
	_, err := s.ReserveWorkLease(ctx, "scoped", token, "src/main.go", time.Minute)
	if err == nil || !strings.Contains(err.Error(), "declares no allowed_files") {
		t.Fatalf("lease with empty allowed_files = %v, want refusal naming the empty scope", err)
	}
}
