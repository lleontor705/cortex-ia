package agents

import (
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
)

func TestCapabilityProviderExposesFactsAndProbePortOnly(t *testing.T) {
	typeOfProvider := reflect.TypeOf((*CapabilityProvider)(nil)).Elem()
	wantMethods := []string{"CapabilityFacts", "CapabilityProber"}

	if typeOfProvider.NumMethod() != len(wantMethods) {
		t.Fatalf("CapabilityProvider methods = %d, want %d", typeOfProvider.NumMethod(), len(wantMethods))
	}
	for index, want := range wantMethods {
		method := typeOfProvider.Method(index)
		if method.Name != want {
			t.Fatalf("method[%d] = %q, want %q", index, method.Name, want)
		}
	}

	probeMethod, _ := typeOfProvider.MethodByName("CapabilityProber")
	wantProber := reflect.TypeOf((*capability.Prober)(nil)).Elem()
	if probeMethod.Type.NumOut() != 1 || probeMethod.Type.Out(0) != wantProber {
		t.Fatalf("CapabilityProber return = %v, want %v", probeMethod.Type, wantProber)
	}
}
