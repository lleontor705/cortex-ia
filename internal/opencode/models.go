// Package opencode provides utilities for reading OpenCode's model configuration.
package opencode

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	Before         []byte `json:"-"`
}

func (r ConfigReceipt) Rollback() error {
	if len(r.Before) == 0 {
		return os.Remove(r.ConfigPath)
	}
	return os.WriteFile(r.ConfigPath, r.Before, 0o644)
}

// ApplyToOpenCodeConfig reads opencode.json, sets "model" field on each agent, and writes back.
func ApplyToOpenCodeConfig(homeDir string, assignments model.OpenCodeModelAssignments) error {
	now := time.Now().UTC()
	resolved := make(map[string]ResolvedAssignment, len(assignments))
	for name, assignment := range assignments {
		resolved[name] = ResolvedAssignment{
			Assignment: assignment,
			Evidence:   DiscoveryEvidence{Source: SourceConfig, ObservedAt: now, FreshUntil: now.Add(discoveryFreshness), Qualified: true},
		}
	}
	_, err := ApplyToOpenCodeConfigResolved(homeDir, resolved)
	return err
}

// ApplyToOpenCodeConfigResolved writes only explicitly resolved assignments.
// It returns a receipt that can restore the exact pre-mutation bytes.
func ApplyToOpenCodeConfigResolved(homeDir string, assignments map[string]ResolvedAssignment) (receipt ConfigReceipt, err error) {
	configPath := filepath.Join(homeDir, ".config", "opencode", "opencode.json")

	// Read existing config
	var config map[string]interface{}
	var before []byte
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			config = make(map[string]interface{})
		} else {
			return ConfigReceipt{}, fmt.Errorf("read opencode.json: %w", err)
		}
	} else {
		before = append([]byte(nil), data...)
		if err := json.Unmarshal(data, &config); err != nil {
			return ConfigReceipt{}, fmt.Errorf("parse opencode.json: %w", err)
		}
	}

	for agentName, resolved := range assignments {
		if resolved.Assignment.Provider == "" || resolved.Assignment.Model == "" {
			return ConfigReceipt{}, fmt.Errorf("%s: %s", ReasonAssignmentUnresolved, agentName)
		}
		if !resolved.Evidence.Qualified || resolved.Evidence.FreshUntil.IsZero() || !resolved.Evidence.FreshUntil.After(time.Now().UTC()) {
			return ConfigReceipt{}, fmt.Errorf("%s: %s", ReasonAssignmentStale, agentName)
		}
	}

	// Get or create agent section
	agentSection, ok := config["agent"].(map[string]interface{})
	if !ok {
		agentSection = make(map[string]interface{})
	}
	delete(agentSection, "team-lead")
	delete(agentSection, "sdd-team-lead")

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

		agentConf, ok := agentSection[configName].(map[string]interface{})
		if !ok {
			agentConf = make(map[string]interface{})
		}
		agentConf["model"] = modelStr
		agentSection[configName] = agentConf
	}

	config["agent"] = agentSection

	// Write back
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return ConfigReceipt{}, fmt.Errorf("marshal opencode.json: %w", err)
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ConfigReceipt{}, fmt.Errorf("create config dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".opencode.json.*.tmp")
	if err != nil {
		return ConfigReceipt{}, fmt.Errorf("create config temp: %w", err)
	}
	tmpName := tmp.Name()
	removeTemp := true
	defer func() {
		if !removeTemp {
			return
		}
		if removeErr := os.Remove(tmpName); removeErr != nil && !os.IsNotExist(removeErr) {
			err = errors.Join(err, fmt.Errorf("remove config temp: %w", removeErr))
			receipt = ConfigReceipt{}
		}
	}()
	if err := tmp.Chmod(0o644); err != nil {
		return ConfigReceipt{}, errors.Join(fmt.Errorf("chmod config temp: %w", err), tmp.Close())
	}
	if _, err := tmp.Write(out); err != nil {
		return ConfigReceipt{}, errors.Join(fmt.Errorf("write config temp: %w", err), tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return ConfigReceipt{}, fmt.Errorf("close config temp: %w", err)
	}
	if err := os.Rename(tmpName, configPath); err != nil {
		return ConfigReceipt{}, fmt.Errorf("replace opencode.json: %w", err)
	}
	removeTemp = false
	return ConfigReceipt{ConfigPath: configPath, BeforeDigest: digestBytes(before), AfterDigest: digestBytes(out), EvidenceDigest: evidenceDigest, Before: before}, nil
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
