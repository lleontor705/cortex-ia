package state

import (
	"fmt"
	"strings"
)

// ProviderV2 records one managed OpenCode custom provider. It mirrors MCPV2:
// only non-secret semantic identity and a sanitized digest are persisted, so
// no API key, header, or options value can reach the state or lock file.
type ProviderV2 struct {
	Name string `json:"name"`
	// ConfigPath is the OpenCode config file relative to the OpenCode root.
	ConfigPath string `json:"config_path"`
	// Models lists the catalog model ids materialized for the provider.
	Models []string `json:"models"`
	// SemanticDigest is the identity-only provider digest: the encoding is
	// owned by the install service over provider identity with every secret
	// (options.apiKey, headers) stripped before hashing, and state stores it
	// verbatim so token rotation never invalidates ownership.
	SemanticDigest string    `json:"semantic_digest"`
	Ownership      Ownership `json:"ownership"`
}

// validateProviders enforces provider identity uniqueness and canonical config
// paths. Provider records carry no configuration values, so only identity
// fields are inspected; model ids are opaque catalog labels.
func validateProviders(providers []ProviderV2) error {
	fold := caseInsensitivePaths()
	seenNames := make(map[string]bool, len(providers))
	seenNamesFolded := make(map[string]bool, len(providers))
	seenConfigFolded := make(map[string]string, len(providers))
	for i, p := range providers {
		field := fmt.Sprintf("providers[%d]", i)
		if strings.TrimSpace(p.Name) == "" {
			return &ValidationError{Field: field + ".name", Reason: "empty"}
		}
		if seenNames[p.Name] {
			return &ValidationError{Field: field + ".name", Reason: "duplicate " + p.Name}
		}
		seenNames[p.Name] = true
		if fold {
			key := strings.ToLower(p.Name)
			if seenNamesFolded[key] {
				return &ValidationError{Field: field + ".name",
					Reason: "case-colliding duplicate " + p.Name}
			}
			seenNamesFolded[key] = true
		}
		if err := validateRelPath(field+".config_path", p.ConfigPath); err != nil {
			return err
		}
		// Managed providers may share one config file, so an exactly repeated
		// config path is canonical; two spellings folding to the same file on
		// case-insensitive platforms are inconsistent evidence and fail closed.
		if fold {
			key := strings.ToLower(p.ConfigPath)
			if first, seen := seenConfigFolded[key]; seen {
				if first != p.ConfigPath {
					return &ValidationError{Field: field + ".config_path",
						Reason: "case-colliding duplicate " + p.ConfigPath}
				}
			} else {
				seenConfigFolded[key] = p.ConfigPath
			}
		}
		if strings.TrimSpace(p.SemanticDigest) == "" {
			return &ValidationError{Field: field + ".semantic_digest", Reason: "empty"}
		}
		if !validOwnership[p.Ownership] {
			return &ValidationError{Field: field + ".ownership", Reason: "unknown ownership " + string(p.Ownership)}
		}
	}
	return nil
}
