package conformance

import "testing"

func TestAggregateMatrixReceiptEmitsExactly324LinkedBindings(t *testing.T) {
	raw := completeRuntimeReceipt(t)
	receipt, err := AggregateMatrixReceipt(raw, MatrixReceiptOptions{
		RouteEvidence:       []string{"route:v1/runtime", "fallback:v1/runtime"},
		QualityEvidence:     []string{"quality:r7"},
		TrustEvidence:       []string{"trust:qualified"},
		PermissionEvidence:  []string{"permission:workspace"},
		DestinationEvidence: []string{"destination:managed-root"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateMatrixReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	if got := len(receipt.Bindings); got != 324 {
		t.Fatalf("bindings = %d, want 324", got)
	}
	for _, binding := range receipt.Bindings {
		if binding.CellExecutionHash == "" || binding.ReceiptHash == "" || binding.EvidenceHash == "" {
			t.Fatalf("detached binding: %+v", binding)
		}
		if len(binding.RouteEvidence) == 0 || len(binding.ProfileEvidence) == 0 || len(binding.QualityEvidence) == 0 || len(binding.TrustEvidence) == 0 || len(binding.PermissionEvidence) == 0 || len(binding.DestinationEvidence) == 0 {
			t.Fatalf("incomplete binding evidence: %+v", binding)
		}
	}
}

func TestAggregateMatrixReceiptBlocksSyntheticDetachedDuplicateAndMismatchedRecords(t *testing.T) {
	raw := completeRuntimeReceipt(t)
	options := MatrixReceiptOptions{RouteEvidence: []string{"route:v1/runtime"}, QualityEvidence: []string{"quality:r7"}, TrustEvidence: []string{"trust"}, PermissionEvidence: []string{"permission"}, DestinationEvidence: []string{"destination"}}
	for _, test := range []struct {
		name   string
		mutate func(*RuntimeReceipt)
	}{
		{"synthetic", func(r *RuntimeReceipt) { r.Cells[0].Evidence["execution"] = "synthetic" }},
		{"detached", func(r *RuntimeReceipt) { r.Cells[0].ReceiptDigest = "sha256:detached" }},
		{"duplicate", func(r *RuntimeReceipt) { r.Cells[1] = r.Cells[0] }},
		{"mismatched", func(r *RuntimeReceipt) { r.Cells[0].EvidenceDigest = "sha256:mismatch" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneRuntimeReceipt(raw)
			test.mutate(&candidate)
			if _, err := AggregateMatrixReceipt(candidate, options); err == nil {
				t.Fatal("expected first-defect rejection")
			}
		})
	}
}

func TestAggregateMatrixReceiptKeepsBlockedCellsPreMutationAccountability(t *testing.T) {
	raw := completeRuntimeReceipt(t)
	blocked := &raw.Cells[0]
	blocked.Disposition = DispositionBlocked
	blocked.ExitCode = 1
	blocked.Evidence["mutation"] = "none"
	blocked.Evidence["pre_mutation"] = "true"
	blocked.EvidenceDigest = digestJSON(blocked.Evidence)
	raw = sealRuntimeReceipt(raw)
	receipt, err := AggregateMatrixReceipt(raw, MatrixReceiptOptions{RouteEvidence: []string{"route:v1/runtime"}, QualityEvidence: []string{"quality:r7"}, TrustEvidence: []string{"trust"}, PermissionEvidence: []string{"permission"}, DestinationEvidence: []string{"destination"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range receipt.Bindings[:9] {
		if binding.Accountability != AccountabilityPreMutation || !binding.PreMutation || binding.ReceiptHash != blocked.ReceiptDigest {
			t.Fatalf("blocked binding lost observed receipt accountability: %+v", binding)
		}
	}
}

func TestAggregateMatrixReceiptFingerprintIsDeterministic(t *testing.T) {
	raw := completeRuntimeReceipt(t)
	options := MatrixReceiptOptions{RouteEvidence: []string{"route:v1/runtime"}, QualityEvidence: []string{"quality:r7"}, TrustEvidence: []string{"trust"}, PermissionEvidence: []string{"permission"}, DestinationEvidence: []string{"destination"}}
	first, err := AggregateMatrixReceipt(raw, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AggregateMatrixReceipt(raw, options)
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint == "" || first.Fingerprint != second.Fingerprint {
		t.Fatalf("fingerprints differ: %q != %q", first.Fingerprint, second.Fingerprint)
	}
}

func completeRuntimeReceipt(t *testing.T) RuntimeReceipt {
	t.Helper()
	adapters := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}
	profiles := []string{"p1", "p2", "p3"}
	raw := RuntimeReceipt{Adapters: adapters, Profiles: profiles, Cells: make([]RuntimeCell, 0, 36)}
	for _, adapter := range adapters {
		for _, profile := range profiles {
			receiptBytes := "immutable receipt bytes:" + adapter + ":" + profile
			cell := RuntimeCell{Adapter: adapter, RequestedProfile: profile, EffectiveProfile: profile, Disposition: DispositionSupported, ReasonID: "observed/supported", Command: "production", Path: "/managed/" + adapter, ExitCode: 0, ReceiptDigest: digestText(receiptBytes), Evidence: map[string]string{"execution": "production", "mutation": "managed-only", "pre_mutation": "false"}}
			cell.EvidenceDigest = digestJSON(cell.Evidence)
			raw.Cells = append(raw.Cells, cell)
		}
	}
	return sealRuntimeReceipt(raw)
}
