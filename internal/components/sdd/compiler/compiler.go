// Package compiler validates and normalizes every input that can affect a
// generated workflow bundle. It is pure: compilation owns no runtime state and
// performs no filesystem or external-service mutation.
package compiler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/capability"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/registry"
	"github.com/lleontor705/cortex-ia/internal/model"
	"gopkg.in/yaml.v3"
)

// Input is the complete deterministic compiler boundary. EvaluationTime is an
// explicit input because evidence freshness can change whether compilation is
// accepted; callers must reuse it when they require repeatable generation.
type Input struct {
	WorkflowDocument  []byte
	CatalogDocument   []byte
	ProbeResults      []capability.ProbeResult
	Target            string
	Profile           string
	Configuration     json.RawMessage
	CompilerVersion   ir.Version
	EvaluationTime    time.Time
	AssetCatalog      ir.AssetCatalog
	Adapter           prompt.AdapterPromptContract
	ProfilePlan       quality.ProfilePlan
	QualityPolicy     *quality.QualityPolicy
	QualitySignals    *quality.ChangeSignals
	Models            prompt.ModelTable
	OperationalAssets []prompt.MaterializedAsset
	Metadata          json.RawMessage
	// CustomSkills carries the registry-normalized, provenance-verified
	// custom skill overlay records. Each declared skill must also compile
	// into AssetCatalog: the overlay is additive and never replaces
	// embedded assets. It stays nil for baseline installs without an
	// overlay.
	CustomSkills []registry.Skill
}

// NormalizedInput contains stable, owned values suitable for renderers and
// manifests. Every field participates in Canonical and Fingerprint.
type NormalizedInput struct {
	Workflow          ir.WorkflowIR               `json:"workflow"`
	Catalog           capability.Catalog          `json:"catalog"`
	ProbeResults      []capability.ProbeResult    `json:"probe_results"`
	Target            string                      `json:"target"`
	Profile           string                      `json:"profile"`
	Configuration     json.RawMessage             `json:"configuration"`
	CompilerVersion   ir.Version                  `json:"compiler_version"`
	EvaluationTime    string                      `json:"evaluation_time"`
	Degradations      []ir.Degradation            `json:"degradations"`
	AssetCatalog      ir.AssetCatalog             `json:"asset_catalog,omitempty"`
	Composition       prompt.CompositionResult    `json:"composition,omitempty"`
	QualityPolicyIR   quality.QualityPolicyIR     `json:"quality_policy_ir,omitempty"`
	QualityTemplate   quality.QualityPlanTemplate `json:"quality_template,omitempty"`
	QualityPlan       quality.QualityPlan         `json:"quality_plan,omitempty"`
	OperationalAssets []prompt.MaterializedAsset  `json:"operational_assets,omitempty"`
	Metadata          json.RawMessage             `json:"metadata,omitempty"`
	CustomSkills      []CustomSkillRecord         `json:"custom_skills,omitempty"`
}

// CustomSkillRecord is the canonical compiled projection of one accepted
// custom skill: identity, origin, and content digest only. Raw content,
// provenance, and any authority notion are deliberately absent (design D8,
// D9); renderers receive skill bytes through the materialized catalog
// contents, never through the canonical evidence.
type CustomSkillRecord struct {
	ID            string `json:"id"`
	ContentSHA256 string `json:"content_sha256"`
	Origin        string `json:"origin"`
}

// Result carries the canonical representation used to calculate Fingerprint.
type Result struct {
	Normalized        NormalizedInput
	Canonical         []byte
	Fingerprint       string
	Composition       prompt.CompositionResult
	QualityPolicyIR   quality.QualityPolicyIR
	QualityTemplate   quality.QualityPlanTemplate
	QualityPlan       quality.QualityPlan
	OperationalAssets []prompt.MaterializedAsset
	Metadata          json.RawMessage
}

type ErrorCode string

const (
	ErrorInvalidInput        ErrorCode = "invalid_input"
	ErrorDuplicateReference  ErrorCode = "duplicate_reference"
	ErrorUnresolvedReference ErrorCode = "unresolved_reference"
	ErrorCyclicReference     ErrorCode = "cyclic_reference"
	// ErrorUnsupportedDeclaration marks a custom skill declaration that
	// attempts to grant authority (agents, tools, permissions, or
	// bindings). Declarative skills carry only identity and content, so
	// such a declaration is rejected unbound before any write.
	ErrorUnsupportedDeclaration ErrorCode = "unsupported_declaration"
)

// ValidationError identifies a compiler-level semantic failure before any
// renderer or installer can observe the input.
type ValidationError struct {
	Code        ErrorCode
	Path        string
	Observed    string
	Remediation string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("compiler validation failed at %s: %s; %s", e.Path, e.Observed, e.Remediation)
}

// Compile validates compatible schemas and semantic references, then returns
// an owned, stably ordered representation and its SHA-256 fingerprint.
func Compile(input Input) (Result, error) {
	workflowResult, err := ir.DecodeWorkflow(input.WorkflowDocument)
	if err != nil {
		return Result{}, fmt.Errorf("decode workflow: %w", err)
	}
	catalogResult, err := capability.DecodeCatalog(input.CatalogDocument, input.EvaluationTime)
	if err != nil {
		return Result{}, fmt.Errorf("decode capability catalog: %w", err)
	}
	if err := validateInput(input, workflowResult.Workflow); err != nil {
		return Result{}, err
	}
	if err := validateCustomSkills(input.CustomSkills, input.AssetCatalog); err != nil {
		return Result{}, err
	}
	var composition prompt.CompositionResult
	var policyIR quality.QualityPolicyIR
	var qualityTemplate quality.QualityPlanTemplate
	var qualityPlan quality.QualityPlan
	if len(input.AssetCatalog.Assets) > 0 {
		if err := input.AssetCatalog.Validate(); err != nil {
			return Result{}, invalid("$.asset_catalog", err.Error(), "provide a complete typed operational asset catalog")
		}
		if input.QualityPolicy != nil {
			var err error
			policyIR, qualityTemplate, err = quality.CompilePolicy(*input.QualityPolicy, quality.TestingCapabilities{}, input.ProfilePlan)
			if err != nil {
				return Result{}, fmt.Errorf("compile quality policy: %w", err)
			}
			if input.QualitySignals != nil {
				qualityPlan, _, err = quality.BuildPlan(quality.PipelineInput{
					Policy: *input.QualityPolicy, Capabilities: quality.TestingCapabilities{},
					Profile: input.ProfilePlan, Signals: *input.QualitySignals,
					Evaluation: quality.EvaluationInput{Change: quality.ChangeContext{
						Kind: input.QualitySignals.Kind, ObservableBehavior: input.QualitySignals.ObservableBehavior,
						Risk: input.QualitySignals.Risk, Reversibility: input.QualitySignals.Reversibility,
						TrustBoundary: input.QualitySignals.TrustBoundary, DependencyBreadth: input.QualitySignals.DependencyBreadth,
						MigrationImpact: input.QualitySignals.MigrationImpact,
					}},
				})
				if err != nil {
					return Result{}, fmt.Errorf("build quality plan: %w", err)
				}
			}
		}
		var err error
		composition, err = prompt.Compose(prompt.CompositionInput{
			Workflow: workflowResult.Workflow, Catalog: input.AssetCatalog, Adapter: input.Adapter,
			Profile: input.ProfilePlan, QualityTemplate: qualityTemplate, QualityPlan: qualityPlan, Models: input.Models, Metadata: input.Metadata,
		})
		if err != nil {
			return Result{}, fmt.Errorf("compose operational assets: %w", err)
		}
	}

	configuration, err := canonicalJSON(input.Configuration)
	if err != nil {
		return Result{}, invalid("$.configuration", string(input.Configuration), "provide valid JSON configuration")
	}
	degradations := append([]ir.Degradation(nil), workflowResult.Degradations...)
	degradations = append(degradations, catalogResult.Degradations...)
	slices.SortFunc(degradations, func(left, right ir.Degradation) int {
		if difference := strings.Compare(string(left.SemanticID), string(right.SemanticID)); difference != 0 {
			return difference
		}
		return strings.Compare(left.Reason, right.Reason)
	})

	normalized := NormalizedInput{
		Workflow:          normalizeWorkflow(workflowResult.Workflow),
		Catalog:           normalizeCatalog(catalogResult.Catalog),
		ProbeResults:      normalizeProbeResults(input.ProbeResults),
		Target:            strings.TrimSpace(input.Target),
		Profile:           input.Profile,
		Configuration:     configuration,
		CompilerVersion:   input.CompilerVersion,
		EvaluationTime:    input.EvaluationTime.UTC().Format(time.RFC3339Nano),
		Degradations:      degradations,
		AssetCatalog:      normalizeAssetCatalog(input.AssetCatalog),
		Composition:       composition,
		QualityPolicyIR:   policyIR,
		QualityTemplate:   qualityTemplate,
		QualityPlan:       qualityPlan,
		OperationalAssets: slices.Clone(input.OperationalAssets),
		Metadata:          slices.Clone(input.Metadata),
		CustomSkills:      customSkillRecords(input.CustomSkills),
	}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		return Result{}, fmt.Errorf("marshal normalized compiler input: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return Result{
		Normalized:        normalized,
		Canonical:         canonical,
		Fingerprint:       hex.EncodeToString(digest[:]),
		Composition:       composition,
		QualityPolicyIR:   policyIR,
		QualityTemplate:   qualityTemplate,
		OperationalAssets: slices.Clone(input.OperationalAssets),
		Metadata:          slices.Clone(input.Metadata),
	}, nil
}

func normalizeAssetCatalog(catalog ir.AssetCatalog) ir.AssetCatalog {
	normalized := catalog
	normalized.Assets = slices.Clone(catalog.Assets)
	for i := range normalized.Assets {
		normalized.Assets[i].Profiles = slices.Clone(catalog.Assets[i].Profiles)
	}
	slices.SortFunc(normalized.Assets, func(left, right ir.AssetSpec) int {
		return strings.Compare(string(left.ID), string(right.ID))
	})
	return normalized
}

// authorityFrontmatterFields are the frontmatter keys a declarative custom
// skill can never declare (design invariant: custom declarations contain only
// skill ID/content). Matching is case-insensitive: the field vocabulary is
// lowercase by convention and a case variant of an authority name grants no
// additional legitimacy.
var authorityFrontmatterFields = map[string]struct{}{
	"agents": {}, "tools": {}, "permissions": {}, "bindings": {},
}

// validateCustomSkills is the compiler-boundary semantic validation of the
// custom skill overlay (REQ-REG-001, REQ-ADAPT-001). Every record must keep
// its registry-normalized identity, digest, and custom origin, must declare
// no authority field in its frontmatter, and must be present exactly once in
// the effective asset catalog with the same digest. All failures are pure
// pre-write rejections; the compiler mutates nothing.
func validateCustomSkills(skills []registry.Skill, catalog ir.AssetCatalog) error {
	seen := make(map[model.SkillID]struct{}, len(skills))
	for index, skill := range skills {
		path := fmt.Sprintf("$.custom_skills[%d]", index)
		if _, err := registry.NormalizeSkillID(string(skill.ID)); err != nil {
			return invalid(path+".id", string(skill.ID), "use the registry-normalized lowercase ASCII skill ID grammar")
		}
		if skill.Origin != registry.OriginCustom {
			return invalid(path+".origin", customOriginLabel(skill.Origin), "the overlay input accepts only custom-origin skill records")
		}
		if skill.ContentSHA256 != ir.FingerprintContent(skill.Content) {
			return invalid(path+".content_sha256", "declared digest does not match the declared content", "carry the digest produced by registry normalization of the same canonical content")
		}
		field, scanErr := scanFrontmatterAuthority(skill.Content)
		if scanErr != nil {
			return invalid(path+".frontmatter", scanErr.Error(), "provide a YAML mapping frontmatter closed by a --- line, or omit frontmatter entirely")
		}
		if field != "" {
			return &ValidationError{
				Code:        ErrorUnsupportedDeclaration,
				Path:        path + ".frontmatter." + field,
				Observed:    fmt.Sprintf("frontmatter declares authority field %q", field),
				Remediation: "remove agents, tools, permissions, and bindings from custom skill frontmatter; declarative skills carry only identity and content",
			}
		}
		if _, exists := seen[skill.ID]; exists {
			return duplicate(path+".id", ir.SemanticID(skill.ID))
		}
		seen[skill.ID] = struct{}{}
		if err := validateCustomSkillCatalogEntry(skill, catalog, path); err != nil {
			return err
		}
	}
	return nil
}

// validateCustomSkillCatalogEntry cross-checks one accepted custom skill
// against the effective asset catalog: it must appear exactly once, as a
// skill-class asset, with the digest the registry normalized.
func validateCustomSkillCatalogEntry(skill registry.Skill, catalog ir.AssetCatalog, path string) error {
	wantID := customSkillSemanticID(skill.ID)
	matches := 0
	for _, spec := range catalog.Assets {
		if spec.ID != wantID {
			continue
		}
		matches++
		if spec.Class != ir.AssetSkill {
			return invalid(path, string(spec.ID), "the effective catalog entry for a custom skill must use the skill asset class")
		}
		if spec.SHA256 != skill.ContentSHA256 {
			return invalid(path, "effective catalog digest disagrees with the declared skill digest", "compile the effective catalog from the same accepted custom skill declarations")
		}
	}
	switch matches {
	case 1:
		return nil
	case 0:
		return &ValidationError{
			Code:        ErrorUnresolvedReference,
			Path:        path,
			Observed:    fmt.Sprintf("custom skill %q is absent from the effective asset catalog", skill.ID),
			Remediation: "compile the effective catalog containing every accepted custom skill declaration",
		}
	default:
		return duplicate(path+".id", wantID)
	}
}

// scanFrontmatterAuthority inspects a custom skill's YAML frontmatter for a
// top-level authority declaration. It returns the lowercased offending field
// name when one is found, an error when frontmatter is present but malformed
// (fail closed: absence of authority fields cannot be proven from an
// unparseable block), and empty/nil when the content carries no frontmatter
// or a well-formed authority-free mapping. Indented occurrences under another
// key are values, not declarations, and never match.
func scanFrontmatterAuthority(content []byte) (string, error) {
	const opening = "---\n"
	text := string(content)
	if !strings.HasPrefix(text, opening) {
		return "", nil
	}
	rest := text[len(opening):]
	end := frontmatterClose(rest)
	if end < 0 {
		return "", fmt.Errorf("frontmatter is not closed by a --- delimiter line")
	}
	block := rest[:end]
	if strings.TrimSpace(block) == "" {
		return "", nil
	}
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(block), &document); err != nil {
		return "", fmt.Errorf("frontmatter is not valid YAML: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("frontmatter must be a YAML mapping")
	}
	mapping := document.Content[0]
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := strings.ToLower(mapping.Content[i].Value)
		if _, authority := authorityFrontmatterFields[key]; authority {
			return key, nil
		}
	}
	return "", nil
}

// frontmatterClose returns the index in rest where the closing --- delimiter
// line begins, or -1 when none exists. A delimiter is a line whose entire
// content is ---; it may end the document without a trailing newline. A line
// like ---foo is ordinary content and never closes the block.
func frontmatterClose(rest string) int {
	const marker = "\n---"
	for offset := 0; ; {
		index := strings.Index(rest[offset:], marker)
		if index < 0 {
			return -1
		}
		index += offset
		after := rest[index+len(marker):]
		if after == "" || after[0] == '\n' {
			return index
		}
		offset = index + len(marker)
	}
}

// customSkillRecords projects validated custom skills into their canonical,
// deterministically ordered records: identity, origin, and content digest
// only.
func customSkillRecords(skills []registry.Skill) []CustomSkillRecord {
	if len(skills) == 0 {
		return nil
	}
	records := make([]CustomSkillRecord, 0, len(skills))
	for _, skill := range skills {
		records = append(records, CustomSkillRecord{
			ID:            string(skill.ID),
			ContentSHA256: skill.ContentSHA256,
			Origin:        customOriginLabel(skill.Origin),
		})
	}
	slices.SortFunc(records, func(left, right CustomSkillRecord) int {
		return strings.Compare(left.ID, right.ID)
	})
	return records
}

// customSkillSemanticID mirrors the unexported derivation in the assets
// package so the compiler cross-checks custom skills against the same catalog
// identity the additive overlay produces.
func customSkillSemanticID(id model.SkillID) ir.SemanticID {
	return ir.SemanticID("asset/skill/" + string(id))
}

// customOriginLabel renders an origin kind with the canonical vocabulary the
// registry receipt also uses ("custom"/"embedded").
func customOriginLabel(origin registry.OriginKind) string {
	switch origin {
	case registry.OriginEmbedded:
		return "embedded"
	case registry.OriginCustom:
		return "custom"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(origin))
	}
}

func validateInput(input Input, workflow ir.WorkflowIR) error {
	if strings.TrimSpace(input.Target) == "" {
		return invalid("$.target", "<empty>", "select a target runtime")
	}
	switch input.Profile {
	case "portable-sequential", "portable-flat", "native-advanced":
	default:
		return invalid("$.profile", input.Profile, "select portable-sequential, portable-flat, or native-advanced")
	}
	if input.CompilerVersion == (ir.Version{}) {
		return invalid("$.compiler_version", input.CompilerVersion.String(), "set the compiler semantic version")
	}
	if input.EvaluationTime.IsZero() {
		return invalid("$.evaluation_time", "<zero>", "provide the evidence evaluation time explicitly")
	}
	if len(input.Configuration) == 0 {
		return invalid("$.configuration", "<missing>", "provide JSON configuration, using {} when empty")
	}
	if err := validateProbeResults(input.ProbeResults); err != nil {
		return err
	}
	return validateWorkflowReferences(workflow)
}

func validateWorkflowReferences(workflow ir.WorkflowIR) error {
	roles := make(map[ir.SemanticID]struct{}, len(workflow.Roles))
	for index, role := range workflow.Roles {
		path := fmt.Sprintf("$.workflow.roles[%d].id", index)
		if err := addSemanticID(roles, role.ID, path); err != nil {
			return err
		}
		for contractIndex, contract := range role.Inputs {
			if err := validateContract(contract, fmt.Sprintf("$.workflow.roles[%d].inputs[%d]", index, contractIndex)); err != nil {
				return err
			}
		}
		for contractIndex, contract := range role.Outputs {
			if err := validateContract(contract, fmt.Sprintf("$.workflow.roles[%d].outputs[%d]", index, contractIndex)); err != nil {
				return err
			}
		}
		for evidenceIndex, evidence := range role.Evidence {
			if err := validateSemanticID(evidence, fmt.Sprintf("$.workflow.roles[%d].evidence[%d]", index, evidenceIndex)); err != nil {
				return err
			}
		}
		for effectIndex, effect := range role.AllowedEffects {
			if err := validateSemanticID(ir.SemanticID(effect), fmt.Sprintf("$.workflow.roles[%d].allowed_effects[%d]", index, effectIndex)); err != nil {
				return err
			}
		}
	}

	phases := make(map[ir.SemanticID]ir.Phase, len(workflow.Phases))
	for index, phase := range workflow.Phases {
		path := fmt.Sprintf("$.workflow.phases[%d].id", index)
		if _, exists := phases[phase.ID]; exists {
			return duplicate(path, phase.ID)
		}
		if err := validateSemanticID(phase.ID, path); err != nil {
			return err
		}
		phases[phase.ID] = phase
	}
	for index, phase := range workflow.Phases {
		if _, exists := roles[phase.Role]; !exists {
			return unresolved(fmt.Sprintf("$.workflow.phases[%d].role", index), phase.Role)
		}
		for dependencyIndex, dependency := range phase.DependsOn {
			path := fmt.Sprintf("$.workflow.phases[%d].depends_on[%d]", index, dependencyIndex)
			if _, exists := phases[dependency]; !exists {
				return unresolved(path, dependency)
			}
		}
	}
	if hasPhaseCycle(phases) {
		return &ValidationError{Code: ErrorCyclicReference, Path: "$.workflow.phases", Observed: "dependency cycle", Remediation: "remove at least one dependency edge from the cycle"}
	}

	tools := make(map[ir.SemanticID]struct{}, len(workflow.Tools))
	for index, tool := range workflow.Tools {
		if err := addSemanticID(tools, tool.ID, fmt.Sprintf("$.workflow.tools[%d].id", index)); err != nil {
			return err
		}
	}
	services := make(map[ir.SemanticID]struct{}, len(workflow.Services))
	for index, service := range workflow.Services {
		if err := addSemanticID(services, service.ID, fmt.Sprintf("$.workflow.services[%d].id", index)); err != nil {
			return err
		}
		if !validVersionRange(service.Version) {
			return invalid(fmt.Sprintf("$.workflow.services[%d].version", index), service.Version.String(), "use an ordered interval within one semantic major")
		}
	}
	profiles := make(map[ir.SemanticID]struct{}, len(workflow.Profiles))
	for index, profile := range workflow.Profiles {
		if err := addSemanticID(profiles, profile.ID, fmt.Sprintf("$.workflow.profiles[%d].id", index)); err != nil {
			return err
		}
	}
	return nil
}

func validateContract(contract ir.Contract, path string) error {
	if err := validateSemanticID(contract.ID, path+".id"); err != nil {
		return err
	}
	if contract.SchemaVersion.Major != ir.ContractSchema.Supported.Minimum.Major {
		return invalid(path+".schema_version", contract.SchemaVersion.String(), "use a supported role-contract schema major")
	}
	return nil
}

func validateProbeResults(results []capability.ProbeResult) error {
	seen := make(map[ir.SemanticID]struct{}, len(results))
	for index, result := range results {
		path := fmt.Sprintf("$.probe_results[%d].record", index)
		if err := addSemanticID(seen, result.Record.ID, path+".id"); err != nil {
			return err
		}
		command := result.Record.Method == capability.ProbeCommand && strings.TrimSpace(result.Record.Command) != "" && strings.TrimSpace(result.Record.Protocol) == ""
		protocol := result.Record.Method == capability.ProbeProtocol && strings.TrimSpace(result.Record.Protocol) != "" && strings.TrimSpace(result.Record.Command) == ""
		if (!command && !protocol) || strings.TrimSpace(result.Record.Result) == "" || result.Record.Timestamp.IsZero() || strings.TrimSpace(result.Record.EvidenceDigest) == "" {
			return invalid(path, "incomplete probe evidence", "provide exactly one command or protocol plus result, timestamp, and redacted evidence digest")
		}
		if result.Refined.ID != "" {
			if err := validateSemanticID(result.Refined.ID, fmt.Sprintf("$.probe_results[%d].refined.id", index)); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasPhaseCycle(phases map[ir.SemanticID]ir.Phase) bool {
	const (
		unvisited = iota
		visiting
		visited
	)
	state := make(map[ir.SemanticID]int, len(phases))
	ids := make([]ir.SemanticID, 0, len(phases))
	for id := range phases {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var visit func(ir.SemanticID) bool
	visit = func(id ir.SemanticID) bool {
		if state[id] == visiting {
			return true
		}
		if state[id] == visited {
			return false
		}
		state[id] = visiting
		dependencies := sortedUnique(phases[id].DependsOn)
		for _, dependency := range dependencies {
			if visit(dependency) {
				return true
			}
		}
		state[id] = visited
		return false
	}
	for _, id := range ids {
		if visit(id) {
			return true
		}
	}
	return false
}

func normalizeWorkflow(workflow ir.WorkflowIR) ir.WorkflowIR {
	normalized := workflow
	normalized.Roles = slices.Clone(workflow.Roles)
	for index := range normalized.Roles {
		role := &normalized.Roles[index]
		role.Inputs = sortedByID(role.Inputs, func(contract ir.Contract) ir.SemanticID { return contract.ID })
		role.Outputs = sortedByID(role.Outputs, func(contract ir.Contract) ir.SemanticID { return contract.ID })
		role.NonGoals = sortedUnique(role.NonGoals)
		role.AllowedEffects = sortedUnique(role.AllowedEffects)
		role.Evidence = sortedUnique(role.Evidence)
		role.TerminalStates = sortedUnique(role.TerminalStates)
	}
	slices.SortFunc(normalized.Roles, func(left, right ir.Role) int { return strings.Compare(string(left.ID), string(right.ID)) })

	normalized.Phases = slices.Clone(workflow.Phases)
	for index := range normalized.Phases {
		normalized.Phases[index].DependsOn = sortedUnique(normalized.Phases[index].DependsOn)
	}
	slices.SortFunc(normalized.Phases, func(left, right ir.Phase) int { return strings.Compare(string(left.ID), string(right.ID)) })
	normalized.Tools = sortedByID(workflow.Tools, func(tool ir.ToolRequirement) ir.SemanticID { return tool.ID })
	normalized.Context.Classes = sortedUnique(workflow.Context.Classes)
	normalized.Services = sortedByID(workflow.Services, func(service ir.ServiceRequirement) ir.SemanticID { return service.ID })
	normalized.Profiles = sortedByID(workflow.Profiles, func(profile ir.Profile) ir.SemanticID { return profile.ID })
	return normalized
}

func normalizeCatalog(catalog capability.Catalog) capability.Catalog {
	normalized := catalog
	normalized.Facts = slices.Clone(catalog.Facts)
	for index := range normalized.Facts {
		fact := &normalized.Facts[index]
		fact.ObservedAt = fact.ObservedAt.UTC()
		fact.FreshUntil = fact.FreshUntil.UTC()
		if fact.Probe != nil {
			probe := *fact.Probe
			probe.Timestamp = probe.Timestamp.UTC()
			fact.Probe = &probe
		}
	}
	slices.SortFunc(normalized.Facts, func(left, right capability.CapabilityFact) int {
		return bytes.Compare(canonicalKey(left), canonicalKey(right))
	})
	return normalized
}

func normalizeProbeResults(results []capability.ProbeResult) []capability.ProbeResult {
	normalized := slices.Clone(results)
	for index := range normalized {
		normalized[index].Record.Timestamp = normalized[index].Record.Timestamp.UTC()
		normalized[index].TrustClasses = sortedUnique(results[index].TrustClasses)
		normalized[index].Permissions = sortedUnique(results[index].Permissions)
		if results[index].Refined.Probe != nil {
			probe := *results[index].Refined.Probe
			probe.Timestamp = probe.Timestamp.UTC()
			normalized[index].Refined.Probe = &probe
		}
		normalized[index].Refined.ObservedAt = normalized[index].Refined.ObservedAt.UTC()
		normalized[index].Refined.FreshUntil = normalized[index].Refined.FreshUntil.UTC()
	}
	slices.SortFunc(normalized, func(left, right capability.ProbeResult) int {
		leftKey := fmt.Sprintf("%s\x00%s\x00%s", left.Record.ID, left.Refined.ID, left.Record.EvidenceDigest)
		rightKey := fmt.Sprintf("%s\x00%s\x00%s", right.Record.ID, right.Refined.ID, right.Record.EvidenceDigest)
		return strings.Compare(leftKey, rightKey)
	})
	return normalized
}

func canonicalJSON(data []byte) (json.RawMessage, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(value)
	return json.RawMessage(canonical), err
}

func canonicalKey(value any) []byte {
	key, _ := json.Marshal(value)
	return key
}

func addSemanticID(seen map[ir.SemanticID]struct{}, id ir.SemanticID, path string) error {
	if err := validateSemanticID(id, path); err != nil {
		return err
	}
	if _, exists := seen[id]; exists {
		return duplicate(path, id)
	}
	seen[id] = struct{}{}
	return nil
}

func validateSemanticID(id ir.SemanticID, path string) error {
	if err := ir.ValidateSemanticID(id); err != nil {
		return invalid(path, string(id), "use a canonical lower-case namespaced semantic ID")
	}
	return nil
}

func validVersionRange(version ir.VersionRange) bool {
	return version.Minimum.Major >= 0 && version.Minimum.Major == version.MaximumTested.Major && compareVersion(version.Minimum, version.MaximumTested) <= 0
}

func compareVersion(left, right ir.Version) int {
	if left.Major != right.Major {
		return left.Major - right.Major
	}
	if left.Minor != right.Minor {
		return left.Minor - right.Minor
	}
	return left.Patch - right.Patch
}

func sortedByID[T any](values []T, id func(T) ir.SemanticID) []T {
	result := slices.Clone(values)
	slices.SortFunc(result, func(left, right T) int { return strings.Compare(string(id(left)), string(id(right))) })
	return result
}

func sortedUnique[T ~string](values []T) []T {
	result := slices.Clone(values)
	slices.Sort(result)
	return slices.Compact(result)
}

func invalid(path, observed, remediation string) *ValidationError {
	return &ValidationError{Code: ErrorInvalidInput, Path: path, Observed: observed, Remediation: remediation}
}

func duplicate(path string, id ir.SemanticID) *ValidationError {
	return &ValidationError{Code: ErrorDuplicateReference, Path: path, Observed: string(id), Remediation: "use each semantic ID exactly once in its collection"}
}

func unresolved(path string, id ir.SemanticID) *ValidationError {
	return &ValidationError{Code: ErrorUnresolvedReference, Path: path, Observed: string(id), Remediation: "declare the referenced semantic ID before compilation"}
}
