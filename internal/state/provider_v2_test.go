package state

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newProviderTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, stateDir), 0o700); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	return home
}

func providerTestDoc(home string, providers ...ProviderV2) MetadataV2 {
	return MetadataV2{
		SchemaVersion: MetadataSchemaV2,
		OpencodeRoot:  filepath.Clean(home),
		TransactionID: "txn-req-prov-002",
		Providers:     providers,
		UpdatedAt:     time.Unix(1700000000, 0).UTC(),
	}
}

func TestREQ_PROV_002ProviderRoundTrip(t *testing.T) {
	home := newProviderTestHome(t)
	doc := providerTestDoc(home,
		ProviderV2{Name: "zeta", ConfigPath: "opencode.jsonc", Models: []string{"m1", "m2"},
			SemanticDigest: "pvd:zeta", Ownership: OwnershipManaged},
		ProviderV2{Name: "alpha", ConfigPath: "opencode.json", Models: []string{"m3"},
			SemanticDigest: "pvd:alpha", Ownership: OwnershipUser},
	)
	if err := SaveMetadataV2(home, doc); err != nil {
		t.Fatalf("save: %v", err)
	}

	reloaded := LoadMetadataV2(home)
	if reloaded.Presence != PresenceV2 {
		t.Fatalf("presence = %v (detail %s)", reloaded.Presence, reloaded.Detail)
	}
	if reloaded.Metadata.SchemaVersion != MetadataSchemaV2 {
		t.Fatalf("schema_version = %d, want %d", reloaded.Metadata.SchemaVersion, MetadataSchemaV2)
	}
	if len(reloaded.Metadata.Providers) != 2 {
		t.Fatalf("providers = %d, want 2", len(reloaded.Metadata.Providers))
	}
	if reloaded.Metadata.Providers[0].Name != "alpha" || reloaded.Metadata.Providers[1].Name != "zeta" {
		t.Fatalf("providers not sorted by name: %+v", reloaded.Metadata.Providers)
	}

	normalizedIn := doc
	normalizedIn.Normalize()
	wantBytes, err := marshalV2(normalizedIn)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	normalizedOut := reloaded.Metadata
	normalizedOut.Normalize()
	gotBytes, err := marshalV2(normalizedOut)
	if err != nil {
		t.Fatalf("marshal output: %v", err)
	}
	if !bytes.Equal(wantBytes, gotBytes) {
		t.Fatalf("providers did not round-trip byte-equivalently:\nwant %s\ngot  %s", wantBytes, gotBytes)
	}
}

func TestREQ_PROV_002ProvidersOmittedWhenAbsent(t *testing.T) {
	home := newProviderTestHome(t)
	if err := SaveMetadataV2(home, providerTestDoc(home)); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, err := os.ReadFile(StatePath(home))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if bytes.Contains(raw, []byte(`"providers"`)) {
		t.Fatalf("absent providers must be omitted, got %s", raw)
	}

	reloaded := LoadMetadataV2(home)
	if reloaded.Presence != PresenceV2 {
		t.Fatalf("presence = %v (detail %s)", reloaded.Presence, reloaded.Detail)
	}
	if len(reloaded.Metadata.Providers) != 0 {
		t.Fatalf("providers = %+v, want none", reloaded.Metadata.Providers)
	}
	if reloaded.Metadata.SchemaVersion != MetadataSchemaV2 {
		t.Fatalf("schema_version = %d, want %d", reloaded.Metadata.SchemaVersion, MetadataSchemaV2)
	}
}

func TestREQ_PROV_002NormalizeProviders(t *testing.T) {
	var empty MetadataV2
	empty.Normalize()
	if empty.Providers == nil || len(empty.Providers) != 0 {
		t.Fatalf("nil providers not normalized to empty slice: %+v", empty.Providers)
	}

	unsorted := MetadataV2{Providers: []ProviderV2{{Name: "zeta"}, {Name: "alpha"}, {Name: "mid"}}}
	unsorted.Normalize()
	got := []string{unsorted.Providers[0].Name, unsorted.Providers[1].Name, unsorted.Providers[2].Name}
	want := []string{"alpha", "mid", "zeta"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("providers order = %v, want %v", got, want)
		}
	}
}

func TestREQ_PROV_002ValidateProvidersRejects(t *testing.T) {
	home := t.TempDir()
	base := func(providers ...ProviderV2) MetadataV2 {
		return providerTestDoc(home, providers...)
	}
	valid := ProviderV2{Name: "nan", ConfigPath: "opencode.jsonc", Models: []string{"glm5.3"},
		SemanticDigest: "pvd:nan", Ownership: OwnershipManaged}
	if err := ValidateV2(base(valid)); err != nil {
		t.Fatalf("valid provider rejected: %v", err)
	}

	cases := []struct {
		name   string
		field  string
		reason string
		doc    MetadataV2
	}{
		{"blank name", "providers[0].name", "empty",
			base(ProviderV2{Name: "  ", ConfigPath: "opencode.jsonc", SemanticDigest: "d", Ownership: OwnershipManaged})},
		{"duplicate name", "providers[1].name", "duplicate nan",
			base(valid, ProviderV2{Name: "nan", ConfigPath: "opencode.json", SemanticDigest: "d", Ownership: OwnershipManaged})},
		{"empty config path", "providers[0].config_path", "empty",
			base(ProviderV2{Name: "nan", ConfigPath: "", SemanticDigest: "d", Ownership: OwnershipManaged})},
		{"traversal config path", "providers[0].config_path", "traversal or root reference",
			base(ProviderV2{Name: "nan", ConfigPath: "../opencode.json", SemanticDigest: "d", Ownership: OwnershipManaged})},
		{"empty digest", "providers[0].semantic_digest", "empty",
			base(ProviderV2{Name: "nan", ConfigPath: "opencode.jsonc", SemanticDigest: " ", Ownership: OwnershipManaged})},
		{"unknown ownership", "providers[0].ownership", "unknown ownership other",
			base(ProviderV2{Name: "nan", ConfigPath: "opencode.jsonc", SemanticDigest: "d", Ownership: Ownership("other")})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateV2(tc.doc)
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("expected *ValidationError, got %v", err)
			}
			if verr.Field != tc.field || verr.Reason != tc.reason {
				t.Fatalf("error = %q/%q, want %q/%q", verr.Field, verr.Reason, tc.field, tc.reason)
			}
		})
	}
}

func TestREQ_PROV_002ValidateProvidersCaseCollision(t *testing.T) {
	if !caseInsensitivePaths() {
		t.Skip("platform resolves paths case-sensitively")
	}
	home := t.TempDir()
	nameErr := ValidateV2(providerTestDoc(home,
		ProviderV2{Name: "Nan", ConfigPath: "opencode.jsonc", SemanticDigest: "d1", Ownership: OwnershipManaged},
		ProviderV2{Name: "nan", ConfigPath: "opencode.json", SemanticDigest: "d2", Ownership: OwnershipManaged},
	))
	var verr *ValidationError
	if !errors.As(nameErr, &verr) || verr.Field != "providers[1].name" {
		t.Fatalf("case-colliding provider names not rejected: %v", nameErr)
	}

	pathErr := ValidateV2(providerTestDoc(home,
		ProviderV2{Name: "alpha", ConfigPath: "opencode.jsonc", SemanticDigest: "d1", Ownership: OwnershipManaged},
		ProviderV2{Name: "beta", ConfigPath: "OpenCode.jsonc", SemanticDigest: "d2", Ownership: OwnershipManaged},
	))
	if !errors.As(pathErr, &verr) || verr.Field != "providers[1].config_path" {
		t.Fatalf("case-colliding provider config paths not rejected: %v", pathErr)
	}
}
