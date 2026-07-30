package conformance

import (
	"strings"
	"testing"
)

func TestRuntimeCellReceiptDigestRequiresCanonicalSha256Boundary(t *testing.T) {
	valid := digestText("immutable receipt bytes")
	for _, tc := range []struct {
		name   string
		digest string
		wantOK bool
	}{
		{name: "canonical", digest: valid, wantOK: true},
		{name: "bare", digest: strings.TrimPrefix(valid, "sha256:"), wantOK: false},
		{name: "empty", digest: "", wantOK: false},
		{name: "double prefix", digest: "sha256:" + valid, wantOK: false},
		{name: "uppercase", digest: "sha256:" + strings.ToUpper(strings.TrimPrefix(valid, "sha256:")), wantOK: false},
		{name: "nonhex", digest: "sha256:" + strings.Repeat("g", 64), wantOK: false},
		{name: "short", digest: "sha256:deadbeef", wantOK: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := RuntimeReceipt{
				Adapters: []string{"a"}, Profiles: []string{"p"},
				Cells: []RuntimeCell{{
					Adapter: "a", RequestedProfile: "p", EffectiveProfile: "p",
					Disposition: DispositionSupported, ReasonID: "observed/supported",
					Command: "production", Path: "/managed/a", ExitCode: 0,
					ReceiptDigest: tc.digest, EvidenceDigest: digestJSON(map[string]string{"execution": "production"}),
					Evidence: map[string]string{"execution": "production"},
				}},
			}
			raw = sealRuntimeReceipt(raw)
			err := raw.Validate()
			if tc.wantOK && err != nil {
				t.Fatalf("canonical digest rejected: %v", err)
			}
			if !tc.wantOK && err == nil {
				t.Fatalf("digest %q was accepted", tc.digest)
			}
		})
	}
}
