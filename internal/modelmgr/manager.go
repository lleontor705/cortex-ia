package modelmgr

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/installmeta"
)

const (
	// agentsKey is OpenCode's per-agent configuration object key.
	agentsKey = "agents"

	// configRoot is the OpenCode global configuration directory, home-relative.
	configRoot = ".config/opencode"

	// jsoncName and jsonName are OpenCode's accepted global config files.
	jsoncName = "opencode.jsonc"
	jsonName  = "opencode.json"

	// providerKey and providersKey are the singular documented container and
	// the OpenCode v2 plural container; modelsKey and variantsKey locate the
	// provider model entry that nan effort authoring extends.
	providerKey  = "provider"
	providersKey = "providers"
	modelsKey    = "models"
	variantsKey  = "variants"

	// markdownDir holds markdown agent definitions under the config root.
	markdownDir = "agents"

	// frontmatterScanLimit bounds how much of a markdown agent file is read:
	// the frontmatter pins live at the top and agent bodies must never be
	// loaded into memory by a read-only listing.
	frontmatterScanLimit = 8 << 10
)

// builtinAgents are the four documented OpenCode v1 agents. The registry pins
// them so a model can be assigned before any config or markdown entry exists.
var builtinAgents = []string{"build", "plan", "general", "explore"}

// ConflictKind distinguishes why an operation failed closed.
type ConflictKind string

const (
	// ConflictUnknownAgent marks an agent absent from builtins, config keys,
	// and markdown agent files.
	ConflictUnknownAgent ConflictKind = "unknown-agent"

	// ConflictMalformed marks config where the agents object or a model value
	// cannot be interpreted unambiguously.
	ConflictMalformed ConflictKind = "malformed-config"
)

// ConflictError is the typed fail-closed result for unknown agents and
// malformed configuration. Callers must treat it as "mutate nothing and
// report": the manager guarantees the config file is never written when this
// is returned.
type ConflictError struct {
	Agent   string
	Kind    ConflictKind
	Detail  string
	Sources []string
}

func (e *ConflictError) Error() string {
	switch e.Kind {
	case ConflictUnknownAgent:
		return fmt.Sprintf("modelmgr: agent %q is not a known agent; resolution sources: %s",
			e.Agent, strings.Join(e.Sources, "; "))
	case ConflictMalformed:
		return fmt.Sprintf("modelmgr: malformed OpenCode agent-model config: %s", e.Detail)
	default:
		if e.Detail != "" {
			return fmt.Sprintf("modelmgr: agent %q: %s", e.Agent, e.Detail)
		}
		return fmt.Sprintf("modelmgr: agent %q conflict", e.Agent)
	}
}

func configShapeError(agent, member, reason string) *ConflictError {
	if member == "" {
		return configMalformed("agents.%s %s", agent, reason)
	}
	return configMalformed("agents.%s.%s %s", agent, member, reason)
}

func configMalformed(format string, args ...any) *ConflictError {
	return &ConflictError{Kind: ConflictMalformed, Detail: fmt.Sprintf(format, args...)}
}

// OwnershipRecord is the transactional-metadata evidence that cortex-ia wrote
// one agents.<agent>.model entry. A record only accredits an entry whose
// reference digest and config path both match.
type OwnershipRecord struct {
	Agent      string
	Digest     string
	ConfigPath string
}

// AgentEntry is the sanitized projection of one registry agent's effective
// model. Model excludes the variant so callers render them separately, and
// only agent names, references, variants, and sources ever travel here.
type AgentEntry struct {
	Agent       string      `json:"agent"`
	Model       string      `json:"model,omitempty"`
	Variant     string      `json:"variant,omitempty"`
	Source      ModelSource `json:"source"`
	Managed     bool        `json:"managed"`
	MarkdownPin string      `json:"markdown_pin,omitempty"`
}

// ListResult describes the effective model of every registry agent.
type ListResult struct {
	ConfigPath string
	Agents     []AgentEntry
}

// AuthoredVariant records one variant object a nan set appended to a provider
// model entry. It carries identifiers only, so a dry-run can preview exactly
// what a real run would write without exposing any other variant setting.
type AuthoredVariant struct {
	Provider string
	Model    string
	ID       string
}

// Result describes one Set or Unset outcome. Previous is always the effective
// value observed before the call, so a set discloses what it overwrote.
type Result struct {
	Agent      string
	ConfigPath string
	// Action is "set", "already-set", "unset", or "already-unset".
	Action string
	// Changed reports whether the config file bytes changed.
	Changed bool
	// Created reports whether the config file did not exist before.
	Created bool
	// Previous is the effective compact reference observed before the call.
	Previous string
	// Value is the resulting compact reference; empty for an unset.
	Value string
	// Warning carries the markdown frontmatter note when one exists.
	Warning string
	// AuthoredVariants lists the variant objects the plan appends to the
	// provider model entry; empty when the set only writes the agents mapping.
	AuthoredVariants []AuthoredVariant
	// Ownership is the record to persist in transactional metadata; nil for
	// unset.
	Ownership *OwnershipRecord
}

// Manager owns agent-model configuration for exactly one OpenCode home. It
// never reads process-global home state and never resolves paths outside the
// OpenCode config root.
type Manager struct {
	homeDir string
}

// New returns a Manager for the given home directory.
func New(homeDir string) *Manager {
	return &Manager{homeDir: homeDir}
}

// ConfigPath follows OpenCode's global load precedence: JSONC is loaded after
// JSON and owns conflicting keys when both files exist, so the manager mutates
// the JSONC file whenever one is present.
func (m *Manager) ConfigPath() string {
	dir := filepath.Join(m.homeDir, filepath.FromSlash(configRoot))
	jsonc := filepath.Join(dir, jsoncName)
	if _, err := os.Stat(jsonc); err == nil {
		return jsonc
	}
	jsonFile := filepath.Join(dir, jsonName)
	if _, err := os.Stat(jsonFile); err == nil {
		return jsonFile
	}
	return jsonc
}

// Set writes the desired reference to agents.<agent>.model through
// filemerge.MutateJSONFile, preserving JSONC comments and every unrelated
// member. Validation, agent resolution, and malformed-config detection all
// fail closed before any mutation.
func (m *Manager) Set(desired Desired, evidence []OwnershipRecord) (Result, error) {
	result, overlay, err := m.planSet(desired, evidence)
	if err != nil {
		return Result{}, err
	}
	mutation, err := filemerge.MutateJSONFile(result.ConfigPath, filemerge.JSONMutation{Overlay: overlay})
	if err != nil {
		return Result{}, fmt.Errorf("set model for agent %q: %w", result.Agent, err)
	}
	result.Changed = mutation.Changed
	result.Created = mutation.Created
	if !mutation.Changed {
		result.Action = "already-set"
	}
	return result, nil
}

// InspectSet reports, read-only, the Set outcome without writing anything. It
// fails closed identically to Set, so a dry-run can never diverge from the
// real mutation on validation or agent resolution.
func (m *Manager) InspectSet(desired Desired, evidence []OwnershipRecord) (Result, error) {
	result, _, err := m.planSet(desired, evidence)
	return result, err
}

// Unset removes agents.<agent>.model through filemerge.MutateJSONFile. The
// shared mutation boundary prunes an agent object and the agents object when
// removal empties them, so an entry cortex-ia created leaves no residue while
// unrelated members of a user-authored entry survive.
func (m *Manager) Unset(agent string, evidence []OwnershipRecord) (Result, error) {
	path := m.ConfigPath()
	config, err := loadConfig(path)
	if err != nil {
		return Result{}, err
	}
	configAgents, err := agentsObject(config, path)
	if err != nil {
		return Result{}, err
	}
	reg, err := m.buildRegistry(configAgents, path)
	if err != nil {
		return Result{}, err
	}
	canonical, ok := reg.resolve(agent)
	if !ok {
		return Result{}, reg.unknownAgent(agent)
	}
	previous, err := m.effectiveEntry(canonical, configAgents, path, reg, evidence)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		Agent:      canonical,
		ConfigPath: path,
		Action:     "unset",
		Previous:   compactEntry(previous),
		Warning:    markdownWarning(reg, canonical),
	}
	entry, present := configAgents[canonical].(map[string]any)
	if _, hasModel := entry["model"]; !present || !hasModel {
		result.Action = "already-unset"
		return result, nil
	}
	mutation, err := filemerge.MutateJSONFile(path, filemerge.JSONMutation{
		RemovePaths: [][]string{{agentsKey, canonical, "model"}},
	})
	if err != nil {
		return Result{}, fmt.Errorf("unset model for agent %q: %w", canonical, err)
	}
	result.Changed = mutation.Changed
	return result, nil
}

// Get reports the effective model of one registry agent without writing.
func (m *Manager) Get(agent string, evidence []OwnershipRecord) (AgentEntry, error) {
	path := m.ConfigPath()
	config, err := loadConfig(path)
	if err != nil {
		return AgentEntry{}, err
	}
	configAgents, err := agentsObject(config, path)
	if err != nil {
		return AgentEntry{}, err
	}
	reg, err := m.buildRegistry(configAgents, path)
	if err != nil {
		return AgentEntry{}, err
	}
	canonical, ok := reg.resolve(agent)
	if !ok {
		return AgentEntry{}, reg.unknownAgent(agent)
	}
	return m.effectiveEntry(canonical, configAgents, path, reg, evidence)
}

// List reports the effective model of every registry agent, sorted
// case-insensitively. It works honestly on any home, installed or not.
func (m *Manager) List(evidence []OwnershipRecord) (ListResult, error) {
	path := m.ConfigPath()
	config, err := loadConfig(path)
	if err != nil {
		return ListResult{}, err
	}
	configAgents, err := agentsObject(config, path)
	if err != nil {
		return ListResult{}, err
	}
	reg, err := m.buildRegistry(configAgents, path)
	if err != nil {
		return ListResult{}, err
	}
	names := reg.names()
	listing := ListResult{ConfigPath: path, Agents: make([]AgentEntry, 0, len(names))}
	for _, name := range names {
		entry, err := m.effectiveEntry(name, configAgents, path, reg, evidence)
		if err != nil {
			return ListResult{}, err
		}
		listing.Agents = append(listing.Agents, entry)
	}
	return listing, nil
}

// planSet validates and resolves a set without writing, returning the receipt
// projection and the exact mutation overlay.
func (m *Manager) planSet(desired Desired, evidence []OwnershipRecord) (Result, []byte, error) {
	if err := desired.Validate(); err != nil {
		return Result{}, nil, err
	}
	if err := desired.validateNanEffort(); err != nil {
		return Result{}, nil, err
	}
	path := m.ConfigPath()
	config, err := loadConfig(path)
	if err != nil {
		return Result{}, nil, err
	}
	configAgents, err := agentsObject(config, path)
	if err != nil {
		return Result{}, nil, err
	}
	reg, err := m.buildRegistry(configAgents, path)
	if err != nil {
		return Result{}, nil, err
	}
	canonical, ok := reg.resolve(desired.Agent)
	if !ok {
		return Result{}, nil, reg.unknownAgent(desired.Agent)
	}
	previous, err := m.effectiveEntry(canonical, configAgents, path, reg, evidence)
	if err != nil {
		return Result{}, nil, err
	}
	digest, err := installmeta.AgentModelIdentityDigest(installmeta.AgentModelIdentity{
		Agent:    canonical,
		Provider: desired.Provider,
		Model:    desired.Model,
		Variant:  desired.Variant,
	})
	if err != nil {
		return Result{}, nil, err
	}
	variants, authored, err := planNanVariants(config, desired)
	if err != nil {
		return Result{}, nil, err
	}
	overlay, err := setOverlay(canonical, desired.Compact(), desired.Provider, desired.Model, providerContainerKey(config, desired.Provider), variants)
	if err != nil {
		return Result{}, nil, err
	}
	result := Result{
		Agent:            canonical,
		ConfigPath:       path,
		Action:           "set",
		Previous:         compactEntry(previous),
		Value:            desired.Compact(),
		Warning:          markdownWarning(reg, canonical),
		AuthoredVariants: authored,
		Ownership:        &OwnershipRecord{Agent: canonical, Digest: digest, ConfigPath: path},
	}
	// An unchanged assignment is only "already-set" when this run also has
	// nothing to author; otherwise the run still mutates the config.
	if previous.Source != SourceMarkdown && previous.Source != SourceUnset &&
		result.Previous == result.Value && len(authored) == 0 {
		result.Action = "already-set"
	}
	return result, overlay, nil
}

// planNanVariants computes the full replacement variants array so a nan set
// authors exactly one {id, settings.reasoningEffort} object appended after the
// existing entries. It reports no authoring when the provider model entry is
// absent (the v1 contract), when the entry already lists the requested id, or
// when the reference is not a nan effort. A present variants value that is not
// an array of objects is malformed config and fails closed before any write.
func planNanVariants(config map[string]any, desired Desired) ([]any, []AuthoredVariant, error) {
	if desired.Provider != NanProvider || desired.Variant == "" {
		return nil, nil, nil
	}
	entry, ok := providerModelEntry(config, desired.Provider, desired.Model)
	if !ok {
		return nil, nil, nil
	}
	raw, present := entry[variantsKey]
	if !present {
		return authoredVariants(desired, nil), authored(desired), nil
	}
	array, ok := raw.([]any)
	if !ok {
		return nil, nil, malformedVariants(desired)
	}
	for _, element := range array {
		if _, ok := element.(map[string]any); !ok {
			return nil, nil, malformedVariants(desired)
		}
	}
	for _, element := range array {
		if id, _ := element.(map[string]any)["id"].(string); id == desired.Variant {
			return nil, nil, nil
		}
	}
	return authoredVariants(desired, array), authored(desired), nil
}

func authored(desired Desired) []AuthoredVariant {
	return []AuthoredVariant{{Provider: desired.Provider, Model: desired.Model, ID: desired.Variant}}
}

// authoredVariants appends the authored object to a fresh slice so the caller's
// decoded array is never mutated in place.
func authoredVariants(desired Desired, existing []any) []any {
	variants := make([]any, 0, len(existing)+1)
	variants = append(variants, existing...)
	variants = append(variants, nanVariantObject(desired.Variant))
	return variants
}

// nanVariantObject is the single authored variant shape. reasoningEffort is the
// only setting ever written; headers and bodies stay untouched.
func nanVariantObject(effort string) map[string]any {
	return map[string]any{
		"id":       effort,
		"settings": map[string]any{"reasoningEffort": effort},
	}
}

func malformedVariants(desired Desired) *ConflictError {
	return configMalformed("provider.%s.models.%s.variants must be an array of variant objects",
		desired.Provider, desired.Model)
}

// providerModelEntry locates <container>.<id>.models.<model>, resolving the
// singular or plural container exactly like the installer. A missing or
// wrong-typed chain is unresolvable, so Set keeps the v1 assignment-only
// contract rather than writing a provider definition it cannot trust.
func providerModelEntry(config map[string]any, provider, model string) (map[string]any, bool) {
	container, ok := config[providerContainerKey(config, provider)].(map[string]any)
	if !ok {
		return nil, false
	}
	providerEntry, ok := container[provider].(map[string]any)
	if !ok {
		return nil, false
	}
	models, ok := providerEntry[modelsKey].(map[string]any)
	if !ok {
		return nil, false
	}
	modelEntry, ok := models[model].(map[string]any)
	if !ok {
		return nil, false
	}
	return modelEntry, true
}

// providerContainerKey resolves the container holding provider id: the
// container already carrying it, else a present singular provider map, else a
// present plural providers map, else the singular default. Effort authoring
// extends whichever container the file already uses and never appends a
// parallel section.
func providerContainerKey(config map[string]any, provider string) string {
	if _, ok := providerBlockInContainer(config, providerKey, provider); ok {
		return providerKey
	}
	if _, ok := providerBlockInContainer(config, providersKey, provider); ok {
		return providersKey
	}
	if _, ok := config[providerKey].(map[string]any); ok {
		return providerKey
	}
	if _, ok := config[providersKey].(map[string]any); ok {
		return providersKey
	}
	return providerKey
}

func providerBlockInContainer(config map[string]any, container, provider string) (map[string]any, bool) {
	providers, ok := config[container].(map[string]any)
	if !ok {
		return nil, false
	}
	block, ok := providers[provider].(map[string]any)
	return block, ok
}

// effectiveEntry resolves the observed model for one registry agent. Config
// entries take precedence over markdown pins; a markdown pin is still surfaced
// so callers can warn that the frontmatter is informational in v1.
func (m *Manager) effectiveEntry(agent string, configAgents map[string]any, path string, reg *registry, evidence []OwnershipRecord) (AgentEntry, error) {
	entry := AgentEntry{Agent: agent, Source: SourceUnset}
	if pin, ok := reg.markdown[strings.ToLower(agent)]; ok && pin.Model != "" {
		entry.MarkdownPin = pin.Model
	}
	raw, present := configAgents[agent]
	if !present {
		applyMarkdownPin(&entry)
		return entry, nil
	}
	object, isObject := raw.(map[string]any)
	if !isObject {
		return AgentEntry{}, configMalformed("agents.%s must be a JSON object", agent)
	}
	value, hasModel := object["model"]
	if !hasModel {
		applyMarkdownPin(&entry)
		return entry, nil
	}
	desired, err := DecodeModelRef(agent, value)
	if err != nil {
		return AgentEntry{}, err
	}
	entry.Model = desired.Provider + "/" + desired.Model
	entry.Variant = desired.Variant
	entry.Source = SourceConfig
	if digest, err := installmeta.AgentModelIdentityDigest(desired.Identity()); err == nil {
		if _, ok := accredit(evidence, agent, digest, path); ok {
			entry.Source = SourceManaged
			entry.Managed = true
		}
	}
	return entry, nil
}

// applyMarkdownPin lifts a markdown frontmatter pin to the effective source
// when no config entry overrides it. An unparseable pin is disclosed raw
// (frontmatter may use templates) rather than failing the read.
func applyMarkdownPin(entry *AgentEntry) {
	if entry.MarkdownPin == "" {
		return
	}
	entry.Source = SourceMarkdown
	if desired, err := ParseModelRef(entry.Agent, entry.MarkdownPin); err == nil {
		entry.Model = desired.Provider + "/" + desired.Model
		entry.Variant = desired.Variant
	}
}

// setOverlay builds the one overlay a set applies. The agents mapping is always
// present; the provider model update is added only when a nan effort authored a
// variants array, authored under the resolved container so both changes travel
// through a single JSONMutation.
func setOverlay(agent, compact, provider, model, container string, variants []any) ([]byte, error) {
	overlay := map[string]any{
		agentsKey: map[string]any{
			agent: map[string]any{"model": compact},
		},
	}
	if variants != nil {
		overlay[container] = map[string]any{
			provider: map[string]any{
				modelsKey: map[string]any{
					model: map[string]any{variantsKey: variants},
				},
			},
		}
	}
	return json.Marshal(overlay)
}

func compactEntry(entry AgentEntry) string {
	if entry.Model != "" {
		if entry.Variant != "" {
			return entry.Model + "#" + entry.Variant
		}
		return entry.Model
	}
	return entry.MarkdownPin
}

func markdownWarning(reg *registry, agent string) string {
	pin, ok := reg.markdown[strings.ToLower(agent)]
	if !ok || pin.Model == "" {
		return ""
	}
	return fmt.Sprintf("markdown agent %s.md frontmatter also pins %s; the config entry takes precedence in v1 and the frontmatter is never rewritten", pin.Name, pin.Model)
}

// markdownAgent is one parsed markdown agent frontmatter projection.
type markdownAgent struct {
	Name  string
	Model string
}

// registry is the case-folded union of builtin agents, config agent keys, and
// markdown agent files. Config spelling always wins so a mutation writes the
// key the user already declared.
type registry struct {
	folded   map[string]string
	markdown map[string]markdownAgent
	configs  []string
	marks    []string
}

func (m *Manager) buildRegistry(configAgents map[string]any, path string) (*registry, error) {
	reg := &registry{
		folded:   make(map[string]string, len(builtinAgents)+len(configAgents)),
		markdown: make(map[string]markdownAgent),
	}
	for _, name := range builtinAgents {
		reg.folded[strings.ToLower(name)] = name
	}
	configNames := make([]string, 0, len(configAgents))
	for name := range configAgents {
		configNames = append(configNames, name)
	}
	sort.Strings(configNames)
	spellings := make(map[string]string, len(configNames))
	for _, name := range configNames {
		folded := strings.ToLower(name)
		if prior, exists := spellings[folded]; exists && prior != name {
			return nil, configMalformed("%q in %q has case-insensitively ambiguous members %q and %q", agentsKey, path, prior, name)
		}
		spellings[folded] = name
	}
	for _, agent := range m.markdownAgents() {
		folded := strings.ToLower(agent.Name)
		reg.folded[folded] = agent.Name
		reg.markdown[folded] = agent
		reg.marks = append(reg.marks, agent.Name)
	}
	// Config keys are applied last so the spelling the user already declared
	// survives a case-folded collision with a builtin or markdown filename.
	for _, name := range configNames {
		reg.folded[strings.ToLower(name)] = name
	}
	reg.configs = configNames
	return reg, nil
}

func (r *registry) resolve(agent string) (string, bool) {
	canonical, ok := r.folded[strings.ToLower(agent)]
	return canonical, ok
}

func (r *registry) names() []string {
	names := make([]string, 0, len(r.folded))
	for _, name := range r.folded {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names
}

func (r *registry) unknownAgent(agent string) *ConflictError {
	return &ConflictError{Agent: agent, Kind: ConflictUnknownAgent, Sources: r.sources()}
}

func (r *registry) sources() []string {
	return []string{
		"builtins [" + strings.Join(builtinAgents, " ") + "]",
		"config agents [" + strings.Join(r.configs, " ") + "]",
		"markdown agents [" + strings.Join(r.marks, " ") + "]",
	}
}

func (m *Manager) markdownAgents() []markdownAgent {
	dir := filepath.Join(m.homeDir, filepath.FromSlash(configRoot), markdownDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	agents := make([]markdownAgent, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		raw, err := readBounded(filepath.Join(dir, entry.Name()), frontmatterScanLimit)
		if err != nil {
			continue
		}
		agents = append(agents, parseMarkdownFrontmatter(strings.TrimSuffix(entry.Name(), ".md"), raw))
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Name < agents[j].Name })
	return agents
}

// parseMarkdownFrontmatter extracts a bounded projection of the leading YAML
// block: only the delimiter pair and the model line are inspected, so a
// read-only pin never needs a full markdown parser.
func parseMarkdownFrontmatter(name string, raw []byte) markdownAgent {
	agent := markdownAgent{Name: name}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 4096), frontmatterScanLimit)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return agent
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "model" {
			continue
		}
		agent.Model = unquoteYAML(strings.TrimSpace(value))
	}
	return agent
}

func unquoteYAML(value string) string {
	for _, quote := range []string{`"`, `'`} {
		if len(value) >= 2 && strings.HasPrefix(value, quote) && strings.HasSuffix(value, quote) {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func readBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	buffer := make([]byte, limit)
	n, err := io.ReadFull(file, buffer)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return buffer[:n], nil
}

// loadConfig reads and decodes the OpenCode global config. A missing file is an
// empty config; JSONC comments, duplicate-member rejection, and root type
// validation are delegated to the shared filemerge boundary.
func loadConfig(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read OpenCode config %q: %w", path, err)
	}
	config, err := filemerge.DecodeJSONObject(raw)
	if err != nil {
		return nil, configMalformed("%s", err.Error())
	}
	return config, nil
}

func agentsObject(config map[string]any, path string) (map[string]any, error) {
	value, present := config[agentsKey]
	if !present || value == nil {
		return map[string]any{}, nil
	}
	agents, ok := value.(map[string]any)
	if !ok {
		return nil, configMalformed("%q in %q must be a JSON object", agentsKey, path)
	}
	return agents, nil
}

func accredit(evidence []OwnershipRecord, agent, digest, path string) (OwnershipRecord, bool) {
	for _, record := range evidence {
		if record.Agent == agent && record.Digest == digest && record.ConfigPath == path {
			return record, true
		}
	}
	return OwnershipRecord{}, false
}
