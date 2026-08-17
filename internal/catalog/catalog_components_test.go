package catalog

import (
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// TestCatalog_DisableClassMatrix verifies the explicit disable-class
// descriptors and fail-closed classification defaults required by spec
// REQ-SEL-001 (SC-SEL-E, SC-SEL-F) and design decision D4: Cortex/ForgeSpec
// are protected-authority, SDD is protected-workflow, transitive dependencies
// of retained selections are protected-required, only explicit catalog
// entries are optional, and unclassified components default to protected.
func TestCatalog_DisableClassMatrix(t *testing.T) {
	// Every catalog entry must carry an explicit descriptor; none may rely
	// on the fail-closed unclassified default.
	for _, c := range AllComponents() {
		if c.Disable == ProtectedUnclassified {
			t.Errorf("component %q has no explicit disable-class descriptor", c.ID)
		}
	}

	// Static matrix: with no retained selection every component keeps its
	// explicit descriptor.
	base := DisableClasses(nil)
	static := []struct {
		id   model.ComponentID
		want DisableClass
	}{
		{model.ComponentCortex, ProtectedAuthority},
		{model.ComponentForgeSpec, ProtectedAuthority},
		{model.ComponentSDD, ProtectedWorkflow},
		{model.ComponentContext7, Optional},
		{model.ComponentConventions, Optional},
		{model.ComponentSkills, Optional},
	}
	for _, tc := range static {
		if got := base[tc.id]; got != tc.want {
			t.Errorf("DisableClasses(nil)[%q] = %v, want %v", tc.id, got, tc.want)
		}
	}

	// Only the Optional class is disableable. Unclassified components
	// default to protected (fail-closed), even when they are known IDs
	// without a catalog descriptor.
	if Optional.Protected() {
		t.Error("Optional must not be protected")
	}
	for _, class := range []DisableClass{ProtectedUnclassified, ProtectedAuthority, ProtectedWorkflow, ProtectedRequired} {
		if !class.Protected() {
			t.Errorf("DisableClass %v must be protected", class)
		}
	}
	for _, id := range []model.ComponentID{
		model.ComponentMailbox,
		model.ComponentPermissions,
		model.ComponentTheme,
		model.ComponentID("does-not-exist"),
	} {
		if got := base[id]; !got.Protected() {
			t.Errorf("unclassified component %q classified %v, want protected", id, got)
		}
	}

	// Unclassified IDs never leak into the classification map.
	if got := len(base); got != len(AllComponents()) {
		t.Errorf("len(DisableClasses(nil)) = %d, want %d", got, len(AllComponents()))
	}

	// Transitive dependencies of retained selections resolve as
	// ProtectedRequired; the retained components themselves keep their
	// explicit descriptor.
	sdd := DisableClasses([]model.ComponentID{model.ComponentSDD})
	for _, dep := range []model.ComponentID{model.ComponentCortex, model.ComponentForgeSpec} {
		if got := sdd[dep]; got != ProtectedRequired {
			t.Errorf("DisableClasses([sdd])[%q] = %v, want %v", dep, got, ProtectedRequired)
		}
	}
	if got := sdd[model.ComponentSDD]; got != ProtectedWorkflow {
		t.Errorf("DisableClasses([sdd])[sdd] = %v, want %v", got, ProtectedWorkflow)
	}
	if got := sdd[model.ComponentSkills]; got != Optional {
		t.Errorf("DisableClasses([sdd])[skills] = %v, want %v", got, Optional)
	}

	conv := DisableClasses([]model.ComponentID{model.ComponentConventions})
	if got := conv[model.ComponentCortex]; got != ProtectedRequired {
		t.Errorf("DisableClasses([conventions])[cortex] = %v, want %v", got, ProtectedRequired)
	}
	if got := conv[model.ComponentConventions]; got != Optional {
		t.Errorf("DisableClasses([conventions])[conventions] = %v, want %v", got, Optional)
	}

	// A directly retained component is not itself a required dependency.
	direct := DisableClasses([]model.ComponentID{model.ComponentCortex})
	if got := direct[model.ComponentCortex]; got != ProtectedAuthority {
		t.Errorf("DisableClasses([cortex])[cortex] = %v, want %v", got, ProtectedAuthority)
	}

	// AC-SEL-2: deterministic across repeated calls, and repeated retained
	// entries normalize to a single occurrence.
	retained := []model.ComponentID{model.ComponentSDD, model.ComponentConventions}
	want := DisableClasses(retained)
	for i := 0; i < 3; i++ {
		if got := DisableClasses(retained); !reflect.DeepEqual(got, want) {
			t.Fatalf("DisableClasses repeated call %d = %v, want %v", i, got, want)
		}
	}
	dup := DisableClasses([]model.ComponentID{model.ComponentSDD, model.ComponentSDD, model.ComponentConventions, model.ComponentConventions})
	if !reflect.DeepEqual(dup, want) {
		t.Errorf("DisableClasses with duplicate retained entries = %v, want %v", dup, want)
	}

	// SC-SEL-F: each class identifies its protection category distinctly.
	seen := make(map[string]DisableClass)
	for _, class := range []DisableClass{ProtectedUnclassified, Optional, ProtectedAuthority, ProtectedWorkflow, ProtectedRequired} {
		name := class.String()
		if name == "" {
			t.Errorf("DisableClass(%d).String() is empty", uint8(class))
		}
		if prev, ok := seen[name]; ok {
			t.Errorf("DisableClass(%d).String() = %q duplicates class %d", uint8(class), name, uint8(prev))
		}
		seen[name] = class
	}
}
