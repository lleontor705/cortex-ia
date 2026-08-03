package ir

import (
	"testing"
)

func TestAssetClassCoversAllRetainedAssetTypes(t *testing.T) {
	want := map[AssetClass]struct{}{
		AssetRootIndex:       {},
		AssetRootModule:      {},
		AssetSharedContract:  {},
		AssetSkill:           {},
		AssetCommand:         {},
		AssetRoleStub:        {},
		AssetProfileOverlay:  {},
		AssetQualityTemplate: {},
		AssetContractSchema:  {},
	}
	if len(want) != 9 {
		t.Fatalf("expected exactly 9 retained asset classes, got %d", len(want))
	}
	for class := range want {
		if err := ValidateAssetClass(class); err != nil {
			t.Fatalf("ValidateAssetClass(%q) error = %v", class, err)
		}
	}
}

func TestValidateAssetClassRejectsUnknown(t *testing.T) {
	for _, class := range []AssetClass{"", "unknown", "RootIndex", "root_index", "mailbox"} {
		t.Run(string(class), func(t *testing.T) {
			if err := ValidateAssetClass(class); err == nil {
				t.Fatalf("ValidateAssetClass(%q) error = nil, want rejection", class)
			}
		})
	}
}

func TestAssetSpecValidate(t *testing.T) {
	valid := AssetSpec{
		ID:         "asset/root-index",
		Class:      AssetRootIndex,
		SourcePath: "internal/assets/generic/sdd-root/index.md",
		Required:   true,
		MaxTokens:  1500,
		SHA256:     "abc123",
	}

	tests := []struct {
		name   string
		mutate func(*AssetSpec)
	}{
		{name: "invalid semantic id", mutate: func(s *AssetSpec) { s.ID = "root-index" }},
		{name: "unknown class", mutate: func(s *AssetSpec) { s.Class = "unknown" }},
		{name: "empty source path", mutate: func(s *AssetSpec) { s.SourcePath = "" }},
		{name: "negative max tokens", mutate: func(s *AssetSpec) { s.MaxTokens = -1 }},
		{name: "required without fingerprint", mutate: func(s *AssetSpec) { s.SHA256 = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := valid
			tt.mutate(&spec)
			if err := spec.Validate(); err == nil {
				t.Fatalf("Validate() error = nil, want rejection for %s", tt.name)
			}
		})
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("valid AssetSpec.Validate() error = %v", err)
	}
}

func TestAssetCatalogValidateRequiresAllMandatoryRootIndexAndRejectsDuplicates(t *testing.T) {
	if err := (AssetCatalog{}).Validate(); err == nil {
		t.Fatal("empty catalog Validate() error = nil, want rejection for missing schema version")
	}

	goodSpec := AssetSpec{
		ID:         "asset/root-index",
		Class:      AssetRootIndex,
		SourcePath: "internal/assets/generic/sdd-root/index.md",
		Required:   true,
		MaxTokens:  1500,
		SHA256:     "deadbeef",
	}
	valid := AssetCatalog{
		SchemaVersion: AssetCatalogSchema.Current,
		Assets:        []AssetSpec{goodSpec},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid catalog.Validate() error = %v", err)
	}

	t.Run("duplicate ids rejected", func(t *testing.T) {
		dup := valid
		dup.Assets = append(dup.Assets, goodSpec)
		if err := dup.Validate(); err == nil {
			t.Fatal("catalog with duplicate asset IDs Validate() error = nil, want rejection")
		}
	})

	t.Run("missing root-index rejected", func(t *testing.T) {
		noRoot := AssetCatalog{
			SchemaVersion: AssetCatalogSchema.Current,
			Assets: []AssetSpec{{
				ID:         "asset/skill-bootstrap",
				Class:      AssetSkill,
				SourcePath: "skills/bootstrap/SKILL.md",
				Required:   true,
				MaxTokens:  1000,
				SHA256:     "cafef00d",
			}},
		}
		if err := noRoot.Validate(); err == nil {
			t.Fatal("catalog without mandatory root-index Validate() error = nil, want rejection")
		}
	})

	t.Run("invalid child asset rejected", func(t *testing.T) {
		badChild := valid
		badChild.Assets = append(badChild.Assets, AssetSpec{
			ID:         "asset/bad",
			Class:      "unknown",
			SourcePath: "x",
			Required:   false,
			MaxTokens:  0,
		})
		if err := badChild.Validate(); err == nil {
			t.Fatal("catalog with invalid child asset Validate() error = nil, want rejection")
		}
	})
}

func TestFingerprintContentIsDeterministic(t *testing.T) {
	a := FingerprintContent([]byte("hello world"))
	b := FingerprintContent([]byte("hello world"))
	if a != b {
		t.Fatalf("same content produced different fingerprints: %q != %q", a, b)
	}
	if a == "" {
		t.Fatal("FingerprintContent returned empty fingerprint")
	}
	if len(a) != 64 {
		t.Fatalf("fingerprint length = %d, want 64 hex chars", len(a))
	}

	c := FingerprintContent([]byte("goodbye world"))
	if a == c {
		t.Fatal("different content produced identical fingerprints")
	}

	// Canonical known SHA-256 of "hello world" guards against algorithm drift.
	const wantHello = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if a != wantHello {
		t.Fatalf("FingerprintContent drift: got %q want %q", a, wantHello)
	}
}
