package mcpinject

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
)

const retiredMailboxName = "agent-mailbox"

// ConfigSelector identifies the exact adapter-owned registration location.
type ConfigSelector struct {
	Strategy model.MCPStrategy
	Path     string
	JSONPath []string
	TOMLPath []string
}

// RetirementEvidence proves that an observed registration was generated and
// owned by cortex-ia. Name matching alone is never ownership evidence.
type RetirementEvidence struct {
	ComponentID     model.ComponentID
	Source          string
	TemplateSHA256  string
	ObservedSHA256  string
	OwnershipSHA256 string
}

// ConfigRetirement is an immutable, byte-exact retirement decision.
type ConfigRetirement struct {
	SemanticID     ir.SemanticID
	Selector       ConfigSelector
	Evidence       RetirementEvidence
	Before         []byte
	After          []byte
	Delete         bool
	NoOpReason     string
	ReloadGuidance string
}

// PlanRetirement removes only an ownership-proven legacy Mailbox registration.
// It never mutates files; callers place the returned bytes in an immutable plan.
func PlanRetirement(homeDir string, adapter agents.Adapter, content string, evidence []RetirementEvidence) (ConfigRetirement, error) {
	selector := mailboxSelector(homeDir, adapter)
	plan := ConfigRetirement{
		SemanticID:     "retirement/agent-mailbox-registration",
		Selector:       selector,
		Before:         []byte(content),
		After:          []byte(content),
		ReloadGuidance: "reload or restart active agent runtimes to discard cached Mailbox tool schemas",
	}
	if selector.Path == "" {
		plan.NoOpReason = "adapter has no Mailbox registration target"
		return plan, nil
	}
	if err := ValidateRetirementPath(selector.Path); err != nil {
		return ConfigRetirement{}, err
	}

	var start, end int
	var found bool
	var err error
	switch selector.Strategy {
	case model.StrategySeparateMCPFiles:
		found = strings.Contains(content, "agent-mailbox-mcp")
		start, end = 0, len(content)
	case model.StrategyMergeIntoSettings, model.StrategyMCPConfigFile:
		start, end, found, err = findJSONMember([]byte(content), selector.JSONPath)
	case model.StrategyTOMLFile:
		start, end, found = findTOMLSection(content, selector.TOMLPath)
	default:
		return ConfigRetirement{}, fmt.Errorf("retire Mailbox registration: unsupported MCP strategy %d", selector.Strategy)
	}
	if err != nil {
		return ConfigRetirement{}, fmt.Errorf("retire Mailbox registration at %q: %w", selector.Path, err)
	}
	if !found {
		plan.NoOpReason = "legacy metadata exists without a managed registration"
		return plan, nil
	}

	proof, err := ownershipProof(content, evidence)
	if err != nil {
		return ConfigRetirement{}, err
	}
	plan.Evidence = proof
	if selector.Strategy == model.StrategySeparateMCPFiles {
		plan.After = nil
		plan.Delete = true
		return plan, nil
	}
	plan.After = append(append([]byte(nil), content[:start]...), content[end:]...)
	return plan, nil
}

func mailboxSelector(homeDir string, adapter agents.Adapter) ConfigSelector {
	selector := ConfigSelector{Strategy: adapter.MCPStrategy()}
	switch adapter.MCPStrategy() {
	case model.StrategySeparateMCPFiles:
		selector.Path = adapter.MCPConfigPath(homeDir, retiredMailboxName)
	case model.StrategyMergeIntoSettings:
		selector.Path = adapter.SettingsPath(homeDir)
		if adapter.Agent() == model.AgentOpenCode {
			selector.JSONPath = []string{"mcp", retiredMailboxName}
		} else {
			selector.JSONPath = []string{"mcpServers", retiredMailboxName}
		}
	case model.StrategyMCPConfigFile:
		selector.Path = adapter.MCPConfigPath(homeDir, retiredMailboxName)
		if adapter.Agent() == model.AgentVSCodeCopilot {
			selector.JSONPath = []string{"servers", retiredMailboxName}
		} else {
			selector.JSONPath = []string{"mcpServers", retiredMailboxName}
		}
	case model.StrategyTOMLFile:
		selector.Path = adapter.SettingsPath(homeDir)
		selector.TOMLPath = []string{"mcp_servers", retiredMailboxName}
	}
	return selector
}

func ownershipProof(content string, evidence []RetirementEvidence) (RetirementEvidence, error) {
	observed := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	for _, proof := range evidence {
		if proof.ComponentID != model.ComponentMailbox {
			continue
		}
		if proof.Source == "" || proof.TemplateSHA256 == "" || proof.OwnershipSHA256 == "" {
			return RetirementEvidence{}, fmt.Errorf("retire Mailbox registration: incomplete ownership evidence")
		}
		if proof.TemplateSHA256 != proof.OwnershipSHA256 || proof.ObservedSHA256 != observed {
			return RetirementEvidence{}, fmt.Errorf("retire Mailbox registration: ownership digest mismatch")
		}
		return proof, nil
	}
	return RetirementEvidence{}, fmt.Errorf("retire Mailbox registration: ownership evidence is required for an existing registration")
}

// ValidateRetirementPath rejects all known external Mailbox data, cache,
// archive, and checkout locations before a plan can contain a mutation.
func ValidateRetirementPath(path string) error {
	normalized := strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	base := strings.ToLower(filepath.Base(normalized))
	protected := strings.Contains(normalized, "/.agent-mailbox/") || strings.HasSuffix(normalized, "/.agent-mailbox") ||
		strings.HasPrefix(base, "mailbox.db") ||
		(strings.Contains(normalized, "/.npm/") && strings.Contains(normalized, "agent-mailbox")) ||
		(strings.Contains(normalized, "/archives/") && strings.Contains(normalized, "agent-mailbox")) ||
		strings.Contains(normalized, "/agent-mailbox-mcp/.git/") || strings.HasSuffix(normalized, "/agent-mailbox-mcp")
	if protected {
		return fmt.Errorf("retirement path %q is protected external Mailbox data and must not be mutated automatically", path)
	}
	return nil
}

func findJSONMember(data []byte, path []string) (int, int, bool, error) {
	if len(path) == 0 {
		return 0, 0, false, nil
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return 0, 0, false, err
	}
	start := skipJSONSpace(data, 0)
	return findJSONMemberInObject(data, start, path)
}

func findJSONMemberInObject(data []byte, objectStart int, path []string) (int, int, bool, error) {
	if objectStart >= len(data) || data[objectStart] != '{' {
		return 0, 0, false, nil
	}
	i := objectStart + 1
	previousComma := -1
	for {
		i = skipJSONSpace(data, i)
		if i >= len(data) || data[i] == '}' {
			return 0, 0, false, nil
		}
		keyStart := i
		key, keyEnd, err := parseJSONString(data, i)
		if err != nil {
			return 0, 0, false, err
		}
		i = skipJSONSpace(data, keyEnd)
		if i >= len(data) || data[i] != ':' {
			return 0, 0, false, fmt.Errorf("expected colon after JSON key %q", key)
		}
		valueStart := skipJSONSpace(data, i+1)
		valueEnd, err := scanJSONValue(data, valueStart)
		if err != nil {
			return 0, 0, false, err
		}
		next := skipJSONSpace(data, valueEnd)
		if key == path[0] {
			if len(path) > 1 {
				return findJSONMemberInObject(data, valueStart, path[1:])
			}
			if next < len(data) && data[next] == ',' {
				return keyStart, next + 1, true, nil
			}
			if previousComma >= 0 {
				return previousComma, valueEnd, true, nil
			}
			return keyStart, valueEnd, true, nil
		}
		if next < len(data) && data[next] == ',' {
			previousComma = next
			i = next + 1
			continue
		}
		return 0, 0, false, nil
	}
}

func parseJSONString(data []byte, start int) (string, int, error) {
	if start >= len(data) || data[start] != '"' {
		return "", start, fmt.Errorf("expected JSON string at byte %d", start)
	}
	for i := start + 1; i < len(data); i++ {
		if data[i] == '\\' {
			i++
			continue
		}
		if data[i] == '"' {
			decoded, err := strconv.Unquote(string(data[start : i+1]))
			return decoded, i + 1, err
		}
	}
	return "", start, fmt.Errorf("unterminated JSON string")
}

func scanJSONValue(data []byte, start int) (int, error) {
	if start >= len(data) {
		return start, fmt.Errorf("missing JSON value")
	}
	if data[start] == '"' {
		_, end, err := parseJSONString(data, start)
		return end, err
	}
	if data[start] == '{' || data[start] == '[' {
		open := data[start]
		close := byte('}')
		if open == '[' {
			close = ']'
		}
		depth := 0
		for i := start; i < len(data); i++ {
			if data[i] == '"' {
				_, end, err := parseJSONString(data, i)
				if err != nil {
					return i, err
				}
				i = end - 1
				continue
			}
			switch data[i] {
			case open:
				depth++
			case close:
				depth--
				if depth == 0 {
					return i + 1, nil
				}
			}
		}
		return start, fmt.Errorf("unterminated JSON container")
	}
	i := start
	for i < len(data) && data[i] != ',' && data[i] != '}' && data[i] != ']' {
		i++
	}
	return i, nil
}

func skipJSONSpace(data []byte, i int) int {
	for i < len(data) && (data[i] == ' ' || data[i] == '\n' || data[i] == '\r' || data[i] == '\t') {
		i++
	}
	return i
}

func findTOMLSection(content string, path []string) (int, int, bool) {
	if len(path) != 2 {
		return 0, 0, false
	}
	header := regexp.MustCompile(`(?m)^[ \t]*\[` + regexp.QuoteMeta(path[0]+"."+path[1]) + `\][ \t]*(?:\r?\n|$)`)
	loc := header.FindStringIndex(content)
	if loc == nil {
		return 0, 0, false
	}
	nextHeader := regexp.MustCompile(`(?m)^[ \t]*\[[^\r\n]+\][ \t]*(?:\r?\n|$)`).FindStringIndex(content[loc[1]:])
	end := len(content)
	if nextHeader != nil {
		end = loc[1] + nextHeader[0]
	}
	return loc[0], end, true
}
