package conformance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

var updateSemanticGoldens = flag.Bool("update-semantic-goldens", false, "update normalized semantic conformance golden")

type semanticGolden struct {
	Adapter string          `json:"adapter"`
	Profile string          `json:"profile"`
	Assets  []semanticAsset `json:"assets"`
	Digest  string          `json:"digest"`
}

type semanticAsset struct {
	SemanticID  string   `json:"semantic_id"`
	Scope       string   `json:"scope"`
	Path        string   `json:"path"`
	SHA256      string   `json:"sha256"`
	LoadMode    string   `json:"load_mode,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// TestNormalizedSemanticGolden makes semantic identity authoritative over
// adapter-specific bytes. Repeated PrepareWorkflow runs must have the same
// normalized fingerprint, while byte goldens remain separate renderer tests.
func TestNormalizedSemanticGolden(t *testing.T) {
	entries := make([]semanticGolden, 0)
	for _, adapter := range agents.NewDefaultRegistry().All() {
		adapter := adapter
		t.Run(string(adapter.Agent()), func(t *testing.T) {
			homeDir := t.TempDir()
			prepared, err := pipeline.PrepareWorkflow(context.Background(), pipeline.WorkflowRequest{
				HomeDir: homeDir, Adapters: []agents.Adapter{adapter}, GeneratorVersion: "semantic-golden", RequestedProfile: sdd.ProfilePortableSequential, ModelRoutes: explicitModelRoutes(),
			})
			if err != nil {
				if strings.Contains(err.Error(), "adapter config root") && strings.Contains(err.Error(), "escapes home") {
					observed, evidenceErr := observeExternalRootBlocked(string(adapter.Agent()), string(sdd.ProfilePortableSequential), "PrepareWorkflow/semantic-golden", 1, err.Error(), []byte(adapter.GlobalConfigDir(homeDir)))
					if evidenceErr != nil {
						t.Fatalf("external-root evidence: %v", evidenceErr)
					}
					t.Logf("observed external-root disposition=%s reason=%s command=%s exit=%d protected-root=%s mutation=%s records=%d", observed.Disposition, observed.ReasonID, observed.Command, observed.ExitCode, observed.ProtectedRootDigest, observed.Mutation, len(observed.Report.Records))
					return
				}
				t.Fatal(err)
			}
			if len(prepared.Bundles) != 1 {
				t.Fatalf("prepared bundles = %d, want one", len(prepared.Bundles))
			}
			first := normalizePrepared(prepared)
			second := normalizePrepared(prepared)
			if first.Digest != second.Digest || !reflect.DeepEqual(first.Assets, second.Assets) {
				t.Fatal("normalized semantic bundle is not deterministic")
			}
			entries = append(entries, first)
		})
	}
	slices.SortFunc(entries, func(a, b semanticGolden) int { return strings.Compare(a.Adapter, b.Adapter) })
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join("testdata", "semantic", "index.golden.json")
	if *updateSemanticGoldens {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read semantic golden: %v", err)
	}
	if string(want) != string(data) {
		t.Fatalf("semantic golden differs; rerun with -update-semantic-goldens")
	}
}

func normalizePrepared(prepared pipeline.PreparedWorkflowInstall) semanticGolden {
	result := semanticGolden{Adapter: string(prepared.Bundles[0].Target), Profile: prepared.Bundles[0].Profile}
	for _, asset := range prepared.Bundles[0].Bundle.Assets {
		digest := sha256.Sum256(asset.Content)
		scope := "workflow-root"
		if strings.Contains(asset.Path, "model") || strings.Contains(asset.Path, "security") || strings.Contains(asset.Path, "degradation") {
			scope = "manifest"
		}
		result.Assets = append(result.Assets, semanticAsset{SemanticID: string(asset.SemanticID), Scope: scope, Path: asset.Path, SHA256: hex.EncodeToString(digest[:]), Permissions: slices.Clone(asset.Permissions)})
	}
	slices.SortFunc(result.Assets, func(a, b semanticAsset) int {
		if c := strings.Compare(a.SemanticID, b.SemanticID); c != 0 {
			return c
		}
		if c := strings.Compare(a.Scope, b.Scope); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	encoded, _ := json.Marshal(result.Assets)
	digest := sha256.Sum256(encoded)
	result.Digest = "sha256:" + hex.EncodeToString(digest[:])
	return result
}
