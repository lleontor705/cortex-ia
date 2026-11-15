package delegation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/diagram"
	"github.com/lleontor705/cortex-ia/internal/docconv"
)

func artifactFixture(t *testing.T) (string, string, ArtifactAuthority) {
	t.Helper()
	home := t.TempDir()
	for _, name := range []string{"HOME", "USERPROFILE", "CORTEX_IA_HOME"} {
		t.Setenv(name, home)
	}
	workspace := t.TempDir()
	dbPath := filepath.Join(home, "state.db")
	s, err := OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	_, err = s.CreateWorkInBoardWithDefinition(ctx, DefaultBoardID, "artifact", "artifact", nil, WorkDefinition{Project: workspace, AllowedFiles: []string{"result.md"}})
	if err != nil {
		t.Fatal(err)
	}
	claim, err := s.ClaimWorkWithLeases(ctx, "artifact", "opencode-session:host", []string{"result.md"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return dbPath, workspace, ArtifactAuthority{Project: workspace, SessionID: "host", Role: "implement", TaskID: "artifact", Path: "result.md", ClaimToken: claim.Token, LeaseToken: claim.ReservedFiles[0].Token}
}

func artifactGuard(t *testing.T, db string, authority ArtifactAuthority) *ArtifactWriteGuard {
	t.Helper()
	data, err := json.Marshal(authority)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := OpenArtifactWriteGuard(db, strings.NewReader(string(data)), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guard.Close() })
	return guard
}

func TestArtifactWriteAuthorityAndContainment(t *testing.T) {
	db, workspace, authority := artifactFixture(t)
	guard := artifactGuard(t, db, authority)
	target := filepath.Join(workspace, "result.md")
	ctx := context.Background()
	if err := guard.Check(ctx, target); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{filepath.Join(workspace, "..", "result.md"), filepath.Join(workspace, "other.md")} {
		if err := guard.Check(ctx, target); err == nil {
			t.Fatal("unleased/outside target accepted")
		}
	}
	for _, field := range []string{"session", "claim", "lease", "workspace"} {
		spoof := authority
		switch field {
		case "session":
			spoof.SessionID = "other"
		case "claim":
			spoof.ClaimToken = "fake"
		case "lease":
			spoof.LeaseToken = "fake"
		case "workspace":
			spoof.Project = t.TempDir()
		}
		if err := artifactGuard(t, db, spoof).Check(ctx, target); err == nil {
			t.Fatal("spoofed authority accepted: " + field)
		}
	}
	authority.Role = "reviewer"
	data, _ := json.Marshal(authority)
	if _, err := OpenArtifactWriteGuard(db, strings.NewReader(string(data)), false); err == nil {
		t.Fatal("reviewer writer accepted")
	}
	if _, err := OpenArtifactWriteGuard(db, nil, false); err == nil {
		t.Fatal("implicit standalone accepted")
	}
	if _, err := OpenArtifactWriteGuard(db, strings.NewReader(string(data)), true); err == nil {
		t.Fatal("mixed standalone/controller accepted")
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, target); err == nil {
		if err := guard.Check(ctx, target); err == nil {
			t.Fatal("physical escape accepted")
		}
	} else {
		t.Log("OS symlink privilege unavailable; mocked host symlink boundary is covered separately")
	}
}

func TestArtifactConversionRechecksExpiryBeforeOutput(t *testing.T) {
	db, workspace, authority := artifactFixture(t)
	guard := artifactGuard(t, db, authority)
	ctx := context.Background()
	input, output := filepath.Join(workspace, "input.txt"), filepath.Join(workspace, "result.md")
	if err := os.WriteFile(input, []byte("synthetic markdown"), 0600); err != nil {
		t.Fatal(err)
	}
	inline, err := docconv.Convert(ctx, docconv.ConvertOptions{FilePath: input})
	if err != nil || inline.Markdown != "synthetic markdown" {
		t.Fatalf("inline read failed: %v", err)
	}
	if err := guard.Check(ctx, output); err != nil {
		t.Fatal(err)
	}
	_, err = docconv.Convert(ctx, docconv.ConvertOptions{FilePath: input, OutputPath: output, BeforeWrite: func(path string) error {
		guard.store.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
		return guard.Check(ctx, path)
	}})
	if err == nil {
		t.Fatal("expired conversion wrote output")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("expired conversion created artifact")
	}
	guard.store.now = time.Now
	_, err = docconv.Convert(ctx, docconv.ConvertOptions{FilePath: input, OutputPath: output, BeforeWrite: func(path string) error { return guard.Check(ctx, path) }})
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(output)
	if err != nil || string(actual) != "synthetic markdown" {
		t.Fatal("authorized output missing")
	}
}

func TestArtifactRendererRechecksBeforeFallbackAndLaunch(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "external"}[external], func(t *testing.T) {
			db, workspace, authority := artifactFixture(t)
			t.Chdir(workspace)
			if external {
				compiler := filepath.Join(os.Getenv("HOME"), "node_modules", "archify", "bin", "archify.mjs")
				if err := os.MkdirAll(filepath.Dir(compiler), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(compiler, []byte("throw new Error('must not launch')"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			guard := artifactGuard(t, db, authority)
			output := filepath.Join(workspace, "result.md")
			checks := 0
			_, err := diagram.Render(context.Background(), []byte(`{"diagram_type":"architecture","meta":{"title":"test"},"components":[]}`), output, diagram.RenderOptions{BeforeWrite: func(path string) error {
				checks++
				if checks > 1 {
					guard.store.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
				}
				return guard.Check(context.Background(), path)
			}})
			if err == nil || checks != 2 {
				t.Fatalf("expected prewrite/prelaunch expiry, checks=%d err=%v", checks, err)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("expired renderer created output")
			}
		})
	}
}

func TestArtifactWriteHonorsExternalWorkspaceFence(t *testing.T) {
	for _, record := range [][2]string{{"blocked", ""}, {"lost", "LEASE_EXPIRED"}, {"failed", "CANCEL_REQUESTED"}, {"failed", "TERMINATION_UNCONFIRMED"}} {
		t.Run(record[0]+record[1], func(t *testing.T) {
			db, workspace, authority := artifactFixture(t)
			guard := artifactGuard(t, db, authority)
			writer, err := OpenStore(db)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Close()
			canonical, err := CanonicalWorkspace(workspace)
			if err != nil {
				t.Fatal(err)
			}
			_, err = writer.db.Exec(`INSERT INTO delegation_jobs(id,role,objective_digest,status,transport,workspace,error_code,created_at,updated_at) VALUES('fenced','investigate','synthetic',?,'direct',?,?,?,?)`, record[0], canonical, record[1], writer.timestamp(), writer.timestamp())
			if err != nil {
				t.Fatal(err)
			}
			if err := guard.Check(context.Background(), filepath.Join(workspace, "result.md")); err == nil {
				t.Fatal("external workspace fence bypassed")
			}
		})
	}
}
