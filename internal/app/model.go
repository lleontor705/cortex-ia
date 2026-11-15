package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/assets"
	"github.com/lleontor705/cortex-ia/internal/install"
	"github.com/lleontor705/cortex-ia/internal/modelmgr"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// modelUsage documents the whole `model` grammar. Its flags are deliberately
// disjoint from the retired `--model*` prefix that preflightCLI rejects
// app-wide before dispatch.
const modelUsage = "usage: cortex-ia model <list|get|set|unset|doctor|catalog> (list [--json] | get <agent> [--json] | set <agent> <provider/model[#variant]> [--effort <level>] [--json] [--dry-run] | unset <agent> [--json] [--dry-run] | doctor [--json] | catalog [--json] [--provider <id>])"

// modelCatalogUsage is the catalog grammar. Like the rest of the surface it
// stays disjoint from the retired --model* prefix preflightCLI rejects.
const modelCatalogUsage = "usage: cortex-ia model catalog [--json] [--provider <id>]"

// modelDoctorTemplate returns the exact embedded asset the installer
// safe-merges, so the doctor regression check inspects the same bytes a real
// install would apply.
func modelDoctorTemplate() ([]byte, error) {
	raw, err := assets.Read("opencode.jsonc")
	if err != nil {
		return nil, err
	}
	return []byte(raw), nil
}

// modelDoctorRunCommand runs the optional `opencode2 models` cross-check. It is
// a package seam so tests never depend on the binary being on PATH.
var modelDoctorRunCommand modelmgr.CommandRunner = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// modelCatalogRoundTripper is the daemon-tier transport seam for catalog
// acquisition. It mirrors modelDoctorRunCommand: production uses the default
// transport, tests can inject a synthetic one, and the text tier reuses the
// existing command runner seam.
var modelCatalogRoundTripper http.RoundTripper = http.DefaultTransport

// runModel dispatches the managed agent-model subcommands. Every ownership,
// validation, and mutation decision belongs to the modelmgr manager and the
// install service; this surface parses intent and renders receipts only.
func runModel(args []string) error {
	if len(args) == 0 {
		return errors.New(modelUsage)
	}

	switch strings.ToLower(args[0]) {
	case "list":
		asJSON, err := parseModelList(args[1:])
		if err != nil {
			return err
		}
		service, err := newService()
		if err != nil {
			return err
		}
		return runModelList(service, asJSON)
	case "get":
		agent, asJSON, err := parseModelGet(args[1:])
		if err != nil {
			return err
		}
		service, err := newService()
		if err != nil {
			return err
		}
		return runModelGet(service, agent, asJSON)
	case "set":
		spec, err := parseModelSet(args[1:])
		if err != nil {
			return err
		}
		service, err := newService()
		if err != nil {
			return err
		}
		return runModelSet(service, spec)
	case "unset":
		spec, err := parseModelUnset(args[1:])
		if err != nil {
			return err
		}
		service, err := newService()
		if err != nil {
			return err
		}
		return runModelUnset(service, spec)
	case "doctor":
		asJSON, err := parseModelDoctor(args[1:])
		if err != nil {
			return err
		}
		service, err := newService()
		if err != nil {
			return err
		}
		return runModelDoctor(service, asJSON)
	case "catalog":
		spec, err := parseModelCatalog(args[1:])
		if err != nil {
			return err
		}
		service, err := newService()
		if err != nil {
			return err
		}
		return runModelCatalog(service, spec)
	default:
		return fmt.Errorf("unknown model action: %s (use: list, get, set, unset, doctor, catalog)", args[0])
	}
}

// parseModelList parses `model list [--json]`.
func parseModelList(args []string) (bool, error) {
	asJSON := false
	for _, arg := range args {
		if strings.EqualFold(arg, "--json") {
			asJSON = true
			continue
		}
		return false, fmt.Errorf("unknown argument: %s (cortex-ia model list takes no arguments besides --json)", arg)
	}
	return asJSON, nil
}

// parseModelDoctor parses `model doctor [--json]`.
func parseModelDoctor(args []string) (bool, error) {
	asJSON := false
	for _, arg := range args {
		if strings.EqualFold(arg, "--json") {
			asJSON = true
			continue
		}
		return false, fmt.Errorf("unknown argument: %s (cortex-ia model doctor takes no arguments besides --json)", arg)
	}
	return asJSON, nil
}

// modelCatalogSpec is the parsed `model catalog` command line. The grammar is
// deliberately limited to --json and --provider so no flag from the retired
// --model* prefix can reappear on this surface.
type modelCatalogSpec struct {
	asJSON   bool
	provider string
}

// parseModelCatalog parses `model catalog [--json] [--provider <id>]`.
func parseModelCatalog(args []string) (modelCatalogSpec, error) {
	var spec modelCatalogSpec
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.EqualFold(arg, "--json"):
			spec.asJSON = true
		case strings.EqualFold(arg, "--provider"):
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return spec, fmt.Errorf("flag --provider requires a provider id argument (%s)", modelCatalogUsage)
			}
			i++
			spec.provider = args[i]
		default:
			return spec, fmt.Errorf("unknown argument: %s (%s)", arg, modelCatalogUsage)
		}
	}
	return spec, nil
}

// parseModelGet parses `model get <agent> [--json]`.
func parseModelGet(args []string) (string, bool, error) {
	var positionals []string
	asJSON := false
	for _, arg := range args {
		switch {
		case strings.EqualFold(arg, "--json"):
			asJSON = true
		case strings.HasPrefix(arg, "-"):
			return "", false, fmt.Errorf("unknown flag: %s (cortex-ia model get accepts <agent> and --json)", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		return "", false, fmt.Errorf("usage: cortex-ia model get <agent> [--json]")
	}
	return positionals[0], asJSON, nil
}

// modelSetSpec is the parsed `model set` command line.
type modelSetSpec struct {
	agent  string
	ref    string
	effort string
	asJSON bool
	dryRun bool
}

// desired builds the typed manager contract. Shape violations fail with a
// typed modelmgr validation error before any service call.
func (s modelSetSpec) desired() (modelmgr.Desired, error) {
	return modelmgr.ParseDesired(s.agent, s.ref, s.effort)
}

// parseModelSet parses `model set <agent> <provider/model[#variant]>
// [--effort <level>] [--json] [--dry-run]`. The two positionals may appear in
// any order relative to the flags.
func parseModelSet(args []string) (modelSetSpec, error) {
	var spec modelSetSpec
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.EqualFold(arg, "--json"):
			spec.asJSON = true
		case strings.EqualFold(arg, "--dry-run"):
			spec.dryRun = true
		case strings.EqualFold(arg, "--effort"):
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return spec, fmt.Errorf("flag --effort requires a level argument")
			}
			i++
			spec.effort = args[i]
		case strings.HasPrefix(arg, "-"):
			return spec, fmt.Errorf("unknown flag: %s (cortex-ia model set accepts <agent> <provider/model[#variant]>, --effort <level>, --json, and --dry-run)", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 2 {
		return spec, fmt.Errorf("usage: cortex-ia model set <agent> <provider/model[#variant]> [--effort <level>] [--json] [--dry-run]")
	}
	spec.agent, spec.ref = positionals[0], positionals[1]
	return spec, nil
}

// modelUnsetSpec is the parsed `model unset` command line.
type modelUnsetSpec struct {
	agent  string
	asJSON bool
	dryRun bool
}

// parseModelUnset parses `model unset <agent> [--json] [--dry-run]`.
func parseModelUnset(args []string) (modelUnsetSpec, error) {
	var spec modelUnsetSpec
	var positionals []string
	for _, arg := range args {
		switch {
		case strings.EqualFold(arg, "--json"):
			spec.asJSON = true
		case strings.EqualFold(arg, "--dry-run"):
			spec.dryRun = true
		case strings.HasPrefix(arg, "-"):
			return spec, fmt.Errorf("unknown flag: %s (cortex-ia model unset accepts <agent>, --json, and --dry-run)", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		return spec, fmt.Errorf("usage: cortex-ia model unset <agent> [--json] [--dry-run]")
	}
	spec.agent = positionals[0]
	return spec, nil
}

// runModelSet assigns the desired reference through the service. Malformed
// references fail closed before any state access; typed ownership conflicts
// fail closed with the manager's own diagnosis.
func runModelSet(service *install.Service, spec modelSetSpec) error {
	desired, err := spec.desired()
	if err != nil {
		return err
	}
	receipt, err := service.ModelSet(desired, install.ModelOptions{DryRun: spec.dryRun})
	if err != nil {
		return wrapModelConflict(fmt.Sprintf("model set %q", spec.agent), err)
	}
	return printModelMutationReceipt("model set", receipt, spec.asJSON)
}

// runModelUnset removes the agent's managed model entry through the service.
func runModelUnset(service *install.Service, spec modelUnsetSpec) error {
	receipt, err := service.ModelUnset(spec.agent, install.ModelOptions{DryRun: spec.dryRun})
	if err != nil {
		return wrapModelConflict(fmt.Sprintf("model unset %q", spec.agent), err)
	}
	return printModelMutationReceipt("model unset", receipt, spec.asJSON)
}

// runModelList prints every registry agent's effective reference, variant,
// source, and ownership state. Listing is read-only and honest on any home.
func runModelList(service *install.Service, asJSON bool) error {
	report, err := service.ModelList()
	if err != nil {
		return wrapModelConflict("model list", err)
	}
	if asJSON {
		return encodeModelJSON(report)
	}
	fmt.Printf("Model configuration: %s\n", report.ConfigPath)
	fmt.Printf("  Install-accredited: %v\n", report.Installed)
	for _, entry := range report.Agents {
		fmt.Printf("  %-24s %s\n", entry.Agent, renderModelEntry(entry))
	}
	return nil
}

// runModelGet prints one registry agent's effective model. An unset agent is
// reported without error.
func runModelGet(service *install.Service, agent string, asJSON bool) error {
	report, err := service.ModelGet(agent)
	if err != nil {
		return wrapModelConflict(fmt.Sprintf("model get %q", agent), err)
	}
	if asJSON {
		return encodeModelJSON(report)
	}
	entry := report.Agent
	fmt.Printf("model get %s — config %s\n", entry.Agent, report.ConfigPath)
	fmt.Printf("  Install-accredited: %v\n", report.Installed)
	fmt.Printf("  Model: %s\n", modelValueOrUnset(entry.Model))
	fmt.Printf("  Variant: %s\n", modelValueOrUnset(entry.Variant))
	fmt.Printf("  Source: %s\n", entry.Source)
	fmt.Printf("  Ownership: %s\n", modelOwnershipLabel(entry.Managed))
	if entry.MarkdownPin != "" {
		fmt.Printf("  Markdown pin: %s (informational; frontmatter is never rewritten)\n", entry.MarkdownPin)
	}
	return nil
}

// runModelDoctor renders the read-only diagnostics and exits non-zero when any
// check reported an error. Only sanitized agent names, references, digests,
// and severities appear.
func runModelDoctor(service *install.Service, asJSON bool) error {
	manager := modelmgr.New(service.HomeDir())
	report := manager.Doctor(modelmgr.DoctorOptions{
		Evidence:   modelDoctorEvidence(service.HomeDir()),
		Template:   modelDoctorTemplate,
		RunCommand: modelDoctorRunCommand,
	})
	if asJSON {
		if err := encodeModelJSON(report); err != nil {
			return err
		}
	} else {
		fmt.Printf("cortex-ia model doctor — home %s\n", report.HomeDir)
		fmt.Printf("  Config: %s\n", report.ConfigPath)
		fmt.Printf("  Verdict: %s\n", report.Verdict())
		for _, finding := range report.Findings {
			fmt.Printf("  %s: %s %s\n", finding.Severity, finding.Check, renderModelFinding(finding))
		}
	}
	if report.HasErrors() {
		return fmt.Errorf("model doctor verdict: %s", report.Verdict())
	}
	return nil
}

// runModelCatalog acquires the selectable catalog through the manager's tier
// orchestration and renders it. Exhausted tiers are not an error: the "none"
// source renders as an honest empty receipt so free-form input is unaffected.
func runModelCatalog(service *install.Service, spec modelCatalogSpec) error {
	catalog := modelmgr.New(service.HomeDir()).Catalog(modelmgr.CatalogOptions{
		RoundTripper: modelCatalogRoundTripper,
		RunCommand:   modelDoctorRunCommand,
	})
	return renderModelCatalog(catalog, spec)
}

// renderModelCatalog applies the provider filter and prints either the stable
// Catalog JSON document or the grouped text receipt. It is split from
// acquisition so receipts stay verifiable without a service home or a daemon.
func renderModelCatalog(catalog modelmgr.Catalog, spec modelCatalogSpec) error {
	if spec.provider != "" {
		catalog = filterModelCatalog(catalog, spec.provider)
	}
	if spec.asJSON {
		return encodeModelJSON(catalog)
	}
	printModelCatalog(catalog, spec.provider)
	return nil
}

// filterModelCatalog keeps entries whose provider matches exactly and
// case-sensitively, consistent with the selector grammar. A fresh slice is
// built so the acquired catalog is never mutated in place.
func filterModelCatalog(catalog modelmgr.Catalog, provider string) modelmgr.Catalog {
	entries := make([]modelmgr.CatalogEntry, 0, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		if entry.Provider == provider {
			entries = append(entries, entry)
		}
	}
	catalog.Entries = entries
	return catalog
}

// printModelCatalog renders entries grouped by provider with variants inline,
// and always discloses the acquisition source plus any truncation. Only
// provider, model, and variant identifiers are printed.
func printModelCatalog(catalog modelmgr.Catalog, provider string) {
	heading := "cortex-ia model catalog"
	if provider != "" {
		heading += fmt.Sprintf(" — provider %s", provider)
	}
	fmt.Printf("%s — source: %s\n", heading, catalog.Source)
	group := ""
	for _, entry := range catalog.Entries {
		if entry.Provider != group {
			group = entry.Provider
			fmt.Printf("  %s\n", group)
		}
		fmt.Printf("    %-44s variants: %s\n", entry.Model, catalogVariantsLabel(entry.Variants))
		if entry.Meta != nil {
			fmt.Printf("      %s\n", renderNanModelMeta(entry.Meta))
		}
	}
	if len(catalog.Entries) == 0 {
		fmt.Println("  (no models)")
	}
	if catalog.Truncated {
		fmt.Println("  Truncated: the source yielded more models than are shown.")
	}
}

// catalogVariantsLabel renders an entry's variants inline, or "(none)" when the
// entry exposes no effort identifiers.
func catalogVariantsLabel(variants []string) string {
	if len(variants) == 0 {
		return "(none)"
	}
	return strings.Join(variants, ", ")
}

// renderNanModelMeta renders one nan entry's static capability metadata. It
// prints identifiers, counts, and vocabulary tokens only: variant settings,
// headers, bodies, and API keys never reach this surface.
func renderNanModelMeta(meta *modelmgr.NanModelMeta) string {
	parts := []string{
		"tier=" + meta.Tier,
		fmt.Sprintf("ctx=%d", meta.ContextTokens),
		fmt.Sprintf("quota=%d", meta.MonthlyQuotaTokens),
		"effort=[" + strings.Join(meta.EffortVocabulary, ",") + "]",
		fmt.Sprintf("adjustable=%t", meta.EffortAdjustable),
		"modalities=[" + strings.Join(meta.Modalities, ",") + "]",
		fmt.Sprintf("picker=%t", meta.PickerEligible),
	}
	if meta.Rolling4hRefTokens > 0 {
		parts = append(parts, fmt.Sprintf("rolling4h=%d", meta.Rolling4hRefTokens))
	}
	if meta.MaxOutputTokens > 0 {
		parts = append(parts, fmt.Sprintf("maxOutput=%d", meta.MaxOutputTokens))
	}
	if meta.EffortMode != "" {
		parts = append(parts, "effortMode="+meta.EffortMode)
	}
	if meta.PreferredOver != "" {
		parts = append(parts, "preferredOver="+meta.PreferredOver)
	}
	return "meta: " + strings.Join(parts, " ")
}

// printModelMutationReceipt renders a set or unset outcome. The text form
// always discloses the action, the previous effective value, the resulting
// compact value, the config path, the verified backup, and warnings.
func printModelMutationReceipt(command string, receipt *install.ModelReceipt, asJSON bool) error {
	if asJSON {
		return encodeModelJSON(receipt)
	}
	label := fmt.Sprintf("%s %s", command, receipt.Agent)
	if receipt.DryRun {
		label += " (dry-run)"
	}
	fmt.Printf("%s — config %s\n", label, receipt.ConfigPath)
	fmt.Printf("  Action: %s\n", defaultString(receipt.Action, "none"))
	fmt.Printf("  Previous: %s\n", modelValueOrUnset(receipt.Previous))
	fmt.Printf("  Value: %s\n", modelValueOrUnset(receipt.Value))
	fmt.Printf("  Changed: %v\n", receipt.Changed)
	fmt.Printf("  Managed: %v\n", receipt.Managed)
	if receipt.DryRun {
		fmt.Println("  Dry-run: nothing was written.")
	}
	if receipt.BackupID != "" {
		fmt.Printf("  Backup: %s\n", receipt.BackupID)
	}
	if receipt.Restored {
		fmt.Printf("  Failed run restored the pre-operation state (error: %s)\n", receipt.RestoreError)
	}
	for _, warning := range receipt.Warnings {
		fmt.Printf("  Warning: %s\n", warning)
	}
	return nil
}

// wrapModelConflict renders a typed fail-closed conflict. The manager
// guarantees nothing was written when it returns one; other errors, including
// ErrNotInstalled, already carry their own guidance.
func wrapModelConflict(action string, err error) error {
	var conflict *modelmgr.ConflictError
	if errors.As(err, &conflict) {
		return fmt.Errorf("%s failed closed (nothing was written): %w", action, conflict)
	}
	return err
}

// modelDoctorEvidence projects recorded managed agent-model ownership onto
// manager records. Doctor is read-only and must stay honest on any home, so an
// absent, legacy, malformed, or disagreeing v2 document yields no evidence
// instead of an error.
func modelDoctorEvidence(homeDir string) []modelmgr.OwnershipRecord {
	metaLoad := state.LoadMetadataV2(homeDir)
	if metaLoad.Presence != state.PresenceV2 {
		return nil
	}
	lockLoad := state.LoadLockV2(homeDir)
	if lockLoad.Presence != state.PresenceV2 || state.CheckAgreementV2(metaLoad.Metadata, lockLoad.Lock) != nil {
		return nil
	}
	records := make([]modelmgr.OwnershipRecord, 0, len(metaLoad.Metadata.AgentModels))
	for _, model := range metaLoad.Metadata.AgentModels {
		if model.Ownership != state.OwnershipManaged {
			continue
		}
		records = append(records, modelmgr.OwnershipRecord{
			Agent:      model.Agent,
			Digest:     model.SemanticDigest,
			ConfigPath: filepath.Join(metaLoad.Metadata.OpencodeRoot, filepath.FromSlash(model.ConfigPath)),
		})
	}
	return records
}

// renderModelEntry renders one agent's effective reference, variant, source,
// and ownership state.
func renderModelEntry(entry modelmgr.AgentEntry) string {
	line := fmt.Sprintf("%-44s source=%s", modelCompactEntry(entry), entry.Source)
	if entry.Managed {
		line += " ownership=managed"
	} else {
		line += " ownership=not-managed"
	}
	if entry.MarkdownPin != "" && entry.Source != modelmgr.SourceMarkdown {
		line += fmt.Sprintf(" markdown_pin=%s", entry.MarkdownPin)
	}
	return line
}

// renderModelFinding renders one typed doctor finding as agent + message plus
// the digests of a drift comparison when present.
func renderModelFinding(finding modelmgr.Finding) string {
	target := finding.Agent
	if target == "" {
		target = "-"
	}
	message := fmt.Sprintf("%s: %s", target, finding.Message)
	if finding.Expected != "" || finding.Observed != "" {
		message += fmt.Sprintf(" (expected: %s, observed: %s)", defaultString(finding.Expected, "-"), defaultString(finding.Observed, "-"))
	}
	return message
}

// modelCompactEntry renders the canonical compact reference, or the markdown
// pin when no config entry provides a value.
func modelCompactEntry(entry modelmgr.AgentEntry) string {
	switch {
	case entry.Model == "":
		return modelValueOrUnset(entry.MarkdownPin)
	case entry.Variant == "":
		return entry.Model
	default:
		return entry.Model + "#" + entry.Variant
	}
}

func modelValueOrUnset(value string) string {
	if value == "" {
		return "(unset)"
	}
	return value
}

func modelOwnershipLabel(managed bool) string {
	if managed {
		return "managed"
	}
	return "not managed"
}

// encodeModelJSON prints a sanitized machine-readable document and nothing else.
func encodeModelJSON(value any) error {
	encoded, err := modelJSONBytes(value)
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}

func modelJSONBytes(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode model JSON: %w", err)
	}
	return encoded, nil
}
