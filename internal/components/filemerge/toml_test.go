package filemerge

import (
	"strings"
	"testing"
)

func TestUpsertMCPServerTOML_NewServer(t *testing.T) {
	content := ""
	result := UpsertMCPServerTOML(content, "cortex", "cortex", []string{"mcp"})

	if !strings.Contains(result, "[mcp_servers.cortex]") {
		t.Error("expected [mcp_servers.cortex] header")
	}
	if !strings.Contains(result, `command = "cortex"`) {
		t.Error("expected command = cortex")
	}
	if !strings.Contains(result, `args = ["mcp"]`) {
		t.Error("expected args = [mcp]")
	}
}

func TestUpsertMCPServerTOML_ReplaceExisting(t *testing.T) {
	content := `[mcp_servers.cortex]
command = "old-cortex"
args = ["old"]

[mcp_servers.other]
command = "other"
`
	result := UpsertMCPServerTOML(content, "cortex", "cortex", []string{"mcp", "--tools=agent"})

	if strings.Contains(result, "old-cortex") {
		t.Error("old cortex config should be removed")
	}
	if !strings.Contains(result, "[mcp_servers.other]") {
		t.Error("other server should be preserved")
	}
	if !strings.Contains(result, `command = "cortex"`) {
		t.Error("new cortex config should be present")
	}
	if !strings.Contains(result, `"--tools=agent"`) {
		t.Error("new args should be present")
	}
}

func TestUpsertMCPServerTOML_MultipleServers(t *testing.T) {
	content := ""
	content = UpsertMCPServerTOML(content, "cortex", "cortex", []string{"mcp"})
	content = UpsertMCPServerTOML(content, "forgespec", "npx", []string{"-y", "forgespec-mcp"})
	content = UpsertMCPServerTOML(content, "agent-mailbox", "npx", []string{"-y", "agent-mailbox-mcp"})

	for _, name := range []string{"cortex", "forgespec", "agent-mailbox"} {
		if !strings.Contains(content, "[mcp_servers."+name+"]") {
			t.Errorf("expected [mcp_servers.%s]", name)
		}
	}
}

func TestUpsertTopLevelTOMLString(t *testing.T) {
	content := `[mcp_servers.cortex]
command = "cortex"
`
	result := UpsertTopLevelTOMLString(content, "model", "claude-sonnet-4-20250514")

	if !strings.Contains(result, `model = "claude-sonnet-4-20250514"`) {
		t.Error("expected model key to be inserted")
	}

	idx := strings.Index(result, "model")
	sectionIdx := strings.Index(result, "[mcp_servers")
	if idx > sectionIdx {
		t.Error("top-level key should appear before section headers")
	}
}

func TestUpsertTOMLBlock_WindowsLineEndings(t *testing.T) {
	content := "[mcp_servers.old]\r\ncommand = \"old\"\r\n"
	result := UpsertTOMLBlock(content, "mcp_servers.old", "command = \"new\"\n")

	if strings.Contains(result, "old") && strings.Contains(result, `command = "old"`) {
		t.Error("old block should be replaced")
	}
	if !strings.Contains(result, `command = "new"`) {
		t.Error("new block should be present")
	}
}
