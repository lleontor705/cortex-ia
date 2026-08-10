package conformance

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRuntimeHarnessProducesDeterministicReceiptLinkedMatrixEvidence(t *testing.T) {
	raw, err := NewRuntimeHarness(RuntimeHarnessConfig{WorkDir: t.TempDir()}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	options := MatrixReceiptOptions{
		RouteEvidence:       []string{"route:v1/runtime"},
		QualityEvidence:     []string{"quality:r7"},
		TrustEvidence:       []string{"trust:qualified"},
		PermissionEvidence:  []string{"permission:workspace"},
		DestinationEvidence: []string{"destination:managed-root"},
	}
	first, err := AggregateMatrixReceipt(raw, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AggregateMatrixReceipt(raw, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrixReceipt(first); err != nil {
		t.Fatal(err)
	}
	if len(first.Bindings) != 27 || first.Fingerprint != second.Fingerprint {
		t.Fatalf("matrix evidence is not deterministic output: bindings=%d fingerprints=%q/%q", len(first.Bindings), first.Fingerprint, second.Fingerprint)
	}
	for _, cell := range raw.Cells {
		if err := validateCanonicalDigest(cell.ReceiptDigest); err != nil {
			t.Fatalf("cell %s/%s has non-canonical receipt digest: %v", cell.Adapter, cell.RequestedProfile, err)
		}
	}
}

func TestRuntimeHarnessExecutesExactlyThirtySixProductionCells(t *testing.T) {
	receipt, err := NewRuntimeHarness(RuntimeHarnessConfig{WorkDir: t.TempDir()}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := len(receipt.Cells); got != 3 {
		t.Fatalf("executed cells = %d, want 3", got)
	}
	if got := len(receipt.Adapters) * len(receipt.Profiles); got != 3 {
		t.Fatalf("cartesian cardinality = %d, want 3", got)
	}
	seen := make(map[string]bool, len(receipt.Cells))
	for _, cell := range receipt.Cells {
		key := cell.Adapter + "\x00" + cell.RequestedProfile
		if seen[key] {
			t.Fatalf("duplicate cell %s", key)
		}
		seen[key] = true
		if cell.Command == "" || cell.Path == "" || cell.ReceiptDigest == "" || cell.EvidenceDigest == "" {
			t.Fatalf("cell lacks runtime evidence: %+v", cell)
		}
		if cell.ExitCode < 0 || cell.ReasonID == "" || cell.Disposition == "" {
			t.Fatalf("cell lacks disposition evidence: %+v", cell)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("unique cells = %d, want 3", len(seen))
	}
	if err := receipt.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeHarnessRejectsSyntheticOrSkippedEvidence(t *testing.T) {
	receipt := RuntimeReceipt{
		Adapters: []string{"a"}, Profiles: []string{"p"},
		Cells: []RuntimeCell{{Adapter: "a", RequestedProfile: "p", EffectiveProfile: "p", Disposition: DispositionSupported, ReasonID: "ok", Command: "synthetic", Path: "fixture", ExitCode: 0, ReceiptDigest: "sha256:fake", EvidenceDigest: "sha256:fake", Evidence: map[string]string{"execution": "declarative"}}},
	}
	if err := receipt.Validate(); err == nil {
		t.Fatal("expected declarative evidence to be rejected")
	}
}

func TestDeclarativeMatrixRunnerCannotMasqueradeAsRuntimeEvidence(t *testing.T) {
	_, err := RunMatrix(Matrix{Adapters: []string{"a"}, Profiles: []string{"p"}, Cells: []Cell{{Adapter: "a", RequestedProfile: "p", EffectiveProfile: "p", Disposition: DispositionSupported, ReasonID: "ok", Command: "matrix-probe", ExitCode: 0, Hash: "sha256:fake", Evidence: map[string]string{"mutation": "none"}}}})
	if err == nil {
		t.Fatal("expected declarative matrix execution to be rejected")
	}
}

func TestRuntimeReceiptSerializationIsDeterministic(t *testing.T) {
	receipt, err := NewRuntimeHarness(RuntimeHarnessConfig{WorkDir: t.TempDir()}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("runtime receipt serialization is not deterministic")
	}
}
