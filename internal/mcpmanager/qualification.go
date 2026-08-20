package mcpmanager

import (
	"fmt"
	"os/exec"
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
