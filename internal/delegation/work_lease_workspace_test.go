package delegation

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkLeaseIsScopedByWorkspace(t *testing.T) {
	ctx := context.Background()
	s, err := OpenStore(filepath.Join(t.TempDir(), "delegation.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	seed := func(id, workspace string) {
		t.Helper()
		if _, err := s.CreateWorkInBoardWithDefinition(ctx, DefaultBoardID, id, id, nil,
			WorkDefinition{Project: workspace, AllowedFiles: []string{"main.go"}}); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("same relative path in different workspaces", func(t *testing.T) {
		seed("repo-a", t.TempDir())
		seed("repo-b", t.TempDir())
		if _, err := s.ClaimWorkWithLeases(ctx, "repo-a", "owner-a", []string{"main.go"}, time.Minute); err != nil {
			t.Fatalf("repo-a lease: %v", err)
		}
		if _, err := s.ClaimWorkWithLeases(ctx, "repo-b", "owner-b", []string{"main.go"}, time.Minute); err != nil {
			t.Fatalf("repo-b lease on same relative path must not conflict: %v", err)
		}
	})

	t.Run("same workspace still conflicts", func(t *testing.T) {
		workspace := t.TempDir()
		seed("same-a", workspace)
		seed("same-b", workspace)
		if _, err := s.ClaimWorkWithLeases(ctx, "same-a", "owner-a", []string{"main.go"}, time.Minute); err != nil {
			t.Fatalf("first lease: %v", err)
		}
		_, err := s.ClaimWorkWithLeases(ctx, "same-b", "owner-b", []string{"main.go"}, time.Minute)
		if !errors.Is(err, ErrWorkConflict) {
			t.Fatalf("second lease in same workspace = %v, want ErrWorkConflict", err)
		}
	})
}
