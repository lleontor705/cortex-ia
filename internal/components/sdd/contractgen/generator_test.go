package contractgen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/phasecontract"
)

func TestGenerateReferencesEmitsRawMarkdownPolicyAndJSONContract(t *testing.T) {
	assets, err := GenerateReferences(GeneratorInput{Version: "1.0.0", SourceFingerprint: "fingerprint"})
	if err != nil {
		t.Fatal(err)
	}

	var markdown, policyJSON *GeneratedAsset
	for i := range assets {
		switch assets[i].SemanticID {
		case "asset/root/policy":
			markdown = &assets[i]
		case "asset/contract/policy":
			policyJSON = &assets[i]
		}
	}
	if markdown == nil || policyJSON == nil {
		t.Fatalf("policy assets missing: markdown=%v json=%v", markdown != nil, policyJSON != nil)
	}

	if !bytes.HasPrefix(markdown.Content, []byte("# Generated")) {
		t.Errorf("policy markdown starts with %q, want raw heading", markdown.Content[:min(len(markdown.Content), 20)])
	}
	if bytes.HasPrefix(markdown.Content, []byte(`"`)) {
		t.Error("policy markdown is JSON-quoted")
	}
	if bytes.Contains(markdown.Content, []byte(`\n`)) {
		t.Error(`policy markdown contains escaped \n sequences`)
	}
	if !bytes.HasSuffix(markdown.Content, []byte("\n")) || bytes.HasSuffix(markdown.Content, []byte("\n\n")) {
		t.Error("policy markdown must have exactly one trailing newline")
	}
	sum := sha256.Sum256(markdown.Content)
	if got := hex.EncodeToString(sum[:]); markdown.SHA256 != got {
		t.Errorf("policy markdown SHA256 = %q, want %q", markdown.SHA256, got)
	}

	var policy any
	if err := json.Unmarshal(policyJSON.Content, &policy); err != nil {
		t.Fatalf("policy JSON is not parseable: %v", err)
	}
}

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
	for _, asset := range assets {
		if asset.SemanticID == "asset/contract/status-vocabulary" {
			if bytes.Contains(asset.Content, []byte(`"init"`)) || bytes.Contains(asset.Content, []byte(`"explore"`)) {
				t.Fatalf("legacy phase leaked into generated vocabulary: %s", asset.Content)
			}
			if !bytes.Contains(asset.Content, []byte(`"bootstrap"`)) || !bytes.Contains(asset.Content, []byte(`"investigate"`)) {
				t.Fatalf("canonical phase missing from generated vocabulary: %s", asset.Content)
			}
		}
		if asset.SemanticID == "asset/contract/phase-schema" && bytes.Contains(asset.Content, []byte(`"aliases"`)) {
			t.Fatalf("generated active schema exposes compatibility aliases: %s", asset.Content)
		}
	}
}
