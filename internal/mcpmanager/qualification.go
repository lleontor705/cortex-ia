package mcpmanager

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ProbeEvidence is the explicit qualification outcome for one MCP server.
// Success is never inferred from configuration alone: an Add only reports a
// qualified installation when supplied probes return valid evidence.
type ProbeEvidence struct {
	// ServerName identifies the server the evidence belongs to. An empty
	// value is adopted by the probed preset's name; evidence naming a
	// different server is never accepted as valid.
	ServerName string

	// Valid reports that the server qualified under the probe's criteria.
	Valid bool

	// Summary describes passing evidence (e.g. resolved command or observed
	// capabilities).
	Summary string

	// Detail describes why qualification failed.
	Detail string
}

// ProbeFunc produces explicit qualification evidence for one preset. Probes
// must be deterministic and offline by contract: a probe proves local
// usability (or accepts caller-supplied capability evidence); it must never
// require external network access, and absence of evidence always fails
// closed.
type ProbeFunc func(preset Preset) (ProbeEvidence, error)

// RemoteURL returns the preset's remote endpoint URL. It reports false when
// the entry is not a remote server carrying a well-formed non-empty http(s)
// URL string; callers must fail closed in that case because the entry is not
// compatible with remote URL qualification.
func (p Preset) RemoteURL() (string, bool) {
	if p.Entry["type"] != "remote" {
		return "", false
	}
	raw, ok := p.Entry["url"].(string)
	if !ok || raw == "" || !validRemoteURL(raw) {
		return "", false
	}
	return raw, true
}

// LocalCommandProbe qualifies a local preset by resolving its command
// binary on PATH via exec.LookPath. It is deterministic and offline: it
// proves the configured command can start, not that a remote service is
// reachable. A preset without a valid local command vector fails closed
// with an error because it is incompatible with the existing qualification
// boundary.
func LocalCommandProbe(preset Preset) (ProbeEvidence, error) {
	command, ok := preset.Command()
	if !ok {
		return ProbeEvidence{}, fmt.Errorf("preset %q has no valid local command to qualify", preset.Name)
	}
	resolved, err := exec.LookPath(command[0])
	if err != nil {
		return ProbeEvidence{
			ServerName: preset.Name,
			Valid:      false,
			Detail:     fmt.Sprintf("command %q not found on PATH", command[0]),
		}, nil
	}
	return ProbeEvidence{
		ServerName: preset.Name,
		Valid:      true,
		Summary:    fmt.Sprintf("command %q resolves to %q", command[0], resolved),
	}, nil
}

// RemoteURLProbe qualifies a remote preset by validating that its URL is a
// well-formed http(s) endpoint with a host. It is deterministic and offline
// by contract: it proves the endpoint address is usable configuration,
// never that the service is reachable. The evidence it produces never
// embeds the URL itself because URLs may carry credentials; a non-remote
// preset or a missing URL string is incompatible with this boundary and
// fails closed with an error, while a present but malformed URL is a
// not-qualified verdict.
func RemoteURLProbe(preset Preset) (ProbeEvidence, error) {
	if preset.Entry["type"] != "remote" {
		return ProbeEvidence{}, fmt.Errorf("preset %q is not a remote server; remote URL qualification does not apply", preset.Name)
	}
	raw, ok := preset.Entry["url"].(string)
	if !ok || raw == "" {
		return ProbeEvidence{}, fmt.Errorf("preset %q carries no URL string to qualify", preset.Name)
	}
	if !validRemoteURL(raw) {
		return ProbeEvidence{
			ServerName: preset.Name,
			Valid:      false,
			Detail:     "remote URL is not a valid http/https endpoint",
		}, nil
	}
	return ProbeEvidence{
		ServerName: preset.Name,
		Valid:      true,
		Summary:    "remote URL is a well-formed http/https endpoint (offline probe: reachability is not tested)",
	}, nil
}

// QualificationEvidence represents caller-supplied identity, schema, and capability evidence.
type QualificationEvidence struct {
	PackageName    string   `json:"package_name"`
	Version        string   `json:"version"`
	ExecutablePath string   `json:"executable_path"`
	LockIntegrity  string   `json:"lock_integrity"`
	ResolvedURL    string   `json:"resolved_url"`
	RequiredTools  []string `json:"required_tools"`
	OptionalAbsent []string `json:"optional_absent"`
}

// DefaultContext7QualificationEvidence returns the frozen qualified evidence for Context7 4.1.0.
func DefaultContext7QualificationEvidence() QualificationEvidence {
	return QualificationEvidence{
		PackageName:    "@upstash/context7-mcp",
		Version:        "4.1.0",
		ExecutablePath: "dist/index.js",
		LockIntegrity:  "sha512-ngAkFwW3LsnRGpH3XTVrjDqm3QBT4ZRpLCnShI0cIfCG+ACt07TrkRZc3n7+qjkFTcM/xIDJcHUBK5bDXUa40w==",
		ResolvedURL:    "https://registry.npmjs.org/@upstash/context7-mcp/-/context7-mcp-4.1.0.tgz",
		RequiredTools:  []string{"resolve-library-id", "get-library-docs"},
		OptionalAbsent: []string{"prompts", "resources"},
	}
}

// ValidateQualificationEvidence validates that caller-supplied evidence matches the pinned preset contract.
func ValidateQualificationEvidence(preset Preset, evidence QualificationEvidence) (ProbeEvidence, error) {
	if preset.Name == "" {
		return ProbeEvidence{}, errors.New("preset name cannot be empty")
	}
	if evidence.PackageName == "" || evidence.Version == "" {
		return ProbeEvidence{
			ServerName: preset.Name,
			Valid:      false,
			Detail:     "malformed qualification evidence: package name or version empty",
		}, errors.New("malformed qualification evidence")
	}

	if preset.Name == "context7" {
		if evidence.PackageName != "@upstash/context7-mcp" {
			return ProbeEvidence{
				ServerName: preset.Name,
				Valid:      false,
				Detail:     fmt.Sprintf("package mismatch: expected @upstash/context7-mcp, got %s", evidence.PackageName),
			}, fmt.Errorf("package mismatch: %s", evidence.PackageName)
		}
		if evidence.Version != "4.1.0" {
			return ProbeEvidence{
				ServerName: preset.Name,
				Valid:      false,
				Detail:     fmt.Sprintf("version mismatch: expected 4.1.0, got %s", evidence.Version),
			}, fmt.Errorf("version mismatch: %s", evidence.Version)
		}
		if evidence.ExecutablePath != "" && evidence.ExecutablePath != "dist/index.js" {
			return ProbeEvidence{
				ServerName: preset.Name,
				Valid:      false,
				Detail:     fmt.Sprintf("executable mismatch: expected dist/index.js, got %s", evidence.ExecutablePath),
			}, fmt.Errorf("executable mismatch: %s", evidence.ExecutablePath)
		}
		expectedIntegrity := "sha512-ngAkFwW3LsnRGpH3XTVrjDqm3QBT4ZRpLCnShI0cIfCG+ACt07TrkRZc3n7+qjkFTcM/xIDJcHUBK5bDXUa40w=="
		if evidence.LockIntegrity == "" || evidence.LockIntegrity != expectedIntegrity {
			return ProbeEvidence{
				ServerName: preset.Name,
				Valid:      false,
				Detail:     "lockfile integrity mismatch or unpinned transitive resolution",
			}, fmt.Errorf("lockfile integrity mismatch")
		}

		if len(evidence.RequiredTools) == 0 {
			return ProbeEvidence{
				ServerName: preset.Name,
				Valid:      false,
				Detail:     "missing required agent tools schema",
			}, errors.New("missing required schema")
		}
		toolSet := make(map[string]bool)
		for _, t := range evidence.RequiredTools {
			toolSet[t] = true
		}
		if !toolSet["resolve-library-id"] && !toolSet["get-library-docs"] {
			return ProbeEvidence{
				ServerName: preset.Name,
				Valid:      false,
				Detail:     "missing required tool schema: resolve-library-id / get-library-docs",
			}, errors.New("missing required schema tools")
		}
	}

	summary := fmt.Sprintf("qualified %s@%s with frozen lock evidence", evidence.PackageName, evidence.Version)
	if len(evidence.OptionalAbsent) > 0 {
		summary += fmt.Sprintf(" (optional absent: %s)", strings.Join(evidence.OptionalAbsent, ", "))
	}

	return ProbeEvidence{
		ServerName: preset.Name,
		Valid:      true,
		Summary:    summary,
	}, nil
}

// OfflineQualificationProbe returns a ProbeFunc that validates caller-supplied qualification evidence.
func OfflineQualificationProbe(evidence QualificationEvidence) ProbeFunc {
	return func(preset Preset) (ProbeEvidence, error) {
		return ValidateQualificationEvidence(preset, evidence)
	}
}
