// Package opencode provides utilities for reading OpenCode's model configuration.
package opencode

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	agentopencode "github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// RunModelsCommand executes `opencode models` and parses the output into
// grouped providers. Each line of output is "provider/model-id".
func RunModelsCommand() ([]model.OpenCodeProvider, error) {
	cmd := exec.Command("opencode", "models")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run 'opencode models': %w", err)
	}
	return ParseModelsOutput(string(output))
}

const discoveryFreshness = 5 * time.Minute

const (
	SourceDiscovery = "opencode-discovery"
	SourceCache     = "opencode-cache"
	SourceConfig    = "opencode-config"

	ReasonDiscoveryUnavailable = "opencode.discovery-unavailable"
	ReasonDiscoveryEmpty       = "opencode.discovery-empty"
	ReasonAssignmentUnresolved = "opencode.assignment-unresolved"
	ReasonAssignmentStale      = "opencode.assignment-stale"
)

// DiscoveryEvidence describes where a model list came from and whether it is
// still usable. It is deliberately separate from the provider list so callers
// cannot mistake observed data for a configured default.
type DiscoveryEvidence struct {
	Source     string    `json:"source"`
	ObservedAt time.Time `json:"observed_at"`
	FreshUntil time.Time `json:"fresh_until"`
	Digest     string    `json:"digest"`
	Qualified  bool      `json:"qualified"`
	ReasonID   string    `json:"reason_id"`
}

type DiscoverySnapshot struct {
	Providers []model.OpenCodeProvider `json:"providers"`
	Evidence  DiscoveryEvidence        `json:"evidence"`
}

type DiscoveryOptions struct {
	Now func() time.Time
	Run func(context.Context) ([]byte, error)
}

type UnresolvedDiscoveryError struct {
	Reason string
}

func (e *UnresolvedDiscoveryError) Error() string { return e.Reason }

// DiscoverModels uses only observed CLI output or an explicit cache. It never
// substitutes a vendor/model list when both sources are unavailable.
func DiscoverModels(ctx context.Context, homeDir string, options DiscoveryOptions) (DiscoverySnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UTC()
	if options.Now != nil {
		now = options.Now().UTC()
	}
	run := options.Run
	if run == nil {
		run = func(ctx context.Context) ([]byte, error) {
			cmd := exec.CommandContext(ctx, "opencode", "models")
			return cmd.Output()
		}
	}
	if output, err := run(ctx); err == nil {
		providers, parseErr := ParseModelsOutput(string(output))
		if parseErr == nil && len(providers) > 0 {
			return DiscoverySnapshot{Providers: providers, Evidence: freshEvidence(SourceDiscovery, output, now)}, nil
		}
	}
	if providers, err := LoadModelsCache(homeDir); err == nil && len(providers) > 0 {
		cacheData, marshalErr := json.Marshal(providers)
		if marshalErr != nil {
			return DiscoverySnapshot{}, fmt.Errorf("marshal cached models: %w", marshalErr)
		}
		return DiscoverySnapshot{Providers: providers, Evidence: freshEvidence(SourceCache, cacheData, now)}, nil
	}
	evidence := DiscoveryEvidence{Source: SourceDiscovery, ObservedAt: now, ReasonID: ReasonDiscoveryUnavailable}
	return DiscoverySnapshot{Evidence: evidence}, &UnresolvedDiscoveryError{Reason: ReasonDiscoveryUnavailable}
}

func freshEvidence(source string, data []byte, now time.Time) DiscoveryEvidence {
	digest := sha256.Sum256(data)
	return DiscoveryEvidence{Source: source, ObservedAt: now, FreshUntil: now.Add(discoveryFreshness), Digest: fmt.Sprintf("sha256:%x", digest[:]), Qualified: true}
}

// ParseModelsOutput parses the output of `opencode models` command.
// Each line is "provider/model-id". Groups by provider.
func ParseModelsOutput(output string) ([]model.OpenCodeProvider, error) {
	providerMap := make(map[string][]model.OpenCodeModel)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Format: "provider/model-id" or "provider/sub/model-id"
		slashIdx := strings.Index(line, "/")
		if slashIdx < 0 {
			continue
		}
		provider := line[:slashIdx]
		modelID := line[slashIdx+1:]
		if provider == "" || modelID == "" {
			continue
		}
		providerMap[provider] = append(providerMap[provider], model.OpenCodeModel{
			ID:       modelID,
			Name:     modelID,
			ToolCall: true,
		})
	}

	var providers []model.OpenCodeProvider
	for id, models := range providerMap {
		sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
		providers = append(providers, model.OpenCodeProvider{
			ID:     id,
			Name:   id,
			Models: models,
		})
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].ID < providers[j].ID })
	return providers, nil
}

// FlatModelList converts grouped providers into a flat list of "provider/model" strings.
func FlatModelList(providers []model.OpenCodeProvider) []string {
	var list []string
	for _, p := range providers {
		for _, m := range p.Models {
			list = append(list, p.ID+"/"+m.ID)
		}
	}
	return list
}

// DetectModels is a compatibility wrapper. An empty result is intentional and
// means discovery was unresolved; it is never a static provider fallback.
func DetectModels(homeDir string) []model.OpenCodeProvider {
	snapshot, _ := DiscoverModels(context.Background(), homeDir, DiscoveryOptions{})
	return snapshot.Providers
}

// --- Cache reading (kept as secondary source) ---

type cacheModel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ToolCall bool   `json:"tool_call"`
}

type cacheProvider struct {
	ID     string                `json:"id"`
	Name   string                `json:"name"`
	Models map[string]cacheModel `json:"models"`
}

// LoadModelsCache reads OpenCode's cached models from ~/.cache/opencode/models.json.
func LoadModelsCache(homeDir string) ([]model.OpenCodeProvider, error) {
	path := filepath.Join(homeDir, ".cache", "opencode", "models.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]cacheProvider
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var providers []model.OpenCodeProvider
	for _, cp := range raw {
		var models []model.OpenCodeModel
		for _, cm := range cp.Models {
			if cm.ToolCall {
				models = append(models, model.OpenCodeModel{
					ID:       cm.ID,
					Name:     cm.Name,
					ToolCall: cm.ToolCall,
				})
			}
		}
		if len(models) == 0 {
			continue
		}
		sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
		providers = append(providers, model.OpenCodeProvider{
			ID:     cp.ID,
			Name:   cp.Name,
			Models: models,
		})
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Name < providers[j].Name })
	return providers, nil
}

type ResolvedAssignment struct {
	Assignment model.OpenCodeModelAssignment `json:"assignment"`
	Evidence   DiscoveryEvidence             `json:"evidence"`
}

type ConfigReceipt struct {
	ConfigPath     string `json:"config_path"`
	BeforeDigest   string `json:"before_digest"`
	AfterDigest    string `json:"after_digest"`
	EvidenceDigest string `json:"evidence_digest"`
	Created        bool   `json:"created"`
	Before         []byte `json:"-"`
}

// GlobalConfigPath returns the effective mutable OpenCode global config.
func GlobalConfigPath(homeDir string) string {
	return agentopencode.GlobalConfigPath(homeDir)
}

func (r ConfigReceipt) Rollback() error {
	current, err := os.ReadFile(r.ConfigPath)
	if err != nil {
		if !r.Created || !os.IsNotExist(err) {
			return err
		}
		current = nil
	}
	if digestBytes(current) != r.AfterDigest {
		return fmt.Errorf("OpenCode config changed after apply; refusing rollback")
	}
	if r.Created {
		err = os.Remove(r.ConfigPath)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(r.ConfigPath); statErr == nil {
		mode = info.Mode().Perm()
	}
	_, err = filemerge.WriteFileAtomic(r.ConfigPath, r.Before, mode)
	return err
}

// ApplyToOpenCodeConfig patches model fields in the effective global config
// and returns a receipt that can restore the exact pre-mutation bytes.
func ApplyToOpenCodeConfig(homeDir string, assignments model.OpenCodeModelAssignments) (ConfigReceipt, error) {
	now := time.Now().UTC()
	resolved := make(map[string]ResolvedAssignment, len(assignments))
	for name, assignment := range assignments {
		resolved[name] = ResolvedAssignment{
			Assignment: assignment,
			Evidence:   DiscoveryEvidence{Source: SourceConfig, ObservedAt: now, FreshUntil: now.Add(discoveryFreshness), Qualified: true},
		}
	}
	return ApplyToOpenCodeConfigResolved(homeDir, resolved)
}

// ApplyToOpenCodeConfigResolved writes only explicitly resolved assignments.
// It returns a receipt that can restore the exact pre-mutation bytes.
func ApplyToOpenCodeConfigResolved(homeDir string, assignments map[string]ResolvedAssignment) (receipt ConfigReceipt, err error) {
	for agentName, resolved := range assignments {
		if resolved.Assignment.Provider == "" || resolved.Assignment.Model == "" {
			return ConfigReceipt{}, fmt.Errorf("%s: %s", ReasonAssignmentUnresolved, agentName)
		}
		if !resolved.Evidence.Qualified || resolved.Evidence.FreshUntil.IsZero() || !resolved.Evidence.FreshUntil.After(time.Now().UTC()) {
			return ConfigReceipt{}, fmt.Errorf("%s: %s", ReasonAssignmentStale, agentName)
		}
	}
	configPath := GlobalConfigPath(homeDir)
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return ConfigReceipt{}, fmt.Errorf("read OpenCode config: %w", err)
	}
	config, err := filemerge.DecodeJSONObject(data)
	if err != nil {
		return ConfigReceipt{}, fmt.Errorf("parse OpenCode config: %w", err)
	}

	// Get or create agent section
	agentSection, ok := config["agent"].(map[string]interface{})
	if !ok {
		agentSection = make(map[string]interface{})
	}
	delete(agentSection, "team-lead")
	delete(agentSection, "sdd-team-lead")
	agentOverlay := make(map[string]any, len(assignments))

	// Apply model assignments to each agent
	var evidenceDigest string
	for agentName, resolved := range assignments {
		if strings.TrimPrefix(agentName, "sdd-") == "team-lead" {
			continue
		}
		modelStr := resolved.Assignment.FormatOpenCodeModel()
		if modelStr == "" {
			continue
		}
		if evidenceDigest == "" {
			evidenceDigest = resolved.Evidence.Digest
		}

		configName := resolveOpenCodeAgentConfigName(agentSection, agentName)

		agentOverlay[configName] = map[string]any{"model": modelStr}
	}

	overlay, err := json.Marshal(map[string]any{"agent": agentOverlay})
	if err != nil {
		return ConfigReceipt{}, fmt.Errorf("marshal OpenCode model overlay: %w", err)
	}
	result, err := filemerge.MutateJSONFile(configPath, filemerge.JSONMutation{
		Overlay: overlay,
		RemovePaths: [][]string{
			{"agent", "team-lead"},
			{"agent", "sdd-team-lead"},
		},
	})
	if err != nil {
		return ConfigReceipt{}, err
	}
	return ConfigReceipt{
		ConfigPath: configPath, BeforeDigest: digestBytes(result.Before), AfterDigest: digestBytes(result.After),
		EvidenceDigest: evidenceDigest, Created: result.Created, Before: result.Before,
	}, nil
}

func digestBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", digest[:])
}

func resolveOpenCodeAgentConfigName(agentSection map[string]interface{}, agentName string) string {
	canonical := strings.TrimPrefix(agentName, "sdd-")
	if _, ok := agentSection[canonical]; ok {
		return canonical
	}
	if _, ok := agentSection[agentName]; ok {
		return agentName
	}

	legacy := "sdd-" + canonical
	if _, ok := agentSection[legacy]; ok {
		return legacy
	}
	return canonical
}
