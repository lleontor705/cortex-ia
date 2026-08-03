package conformance

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// PromptLayer identifies the stage at which an asset is governed. Scans never
// silently drop a cataloged, generated, or installed asset.
type PromptLayer string

const (
	PromptLayerSource    PromptLayer = "source"
	PromptLayerCataloged PromptLayer = "cataloged"
	PromptLayerGenerated PromptLayer = "generated"
	PromptLayerInstalled PromptLayer = "installed"
)

type PromptAsset struct {
	Path    string
	Layer   PromptLayer
	Content string
}

type PromptViolationCode string

const (
	ViolationDuplicate             PromptViolationCode = "duplicate"
	ViolationStaleTool             PromptViolationCode = "stale-tool"
	ViolationTerminalVocabulary    PromptViolationCode = "terminal-vocabulary"
	ViolationUnqualifiedCapability PromptViolationCode = "unqualified-capability"
	ViolationBudget                PromptViolationCode = "budget"
	ViolationCommandStructure      PromptViolationCode = "command-structure"
	ViolationInventory             PromptViolationCode = "inventory"
)

type PromptViolation struct {
	Code   PromptViolationCode
	Path   string
	Detail string
}

type PromptGovernanceReport struct {
	AssetsScanned int
	Complete      bool
	Violations    []PromptViolation
}

// ValidatePromptInventory fails closed when an expected source, cataloged,
// generated, or installed path is absent. Callers should build expected from
// the canonical catalog and materialization receipt rather than a directory
// glob, so an omitted asset cannot become an implicit exemption.
func ValidatePromptInventory(assets []PromptAsset, expected []string) PromptGovernanceReport {
	report := ScanPromptGovernance(assets)
	seen := make(map[string]bool, len(assets))
	for _, asset := range assets {
		seen[asset.Path] = true
	}
	for _, path := range expected {
		if !seen[path] {
			report.Complete = false
			report.Violations = append(report.Violations, PromptViolation{Code: ViolationInventory, Path: path, Detail: "expected cataloged/generated/installed asset is absent from scan inventory"})
		}
	}
	sort.Slice(report.Violations, func(i, j int) bool {
		if report.Violations[i].Path != report.Violations[j].Path {
			return report.Violations[i].Path < report.Violations[j].Path
		}
		return report.Violations[i].Code < report.Violations[j].Code
	})
	return report
}

func (r PromptGovernanceReport) Has(code PromptViolationCode) bool {
	for _, violation := range r.Violations {
		if violation.Code == code {
			return true
		}
	}
	return false
}

var promptStalePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)agent[- ]mailbox`),
	regexp.MustCompile(`(?i)\ba2a(?:[_-]|\b)`),
	regexp.MustCompile(`(?i)\b(?:mailbox|team[- ]lead)\b`),
	regexp.MustCompile(`(?i)\b(?:msg|resource|dlq)_(?:send|request|broadcast|acquire|release|check|list|retry|purge)`),
	regexp.MustCompile(`(?i)\b(?:gpt-4|claude-3|gemini-pro)\b`),
}

var terminalStatus = regexp.MustCompile(`(?i)\b(?:phase\s+)?status\s*[:=]\s*(?:pass|fail|inconclusive|success|partial|blocked|done|completed)\b`)
var terminalVerdict = regexp.MustCompile(`(?i)\bverification\s+verdict\s*[:=]\s*(?:success|partial|failed|done|completed)\b`)
var capabilityClaim = regexp.MustCompile(`(?i)\b(?:kiro|vs\s*code|vscode)\b.{0,100}\b(?:supports?|executes?|provides?|enables?)\b.{0,100}\b(?:direct[- ]child|parallel|native[- ]advanced|runtime[- ]enforced)\b`)
var capabilityQualification = regexp.MustCompile(`(?i)\b(?:qualified|runtime[- ]enforced|advisory|degraded|blocked|unsupported|cannot|does not|sequential|opt[- ]in)\b`)

func ScanPromptGovernance(assets []PromptAsset) PromptGovernanceReport {
	report := PromptGovernanceReport{AssetsScanned: len(assets), Complete: len(assets) > 0}
	seenPaths := make(map[string]struct{}, len(assets))
	paragraphs := make(map[string]string)
	for _, asset := range assets {
		if strings.TrimSpace(asset.Path) == "" || strings.TrimSpace(asset.Content) == "" {
			report.Complete = false
			report.Violations = append(report.Violations, PromptViolation{Code: ViolationInventory, Path: asset.Path, Detail: "empty prompt asset"})
			continue
		}
		if _, duplicate := seenPaths[asset.Path]; duplicate {
			report.Violations = append(report.Violations, PromptViolation{Code: ViolationInventory, Path: asset.Path, Detail: "asset appears more than once"})
		} else {
			seenPaths[asset.Path] = struct{}{}
		}
		for _, pattern := range promptStalePatterns {
			if match := pattern.FindString(asset.Content); match != "" {
				report.Violations = append(report.Violations, PromptViolation{Code: ViolationStaleTool, Path: asset.Path, Detail: match})
			}
		}
		if terminalStatus.MatchString(asset.Content) || terminalVerdict.MatchString(asset.Content) {
			report.Violations = append(report.Violations, PromptViolation{Code: ViolationTerminalVocabulary, Path: asset.Path, Detail: "phase status and verification verdict vocabularies are crossed"})
		}
		if capabilityClaim.MatchString(asset.Content) && !capabilityQualification.MatchString(asset.Content) {
			report.Violations = append(report.Violations, PromptViolation{Code: ViolationUnqualifiedCapability, Path: asset.Path, Detail: "adapter capability claim lacks qualification"})
		}
		if strings.Contains(asset.Path, "/commands/") {
			checkCommand(asset, &report)
		}
		if strings.Contains(asset.Path, "sdd-orchestrator-root-index.md") {
			checkBudget(asset, 900, 1200, &report)
		} else if strings.Contains(asset.Path, "/sdd-root/") {
			checkBudget(asset, 150, 300, &report)
		}
		for _, paragraph := range normalizedParagraphs(asset.Content) {
			if previous, duplicate := paragraphs[paragraph]; duplicate && previous != asset.Path {
				report.Violations = append(report.Violations, PromptViolation{Code: ViolationDuplicate, Path: asset.Path, Detail: "normalized paragraph duplicates " + previous})
			} else {
				paragraphs[paragraph] = asset.Path
			}
		}
	}
	sort.Slice(report.Violations, func(i, j int) bool {
		if report.Violations[i].Path != report.Violations[j].Path {
			return report.Violations[i].Path < report.Violations[j].Path
		}
		return report.Violations[i].Code < report.Violations[j].Code
	})
	return report
}

func checkBudget(asset PromptAsset, minimum, maximum int, report *PromptGovernanceReport) {
	tokens := promptTokenCount(asset.Content)
	if tokens < minimum || tokens > maximum {
		report.Violations = append(report.Violations, PromptViolation{Code: ViolationBudget, Path: asset.Path, Detail: "token budget outside inclusive bounds"})
	}
}

// promptTokenCount is the conservative deterministic estimator used by the
// governance budgets: one token per three UTF-8 runes, rounded up.
func promptTokenCount(content string) int {
	return (utf8.RuneCountInString(content) + 2) / 3
}

func checkCommand(asset PromptAsset, report *PromptGovernanceReport) {
	body := asset.Content
	if parts := strings.SplitN(body, "---", 3); len(parts) == 3 {
		body = parts[2]
	}
	words := len(strings.Fields(body))
	if words < 60 || words > 140 {
		report.Violations = append(report.Violations, PromptViolation{Code: ViolationBudget, Path: asset.Path, Detail: "command must contain 60..140 words"})
	}
	lower := strings.ToLower(body)
	for _, forbidden := range []string{"pre-flight", "workflow:", "mem_search", "tb_", "sub-agent", "delegate", "pipeline", "run sub-agents"} {
		if strings.Contains(lower, forbidden) {
			report.Violations = append(report.Violations, PromptViolation{Code: ViolationCommandStructure, Path: asset.Path, Detail: "command contains orchestration policy: " + forbidden})
		}
	}
	if !strings.Contains(lower, "activate") || !strings.Contains(lower, "context") || !strings.Contains(lower, "dispatch") {
		report.Violations = append(report.Violations, PromptViolation{Code: ViolationCommandStructure, Path: asset.Path, Detail: "command must activate, capture context, and reference executable dispatch"})
	}
}

func normalizedParagraphs(content string) []string {
	paragraphs := make([]string, 0)
	for _, paragraph := range strings.Split(content, "\n\n") {
		if normalized := strings.Join(strings.Fields(paragraph), " "); normalized != "" {
			paragraphs = append(paragraphs, normalized)
		}
	}
	return paragraphs
}
