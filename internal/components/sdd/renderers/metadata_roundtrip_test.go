package renderers_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/canonical"
	. "github.com/lleontor705/cortex-ia/internal/components/sdd/renderers"
)

func TestBundlePreservesMetadataThroughRoleAndHandoffManifest(t *testing.T) {
	sentinel := json.RawMessage(`{"contract":"contract-sentinel","profile":"profile-sentinel","quality":"quality-sentinel","trust":"trust-sentinel","gate":"gate-sentinel","observability":"trace-sentinel"}`)
	workflow := canonical.Workflow()
	resolved := ResolvedWorkflow{Workflow: workflow, Target: "codex", Profile: "portable-sequential", Metadata: sentinel, AllowedAssetKinds: []AssetKind{AssetInstruction, AssetRule, AssetSkill, AssetAgent, AssetSchema, AssetPermission, AssetModel, AssetFixture, AssetCommand, AssetMCP}, AllowedPermissions: []string{"filesystem/read", "filesystem/write", "mcp/cortex", "mcp/forgespec", "process/execute", "tool/read", "tool/search"}}
	bundle, err := Render(context.Background(), NewCodexRenderer(), resolved)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bundle.Metadata), "contract-sentinel") {
		t.Fatalf("bundle lost metadata: %s", bundle.Metadata)
	}
}
