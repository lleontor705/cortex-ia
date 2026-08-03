package canonical

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
)

func TestFactoryReturnsDeterministicCanonicalWorkflowAndRenderer(t *testing.T) {
	factory := NewFactory()
	input := FactoryInput{Target: "codex", RuntimeVersion: ir.MustParseVersion("1.0.0")}

	first, err := factory.Create(input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := factory.Create(input)
	if err != nil {
		t.Fatalf("Create() second call error = %v", err)
	}
	if !reflect.DeepEqual(first.Workflow, second.Workflow) || first.Renderer.Target() != second.Renderer.Target() {
		t.Fatalf("Create() is not deterministic: first=%+v second=%+v", first, second)
	}
	if first.Workflow.SchemaVersion != ir.WorkflowSchema.Current || first.Workflow.Version == (ir.Version{}) {
		t.Fatalf("workflow versions = schema %s workflow %s", first.Workflow.SchemaVersion, first.Workflow.Version)
	}
	if first.Workflow.ID != "workflow/sdd" {
		t.Fatalf("workflow ID = %q", first.Workflow.ID)
	}
	if first.Renderer.Target() != renderers.TargetID("codex") {
		t.Fatalf("renderer target = %q", first.Renderer.Target())
	}
	if len(first.AllowedAssetKinds) == 0 || len(first.AllowedPermissions) == 0 {
		t.Fatal("factory omitted renderer/profile bounds")
	}
	for _, role := range first.Workflow.Roles {
		if err := ir.ValidateSemanticID(role.ID); err != nil {
			t.Fatalf("role semantic ID %q: %v", role.ID, err)
		}
	}
}

func TestFactorySelectsEverySupportedProductionRenderer(t *testing.T) {
	factory := NewFactory()
	for _, target := range []renderers.TargetID{
		"claude", "codex", "opencode", "vscode",
	} {
		t.Run(string(target), func(t *testing.T) {
			product, err := factory.Create(FactoryInput{Target: target, RuntimeVersion: ir.MustParseVersion("1.0.0")})
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if product.Renderer.Target() != target {
				t.Fatalf("renderer target = %q, want %q", product.Renderer.Target(), target)
			}
		})
	}
}

func TestFactoryRejectsUnsupportedTargetAndVersionBeforeRendering(t *testing.T) {
	factory := NewFactory()
	for _, test := range []struct {
		name  string
		input FactoryInput
		want  string
	}{
		{name: "target", input: FactoryInput{Target: "unknown", RuntimeVersion: ir.MustParseVersion("1.0.0")}, want: "unsupported workflow target"},
		{name: "major", input: FactoryInput{Target: "codex", RuntimeVersion: ir.MustParseVersion("2.0.0")}, want: "unsupported codex renderer version"},
		{name: "missing", input: FactoryInput{Target: "codex"}, want: "runtime version is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := factory.Create(test.input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Create() error = %v, want containing %q", err, test.want)
			}
		})
	}
}
