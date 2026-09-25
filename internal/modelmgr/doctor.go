package modelmgr

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/installmeta"
)

// Severity is the stable, ordered seriousness of one doctor finding. The
// string values are part of the CLI and TUI contract: they are rendered
// verbatim, so they must never be reworded.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
	SeverityOK      Severity = "ok"
	SeveritySkipped Severity = "skipped"
)

// severityRank orders severities so a report can collapse to its worst finding.
var severityRank = map[Severity]int{
	SeveritySkipped: 0,
	SeverityOK:      1,
	SeverityInfo:    2,
	SeverityWarning: 3,
	SeverityError:   4,
}

// CheckID identifies the doctor check that produced a finding.
type CheckID string

const (
	// CheckConfigDecode covers JSONC syntax health and duplicate members.
	CheckConfigDecode CheckID = "config-decode"
	// CheckModelShape validates every agents.*.model value.
	CheckModelShape CheckID = "model-shape"
	// CheckManagedDrift compares recorded managed digests with observed values.
	CheckManagedDrift CheckID = "managed-drift"
	// CheckMarkdownPin reports markdown frontmatter model pins.
	CheckMarkdownPin CheckID = "markdown-pin"
	// CheckDefaultAgent validates the default_agent reference.
	CheckDefaultAgent CheckID = "default-agent"
	// CheckTemplateGuard pins the embedded template against agents/model keys.
	CheckTemplateGuard CheckID = "template-guard"
	// CheckOpencode2Models is the optional `opencode2 models` cross-check.
	CheckOpencode2Models CheckID = "opencode2-models"
	// CheckNanVariants validates nan agent effort ids against the static
	// vocabulary. It is read-only and warns instead of rewriting.
	CheckNanVariants CheckID = "nan-variants"
)

// defaultAgentKey is OpenCode's root default-agent selector.
const defaultAgentKey = "default_agent"

// subagentOnlyBuiltins are the documented builtin agents that are not
// primaries, so a default_agent pointing at one is a configuration mistake.
var subagentOnlyBuiltins = map[string]struct{}{
	"general": {},
	"explore": {},
}

// templateGuardKeys are the template members the installer's safe-merge would
// force onto every user config, silently pinning models.
var templateGuardKeys = []string{agentsKey, "model"}

// Finding is one typed, severity-tagged doctor result. Every check emits at
// least one finding, and a failing check never prevents the remaining checks
// from running. Values are never included: model references are non-secret,
// but config member values are disclosed only as digests.
type Finding struct {
	Check    CheckID  `json:"check"`
	Severity Severity `json:"severity"`
	Agent    string   `json:"agent,omitempty"`
	Message  string   `json:"message"`
	Expected string   `json:"expected,omitempty"`
	Observed string   `json:"observed,omitempty"`
}

// DoctorReport is the read-only result of one doctor pass. It never mutates
// the configuration and works honestly on uninstalled homes.
type DoctorReport struct {
	HomeDir    string    `json:"home_dir"`
	ConfigPath string    `json:"config_path"`
	Findings   []Finding `json:"findings"`
}

// Verdict collapses the report to its worst finding severity.
func (r DoctorReport) Verdict() Severity {
	verdict := SeverityOK
	for _, finding := range r.Findings {
		if severityRank[finding.Severity] > severityRank[verdict] {
			verdict = finding.Severity
		}
	}
	return verdict
}

// HasErrors reports whether any check failed.
func (r DoctorReport) HasErrors() bool {
	for _, finding := range r.Findings {
		if finding.Severity == SeverityError {
			return true
		}
	}
	return false
}

func (r *DoctorReport) add(check CheckID, severity Severity, agent, message string) {
	r.Findings = append(r.Findings, Finding{Check: check, Severity: severity, Agent: agent, Message: message})
}

// CommandRunner executes one external command and returns its combined output.
// Doctor injects it so the opencode2 cross-check is a non-fatal seam that tests
// can stub without touching PATH.
type CommandRunner func(name string, args ...string) ([]byte, error)

// DoctorOptions carries every injectable seam doctor needs. Evidence is the
// recorded managed-entry ownership set (nil on an uninstalled home); Template
// returns the embedded asset bytes the regression guard inspects; RunCommand
// runs the optional opencode2 cross-check. A nil seam degrades to a SKIPPED
// finding instead of failing the report.
type DoctorOptions struct {
	Evidence   []OwnershipRecord
	Template   func() ([]byte, error)
	RunCommand CommandRunner
}

// Doctor runs every check read-only and returns typed findings. It never
// returns an error: an unreadable config, malformed metadata, or an absent
// external binary is a finding, so one broken check can never hide the rest.
func (m *Manager) Doctor(opts DoctorOptions) DoctorReport {
	report := DoctorReport{HomeDir: m.homeDir, ConfigPath: m.ConfigPath()}
	config, decoded := m.decodeForDoctor(&report)
	configAgents, agentsDecoded := doctorAgents(config, decoded)
	var reg *registry
	// The registry is built even without a config agents entry: the builtins
	// and markdown agents still participate in default_agent resolution.
	if agentsDecoded {
		built, err := m.buildRegistry(configAgents, report.ConfigPath)
		if err != nil {
			report.add(CheckModelShape, SeverityError, "", err.Error())
		} else {
			reg = built
		}
	}
	m.checkModelShapes(&report, config, configAgents, agentsDecoded)
	m.checkManagedDrift(&report, configAgents, reg, agentsDecoded, opts.Evidence)
	m.checkMarkdownPins(&report, configAgents)
	m.checkNanVariants(&report, configAgents, agentsDecoded)
	m.checkDefaultAgent(&report, config, reg)
	m.checkTemplateGuard(&report, opts.Template)
	m.checkOpencode2Models(&report, opts.RunCommand)
	return report
}

func (m *Manager) decodeForDoctor(report *DoctorReport) (map[string]any, bool) {
	raw, err := os.ReadFile(report.ConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			report.add(CheckConfigDecode, SeverityOK, "", "no OpenCode config present; doctor reports defaults")
			return map[string]any{}, true
		}
		report.add(CheckConfigDecode, SeverityError, "", "cannot read OpenCode config: "+err.Error())
		return nil, false
	}
	config, err := filemerge.DecodeJSONObject(raw)
	if err != nil {
		report.add(CheckConfigDecode, SeverityError, "", "OpenCode config is not a valid JSONC object: "+err.Error())
		return nil, false
	}
	report.add(CheckConfigDecode, SeverityOK, "", "OpenCode config decodes cleanly with unique members")
	return config, true
}

// doctorAgents projects the root agents object. A non-object value is reported
// by the shape check, so the boolean distinguishes "decoded config" from "no
// usable agents object".
func doctorAgents(config map[string]any, decoded bool) (map[string]any, bool) {
	if !decoded {
		return nil, false
	}
	raw, present := config[agentsKey]
	if !present || raw == nil {
		return map[string]any{}, true
	}
	agents, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	return agents, true
}

func (m *Manager) checkModelShapes(report *DoctorReport, config map[string]any, configAgents map[string]any, decoded bool) {
	if !decoded {
		report.add(CheckModelShape, SeveritySkipped, "", "skipped because the config did not decode")
		return
	}
	if configAgents == nil {
		report.add(CheckModelShape, SeverityError, "", fmt.Sprintf("%q must be a JSON object", agentsKey))
		return
	}
	problems := 0
	for _, agent := range sortedKeys(configAgents) {
		object, ok := configAgents[agent].(map[string]any)
		if !ok {
			report.add(CheckModelShape, SeverityError, agent, fmt.Sprintf("agents.%s must be a JSON object", agent))
			problems++
			continue
		}
		value, hasModel := object["model"]
		if !hasModel {
			continue
		}
		if _, err := DecodeModelRef(agent, value); err != nil {
			report.add(CheckModelShape, SeverityError, agent, err.Error())
			problems++
		}
	}
	if problems == 0 {
		report.add(CheckModelShape, SeverityOK, "", fmt.Sprintf("validated the shape of %d agents entries", len(configAgents)))
	}
}

func (m *Manager) checkManagedDrift(report *DoctorReport, configAgents map[string]any, reg *registry, decoded bool, evidence []OwnershipRecord) {
	if !decoded {
		report.add(CheckManagedDrift, SeveritySkipped, "", "skipped because the config did not decode")
		return
	}
	if len(evidence) == 0 {
		report.add(CheckManagedDrift, SeverityOK, "", "no managed agent-model entries recorded; nothing can drift")
		return
	}
	if configAgents == nil {
		configAgents = map[string]any{}
	}
	for _, record := range sortEvidence(evidence) {
		if !installmeta.ValidAgentModelDigest(record.Digest) {
			report.Findings = append(report.Findings, Finding{
				Check:    CheckManagedDrift,
				Severity: SeverityError,
				Agent:    record.Agent,
				Message:  "recorded managed digest is not a current amdv1 digest; ownership metadata is corrupt",
				Expected: record.Digest,
			})
			continue
		}
		observedRef, observedDigest := observedRefDigest(record.Agent, configAgents, reg)
		if observedDigest == "" {
			report.Findings = append(report.Findings, Finding{
				Check:    CheckManagedDrift,
				Severity: SeverityWarning,
				Agent:    record.Agent,
				Message:  "managed entry is absent or unreadable at the observed config path",
				Expected: record.Digest,
			})
			continue
		}
		if observedDigest != record.Digest {
			report.Findings = append(report.Findings, Finding{
				Check:    CheckManagedDrift,
				Severity: SeverityWarning,
				Agent:    record.Agent,
				Message:  fmt.Sprintf("managed entry drifted: observed %s does not match the recorded digest", observedRef),
				Expected: record.Digest,
				Observed: observedDigest,
			})
			continue
		}
		report.add(CheckManagedDrift, SeverityOK, record.Agent, fmt.Sprintf("managed entry %s matches the recorded digest", observedRef))
	}
}

func (m *Manager) checkMarkdownPins(report *DoctorReport, configAgents map[string]any) {
	pins := m.markdownAgents()
	pinned := 0
	for _, pin := range pins {
		if pin.Model == "" {
			continue
		}
		pinned++
		message := fmt.Sprintf("markdown agent %s.md frontmatter pins %s; v1 surfaces the pin and never rewrites frontmatter", pin.Name, pin.Model)
		if _, overridden := configEntryModel(configAgents, pin.Name); overridden {
			message += " (a config entry overrides it)"
		}
		report.add(CheckMarkdownPin, SeverityWarning, pin.Name, message)
	}
	if pinned == 0 {
		report.add(CheckMarkdownPin, SeverityOK, "", "no markdown agent frontmatter pins found")
	}
}

// checkNanVariants validates every config agent whose reference names a nan
// effort against the static vocabulary. It never rewrites anything, matching
// the markdown-pin never-rewrite posture, and it stays silent for nan models
// absent from the catalog so acquisition drift cannot manufacture warnings.
func (m *Manager) checkNanVariants(report *DoctorReport, configAgents map[string]any, decoded bool) {
	if !decoded {
		report.add(CheckNanVariants, SeveritySkipped, "", "skipped because the config did not decode")
		return
	}
	validated := 0
	problems := 0
	for _, agent := range sortedKeys(configAgents) {
		object, ok := configAgents[agent].(map[string]any)
		if !ok {
			continue
		}
		value, hasModel := object["model"]
		if !hasModel {
			continue
		}
		desired, err := DecodeModelRef(agent, value)
		if err != nil {
			// checkModelShape already reports the malformed value.
			continue
		}
		if desired.Provider != NanProvider || desired.Variant == "" {
			continue
		}
		meta, ok := NanModelMetaFor(desired.Model)
		if !ok {
			continue
		}
		validated++
		if containsLevel(meta.EffortVocabulary, desired.Variant) {
			continue
		}
		problems++
		report.add(CheckNanVariants, SeverityWarning, agent,
			fmt.Sprintf("nan model %s does not list effort %q; valid efforts: %s",
				desired.Provider+"/"+desired.Model, desired.Variant, vocabularyLabel(meta.EffortVocabulary)))
	}
	if problems > 0 {
		return
	}
	if validated == 0 {
		report.add(CheckNanVariants, SeverityOK, "", "no nan agent effort references to validate")
		return
	}
	report.add(CheckNanVariants, SeverityOK, "",
		fmt.Sprintf("validated %d nan effort reference(s) against the static vocabulary", validated))
}

func containsLevel(vocabulary []string, level string) bool {
	for _, candidate := range vocabulary {
		if candidate == level {
			return true
		}
	}
	return false
}

func (m *Manager) checkDefaultAgent(report *DoctorReport, config map[string]any, reg *registry) {
	raw, present := config[defaultAgentKey]
	if !present || raw == nil {
		report.add(CheckDefaultAgent, SeverityOK, "", "no default_agent declared; OpenCode uses its builtin default")
		return
	}
	name, ok := raw.(string)
	if !ok || strings.TrimSpace(name) == "" {
		report.add(CheckDefaultAgent, SeverityError, "", defaultAgentKey+" must be a non-empty string")
		return
	}
	if reg == nil {
		report.add(CheckDefaultAgent, SeveritySkipped, name, "skipped because the agent registry is unavailable")
		return
	}
	canonical, resolved := reg.resolve(name)
	if !resolved {
		report.add(CheckDefaultAgent, SeverityWarning, name, defaultAgentKey+" references an agent absent from builtins, config, and markdown agents")
		return
	}
	if _, isSubagent := subagentOnlyBuiltins[strings.ToLower(canonical)]; isSubagent {
		report.add(CheckDefaultAgent, SeverityWarning, canonical, defaultAgentKey+" references a subagent, not a visible primary agent")
		return
	}
	report.add(CheckDefaultAgent, SeverityOK, canonical, defaultAgentKey+" references a visible primary agent")
}

func (m *Manager) checkTemplateGuard(report *DoctorReport, template func() ([]byte, error)) {
	if template == nil {
		report.add(CheckTemplateGuard, SeveritySkipped, "", "skipped because no template source was injected")
		return
	}
	raw, err := template()
	if err != nil {
		report.add(CheckTemplateGuard, SeverityError, "", "cannot read the embedded template: "+err.Error())
		return
	}
	object, err := filemerge.DecodeJSONObject(raw)
	if err != nil {
		report.add(CheckTemplateGuard, SeverityError, "", "embedded template is not a valid JSONC object: "+err.Error())
		return
	}
	var offenders []string
	for _, key := range templateGuardKeys {
		if _, present := object[key]; present {
			offenders = append(offenders, key)
		}
	}
	if len(offenders) > 0 {
		report.add(CheckTemplateGuard, SeverityError, "",
			"embedded template declares "+strings.Join(offenders, " and ")+" keys; installer safe-merge makes template keys win and would silently pin models")
		return
	}
	report.add(CheckTemplateGuard, SeverityOK, "", "embedded template declares no agents or model keys")
}

func (m *Manager) checkOpencode2Models(report *DoctorReport, run CommandRunner) {
	if run == nil {
		report.add(CheckOpencode2Models, SeveritySkipped, "", "skipped because no command runner was injected")
		return
	}
	output, err := run("opencode2", "models")
	if err != nil {
		report.add(CheckOpencode2Models, SeveritySkipped, "", "opencode2 is unavailable: "+err.Error())
		return
	}
	refs := catalogReferences(ParseModelCatalog(output))
	if len(refs) == 0 {
		report.add(CheckOpencode2Models, SeveritySkipped, "", "opencode2 models output was not parseable")
		return
	}
	echo := refs
	if len(echo) > opencode2ModelsRefLimit {
		echo = echo[:opencode2ModelsRefLimit]
	}
	report.add(CheckOpencode2Models, SeverityInfo, "",
		fmt.Sprintf("opencode2 reported %d model reference(s): %s", len(refs), strings.Join(echo, ", ")))
}

// observedRefDigest resolves the observed config value for one recorded agent
// and returns its compact reference and semantic digest. An absent entry, a
// malformed value, or an unresolvable agent yields empty strings.
func observedRefDigest(agent string, configAgents map[string]any, reg *registry) (string, string) {
	canonical := agent
	if reg != nil {
		if resolved, ok := reg.resolve(agent); ok {
			canonical = resolved
		}
	}
	raw, present := configAgents[canonical]
	if !present {
		return "", ""
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return "", ""
	}
	value, hasModel := object["model"]
	if !hasModel {
		return "", ""
	}
	desired, err := DecodeModelRef(canonical, value)
	if err != nil {
		return "", ""
	}
	digest, err := installmeta.AgentModelIdentityDigest(desired.Identity())
	if err != nil {
		return "", ""
	}
	return desired.Compact(), digest
}

func configEntryModel(configAgents map[string]any, agent string) (any, bool) {
	for name, raw := range configAgents {
		if !strings.EqualFold(name, agent) {
			continue
		}
		object, ok := raw.(map[string]any)
		if !ok {
			return nil, false
		}
		value, present := object["model"]
		return value, present
	}
	return nil, false
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sortEvidence returns a deterministic copy so the report is stable across
// runs without mutating the caller's slice.
func sortEvidence(evidence []OwnershipRecord) []OwnershipRecord {
	sorted := append([]OwnershipRecord(nil), evidence...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Agent != sorted[j].Agent {
			return sorted[i].Agent < sorted[j].Agent
		}
		return sorted[i].Digest < sorted[j].Digest
	})
	return sorted
}

// opencode2ModelsRefLimit bounds how many parsed references are echoed, so an
// unexpectedly verbose command can never bloat a receipt. The reported count is
// not capped: it reflects every reference the shared parser resolved.
const opencode2ModelsRefLimit = 16

// normalizeModelField strips decoration (ANSI color, table borders, quotes,
// commas) from one output token, leaving only characters a model reference can
// contain.
func normalizeModelField(field string) string {
	var builder strings.Builder
	for _, r := range field {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '/', r == '#', r == '.', r == '-', r == '_', r == ':', r == '@':
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
