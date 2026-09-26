// Package providermgr owns the per-provider JSON catalogs stored under the
// providers/ directory of the Cortex-IA state root. A catalog is declarative
// data: it names a provider, the OpenCode npm package and base URL that serve
// it, and the chat models with their reasoning-effort vocabularies. The
// package materializes the embedded Nan catalog only when the provider file is
// absent, validates catalogs strictly with typed fail-closed errors that never
// echo field values, and returns defensive copies so callers can never mutate
// shared catalog state.
package providermgr

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SchemaV1 is the only catalog schema version this package understands. A
// catalog declaring any other value fails closed before its contents are used.
const SchemaV1 = "cortex-ia/providers/v1"

const (
	seedProviderID   = "nan"
	providersDirName = "providers"
	providersDirMode = 0o755
	catalogFileMode  = 0o644
)

//go:embed seed/nan.json
var nanSeed []byte

// seedCatalogs pairs each embedded seed with the provider id it materializes.
var seedCatalogs = []struct {
	providerID string
	fileName   string
	content    []byte
}{
	{providerID: seedProviderID, fileName: seedProviderID + ".json", content: nanSeed},
}

// Provider is one validated provider catalog. Values are immutable by
// construction: every accessor returns a defensive copy.
type Provider struct {
	schema        string
	id            string
	name          string
	npm           string
	pkg           string
	baseURL       string
	docs          string
	effortPosture string
	models        []ModelDef
}

// ModelDef is one chat model declared by a provider catalog.
type ModelDef struct {
	id         string
	name       string
	efforts    []string
	context    string
	premium    bool
	limit      *ModelLimit
	modalities *ModelModalities
}

// ModelLimit is the numeric context and output token budget a model declares.
// It is optional: a catalog model without a limit emits no limit object, so an
// informational model (such as a non-chat utility entry) stays valid.
type ModelLimit struct {
	Context int64
	Output  int64
}

// ModelModalities is the accepted input and output media types of a model. It
// is optional and, like Limit, mirrored from the published provider matrix.
type ModelModalities struct {
	Input  []string
	Output []string
}

// InvalidCatalogError reports a provider catalog that violates the typed
// contract: an unsupported schema version, a missing or malformed field, or an
// unknown member. No operation mutates or overwrites the offending file. The
// error names the provider and field but never echoes the offending value.
type InvalidCatalogError struct {
	Provider string
	Field    string
	Reason   string
}

func (e *InvalidCatalogError) Error() string {
	return fmt.Sprintf("providermgr: provider %q catalog field %q is invalid: %s", e.Provider, e.Field, e.Reason)
}

func invalidCatalog(provider, field, reason string) error {
	return &InvalidCatalogError{Provider: provider, Field: field, Reason: reason}
}

// Schema returns the validated schema version of the catalog.
func (p Provider) Schema() string { return p.schema }

// ID returns the provider id, which is also the catalog file's base name.
func (p Provider) ID() string { return p.id }

// Name returns the provider's display name.
func (p Provider) Name() string { return p.name }

// NPM returns the OpenCode provider npm package.
func (p Provider) NPM() string { return p.npm }

// Package returns the optional OpenCode v2 runtime provider package, empty when
// the catalog declares none. A caller writing the v2 plural shape falls back to
// NPM when it is empty.
func (p Provider) Package() string { return p.pkg }

// BaseURL returns the provider API base URL.
func (p Provider) BaseURL() string { return p.baseURL }

// Docs returns the optional documentation pointer, empty when absent.
func (p Provider) Docs() string { return p.docs }

// EffortPosture returns the optional prose describing what the provider does
// with a requested reasoning effort, empty when absent.
func (p Provider) EffortPosture() string { return p.effortPosture }

// Models returns a deep copy of the declared chat models in catalog order.
// Mutating the result never affects the provider.
func (p Provider) Models() []ModelDef {
	models := make([]ModelDef, len(p.models))
	for i, model := range p.models {
		models[i] = model.clone()
	}
	return models
}

// Model returns a deep copy of the named model. It reports false when the
// model is not declared.
func (p Provider) Model(id string) (ModelDef, bool) {
	for _, model := range p.models {
		if model.id == id {
			return model.clone(), true
		}
	}
	return ModelDef{}, false
}

// ID returns the model id.
func (m ModelDef) ID() string { return m.id }

// Name returns the model's display name.
func (m ModelDef) Name() string { return m.name }

// Efforts returns a deep copy of the reasoning-effort vocabulary. The result
// is empty, never nil, when the model has no enumerable vocabulary.
func (m ModelDef) Efforts() []string {
	return append(make([]string, 0, len(m.efforts)), m.efforts...)
}

// Context returns the optional informational context size, empty when absent.
func (m ModelDef) Context() string { return m.context }

// Premium reports whether the model is premium-gated.
func (m ModelDef) Premium() bool { return m.premium }

// Limit returns the numeric context/output token budget. It reports false when
// the model declares none.
func (m ModelDef) Limit() (ModelLimit, bool) {
	if m.limit == nil {
		return ModelLimit{}, false
	}
	return *m.limit, true
}

// Modalities returns a deep copy of the accepted media types. It reports false
// when the model declares none, so a caller can omit the field entirely.
func (m ModelDef) Modalities() (ModelModalities, bool) {
	if m.modalities == nil {
		return ModelModalities{}, false
	}
	return ModelModalities{
		Input:  copyStrings(m.modalities.Input),
		Output: copyStrings(m.modalities.Output),
	}, true
}

func (m ModelDef) clone() ModelDef {
	clone := m
	clone.efforts = copyStrings(m.efforts)
	if m.limit != nil {
		limit := *m.limit
		clone.limit = &limit
	}
	if m.modalities != nil {
		clone.modalities = &ModelModalities{
			Input:  copyStrings(m.modalities.Input),
			Output: copyStrings(m.modalities.Output),
		}
	}
	return clone
}

func copyStrings(values []string) []string {
	return append(make([]string, 0, len(values)), values...)
}

// StateRoot resolves the Cortex-IA state root for the given user home. An
// explicit CORTEX_IA_HOME wins; otherwise the root is <homeDir>/.cortex-ia.
func StateRoot(homeDir string) string {
	if override := strings.TrimSpace(os.Getenv("CORTEX_IA_HOME")); override != "" {
		absolute, err := filepath.Abs(override)
		if err != nil {
			return filepath.Clean(override)
		}
		return filepath.Clean(absolute)
	}
	return filepath.Join(homeDir, ".cortex-ia")
}

// Load materializes absent provider seeds under <StateRoot>/providers and
// returns every validated catalog sorted by provider id. Seeding is
// idempotent: an existing file — valid or malformed — is never overwritten,
// and a malformed catalog fails closed with *InvalidCatalogError.
func Load(homeDir string) ([]Provider, error) {
	providersDir := filepath.Join(StateRoot(homeDir), providersDirName)
	if err := os.MkdirAll(providersDir, providersDirMode); err != nil {
		return nil, fmt.Errorf("create providers directory: %w", err)
	}
	if err := seedAbsent(providersDir); err != nil {
		return nil, err
	}
	return loadDir(providersDir)
}

func seedAbsent(providersDir string) error {
	for _, seed := range seedCatalogs {
		if err := writeIfAbsent(filepath.Join(providersDir, seed.fileName), seed.content); err != nil {
			return fmt.Errorf("seed provider catalog %q: %w", seed.providerID, err)
		}
	}
	return nil
}

func writeIfAbsent(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, catalogFileMode)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func loadDir(providersDir string) ([]Provider, error) {
	entries, err := os.ReadDir(providersDir)
	if err != nil {
		return nil, fmt.Errorf("read providers directory: %w", err)
	}
	providers := make([]Provider, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isCatalogFile(entry.Name()) {
			continue
		}
		providerID := strings.TrimSuffix(entry.Name(), ".json")
		raw, err := os.ReadFile(filepath.Join(providersDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read provider catalog %q: %w", providerID, err)
		}
		provider, err := parseProvider(providerID, raw)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].id < providers[j].id })
	return providers, nil
}

// isCatalogFile reports whether a directory entry is a provider catalog. It
// skips dot-prefixed names so AppleDouble sidecars (`._name.json`) that exFAT
// and network volumes synthesize are never parsed as catalogs.
func isCatalogFile(name string) bool {
	return !strings.HasPrefix(name, ".") && filepath.Ext(name) == ".json"
}

var catalogFields = map[string]bool{
	"schema":         true,
	"id":             true,
	"name":           true,
	"npm":            true,
	"package":        true,
	"baseURL":        true,
	"docs":           true,
	"effort_posture": true,
	"models":         true,
}

var modelFields = map[string]bool{
	"id":         true,
	"name":       true,
	"efforts":    true,
	"context":    true,
	"premium":    true,
	"limit":      true,
	"modalities": true,
}

func parseProvider(providerID string, raw []byte) (Provider, error) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return Provider{}, invalidCatalog(providerID, "catalog", "must be a JSON object")
	}
	for key := range root {
		if !catalogFields[key] {
			return Provider{}, invalidCatalog(providerID, key, "is not a known catalog field")
		}
	}

	schema, ok := root["schema"].(string)
	if !ok || schema != SchemaV1 {
		return Provider{}, invalidCatalog(providerID, "schema", "must declare a supported schema version")
	}

	id, err := requiredString(providerID, root, "id", "id")
	if err != nil {
		return Provider{}, err
	}
	name, err := requiredString(providerID, root, "name", "name")
	if err != nil {
		return Provider{}, err
	}
	npm, err := requiredString(providerID, root, "npm", "npm")
	if err != nil {
		return Provider{}, err
	}
	pkg, err := optionalString(providerID, root, "package", "package")
	if err != nil {
		return Provider{}, err
	}
	baseURL, err := requiredString(providerID, root, "baseURL", "baseURL")
	if err != nil {
		return Provider{}, err
	}
	docs, err := optionalString(providerID, root, "docs", "docs")
	if err != nil {
		return Provider{}, err
	}
	posture, err := optionalString(providerID, root, "effort_posture", "effort_posture")
	if err != nil {
		return Provider{}, err
	}

	models, err := parseModels(providerID, root["models"])
	if err != nil {
		return Provider{}, err
	}

	return Provider{
		schema:        schema,
		id:            id,
		name:          name,
		npm:           npm,
		pkg:           pkg,
		baseURL:       baseURL,
		docs:          docs,
		effortPosture: posture,
		models:        models,
	}, nil
}

func parseModels(providerID string, raw any) ([]ModelDef, error) {
	if raw == nil {
		return nil, invalidCatalog(providerID, "models", "is required")
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, invalidCatalog(providerID, "models", "must be an array")
	}
	if len(items) == 0 {
		return nil, invalidCatalog(providerID, "models", "must declare at least one model")
	}

	models := make([]ModelDef, 0, len(items))
	seen := make(map[string]bool, len(items))
	for index, item := range items {
		model, err := parseModel(providerID, index, item)
		if err != nil {
			return nil, err
		}
		if seen[model.id] {
			return nil, invalidCatalog(providerID, modelFieldPath(index, "id"), "must be unique across models")
		}
		seen[model.id] = true
		models = append(models, model)
	}
	return models, nil
}

func parseModel(providerID string, index int, item any) (ModelDef, error) {
	object, ok := item.(map[string]any)
	if !ok {
		return ModelDef{}, invalidCatalog(providerID, fmt.Sprintf("models[%d]", index), "must be a JSON object")
	}
	for key := range object {
		if !modelFields[key] {
			return ModelDef{}, invalidCatalog(providerID, modelFieldPath(index, key), "is not a known model field")
		}
	}

	id, err := requiredString(providerID, object, "id", modelFieldPath(index, "id"))
	if err != nil {
		return ModelDef{}, err
	}
	name, err := requiredString(providerID, object, "name", modelFieldPath(index, "name"))
	if err != nil {
		return ModelDef{}, err
	}

	efforts, err := parseEfforts(providerID, object, index)
	if err != nil {
		return ModelDef{}, err
	}
	context, err := optionalString(providerID, object, "context", modelFieldPath(index, "context"))
	if err != nil {
		return ModelDef{}, err
	}
	premium, err := optionalBool(providerID, object, "premium", modelFieldPath(index, "premium"))
	if err != nil {
		return ModelDef{}, err
	}
	limit, err := parseLimit(providerID, object, index)
	if err != nil {
		return ModelDef{}, err
	}
	modalities, err := parseModalities(providerID, object, index)
	if err != nil {
		return ModelDef{}, err
	}

	return ModelDef{
		id:         id,
		name:       name,
		efforts:    efforts,
		context:    context,
		premium:    premium,
		limit:      limit,
		modalities: modalities,
	}, nil
}

func parseLimit(providerID string, object map[string]any, index int) (*ModelLimit, error) {
	field := modelFieldPath(index, "limit")
	raw, present := object["limit"]
	if !present || raw == nil {
		return nil, nil
	}
	limitObject, ok := raw.(map[string]any)
	if !ok {
		return nil, invalidCatalog(providerID, field, "must be a JSON object")
	}
	for key := range limitObject {
		if key != "context" && key != "output" {
			return nil, invalidCatalog(providerID, field+"."+key, "is not a known limit field")
		}
	}
	context, err := requiredPositiveInt(providerID, limitObject, "context", field+".context")
	if err != nil {
		return nil, err
	}
	output, err := requiredPositiveInt(providerID, limitObject, "output", field+".output")
	if err != nil {
		return nil, err
	}
	return &ModelLimit{Context: context, Output: output}, nil
}

func parseModalities(providerID string, object map[string]any, index int) (*ModelModalities, error) {
	field := modelFieldPath(index, "modalities")
	raw, present := object["modalities"]
	if !present || raw == nil {
		return nil, nil
	}
	modalitiesObject, ok := raw.(map[string]any)
	if !ok {
		return nil, invalidCatalog(providerID, field, "must be a JSON object")
	}
	for key := range modalitiesObject {
		if key != "input" && key != "output" {
			return nil, invalidCatalog(providerID, field+"."+key, "is not a known modalities field")
		}
	}
	input, err := requiredMediaTypes(providerID, modalitiesObject, "input", field+".input")
	if err != nil {
		return nil, err
	}
	output, err := requiredMediaTypes(providerID, modalitiesObject, "output", field+".output")
	if err != nil {
		return nil, err
	}
	return &ModelModalities{Input: input, Output: output}, nil
}

func requiredPositiveInt(providerID string, object map[string]any, key, field string) (int64, error) {
	raw, present := object[key]
	if !present {
		return 0, invalidCatalog(providerID, field, "is required")
	}
	number, ok := raw.(float64)
	if !ok || number <= 0 || number != math.Trunc(number) {
		return 0, invalidCatalog(providerID, field, "must be a positive integer")
	}
	return int64(number), nil
}

func requiredMediaTypes(providerID string, object map[string]any, key, field string) ([]string, error) {
	raw, present := object[key]
	if !present {
		return nil, invalidCatalog(providerID, field, "is required")
	}
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil, invalidCatalog(providerID, field, "must be a non-empty array of strings")
	}
	media := make([]string, 0, len(items))
	for position, item := range items {
		value, ok := item.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return nil, invalidCatalog(providerID, fmt.Sprintf("%s[%d]", field, position), "must be a non-empty string")
		}
		media = append(media, value)
	}
	return media, nil
}

func parseEfforts(providerID string, object map[string]any, index int) ([]string, error) {
	field := modelFieldPath(index, "efforts")
	raw, present := object["efforts"]
	if !present {
		return nil, invalidCatalog(providerID, field, "is required")
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, invalidCatalog(providerID, field, "must be an array of strings")
	}
	efforts := make([]string, 0, len(items))
	for position, item := range items {
		value, ok := item.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return nil, invalidCatalog(providerID, fmt.Sprintf("%s[%d]", field, position), "must be a non-empty string")
		}
		efforts = append(efforts, value)
	}
	return efforts, nil
}

func requiredString(providerID string, object map[string]any, key, field string) (string, error) {
	value, ok := object[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", invalidCatalog(providerID, field, "must be a non-empty string")
	}
	return value, nil
}

func optionalString(providerID string, object map[string]any, key, field string) (string, error) {
	raw, present := object[key]
	if !present || raw == nil {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", invalidCatalog(providerID, field, "must be a string")
	}
	return value, nil
}

func optionalBool(providerID string, object map[string]any, key, field string) (bool, error) {
	raw, present := object[key]
	if !present || raw == nil {
		return false, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, invalidCatalog(providerID, field, "must be a boolean")
	}
	return value, nil
}

func modelFieldPath(index int, field string) string {
	return fmt.Sprintf("models[%d].%s", index, field)
}
