package installmeta

import (
	"testing"
)

func TestMCPIdentityDigest(t *testing.T) {
	identity1 := MCPServerIdentity{
		Name:        "cortex",
		Type:        "local",
		Command:     "cortex",
		Args:        []string{"mcp", "--tools=agent"},
		EnvNames:    []string{"DEBUG", "PORT"},
		HeaderNames: []string{"Authorization"},
	}

	digest1, err := MCPIdentityDigest(identity1)
	if err != nil {
		t.Fatalf("MCPIdentityDigest failed: %v", err)
	}
	if !ValidMCPServerDigest(digest1) {
		t.Fatalf("expected valid digest, got: %s", digest1)
	}

	// Permutation of EnvNames and HeaderNames should yield identical digest (canonical sort)
	identity2 := MCPServerIdentity{
		Name:        "cortex",
		Type:        "local",
		Command:     "cortex",
		Args:        []string{"mcp", "--tools=agent"},
		EnvNames:    []string{"PORT", "DEBUG"},
		HeaderNames: []string{"Authorization"},
	}
	digest2, err := MCPIdentityDigest(identity2)
	if err != nil {
		t.Fatalf("MCPIdentityDigest 2 failed: %v", err)
	}
	if digest1 != digest2 {
		t.Errorf("expected deterministic digest across env ordering: %s vs %s", digest1, digest2)
	}

	// Parse digest
	ver, sum, err := ParseMCPServerDigest(digest1)
	if err != nil {
		t.Fatalf("ParseMCPServerDigest failed: %v", err)
	}
	if ver != MCPDigestVersion || sum == "" {
		t.Errorf("unexpected parsed digest: version=%d, sum=%s", ver, sum)
	}
}

func TestMCPServerIdentityFromEntry(t *testing.T) {
	entry := map[string]any{
		"type":    "local",
		"command": []any{"cortex", "mcp", "--tools=agent"},
		"enabled": true,
		"env": map[string]any{
			"PORT": "8080",
		},
	}

	identity, err := MCPServerIdentityFromEntry("cortex", entry)
	if err != nil {
		t.Fatalf("MCPServerIdentityFromEntry failed: %v", err)
	}
	if identity.Name != "cortex" || identity.Command != "cortex" {
		t.Errorf("unexpected extracted identity: %+v", identity)
	}
	if len(identity.Args) != 2 || identity.Args[0] != "mcp" {
		t.Errorf("unexpected extracted args: %+v", identity.Args)
	}
	if len(identity.EnvNames) != 1 || identity.EnvNames[0] != "PORT" {
		t.Errorf("unexpected extracted env names: %+v", identity.EnvNames)
	}

	// Direct digest from entry
	entryDigest, err := MCPServerDigest("cortex", entry)
	if err != nil {
		t.Fatalf("MCPServerDigest failed: %v", err)
	}
	if !ValidMCPServerDigest(entryDigest) {
		t.Errorf("expected valid entry digest: %s", entryDigest)
	}
}
