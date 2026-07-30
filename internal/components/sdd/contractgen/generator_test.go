package contractgen

import (
	"bytes"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/phasecontract"
)

func TestGenerateReferencesIsDeterministicAndFingerprintBound(t *testing.T) {
	in := GeneratorInput{Version: "1.0.0", SourceFingerprint: "source-policy-test"}
	first, err := GenerateReferences(in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateReferences(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || len(first) != len(second) {
		t.Fatalf("generated asset counts = %d and %d", len(first), len(second))
	}
	for i := range first {
		if first[i].SemanticID != second[i].SemanticID || first[i].RelativePath != second[i].RelativePath || !bytes.Equal(first[i].Content, second[i].Content) || first[i].SHA256 != second[i].SHA256 {
			t.Fatalf("generation %d is not byte-identical", i)
		}
		if first[i].SourceFingerprint != in.SourceFingerprint || first[i].Version != in.Version {
			t.Errorf("asset %q lost source metadata: %#v", first[i].SemanticID, first[i])
		}
	}
}

func TestGenerateReferencesUsesExecutablePolicyAndContractVocabulary(t *testing.T) {
	assets, err := GenerateReferences(GeneratorInput{Version: "1.0.0", SourceFingerprint: "fingerprint"})
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) < 3 {
		t.Fatalf("generated assets = %d, want contract/status/schema/root references", len(assets))
	}
	for _, asset := range assets {
		if len(asset.Content) == 0 || asset.SHA256 == "" {
			t.Errorf("asset %q is incomplete", asset.SemanticID)
		}
	}
	if _, err := phasecontract.RetryPolicyForKey("retry/apply"); err != nil {
		t.Fatalf("generator test cannot resolve executable phase policy: %v", err)
	}
}
