package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fingerprintLargeFileSize = int64(17 * 1024 * 1024)

// writeDeterministicBinary streams a reproducible payload so the oracle never holds
// the artifact in memory alongside the production hashing path.
func writeDeterministicBinary(t *testing.T, abs string, size, seed int64) [32]byte {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(abs)
	if err != nil {
		t.Fatal(err)
	}
	source := rand.New(rand.NewSource(seed))
	digest := sha256.New()
	chunk := make([]byte, 64*1024)
	for remaining := size; remaining > 0; {
		n := int64(len(chunk))
		if remaining < n {
			n = remaining
		}
		if _, err := source.Read(chunk[:n]); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if _, err := digest.Write(chunk[:n]); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if _, err := file.Write(chunk[:n]); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		remaining -= n
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	var sum [32]byte
	copy(sum[:], digest.Sum(nil))
	return sum
}

func hashFileStreaming(t *testing.T, abs string) [32]byte {
	t.Helper()
	file, err := os.Open(abs)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		t.Fatal(err)
	}
	var sum [32]byte
	copy(sum[:], digest.Sum(nil))
	return sum
}

func TestLargeBinaryBindingCarriesRealContentHash(t *testing.T) {
	ctx := context.Background()
	store, workspace := drTestStore(t)
	rel := "bin/big-artifact.bin"
	abs := filepath.Join(workspace, filepath.FromSlash(rel))
	original := writeDeterministicBinary(t, abs, fingerprintLargeFileSize, 20260922)

	item, err := store.CreateWorkInBoardWithDefinition(ctx, "dr-board", "large-binary", "Large Binary", nil, WorkDefinition{
		Project: workspace, Objective: "objective", Acceptance: "acceptance", Verification: "verification", AllowedFiles: []string{rel},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	claim, err := store.ClaimWork(ctx, item.ID, "impl-owner", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	inReview, err := store.TransitionWork(ctx, item.ID, claim.Token, claim.Revision, WorkInReview)
	if err != nil {
		t.Fatalf("in_review must fingerprint a %d MiB artifact: %v", fingerprintLargeFileSize>>20, err)
	}

	want := "file:" + hex.EncodeToString(original[:])
	bound, err := store.ComputeWorkFingerprint(ctx, item.ID)
	if err != nil {
		t.Fatalf("compute fingerprint: %v", err)
	}
	if len(bound.Files) != 1 || bound.Files[0].Path != rel || bound.Files[0].Digest != want {
		t.Fatalf("large artifact digest must be the real content SHA-256:\n got %+v\nwant %s", bound.Files, want)
	}

	var bindingJSON string
	if err := store.db.QueryRowContext(ctx, `SELECT binding_json FROM work_reviews WHERE item_id=?`, item.ID).Scan(&bindingJSON); err != nil {
		t.Fatalf("read review binding: %v", err)
	}
	var binding ReviewBinding
	if err := json.Unmarshal([]byte(bindingJSON), &binding); err != nil {
		t.Fatalf("decode review binding: %v", err)
	}
	if wantChange := hashJSON([]WorkFileDigest{{Path: rel, Digest: want}}); binding.ChangeSHA256 != wantChange {
		t.Fatalf("in_review binding ChangeSHA256 = %s, want content-derived %s", binding.ChangeSHA256, wantChange)
	}
	if len(bindingJSON) > 65536 {
		t.Fatalf("binding must stay within the 64 KiB column cap, got %d bytes", len(bindingJSON))
	}

	file, err := os.OpenFile(abs, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	offset := fingerprintLargeFileSize / 2
	current := make([]byte, 1)
	if _, err := file.ReadAt(current, offset); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte{current[0] ^ 0xFF}, offset); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	mutated := hashFileStreaming(t, abs)
	if mutated == original {
		t.Fatal("drift fixture did not change file content")
	}

	if _, err := store.ApproveWork(ctx, item.ID, "independent-reviewer", "PASS", "evidence", inReview.Revision); err == nil || !strings.Contains(err.Error(), "binding changed") {
		t.Fatalf("post-binding drift must invalidate approval, got: %v", err)
	}

	drifted, err := store.ComputeWorkFingerprint(ctx, item.ID)
	if err != nil {
		t.Fatalf("compute drifted fingerprint: %v", err)
	}
	if wantDrift := "file:" + hex.EncodeToString(mutated[:]); drifted.Files[0].Digest != wantDrift || drifted.Files[0].Digest == want {
		t.Fatalf("drifted digest = %s, want %s", drifted.Files[0].Digest, wantDrift)
	}
}
