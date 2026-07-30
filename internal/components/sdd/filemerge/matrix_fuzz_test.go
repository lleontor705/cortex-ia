package filemerge

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeSafetyMatrixHasCommittedDeterministicCorpus(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("testdata", "fuzz", "FuzzMergeSafetyMatrix"))
	if err != nil {
		t.Fatalf("read committed fuzz corpus: %v", err)
	}
	if len(entries) < 3 {
		t.Fatalf("committed fuzz corpus entries = %d, want at least 3 safety scenarios", len(entries))
	}
}

func FuzzMergeSafetyMatrix(f *testing.F) {
	f.Fuzz(func(t *testing.T, base, currentRegion, generated, prefix, suffix []byte) {
		base = boundedFixture(base)
		currentRegion = boundedFixture(currentRegion)
		generated = boundedFixture(generated)
		prefix = boundedFixture(prefix)
		suffix = boundedFixture(suffix)
		current := append(append(append([]byte(nil), prefix...), currentRegion...), suffix...)

		result, err := Merge(current, []ManagedRegion{{
			SemanticID: "region/fuzz-safety-matrix", Start: len(prefix), End: len(prefix) + len(currentRegion), RecordedBase: base, Generated: generated,
		}})
		if err != nil {
			t.Fatalf("Merge() error = %v", err)
		}
		if len(result.Conflicts) != 0 {
			if !bytes.Equal(result.Content, current) {
				t.Fatal("conflicting merge changed current bytes")
			}
			for _, conflict := range result.Conflicts {
				if conflict.RecordedBaseRef == "" || conflict.CurrentRef == "" || conflict.GeneratedRef == "" {
					t.Fatalf("conflict omitted deterministic references: %#v", conflict)
				}
			}
			return
		}
		if !bytes.Equal(result.Content[:len(prefix)], prefix) || !bytes.Equal(result.Content[len(result.Content)-len(suffix):], suffix) {
			t.Fatal("conflict-free merge changed bytes outside the managed region")
		}
	})
}

func boundedFixture(value []byte) []byte {
	const max = 64
	if len(value) > max {
		return value[:max]
	}
	return value
}
