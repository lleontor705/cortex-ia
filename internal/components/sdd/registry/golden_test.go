package registry

// WU-18A golden generator/comparator for the canonical registry receipt
// (tasks contract rev 3, REQ-DET-001). This file is the registry package's
// only -update writer: the flag below is registered exactly once, and the
// golden artifact testdata/golden/sdd/registry-receipt.json is owned by
// WU-18 (task-fe5b483c22274ab18c299cbf44d95644), which generates it via
// this canonical path and inspects the resulting diff.
//
// Behavior contract:
//   - go test ./internal/components/sdd/registry/... -count=1
//     byte-compares the committed golden and never writes anything; while
//     the golden is absent the test skips with the exact regeneration
//     command so pre-generation runs stay green.
//   - go test ./internal/components/sdd/registry/... -update -count=1
//     writes ONLY testdata/golden/sdd/registry-receipt.json from the fixed
//     deterministic input set below. No other golden or source file is ever
//     touched, and a mismatch found without -update is reported with
//     lengths and the first differing offset but never rewritten.
//
// The fixture resolves through the real Resolve orchestrator against the
// real embedded baseline catalog and the WU-08 test policy, reusing the
// registry_test.go fixtures so there is exactly one fixture policy owner.
// Identical effective inputs are proven location-independent by
// TestSpec_REQ_DET_001_IdenticalInputsStableEvidence, so temporary roots
// cannot leak into the canonical bytes.

import (
	"bytes"
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// goldenUpdate is the registry package's single -update flag (AC4): it is
// registered exactly once via one flag.Bool call, so the test binary never
// panics on duplicate registration and -update reaches this writer only.
var goldenUpdate = flag.Bool("update", false, "regenerate testdata/golden/sdd/registry-receipt.json from the canonical fixed input set")

// goldenReceipt builds the fixed deterministic golden input set: the real
// embedded baseline catalog, the canonical test policy, two custom skills
// with fixed LF-stable bodies, and the fixed retained component selection
// handed over by the pipeline (design D4). Resolve seals the resulting
// receipt — EffectiveComponents projects the retained selection, never the
// policy classification map — and CanonicalReceiptJSON projects it to
// canonical bytes with every collection sorted and no volatile fields.
func goldenReceipt(t *testing.T) Receipt {
	t.Helper()
	root := t.TempDir()
	configFile := writeOverlayConfig(t, root)
	alphaDir := writeSkillDir(t, root, "alpha", "alpha", "Golden alpha body: fixed bytes, fixed identity.")
	betaDir := writeSkillDir(t, root, "beta", "beta", "Golden beta body: fixed bytes, fixed identity.")
	retained := []model.ComponentID{
		model.ComponentCortex,
		model.ComponentContext7,
		model.ComponentForgeSpec,
		model.ComponentSDD,
		model.ComponentSkills,
	}
	resolved := mustResolve(t, embeddedBaseline(t), testPolicy(), newRetainedRequest(configFile, retained, []string{alphaDir, betaDir}))
	return resolved.CanonicalReceipt
}

// repoGoldenPath locates the golden file relative to the repository root.
// The test binary runs with its working directory at the package source
// directory, so the root is the nearest ancestor holding go.mod; this stays
// correct wherever the repository is checked out.
func repoGoldenPath(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("determine test working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "testdata", "golden", "sdd", "registry-receipt.json")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repository root with go.mod not found above %s", dir)
		}
		dir = parent
	}
}

// firstDifference returns the first byte offset at which the committed
// golden and the freshly generated encoding disagree. When one is a prefix
// of the other, the shared prefix length is the first differing offset.
func firstDifference(golden, current []byte) int {
	limit := min(len(golden), len(current))
	for offset := 0; offset < limit; offset++ {
		if golden[offset] != current[offset] {
			return offset
		}
	}
	return limit
}

// TestGoldenRegistryReceipt pins the canonical JSON encoding of the receipt
// produced by Resolve on the fixed golden input set. It first proves the
// fixture is deterministic across two independent builds (distinct
// temporary roots and baseline materializations), then writes the golden
// under -update or byte-compares it otherwise.
func TestGoldenRegistryReceipt(t *testing.T) {
	first := goldenReceipt(t)
	second := goldenReceipt(t)

	canonical := CanonicalReceiptJSON(first)
	if !bytes.Equal(canonical, CanonicalReceiptJSON(second)) {
		t.Fatal("fixed golden input set produced non-deterministic canonical receipt JSON; refusing to write or compare the golden")
	}
	if err := ValidateReceipt(first); err != nil {
		t.Fatalf("golden receipt fixture is not sealed correctly: %v", err)
	}

	goldenPath := repoGoldenPath(t)

	if *goldenUpdate {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("create golden directory: %v", err)
		}
		if err := os.WriteFile(goldenPath, canonical, 0o644); err != nil {
			t.Fatalf("write golden receipt %s: %v", goldenPath, err)
		}
		t.Logf("updated golden receipt %s (%d bytes)", goldenPath, len(canonical))
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Skipf("golden receipt %s does not exist yet; generate it with: go test ./internal/components/sdd/registry/... -update -count=1", goldenPath)
		}
		t.Fatalf("read golden receipt %s: %v", goldenPath, err)
	}
	if !bytes.Equal(golden, canonical) {
		t.Fatalf("golden receipt mismatch: committed golden is stale (%s)\n  golden:  %d bytes\n  current: %d bytes\n  first differing offset: %d\nregenerate intentionally with: go test ./internal/components/sdd/registry/... -update -count=1",
			goldenPath, len(golden), len(canonical), firstDifference(golden, canonical))
	}
}
