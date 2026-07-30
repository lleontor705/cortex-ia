package cursor_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/agents/cursor"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
)

var _ agents.CapabilityProvider = cursor.NewAdapter()

func TestCursorCapabilityFactsSelectProfilesDeterministically(t *testing.T) {
	now := time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC)
	documented := cursor.NewAdapter().CapabilityFacts()

	flatFromCatalog := sdd.SelectWorkflowProfile(sdd.ProfileSelectionInput{
		Now:                now,
		Facts:              documented,
		NativeCapabilities: []capability.CapabilityID{"delegation/parallel"},
	})
	if flatFromCatalog.Profile != sdd.ProfilePortableFlat {
		t.Fatalf("catalog-qualified profile = %q, want %q", flatFromCatalog.Profile, sdd.ProfilePortableFlat)
	}

	qualified := make([]capability.CapabilityFact, len(documented))
	copy(qualified, documented)
	for i := range qualified {
		qualified[i].EvidenceClass = capability.EvidenceExecutableProbe
		qualified[i].EvidenceRef = "sha256:cursor-installed-version"
		qualified[i].ObservedAt = now
		qualified[i].Enforcement = capability.EnforcementRuntime
	}

	flat := sdd.SelectWorkflowProfile(sdd.ProfileSelectionInput{
		Now:                now,
		Facts:              qualified,
		NativeCapabilities: []capability.CapabilityID{"delegation/parallel"},
	})
	if flat.Profile != sdd.ProfilePortableFlat {
		t.Fatalf("profile without experimental opt-in = %q, want %q", flat.Profile, sdd.ProfilePortableFlat)
	}

	nativeInput := sdd.ProfileSelectionInput{
		Now:                now,
		Facts:              qualified,
		NativeCapabilities: []capability.CapabilityID{"delegation/parallel"},
		ExperimentalOptIns: []capability.CapabilityID{"delegation/parallel"},
	}
	first := sdd.SelectWorkflowProfile(nativeInput)
	second := sdd.SelectWorkflowProfile(nativeInput)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("native profile selection is not deterministic: %#v != %#v", first, second)
	}
	if first.Profile != sdd.ProfileNativeAdvanced {
		t.Fatalf("profile with experimental opt-in = %q, want %q", first.Profile, sdd.ProfileNativeAdvanced)
	}
}

func TestCursorCapabilitySurfaceHasNoRuntimeControlAuthority(t *testing.T) {
	proberType := reflect.TypeOf(cursor.NewAdapter().CapabilityProber())
	if proberType.NumMethod() != 1 || proberType.Method(0).Name != "Probe" {
		t.Fatalf("capability prober methods = %v, want Probe only", exportedMethods(proberType))
	}
}

func exportedMethods(value reflect.Type) []string {
	methods := make([]string, value.NumMethod())
	for i := range methods {
		methods[i] = value.Method(i).Name
	}
	return methods
}
