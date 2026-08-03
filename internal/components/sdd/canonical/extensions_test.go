package canonical

import (
	"reflect"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestWorkflowHasOnlyProviderNeutralUnsupportedA2AExtension(t *testing.T) {
	workflow := Workflow()
	if len(workflow.Extensions) != 1 {
		t.Fatalf("Extensions = %+v, want exactly one", workflow.Extensions)
	}
	want := ir.ExtensionContract{
		ID:                "extension/remote-agent-a2a",
		SchemaVersion:     ir.ExtensionSchema.Current,
		DefaultResolution: ir.ResolutionUnsupported,
	}
	if !reflect.DeepEqual(workflow.Extensions[0], want) {
		t.Fatalf("Extensions[0] = %+v, want %+v", workflow.Extensions[0], want)
	}
}

func TestWorkflowContainsNoLiveCoordinationServiceToolOrTrustClass(t *testing.T) {
	workflow := Workflow()
	if got, want := len(workflow.Services), 2; got != want {
		t.Fatalf("Services = %+v, want %d canonical authorities", workflow.Services, want)
	}
	if got, want := len(workflow.Tools), 3; got != want {
		t.Fatalf("Tools = %+v, want %d canonical tools", workflow.Tools, want)
	}
	for _, class := range workflow.Context.Classes {
		if class == ir.TrustPeerMessage {
			t.Fatalf("Context.Classes contains retired peer-message trust class: %+v", workflow.Context.Classes)
		}
	}
}
