// Package uninstall reverses a previous cortex-ia install on a per-agent,
// per-component basis. It runs a Prepare→Apply pipeline of operations whose
// failures roll back from a fresh pre-uninstall snapshot.
package uninstall

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// opType describes the kind of file mutation an operation performs.
type opType int

const (
	opRewriteFile   opType = iota // strip a marker section from a file
	opRemoveFile                  // delete a single file
	opRemoveTree                  // recursively delete a directory
	opRemoveIfEmpty               // remove a directory only if it is empty
	opRemoveJSONKey               // delete a top-level key from a JSON file
)

// operation is a single planned mutation. SectionID is used by opRewriteFile;
// JSONPath is used by opRemoveJSONKey.
type operation struct {
	typeID    opType
	path      string
	sectionID string
	jsonPath  []string
	component model.ComponentID
	agent     model.AgentID
}

// markersByComponent enumerates the cortex-ia: marker section IDs each
// component injects into system-prompt-style files.
//
// Auditable source-of-truth: derived from the actual injectors in
// internal/components/{persona,conventions,sdd,permissions}/. Keep this map
// in lock-step with the InjectMarkdownSection calls in those packages — the
// uninstaller calls these IDs back through the same primitive (with an empty
// body) to strip them.
var markersByComponent = map[model.ComponentID][]string{
	model.ComponentPersona:     {"cortex-persona"},
	model.ComponentConventions: {"cortex-protocol"},
	model.ComponentPermissions: {"cortex-permissions"},
	model.ComponentSDD:         {"sdd-orchestrator"},
}

// mcpServerNamesByComponent lists the MCP server names each MCP-style component
// registers. They are used to:
//   - delete <agent>.json under StrategySeparateMCPFiles
//   - remove the matching mcpServers/mcp/servers key under merge strategies
var mcpServerNamesByComponent = map[model.ComponentID]string{
	model.ComponentCortex:    "cortex",
	model.ComponentForgeSpec: "forgespec",
	model.ComponentMailbox:   "agent-mailbox",
	model.ComponentContext7:  "context7",
}

// componentOperations builds the per-(agent, component) operation list.
// Caller is responsible for ordering and execution; this function only plans.
func componentOperations(homeDir string, adapter agents.Adapter, component model.ComponentID) []operation {
	ops := make([]operation, 0, 4)
	agent := adapter.Agent()

	// Marker-based components: rewrite the system prompt file with the
	// marker section stripped (no-op if file or marker missing).
	if markers, ok := markersByComponent[component]; ok {
		if file := adapter.SystemPromptFile(homeDir); file != "" {
			for _, id := range markers {
				ops = append(ops, operation{
					typeID:    opRewriteFile,
					path:      file,
					sectionID: id,
					component: component,
					agent:     agent,
				})
			}
		}
	}

	// MCP-based components: drop the per-server config.
	if name, ok := mcpServerNamesByComponent[component]; ok && adapter.SupportsMCP() {
		switch adapter.MCPStrategy() {
		case model.StrategySeparateMCPFiles:
			if path := adapter.MCPConfigPath(homeDir, name); path != "" {
				ops = append(ops, operation{typeID: opRemoveFile, path: path, component: component, agent: agent})
			}
		case model.StrategyMergeIntoSettings:
			if path := adapter.SettingsPath(homeDir); path != "" {
				key := mcpKeyForAgent(adapter.Agent(), name)
				ops = append(ops, operation{typeID: opRemoveJSONKey, path: path, jsonPath: key, component: component, agent: agent})
			}
		case model.StrategyMCPConfigFile:
			if path := adapter.MCPConfigPath(homeDir, name); path != "" {
				key := mcpKeyForAgent(adapter.Agent(), name)
				ops = append(ops, operation{typeID: opRemoveJSONKey, path: path, jsonPath: key, component: component, agent: agent})
			}
		case model.StrategyTOMLFile:
			// TOML upserts are append-style; safest to leave the user's TOML alone
			// and surface the section name as a manual action.
			ops = append(ops, operation{typeID: opRemoveJSONKey, path: adapter.SettingsPath(homeDir), jsonPath: []string{"mcp_servers", name}, component: component, agent: agent})
		}
	}

	// Skills component: remove the per-agent skills directory the loader wrote.
	if component == model.ComponentSkills && adapter.SupportsSkills() {
		if dir := adapter.SkillsDir(homeDir); dir != "" {
			ops = append(ops, operation{typeID: opRemoveIfEmpty, path: dir, component: component, agent: agent})
		}
	}

	// SDD component: remove sub-agent and command directories the SDD injector wrote.
	if component == model.ComponentSDD {
		if adapter.SupportsSubAgents() {
			if dir := adapter.SubAgentsDir(homeDir); dir != "" {
				ops = append(ops, operation{typeID: opRemoveIfEmpty, path: dir, component: component, agent: agent})
			}
		}
		if adapter.SupportsSlashCommands() {
			if dir := adapter.CommandsDir(homeDir); dir != "" {
				ops = append(ops, operation{typeID: opRemoveIfEmpty, path: dir, component: component, agent: agent})
			}
		}
	}

	return ops
}

// mcpKeyForAgent returns the JSON path within a settings/mcp file where the
// named MCP server lives, per-adapter.
func mcpKeyForAgent(agent model.AgentID, serverName string) []string {
	switch agent {
	case model.AgentOpenCode, model.AgentKilocode:
		return []string{"mcp", serverName}
	case model.AgentVSCodeCopilot:
		return []string{"servers", serverName}
	default:
		return []string{"mcpServers", serverName}
	}
}

// applyOperation executes a single operation. Returns whether the file system
// actually changed (so callers can short-circuit a no-op rollback).
func applyOperation(op operation) (changed bool, err error) {
	switch op.typeID {
	case opRewriteFile:
		return rewriteMarkdownSection(op.path, op.sectionID)
	case opRemoveFile:
		return removeFile(op.path)
	case opRemoveTree:
		return removeTree(op.path)
	case opRemoveIfEmpty:
		return removeIfEmpty(op.path)
	case opRemoveJSONKey:
		return removeJSONKey(op.path, op.jsonPath)
	default:
		return false, fmt.Errorf("uninstall: unknown op type %d", op.typeID)
	}
}

// rewriteMarkdownSection strips a single <!-- cortex-ia:ID --> ... section
// from a file. No-op when the file or marker is absent.
func rewriteMarkdownSection(path, sectionID string) (bool, error) {
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %q: %w", path, err)
	}
	updated := filemerge.InjectMarkdownSection(string(existing), sectionID, "")
	if updated == string(existing) {
		return false, nil
	}
	return writeFileAtomic(path, []byte(updated))
}

// removeFile deletes a single file. Missing file ⇒ no-op.
func removeFile(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %q: %w", path, err)
	}
	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("remove %q: %w", path, err)
	}
	return true, nil
}

// removeTree recursively deletes a directory. Missing dir ⇒ no-op.
func removeTree(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %q: %w", path, err)
	}
	if err := os.RemoveAll(path); err != nil {
		return false, fmt.Errorf("remove tree %q: %w", path, err)
	}
	return true, nil
}

// removeIfEmpty deletes a directory only if it is empty (or contains only
// other empty subdirs). Files inside ⇒ no-op (returns false, nil).
func removeIfEmpty(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %q: %w", path, err)
	}
	if !info.IsDir() {
		return false, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("read dir %q: %w", path, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			return false, nil // user content present; leave alone
		}
		// Recurse: only safe to remove if every subdir is also recursively empty.
		sub := filepath.Join(path, e.Name())
		if changed, err := removeIfEmpty(sub); err != nil {
			return false, err
		} else if !changed {
			// Subdir was non-empty and skipped → parent stays too.
			return false, nil
		}
	}
	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("remove empty dir %q: %w", path, err)
	}
	return true, nil
}

// removeJSONKey loads a JSON object, deletes the nested key path, and writes
// the result back. Returns false on a no-op (missing file, missing key).
// If the parent map becomes empty after deletion, the parent key is also removed.
func removeJSONKey(path string, keyPath []string) (bool, error) {
	if len(keyPath) == 0 {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %q: %w", path, err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		// Not a JSON object; safest to leave the file alone.
		return false, nil
	}

	deleted := deleteNestedKey(root, keyPath)
	if !deleted {
		return false, nil
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal %q: %w", path, err)
	}
	out = append(out, '\n')
	return writeFileAtomic(path, out)
}

// deleteNestedKey traverses the given path and deletes the leaf. Empty parent
// maps are removed up the chain. Returns true if anything was deleted.
func deleteNestedKey(root map[string]any, keyPath []string) bool {
	if len(keyPath) == 1 {
		if _, ok := root[keyPath[0]]; !ok {
			return false
		}
		delete(root, keyPath[0])
		return true
	}
	child, ok := root[keyPath[0]].(map[string]any)
	if !ok {
		return false
	}
	if !deleteNestedKey(child, keyPath[1:]) {
		return false
	}
	if len(child) == 0 {
		delete(root, keyPath[0])
	}
	return true
}

// writeFileAtomic writes via filemerge.WriteFileAtomic and reports whether the
// content actually changed.
func writeFileAtomic(path string, data []byte) (bool, error) {
	wr, err := filemerge.WriteFileAtomic(path, data, 0o644)
	if err != nil {
		return false, err
	}
	return wr.Changed, nil
}

// dedupeOperations collapses operations that target the same file+section/key
// pair. Necessary because marker sections may be planned twice when SDD has
// multiple marker IDs and the same path is reused.
func dedupeOperations(ops []operation) []operation {
	seen := make(map[string]struct{}, len(ops))
	out := make([]operation, 0, len(ops))
	for _, op := range ops {
		key := fmt.Sprintf("%d|%s|%s|%s", op.typeID, op.path, op.sectionID, joinPath(op.jsonPath))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, op)
	}
	return out
}

func joinPath(p []string) string {
	if len(p) == 0 {
		return ""
	}
	cp := make([]string, len(p))
	copy(cp, p)
	sort.Strings(cp) // canonical order for hashing
	out := ""
	for _, s := range cp {
		out += "/" + s
	}
	return out
}
