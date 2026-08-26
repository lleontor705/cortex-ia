// Package mcpmanager manages the native "mcp" object of the OpenCode global
// configuration (opencode.json / opencode.jsonc) with add, list, and remove
// operations for cortex-ia-managed MCP presets.
//
// The manager is OpenCode-exclusive by design: it never resolves or mutates
// any other agent's configuration surface. Ownership of one "mcp" entry is
// accredited exclusively by transactional metadata: an OwnershipRecord that
// the installer pipeline persisted after a successful apply. Name matching
// and template equality are never ownership evidence. An entry that merely
// equals a managed preset is unmanaged-equivalent: user-owned, never
// rewritten, and never implicitly removed. Entries under managed names that
// differ from the managed template are user-owned and produce typed
// fail-closed conflicts instead of writes.
package mcpmanager

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/lleontor705/cortex-ia/internal/installmeta"
)

// Preset describes one MCP server entry cortex-ia is able to manage inside
// OpenCode's "mcp" object.
type Preset struct {
	// Name is the OpenCode MCP server key (e.g. "cortex").
	Name string

	// Entry is the canonical managed entry for the server, using OpenCode's
	// native local-server shape: {"type":"local","command":[...],"enabled":true}.
	Entry map[string]any
}

// Command returns the preset's local-server command vector. It reports false
// when the entry is not a local server with a non-empty vector of non-empty
// string parts; callers must fail closed in that case because the preset is
// not compatible with the existing command-based qualification boundary.
func (p Preset) Command() ([]string, bool) {
	if p.Entry["type"] != "local" {
		return nil, false
	}
	raw, isVector := p.Entry["command"].([]any)
	if !isVector || len(raw) == 0 {
		return nil, false
	}
	command := make([]string, 0, len(raw))
	for _, part := range raw {
		str, ok := part.(string)
		if !ok || str == "" {
			return nil, false
		}
		command = append(command, str)
	}
	return command, true
}

var managedPresets = []Preset{
	{
		Name: "cortex",
		Entry: map[string]any{
			"type":    "local",
			"command": []any{"cortex", "mcp", "--tools=agent"},
			"enabled": true,
		},
	},
	{
		Name: "context7",
		Entry: map[string]any{
			"type":    "local",
			"command": []any{"npx", "-y", "@upstash/context7-mcp"},
			"enabled": true,
		},
	},
}

// retiredPresets exist only so sync can safely remove entries accredited by
// an older cortex-ia installation. They are never selectable or addable.
var retiredPresets = []Preset{
	{
		Name: "forgespec",
		Entry: map[string]any{
			"type":    "local",
			"command": []any{"npx", "-y", "forgespec-mcp"},
			"enabled": true,
		},
	},
}

// Presets returns the managed presets sorted by name. The returned presets
// are deep copies: mutating any returned entry or nested value can never
// mutate the catalog.
func Presets() []Preset {
	presets := make([]Preset, len(managedPresets))
	for i, preset := range managedPresets {
		presets[i] = Preset{Name: preset.Name, Entry: deepCopyEntry(preset.Entry)}
	}
	sort.Slice(presets, func(i, j int) bool { return presets[i].Name < presets[j].Name })
	return presets
}

// Lookup returns a deep copy of the managed preset with the given server
// name. Mutating the returned entry never affects the catalog.
func Lookup(name string) (Preset, bool) {
	for _, preset := range managedPresets {
		if preset.Name == name {
			return Preset{Name: preset.Name, Entry: deepCopyEntry(preset.Entry)}, true
		}
	}
	return Preset{}, false
}

// RetiredPresets returns removal-only legacy templates. Callers must never
// expose them as install choices.
func RetiredPresets() []Preset {
	presets := make([]Preset, len(retiredPresets))
	for i, preset := range retiredPresets {
		presets[i] = Preset{Name: preset.Name, Entry: deepCopyEntry(preset.Entry)}
	}
	return presets
}

func lookupRetired(name string) (Preset, bool) {
	for _, preset := range retiredPresets {
		if preset.Name == name {
			return Preset{Name: preset.Name, Entry: deepCopyEntry(preset.Entry)}, true
		}
	}
	return Preset{}, false
}

// IsRetired reports whether name is recognized only for accredited removal.
func IsRetired(name string) bool {
	_, ok := lookupRetired(name)
	return ok
}

// DefaultSelection returns the default preset selection for fresh installs:
// Cortex is selected; Context7 is optional and starts unselected. Task
// control is built into the cortex-ia CLI and is not an MCP preset.
// A fresh map is returned on every call.
func DefaultSelection() map[string]bool {
	return map[string]bool{
		"cortex":   true,
		"context7": false,
	}
}

// SemanticDigest returns the versioned, secret-free semantic digest of the
// named MCP entry, computed by the shared internal/installmeta leaf so state
// v2 metadata and MCP ownership records always agree on exactly one canonical
// encoding. The digest covers identity only: server name, type, command
// vector, and env/header variable NAMES. Env and header values, URLs, and
// runtime flags such as "enabled" are never hashed. Exact template equality
// remains semanticEqual's job; the digest is persistent identity evidence.
func SemanticDigest(name string, entry map[string]any) (string, error) {
	return installmeta.MCPServerDigest(name, entry)
}

// semanticEqual reports whether an observed configuration entry carries
// exactly the managed preset's value. Comparison happens on canonical
// encodings so member order and formatting never produce false conflicts.
// Equality is not ownership: only an OwnershipRecord accredits ownership.
func semanticEqual(observed map[string]any, preset map[string]any) (bool, error) {
	observedJSON, err := json.Marshal(observed)
	if err != nil {
		return false, fmt.Errorf("encode observed MCP entry: %w", err)
	}
	presetJSON, err := json.Marshal(preset)
	if err != nil {
		return false, fmt.Errorf("encode managed MCP entry: %w", err)
	}
	return string(observedJSON) == string(presetJSON), nil
}

// deepCopyEntry clones a JSON-shaped entry so callers of Presets and Lookup
// receive values they can never use to mutate the catalog.
func deepCopyEntry(entry map[string]any) map[string]any {
	if entry == nil {
		return nil
	}
	copied := make(map[string]any, len(entry))
	for key, value := range entry {
		copied[key] = deepCopyValue(value)
	}
	return copied
}

func deepCopyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return deepCopyEntry(typed)
	case []any:
		copied := make([]any, len(typed))
		for i, item := range typed {
			copied[i] = deepCopyValue(item)
		}
		return copied
	default:
		return value
	}
}
