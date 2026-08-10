// Package uninstall reverses a previous cortex-ia install on a per-agent,
// per-component basis. It runs a Prepare→Apply pipeline of operations whose
// failures roll back from a fresh pre-uninstall snapshot.
package uninstall

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	opencodelayout "github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	forgespeccomp "github.com/lleontor705/cortex-ia/internal/components/forgespec"
	sddinstall "github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// opType describes the kind of file mutation an operation performs.
type opType int

const (
	opRewriteFile              opType = iota // strip a marker section from a file
	opRemoveFile                             // delete a single file
	opRemoveTree                             // recursively delete a directory
	opRemoveIfEmpty                          // remove a directory only if it is empty
	opRemoveJSONKey                          // delete a top-level key from a JSON file
	opRemoveTOMLRegion                       // delete one ownership-proven TOML MCP table
	opRemoveOwnedWorkflowAsset               // delete one lock+ownership-proven workflow asset and evidence
	opRetainWorkflowAsset                    // report an unsafe locked workflow asset without mutation
)

// operation is a single planned mutation. SectionID is used by opRewriteFile;
// JSONPath is used by opRemoveJSONKey; TOMLServer is used by opRemoveTOMLRegion.
type operation struct {
	typeID      opType
	path        string
	sectionID   string
	jsonPath    []string
	tomlServer  string
	assetPath   string
	root        string
	backupPaths []string
	reason      string
	component   model.ComponentID
	agent       model.AgentID
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
			if path := adapter.SettingsPath(homeDir); path != "" {
				ops = append(ops, operation{typeID: opRemoveTOMLRegion, path: path, tomlServer: name, component: component, agent: agent})
			}
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
		if adapter.Agent() == model.AgentOpenCode {
			layout := opencodelayout.NativeLayout()
			ops = append(ops, operation{typeID: opRemoveIfEmpty, path: filepath.Join(homeDir, filepath.FromSlash(layout.WorkflowRoot)), component: component, agent: agent})
		}
	}

	return ops
}

// mcpKeyForAgent returns the JSON path within a settings/mcp file where the
// named MCP server lives, per-adapter.
func mcpKeyForAgent(agent model.AgentID, serverName string) []string {
	return []string{"mcp", serverName}
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
	case opRemoveTOMLRegion:
		return removeTOMLRegion(op.path, op.tomlServer)
	case opRemoveOwnedWorkflowAsset:
		return removeOwnedWorkflowAsset(op.root, op.assetPath)
	case opRetainWorkflowAsset:
		return false, nil
	default:
		return false, fmt.Errorf("uninstall: unknown op type %d", op.typeID)
	}
}

func removeOwnedWorkflowAsset(root, assetPath string) (bool, error) {
	evidence, err := sddinstall.NewOwnershipStore(root).ReadEvidence(assetPath)
	if err != nil {
		return false, fmt.Errorf("verify workflow ownership %q: %w", assetPath, err)
	}
	target := filepath.Join(root, filepath.FromSlash(assetPath))
	current, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read managed workflow asset %q: %w", target, err)
	}
	inspection := sddinstall.InspectOwnership(current, &evidence.Ownership, evidence.Base)
	if inspection.State != sddinstall.OwnershipClean {
		return false, fmt.Errorf("refuse workflow asset %q removal: %s", assetPath, inspection.Reason)
	}
	for _, relative := range []string{assetPath, evidence.OwnershipPath, evidence.BasePath} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(relative))); err != nil && !os.IsNotExist(err) {
			return false, fmt.Errorf("remove managed workflow path %q: %w", relative, err)
		}
	}
	return true, nil
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
	extension := strings.ToLower(filepath.Ext(path))
	if extension != ".json" && extension != ".jsonc" {
		return false, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %q: %w", path, err)
	}
	result, err := filemerge.MutateJSONFile(path, filemerge.JSONMutation{RemovePaths: [][]string{keyPath}})
	if err != nil {
		return false, err
	}
	return result.Changed, nil
}

const tomlOwnershipMarkerPrefix = "# cortex-ia:toml-ownership "

type tomlServerDefinition struct {
	command string
	args    []string
}

var tomlServersByName = map[string]tomlServerDefinition{
	"cortex":    {command: "cortex", args: []string{"mcp", "--tools=agent"}},
	"forgespec": {command: "npx", args: []string{"-y", forgespeccomp.QualifiedNPMPackage}},
	"context7":  {command: "npx", args: []string{"-y", "@upstash/context7-mcp"}},
}

type tomlRegionOwnership struct {
	Owner        string   `json:"owner"`
	SemanticID   string   `json:"semantic_id"`
	TablePath    []string `json:"table_path"`
	Command      []string `json:"command"`
	BaseSHA256   string   `json:"base_sha256"`
	OwnershipSHA string   `json:"ownership_sha256"`
}

// removeTOMLRegion removes a single MCP table only after the semantic planner
// has accepted its exact command vector and either current ownership evidence
// or the one finite pre-evidence Cortex output proves it is ours.
func removeTOMLRegion(path, server string) (bool, error) {
	definition, ok := tomlServersByName[server]
	if !ok {
		return false, fmt.Errorf("TOML removal: unknown managed server %q", server)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %q: %w", path, err)
	}
	if ownership, found, err := tomlOwnershipCandidate(content, server); err != nil {
		return false, err
	} else if found {
		definition = tomlServerDefinition{command: ownership.Command[0], args: ownership.Command[1:]}
	}
	request := filemerge.TOMLRegionRequest{
		TablePath:       []string{"mcp_servers", server},
		ExpectedCommand: definition.command,
		ExpectedArgs:    definition.args,
	}
	plan, err := filemerge.PlanTOMLRegionRemoval(content, request)
	if err != nil {
		return false, err
	}
	if plan.Decision == "not_found" {
		return false, nil
	}
	region := content[plan.SpanStart:plan.SpanEnd]
	if err := validateTOMLRegionOwnership(region, server, definition); err != nil {
		if server != "cortex" || !isExactLegacyCortexRegion(region) {
			return false, err
		}
	}
	return writeFileAtomic(path, plan.After)
}

// tomlOwnershipCandidate reads evidence only from the canonical header emitted
// by the injector. The planner still parses the complete document before any
// mutation, so this preliminary scan cannot authorize malformed TOML.
func tomlOwnershipCandidate(content []byte, server string) (tomlRegionOwnership, bool, error) {
	header := "[mcp_servers." + server + "]"
	inTarget := false
	var marker string
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(trimmed, "[") {
			inTarget = trimmed == header
			continue
		}
		if !inTarget || !strings.HasPrefix(trimmed, tomlOwnershipMarkerPrefix) {
			continue
		}
		if marker != "" {
			return tomlRegionOwnership{}, false, fmt.Errorf("TOML removal: ambiguous ownership evidence")
		}
		marker = strings.TrimPrefix(trimmed, tomlOwnershipMarkerPrefix)
	}
	if marker == "" {
		return tomlRegionOwnership{}, false, nil
	}
	ownership, err := decodeTOMLOwnership(marker)
	if err != nil {
		return tomlRegionOwnership{}, false, err
	}
	if len(ownership.Command) == 0 || ownership.Owner != "cortex-ia" || ownership.SemanticID != "mcp/codex/"+server || !sameStringSlice(ownership.TablePath, []string{"mcp_servers", server}) {
		return tomlRegionOwnership{}, false, fmt.Errorf("TOML removal: contradictory ownership evidence")
	}
	definition := tomlServerDefinition{command: ownership.Command[0], args: ownership.Command[1:]}
	if ownership.BaseSHA256 != tomlRegionBaseSHA256(server, definition) || ownership.OwnershipSHA != tomlOwnershipDigest(ownership) {
		return tomlRegionOwnership{}, false, fmt.Errorf("TOML removal: stale ownership evidence")
	}
	return ownership, true, nil
}

func validateTOMLRegionOwnership(region []byte, server string, definition tomlServerDefinition) error {
	var marker string
	for _, line := range strings.Split(string(region), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(line, tomlOwnershipMarkerPrefix) {
			if marker != "" {
				return fmt.Errorf("TOML removal: ambiguous ownership evidence")
			}
			marker = strings.TrimPrefix(line, tomlOwnershipMarkerPrefix)
		}
	}
	if marker == "" {
		return fmt.Errorf("TOML removal: ownership evidence is missing")
	}
	ownership, err := decodeTOMLOwnership(marker)
	if err != nil {
		return err
	}
	expectedCommand := append([]string{definition.command}, definition.args...)
	expectedPath := []string{"mcp_servers", server}
	if ownership.Owner != "cortex-ia" || ownership.SemanticID != "mcp/codex/"+server || !sameStringSlice(ownership.TablePath, expectedPath) || !sameStringSlice(ownership.Command, expectedCommand) {
		return fmt.Errorf("TOML removal: contradictory ownership evidence")
	}
	if ownership.BaseSHA256 != tomlRegionBaseSHA256(server, definition) || ownership.OwnershipSHA != tomlOwnershipDigest(ownership) {
		return fmt.Errorf("TOML removal: stale ownership evidence")
	}
	return nil
}

func decodeTOMLOwnership(marker string) (tomlRegionOwnership, error) {
	decoder := json.NewDecoder(strings.NewReader(marker))
	decoder.DisallowUnknownFields()
	var ownership tomlRegionOwnership
	if err := decoder.Decode(&ownership); err != nil {
		return tomlRegionOwnership{}, fmt.Errorf("TOML removal: malformed ownership evidence: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return tomlRegionOwnership{}, fmt.Errorf("TOML removal: malformed ownership evidence")
	}
	return ownership, nil
}

func isExactLegacyCortexRegion(region []byte) bool {
	return string(region) == "[mcp_servers.cortex]\ncommand = \"cortex\"\nargs = [\"mcp\", \"--tools=agent\"]\n"
}

func tomlRegionBaseSHA256(server string, definition tomlServerDefinition) string {
	base := "[mcp_servers." + server + "]\ncommand = " + fmt.Sprintf("%q", definition.command) + "\nargs = ["
	for i, arg := range definition.args {
		if i > 0 {
			base += ", "
		}
		base += fmt.Sprintf("%q", arg)
	}
	base += "]\n"
	return fmt.Sprintf("%x", sha256.Sum256([]byte(base)))
}

func tomlOwnershipDigest(ownership tomlRegionOwnership) string {
	values := append([]string{ownership.Owner, ownership.SemanticID}, ownership.TablePath...)
	values = append(values, ownership.Command...)
	values = append(values, ownership.BaseSHA256)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(values, "\x00"))))
}

func sameStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
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
		key := fmt.Sprintf("%d|%s|%s|%s|%s", op.typeID, op.path, op.sectionID, joinPath(op.jsonPath), op.tomlServer)
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
