package targets

import (
	"fmt"
	"strings"
)

// TargetID specifies an installation target platform.
type TargetID string

const (
	TargetOpenCode TargetID = "opencode"
	TargetClaude   TargetID = "claude"
	TargetAGY      TargetID = "agy"
	TargetAll      TargetID = "all"
)

var targetAliases = map[string]TargetID{
	"opencode":    TargetOpenCode,
	"claude":      TargetClaude,
	"claude-code": TargetClaude,
	"agy":         TargetAGY,
	"antigravity": TargetAGY,
}

// TargetResult reports the outcome of installing or updating a target.
type TargetResult struct {
	Target  TargetID `json:"target"`
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Changes []string `json:"changes,omitempty"`
}

// AllTargets returns every installable target in install order.
func AllTargets() []TargetID {
	return []TargetID{TargetOpenCode, TargetClaude, TargetAGY}
}

// TargetLabel returns the human-readable name of a target.
func TargetLabel(id TargetID) string {
	switch id {
	case TargetOpenCode:
		return "OpenCode"
	case TargetClaude:
		return "Claude Code"
	case TargetAGY:
		return "Antigravity (agy)"
	}
	return string(id)
}

// ParseTargets parses target arguments into a validated list of TargetIDs.
// An empty input selects OpenCode and Claude Code; "all" selects every target.
func ParseTargets(input string) ([]TargetID, error) {
	if strings.TrimSpace(input) == "" {
		return []TargetID{TargetOpenCode, TargetClaude}, nil
	}

	var result []TargetID
	seen := make(map[TargetID]bool)
	for _, p := range strings.Split(input, ",") {
		cleaned := strings.ToLower(strings.TrimSpace(p))
		if cleaned == string(TargetAll) {
			return AllTargets(), nil
		}
		id, ok := targetAliases[cleaned]
		if !ok {
			return nil, fmt.Errorf("unknown target %q (valid: opencode, claude, agy, all)", p)
		}
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result, nil
}
