package renderers

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// TestGeminiBaselineDeterminism is the Stage 0 eligibility gate for the Gemini
// renderer. It fails when rendering emits non-deterministic, non-canonical, or
// filesystem-dirtying output, blocking Stage 1 until the baseline is clean.
//
// Contract:
//   - (a) repeated generation is byte-identical (determinism),
//   - (b) unordered source input canonicalizes to identical output,
//   - (c) every agent asset ends with exactly one trailing newline.
func TestGeminiBaselineDeterminism(t *testing.T) {
	profiles := []struct {
		name    string
		profile string
		optIn   bool
	}{
		{name: "portable-flat", profile: "portable-flat"},
		{name: "native-advanced", profile: "native-advanced", optIn: true},
	}

	for _, tt := range profiles {
		t.Run(tt.name, func(t *testing.T) {
			resolved := geminiResolvedWorkflow(tt.profile, tt.optIn)

			first, err := Render(context.Background(), NewGeminiRenderer(), resolved)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			// (a) repeated generation is byte-identical.
			second, err := Render(context.Background(), NewGeminiRenderer(), resolved)
			if err != nil {
				t.Fatalf("Render() second error = %v", err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("repeated Gemini render diverged for profile %q", tt.name)
			}

			// (b) unordered source input canonicalizes to identical output.
			shuffled := geminiResolvedWorkflow(tt.profile, tt.optIn)
			shuffledRoles := slices.Clone(shuffled.Workflow.Roles)
			slices.Reverse(shuffledRoles)
			shuffledPhases := slices.Clone(shuffled.Workflow.Phases)
			slices.Reverse(shuffledPhases)
			shuffled.Workflow.Roles = shuffledRoles
			shuffled.Workflow.Phases = shuffledPhases
			for index := range shuffled.Workflow.Roles {
				role := &shuffled.Workflow.Roles[index]
				terminals := slices.Clone(role.TerminalStates)
				slices.Reverse(terminals)
				role.TerminalStates = terminals
				evidence := slices.Clone(role.Evidence)
				slices.Reverse(evidence)
				role.Evidence = evidence
				effects := slices.Clone(role.AllowedEffects)
				slices.Reverse(effects)
				role.AllowedEffects = effects
			}
			canonical, err := Render(context.Background(), NewGeminiRenderer(), shuffled)
			if err != nil {
				t.Fatalf("Render() shuffled error = %v", err)
			}
			if !reflect.DeepEqual(first, canonical) {
				t.Fatalf("unordered Gemini input did not canonicalize for profile %q", tt.name)
			}

			// (c) every agent asset ends with exactly one trailing newline (no
			// trailing blank line). A trailing blank line is non-canonical and
			// produces dirty golden diffs.
			for _, asset := range first.Assets {
				if asset.Kind != AssetAgent {
					continue
				}
				if len(asset.Content) == 0 {
					t.Fatalf("agent asset %q has empty content", asset.Path)
				}
				if asset.Content[len(asset.Content)-1] != '\n' {
					t.Fatalf("agent asset %q does not end with a newline", asset.Path)
				}
				if len(asset.Content) >= 2 && asset.Content[len(asset.Content)-2] == '\n' {
					t.Fatalf("agent asset %q ends with a trailing blank line (non-canonical)", asset.Path)
				}
			}
		})
	}
}

// TestGeminiBaselineZeroResidue asserts that rendering leaves no filesystem
// side effects: the on-disk goldens are byte-identical before and after a
// render pass. The renderer must remain pure (no writes outside an explicit
// materialization step).
func TestGeminiBaselineZeroResidue(t *testing.T) {
	if testing.Short() {
		t.Skip("filesystem residue check skipped in short mode")
	}
	goldens := []string{
		filepath.Join("testdata", "gemini", "portable-flat.golden"),
		filepath.Join("testdata", "gemini", "portable-sequential.golden"),
		filepath.Join("testdata", "gemini", "native-advanced.golden"),
	}
	before := make(map[string][]byte, len(goldens))
	for _, path := range goldens {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read golden %s: %v", path, err)
		}
		before[path] = data
	}

	for _, profile := range []struct {
		profile string
		optIn   bool
	}{
		{"portable-sequential", false},
		{"portable-flat", false},
		{"native-advanced", true},
	} {
		if _, err := Render(context.Background(), NewGeminiRenderer(), geminiResolvedWorkflow(profile.profile, profile.optIn)); err != nil {
			t.Fatalf("Render() %s error = %v", profile.profile, err)
		}
	}

	for _, path := range goldens {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read golden %s after render: %v", path, err)
		}
		if !bytes.Equal(before[path], got) {
			t.Fatalf("rendering dirtied golden %s (zero-residue violated)", path)
		}
	}
}
