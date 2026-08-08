package filemerge

import (
	"bytes"
	"strings"
	"testing"
)

func TestPlanMCPServerTOMLRemoval(t *testing.T) {
	request := TOMLRegionRequest{
		TablePath:       []string{"mcp_servers", "cortex"},
		ExpectedCommand: "cortex",
		ExpectedArgs:    []string{"mcp", "--tools=agent"},
	}

	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "removes one quoted table and preserves CRLF siblings",
			content: "# global\r\n[mcp_servers.\"cortex\"] # managed\r\ncommand = \"cortex\"\r\nargs = [\"mcp\", \"--tools=agent\"]\r\n# managed comment\r\n[mcp_servers.other]\r\ncommand = \"other\"\r\n",
			want:    "# global\r\n[mcp_servers.other]\r\ncommand = \"other\"\r\n",
		},
		{
			name:    "refuses customized semantics without changing bytes",
			content: "[mcp_servers.cortex]\ncommand = \"custom\"\nargs = [\"mcp\", \"--tools=agent\"]\n",
			wantErr: true,
		},
		{
			name:    "refuses descendant ambiguity without changing bytes",
			content: "[mcp_servers.cortex]\ncommand = \"cortex\"\nargs = [\"mcp\", \"--tools=agent\"]\n[mcp_servers.cortex.extra]\nenabled = true\n",
			wantErr: true,
		},
		{
			name:    "refuses duplicate table without changing bytes",
			content: "[mcp_servers.cortex]\ncommand = \"cortex\"\nargs = [\"mcp\", \"--tools=agent\"]\n[mcp_servers.cortex]\ncommand = \"cortex\"\nargs = [\"mcp\", \"--tools=agent\"]\n",
			wantErr: true,
		},
		{
			name:    "refuses multiline values without changing bytes",
			content: "[mcp_servers.cortex]\ncommand = \"cortex\"\nargs = [\n  \"mcp\",\n  \"--tools=agent\",\n]\n",
			wantErr: true,
		},
		{
			name:    "refuses invalid TOML without changing bytes",
			content: "[mcp_servers.cortex\ncommand = \"cortex\"\nargs = [\"mcp\", \"--tools=agent\"]\n",
			wantErr: true,
		},
		{
			name:    "refuses overlapping dotted assignment without changing bytes",
			content: "mcp_servers.cortex.command = \"cortex\"\n[mcp_servers.cortex]\ncommand = \"cortex\"\nargs = [\"mcp\", \"--tools=agent\"]\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := PlanTOMLRegionRemoval([]byte(tt.content), request)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PlanTOMLRegionRemoval() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if !bytes.Equal(plan.After, []byte(tt.content)) {
					t.Fatalf("refusal changed content:\n got %q\nwant %q", plan.After, tt.content)
				}
				return
			}
			if got := string(plan.After); got != tt.want {
				t.Fatalf("planned content = %q, want %q", got, tt.want)
			}
			if plan.SpanStart >= plan.SpanEnd {
				t.Fatalf("invalid removal span [%d:%d]", plan.SpanStart, plan.SpanEnd)
			}
		})
	}
}

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
	result := UpsertTopLevelTOMLString(content, "model", "provider-test/model-test")

	if !strings.Contains(result, `model = "provider-test/model-test"`) {
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
