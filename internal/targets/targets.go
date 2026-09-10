package targets

import (
	"fmt"
	"os"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/clidetect"
)

// TargetID specifies an installation target platform.
type TargetID string

const (
	TargetOpenCode TargetID = "opencode"
	TargetAGY      TargetID = "agy"
	TargetClaude   TargetID = "claude"
	TargetAll      TargetID = "all"
)

// TargetResult reports the outcome of installing or updating a target.
type TargetResult struct {
	Target  TargetID `json:"target"`
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Changes []string `json:"changes,omitempty"`
}

// ParseTargets parses target arguments into a validated list of TargetIDs.
func ParseTargets(input string) ([]TargetID, error) {
	if strings.TrimSpace(input) == "" || strings.ToLower(input) == "all" {
		return []TargetID{TargetOpenCode, TargetAGY, TargetClaude}, nil
	}

	parts := strings.Split(input, ",")
	var result []TargetID
	seen := make(map[TargetID]bool)

	for _, p := range parts {
		cleaned := TargetID(strings.ToLower(strings.TrimSpace(p)))
		switch cleaned {
		case TargetOpenCode, TargetAGY, TargetClaude:
			if !seen[cleaned] {
				seen[cleaned] = true
				result = append(result, cleaned)
			}
		case TargetAll:
			return []TargetID{TargetOpenCode, TargetAGY, TargetClaude}, nil
		default:
			return nil, fmt.Errorf("unknown target %q (valid: opencode, agy, claude, all)", p)
		}
	}
	return result, nil
}

// AvailableTargets inspects the host and returns target recommendations based on detected CLIs.
func AvailableTargets(homeDir string) []clidetect.CLIInfo {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	return clidetect.DetectAll(homeDir)
}
