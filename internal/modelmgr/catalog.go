package modelmgr

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// Catalog source identifiers. "none" is a first-class acquisition result, not
// an error condition.
const (
	CatalogSourceDaemon    = "daemon-api"
	CatalogSourceOpencode2 = "opencode2-models"
	CatalogSourceNone      = "none"
)

// defaultDaemonBaseURL is the documented OpenCode v2 background daemon default.
const defaultDaemonBaseURL = "http://localhost:4096"

// opencodeBinEnv names the environment override that pins the OpenCode CLI used
// for catalog and doctor acquisition. It is authoritative when set, so both
// surfaces resolve the same binary.
const opencodeBinEnv = "CORTEX_IA_OPENCODE_BIN"

// daemonCatalogTimeout bounds the daemon tier so a hung daemon degrades to the
// text tier quickly instead of stalling the caller.
const daemonCatalogTimeout = 2 * time.Second

// daemonCatalogBodyLimit bounds how much of a daemon response is buffered so a
// pathological payload cannot exhaust memory.
const daemonCatalogBodyLimit = 8 << 20

// catalogEntryLimit bounds how many entries a catalog retains; overflow is
// disclosed through Catalog.Truncated.
const catalogEntryLimit = 512

// CatalogEntry is one selectable provider/model plus its effort variants.
// Provider is the leading '/'-segment while Model keeps any further segments:
// openrouter/anthropic/claude-sonnet-4.5 splits into provider "openrouter" and
// model "anthropic/claude-sonnet-4.5". Acquired variants are deduplicated and
// sorted; a static nan vocabulary keeps the order it declares because that order
// is the effort scale. Meta is additive static capability metadata and is
// present only for provider nan entries, so every other provider keeps its
// original receipt shape.
type CatalogEntry struct {
	Provider string        `json:"provider"`
	Model    string        `json:"model"`
	Variants []string      `json:"variants"`
	Meta     *NanModelMeta `json:"meta,omitempty"`
}

// Catalog is the typed acquisition result. Truncated is set when the source
// yielded more than catalogEntryLimit entries; Source is always one of the
// CatalogSource identifiers. Entries is never nil, so receipts stay stable.
type Catalog struct {
	Entries   []CatalogEntry `json:"entries"`
	Truncated bool           `json:"truncated"`
	Source    string         `json:"source"`
}

// CatalogOptions carries the injectable acquisition seams. A nil RoundTripper
// uses http.DefaultTransport; an empty DaemonBaseURL is discovered from
// `opencode service status` when a RunCommand is present, then falls back to the
// documented default; a nil RunCommand skips discovery and the text tier.
type CatalogOptions struct {
	DaemonBaseURL string
	RoundTripper  http.RoundTripper
	RunCommand    CommandRunner
}

// daemonModel is the bounded decode target for GET /api/model. Unknown members
// are ignored so additive upstream evolution cannot break acquisition. Variant
// settings/headers/body are deliberately absent: only Variant.id is retained,
// because those members can carry secrets.
type daemonModel struct {
	ModelID    string          `json:"modelID"`
	ProviderID string          `json:"providerID"`
	Enabled    bool            `json:"enabled"`
	Variants   []daemonVariant `json:"variants"`
}

type daemonVariant struct {
	ID string `json:"id"`
}

// opencodeCandidates returns the CLI names to try, in resolution order. An
// explicit CORTEX_IA_OPENCODE_BIN is authoritative and suppresses the fallback;
// otherwise the v2 name is tried before the installed v1 name.
func opencodeCandidates() []string {
	if override := strings.TrimSpace(os.Getenv(opencodeBinEnv)); override != "" {
		return []string{override}
	}
	return []string{"opencode2", "opencode"}
}

// runOpencodeCommand runs `<binary> args...` through the injected runner,
// trying each resolved binary until one succeeds. It returns the combined
// output and the binary that produced it so a caller can label its receipt.
func runOpencodeCommand(run CommandRunner, args ...string) ([]byte, string, error) {
	var lastErr error
	for _, binary := range opencodeCandidates() {
		output, err := run(binary, args...)
		if err == nil {
			return output, binary, nil
		}
		lastErr = err
	}
	return nil, "", lastErr
}

// resolveDaemonBaseURL picks the daemon origin for the HTTP tier. An explicit
// option wins for tests and embedders; otherwise `opencode service status` is
// consulted, and any failure falls back to the documented default.
func resolveDaemonBaseURL(opts CatalogOptions) string {
	if configured := strings.TrimRight(opts.DaemonBaseURL, "/"); configured != "" {
		return configured
	}
	if opts.RunCommand != nil {
		if discovered := discoverDaemonBaseURL(opts.RunCommand); discovered != "" {
			return discovered
		}
	}
	return defaultDaemonBaseURL
}

// discoverDaemonBaseURL reads the background server origin from
// `opencode service status` output. It returns "" for any failure so a missing
// daemon binary or an unrecognized status layout degrades to the default.
func discoverDaemonBaseURL(run CommandRunner) string {
	output, _, err := runOpencodeCommand(run, "service", "status")
	if err != nil {
		return ""
	}
	return parseDaemonBaseURL(string(output))
}

// parseDaemonBaseURL extracts the first http(s) origin from status output,
// discarding any path or query so the caller appends its own API path.
func parseDaemonBaseURL(output string) string {
	for _, field := range strings.Fields(output) {
		parsed, err := url.Parse(strings.Trim(field, "\"'()[]{},;"))
		if err != nil || parsed.Host == "" {
			continue
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			continue
		}
		return parsed.Scheme + "://" + parsed.Host
	}
	return ""
}

// Catalog acquires the selectable catalog through three ordered tiers:
// daemon-api, opencode2-models, then none. No failure surfaces an error:
// exhausting every tier yields Catalog{Source: CatalogSourceNone} with zero
// entries so free-form model input stays fully functional.
func (m *Manager) Catalog(opts CatalogOptions) Catalog {
	if entries, ok := fetchDaemonCatalog(opts); ok {
		return buildCatalog(entries, CatalogSourceDaemon)
	}
	if opts.RunCommand != nil {
		if output, _, err := runOpencodeCommand(opts.RunCommand, "models"); err == nil {
			if entries := ParseModelCatalog(output); len(entries) > 0 {
				return buildCatalog(entries, CatalogSourceOpencode2)
			}
		}
	}
	return buildCatalog(nil, CatalogSourceNone)
}

// fetchDaemonCatalog performs the daemon-api tier. It reports ok=false for
// every documented failure mode (unreachable, timeout, non-2xx, malformed
// payload) so the caller degrades to the text tier instead of failing.
func fetchDaemonCatalog(opts CatalogOptions) ([]CatalogEntry, bool) {
	base := resolveDaemonBaseURL(opts)
	transport := opts.RoundTripper
	if transport == nil {
		transport = http.DefaultTransport
	}
	client := &http.Client{Timeout: daemonCatalogTimeout, Transport: transport}
	response, err := client.Get(base + "/api/model")
	if err != nil {
		return nil, false
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, daemonCatalogBodyLimit))
	if err != nil {
		return nil, false
	}
	var models []daemonModel
	if err := json.Unmarshal(body, &models); err != nil || models == nil {
		return nil, false
	}
	entries := make([]CatalogEntry, 0, len(models))
	for _, model := range models {
		if !model.Enabled {
			continue
		}
		entry, ok := catalogEntryFromRef(model.ProviderID + "/" + model.ModelID)
		if !ok {
			continue
		}
		for _, variant := range model.Variants {
			if variant.ID == "" || tokenProblem(variant.ID, false) != "" {
				continue
			}
			entry.Variants = append(entry.Variants, variant.ID)
		}
		entries = append(entries, entry)
	}
	return entries, true
}

// catalogEntryFromRef splits a provider/model reference at the first '/',
// keeping any further '/' segments in Model.
func catalogEntryFromRef(ref string) (CatalogEntry, bool) {
	desired, err := ParseModelRef("catalog", ref)
	if err != nil {
		return CatalogEntry{}, false
	}
	return CatalogEntry{Provider: desired.Provider, Model: desired.Model}, true
}

// ParseModelCatalog is the shared lenient text-tier parser for the resolved
// OpenCode binary's `models` output. The command's layout is not a contract, so
// any token that normalizes to a shape-valid reference is accepted. Tokens with
// the same provider/model merge into one entry and '#variant' tokens extend that
// entry's Variants. Entries are deduplicated and deterministically ordered.
func ParseModelCatalog(output []byte) []CatalogEntry {
	raw := make([]CatalogEntry, 0, 16)
	for _, line := range strings.Split(string(output), "\n") {
		for _, field := range strings.Fields(line) {
			normalized := normalizeModelField(field)
			if !strings.Contains(normalized, "/") {
				continue
			}
			desired, err := ParseModelRef("opencode2", normalized)
			if err != nil {
				continue
			}
			entry := CatalogEntry{Provider: desired.Provider, Model: desired.Model}
			if desired.Variant != "" {
				entry.Variants = []string{desired.Variant}
			}
			raw = append(raw, entry)
		}
	}
	return normalizeEntries(raw)
}

// catalogReferences projects entries into compact references for the doctor
// cross-check: one provider/model#variant per variant, or one bare
// provider/model for a variant-less entry. The result is sorted ascending.
func catalogReferences(entries []CatalogEntry) []string {
	refs := make([]string, 0, len(entries))
	for _, entry := range entries {
		base := entry.Provider + "/" + entry.Model
		if len(entry.Variants) == 0 {
			refs = append(refs, base)
			continue
		}
		for _, variant := range entry.Variants {
			refs = append(refs, base+"#"+variant)
		}
	}
	sort.Strings(refs)
	return refs
}

// buildCatalog normalizes entries, attaches static nan metadata, applies the
// entry cap, and stamps the source. Entries is always non-nil so JSON receipts
// carry a stable [].
func buildCatalog(entries []CatalogEntry, source string) Catalog {
	normalized := attachNanMeta(normalizeEntries(entries))
	truncated := false
	if len(normalized) > catalogEntryLimit {
		normalized = normalized[:catalogEntryLimit]
		truncated = true
	}
	return Catalog{Entries: normalized, Truncated: truncated, Source: source}
}

type catalogKey struct {
	provider string
	model    string
}

// normalizeEntries merges duplicate provider/model entries, unions and sorts
// their variants, and orders providers then models ascending. New slices are
// built so caller-owned entries are never mutated in place.
func normalizeEntries(entries []CatalogEntry) []CatalogEntry {
	index := make(map[catalogKey]struct{}, len(entries))
	merged := make([]CatalogEntry, 0, len(entries))
	variants := make(map[catalogKey]map[string]struct{}, len(entries))
	for _, entry := range entries {
		key := catalogKey{provider: entry.Provider, model: entry.Model}
		if _, seen := index[key]; !seen {
			index[key] = struct{}{}
			merged = append(merged, CatalogEntry{Provider: entry.Provider, Model: entry.Model})
			variants[key] = make(map[string]struct{})
		}
		for _, variant := range entry.Variants {
			if variant == "" {
				continue
			}
			variants[key][variant] = struct{}{}
		}
	}
	for i := range merged {
		set := variants[catalogKey{provider: merged[i].Provider, model: merged[i].Model}]
		list := make([]string, 0, len(set))
		for variant := range set {
			list = append(list, variant)
		}
		sort.Strings(list)
		merged[i].Variants = list
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Provider != merged[j].Provider {
			return merged[i].Provider < merged[j].Provider
		}
		return merged[i].Model < merged[j].Model
	})
	return merged
}
