package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/providermgr"
	"github.com/lleontor705/cortex-ia/internal/state"
)

// ErrProviderUnmanaged is returned only for a genuinely irreconcilable winning
// config: an existing provider entry that is not a JSON object and so cannot be
// merged into. A hand-authored provider block that is an object is adopted in
// place instead of refused.
var ErrProviderUnmanaged = errors.New("install service: provider block exists in the winning config with a shape that cannot be merged")

// The provider config member names mirror the shape internal/modelmgr reads
// (provider.<id>.models.<model>.variants) so the writer and the reader agree
// on exactly one encoding. providerKey is the documented singular container
// while providersKey is the OpenCode v2 plural container; only one is ever
// written for a given provider.
const (
	providerKey        = "provider"
	providersKey       = "providers"
	npmKey             = "npm"
	packageKey         = "package"
	nameKey            = "name"
	optionsKey         = "options"
	settingsKey        = "settings"
	baseURLKey         = "baseURL"
	apiKeyKey          = "apiKey"
	modelsKey          = "models"
	variantsKey        = "variants"
	reasoningEffortKey = "reasoningEffort"
	limitKey           = "limit"
	contextKey         = "context"
	outputKey          = "output"
	modalitiesKey      = "modalities"
	capabilitiesKey    = "capabilities"
	toolsKey           = "tools"
	inputKey           = "input"
	idKey              = "id"
)

const (
	providerDigestVersion = 1
	providerDigestPrefix  = "pvd"
	providerDigestDomain  = "cortex-ia/install/provider-identity-digest\n"
	providerBackupPrefix  = "provider"
)

// Mutate seams so fault injection can prove the backup-first ordering and the
// both-file restore path without leaving test hooks in production behavior.
var (
	providerTwinMutate   = filemerge.MutateJSONFile
	providerWinnerMutate = filemerge.MutateJSONFile
)

// ProviderCatalogLimit is the numeric context/output token budget a catalog
// model declares.
type ProviderCatalogLimit struct {
	Context int64 `json:"context"`
	Output  int64 `json:"output"`
}

// ProviderCatalogModalities is the accepted input and output media types a
// catalog model declares.
type ProviderCatalogModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// ProviderCatalogVariant is one effort variant the installer authors for a
// model, mirroring the emitted {id, settings.reasoningEffort} shape. A
// catalog-authored variant always carries ID == ReasoningEffort.
type ProviderCatalogVariant struct {
	ID              string `json:"id"`
	ReasoningEffort string `json:"reasoning_effort"`
}

// ProviderCatalogModel is one sanitized model projection of a catalog.
type ProviderCatalogModel struct {
	ID         string                     `json:"id"`
	Name       string                     `json:"name"`
	Efforts    []string                   `json:"efforts"`
	Limit      *ProviderCatalogLimit      `json:"limit,omitempty"`
	Modalities *ProviderCatalogModalities `json:"modalities,omitempty"`
	Variants   []ProviderCatalogVariant   `json:"variants,omitempty"`
}

// ProviderCatalogEntry is one sanitized provider projection; it never carries
// any endpoint secret. NPM and Package are the runtime package keys for the
// singular and plural config containers respectively, and the installer emits
// exactly one of them for the resolved container.
type ProviderCatalogEntry struct {
	ID      string                 `json:"id"`
	Name    string                 `json:"name"`
	NPM     string                 `json:"npm"`
	Package string                 `json:"package"`
	BaseURL string                 `json:"base_url"`
	Models  []ProviderCatalogModel `json:"models"`
}

// ProviderCatalogReport is the read-only listing of every provider catalog.
type ProviderCatalogReport struct {
	Providers []ProviderCatalogEntry `json:"providers"`
}

// ProviderInstallOptions is the explicit per-call intent for provider install
// and preview. The token is a transient argument: it travels only into
// provider.<id>.options.apiKey and is never representable on any receipt.
type ProviderInstallOptions struct {
	ProviderID  string
	Token       string
	DryRun      bool
	LockTimeout time.Duration
	Now         func() time.Time
}

func (o ProviderInstallOptions) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

// ProviderInstallReceipt is the typed outcome of one provider install or
// preview. It deliberately has no token field; ReconciledTwinPath is set only
// when a twin cleanup executed, and both mutations share one BackupID.
type ProviderInstallReceipt struct {
	Action string `json:"action"`
	// Entry, Container, RuntimeMember, and RuntimeKey disclose the loaded
	// catalog content and the resolved container shape the installer will write,
	// so a preview shows exactly what an install emits.
	Entry              *ProviderCatalogEntry `json:"entry,omitempty"`
	Container          string                `json:"container,omitempty"`
	RuntimeMember      string                `json:"runtime_member,omitempty"`
	RuntimeKey         string                `json:"runtime_key,omitempty"`
	Provider           string                `json:"provider"`
	ConfigPath         string                `json:"config_path,omitempty"`
	ReconciledTwinPath string                `json:"reconciled_twin_path,omitempty"`
	ModelsWritten      int                   `json:"models_written"`
	VariantsWritten    int                   `json:"variants_written"`
	DryRun             bool                  `json:"dry_run"`
	BackupID           string                `json:"backup_id,omitempty"`
	RollbackCmd        string                `json:"rollback_cmd,omitempty"`
	Detail             []string              `json:"detail,omitempty"`
}

// providerInstallPlan is the fully resolved decision for one call: the winner
// and twin paths, whether each needs mutation, and the exact overlay bytes.
type providerInstallPlan struct {
	meta          state.MetadataV2
	provider      providermgr.Provider
	container     string
	runtimeMember string
	runtimeKey    string
	winnerPath    string
	twinPath      string
	winnerRel     string
	twinRel       string
	winnerNeeds   bool
	reconcile     bool
	adopt         bool
	overlay       []byte
	models        []string
	variants      int
}

// ProviderCatalog loads and lists every provider catalog under the state root.
// It honors CORTEX_IA_HOME exactly like providermgr.Load.
func (s *Service) ProviderCatalog() (*ProviderCatalogReport, error) {
	providers, err := providermgr.Load(s.homeDir)
	if err != nil {
		return nil, err
	}
	report := &ProviderCatalogReport{Providers: make([]ProviderCatalogEntry, 0, len(providers))}
	for _, provider := range providers {
		report.Providers = append(report.Providers, catalogEntryProjection(provider))
	}
	return report, nil
}

// catalogEntryProjection is the single catalog-to-display projection shared by
// the listing and the install/preview receipt, so the installer always shows
// exactly what the loaded catalog holds.
func catalogEntryProjection(provider providermgr.Provider) ProviderCatalogEntry {
	entry := ProviderCatalogEntry{
		ID:      provider.ID(),
		Name:    provider.Name(),
		NPM:     provider.NPM(),
		Package: provider.Package(),
		BaseURL: provider.BaseURL(),
		Models:  make([]ProviderCatalogModel, 0, len(provider.Models())),
	}
	for _, model := range provider.Models() {
		entry.Models = append(entry.Models, catalogModelProjection(model))
	}
	return entry
}

func catalogModelProjection(model providermgr.ModelDef) ProviderCatalogModel {
	projection := ProviderCatalogModel{
		ID:      model.ID(),
		Name:    model.Name(),
		Efforts: model.Efforts(),
	}
	if limit, ok := model.Limit(); ok {
		projection.Limit = &ProviderCatalogLimit{Context: limit.Context, Output: limit.Output}
	}
	if media, ok := model.Modalities(); ok {
		projection.Modalities = &ProviderCatalogModalities{Input: media.Input, Output: media.Output}
	}
	for _, effort := range projection.Efforts {
		projection.Variants = append(projection.Variants, ProviderCatalogVariant{ID: effort, ReasoningEffort: effort})
	}
	return projection
}

// ProviderPreview resolves the provider transaction and returns the dry-run
// receipt without locking, backing up, or writing anything. When twin
// reconciliation applies it discloses the twin path, the block removal, the
// winner write, and the backup-first ordering.
func (s *Service) ProviderPreview(opts ProviderInstallOptions) (*ProviderInstallReceipt, error) {
	plan, err := s.planProvider(opts)
	if err != nil {
		return nil, err
	}
	return plan.receipt(true), nil
}

// ProviderInstall executes the provider transaction. It requires an agreed v2
// installation (the Providers ownership record is state-only evidence). The
// strict order is: plan, verified backup protecting BOTH config files, twin
// RemovePaths cleanup, winner field-wise overlay, ownership commit. An
// already-present provider performs no backup; an adopted unmanaged block
// still commits its ownership record. Any failure after the backup restores
// both files from the verified backup and leaves state.json uncommitted.
func (s *Service) ProviderInstall(opts ProviderInstallOptions) (*ProviderInstallReceipt, error) {
	if opts.DryRun {
		return s.ProviderPreview(opts)
	}

	release, err := s.lockForMutation(opts.LockTimeout)
	if err != nil {
		return nil, fmt.Errorf("provider install %q: %w", opts.ProviderID, err)
	}
	defer release()

	// Plan under the lock: another process may have committed a different
	// config or ownership record between the caller's preview and this call.
	plan, err := s.planProvider(opts)
	if err != nil {
		return nil, err
	}
	receipt := plan.receipt(false)
	if !plan.winnerNeeds && !plan.reconcile && !plan.adopt {
		return receipt, nil
	}

	now := opts.now()
	txn, err := s.beginServiceTxn(providerBackupPrefix, now)
	if err != nil {
		return receipt, fmt.Errorf("provider install %q: %w", plan.provider.ID(), err)
	}
	receipt.BackupID = txn.backupID
	receipt.RollbackCmd = "cortex-ia rollback " + txn.backupID
	stage := fmt.Sprintf("provider install %q", plan.provider.ID())
	fail := func(err error) error {
		return txn.abort(&txnStatus{}, stage, err)
	}

	if plan.reconcile {
		if err := txn.run(plan.twinPath, func() error {
			_, inner := providerTwinMutate(plan.twinPath, filemerge.JSONMutation{
				RemovePaths: [][]string{
					{providerKey, plan.provider.ID()},
					{providersKey, plan.provider.ID()},
				},
			})
			return inner
		}); err != nil {
			return receipt, fail(err)
		}
	}
	if plan.winnerNeeds {
		if err := txn.run(plan.winnerPath, func() error {
			_, inner := providerWinnerMutate(plan.winnerPath, filemerge.JSONMutation{Overlay: plan.overlay})
			return inner
		}); err != nil {
			return receipt, fail(err)
		}
	}
	record, err := plan.ownershipRecord()
	if err != nil {
		return receipt, fail(fmt.Errorf("record ownership: %w", err))
	}
	if err := txn.commitProviderState(plan.meta, upsertProviderRecord(plan.meta, record), now); err != nil {
		return receipt, fail(fmt.Errorf("commit state v2: %w", err))
	}
	if err := txn.commit(); err != nil {
		return receipt, fail(err)
	}
	return receipt, nil
}

// planProvider resolves the winner config, the twin candidate, the ownership
// gate, and the exact overlay without mutating anything.
func (s *Service) planProvider(opts ProviderInstallOptions) (*providerInstallPlan, error) {
	id := strings.TrimSpace(opts.ProviderID)
	if id == "" {
		return nil, errors.New("provider install: provider id is required")
	}
	provider, err := s.catalogProvider(id)
	if err != nil {
		return nil, err
	}
	meta, _, err := s.v2Context()
	if err != nil {
		return nil, err
	}
	winnerPath, twinPath, err := s.providerConfigPaths()
	if err != nil {
		return nil, err
	}
	winnerRel, err := providerRelPath(s.homeDir, winnerPath)
	if err != nil {
		return nil, err
	}
	twinRel, err := providerRelPath(s.homeDir, twinPath)
	if err != nil {
		return nil, err
	}
	winnerConfig, err := loadProviderConfig(winnerPath)
	if err != nil {
		return nil, err
	}
	twinConfig, err := loadProviderConfig(twinPath)
	if err != nil {
		return nil, err
	}
	winnerBlock, winnerPresent := providerConfigBlock(winnerConfig, id)
	_, twinPresent := providerConfigBlock(twinConfig, id)

	container := providerContainerKey(winnerConfig, id)
	if err := providerBlockShapeError(winnerConfig, container, id); err != nil {
		return nil, err
	}
	runtimeMember, runtimeKey := providerRuntime(provider, container)
	entry, variants := providerEntryForCatalog(provider, opts.Token, container, providerModelsObject(winnerBlock))
	overlay, err := encodeProviderOverlay(container, id, entry)
	if err != nil {
		return nil, err
	}
	overlayEntry, err := decodeOverlayEntry(overlay, container, id)
	if err != nil {
		return nil, err
	}
	return &providerInstallPlan{
		meta:          meta,
		provider:      provider,
		container:     container,
		runtimeMember: runtimeMember,
		runtimeKey:    runtimeKey,
		winnerPath:    winnerPath,
		twinPath:      twinPath,
		winnerRel:     winnerRel,
		twinRel:       twinRel,
		winnerNeeds:   !objectContains(winnerBlock, overlayEntry),
		reconcile:     twinPresent,
		adopt:         winnerPresent && !coveringProviderRecord(meta, id, winnerRel),
		overlay:       overlay,
		models:        providerModelIDs(provider),
		variants:      variants,
	}, nil
}

func (s *Service) catalogProvider(id string) (providermgr.Provider, error) {
	providers, err := providermgr.Load(s.homeDir)
	if err != nil {
		return providermgr.Provider{}, err
	}
	for _, provider := range providers {
		if provider.ID() == id {
			return provider, nil
		}
	}
	return providermgr.Provider{}, fmt.Errorf("provider install: no catalog declares provider %q", id)
}

// providerConfigPaths resolves the winning config by JSONC-else-JSON
// precedence and pairs it with the non-winning twin candidate.
func (s *Service) providerConfigPaths() (winner, twin string, err error) {
	root, err := opencodeRoot(s.homeDir)
	if err != nil {
		return "", "", err
	}
	rel := opencode.NativeLayout().ResolveConfigRelPath(s.homeDir)
	winner = filepath.Join(s.homeDir, filepath.FromSlash(rel))
	twinName := "opencode.jsonc"
	if filepath.Base(winner) == twinName {
		twinName = "opencode.json"
	}
	return winner, filepath.Join(root, twinName), nil
}

func providerRelPath(homeDir, abs string) (string, error) {
	root, err := opencodeRoot(homeDir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", fmt.Errorf("resolve provider config path: %w", err)
	}
	return filepath.ToSlash(rel), nil
}

func loadProviderConfig(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read provider config %q: %w", path, err)
	}
	object, err := filemerge.DecodeJSONObject(raw)
	if err != nil {
		return nil, fmt.Errorf("provider config %q is not a valid JSON object: %w", path, err)
	}
	return object, nil
}

// providerContainerKey resolves the single top-level container that should hold
// provider id. It prefers the container already carrying the provider (so a
// provider is never duplicated across the singular and plural containers), then
// a present singular provider map, then a present plural providers map, and
// finally defaults to the documented singular container.
func providerContainerKey(config map[string]any, id string) string {
	if _, ok := containerBlock(config, providerKey, id); ok {
		return providerKey
	}
	if _, ok := containerBlock(config, providersKey, id); ok {
		return providersKey
	}
	if _, ok := config[providerKey].(map[string]any); ok {
		return providerKey
	}
	if _, ok := config[providersKey].(map[string]any); ok {
		return providersKey
	}
	return providerKey
}

// providerConfigBlock locates provider.<id> in the resolved container.
func providerConfigBlock(config map[string]any, id string) (map[string]any, bool) {
	return containerBlock(config, providerContainerKey(config, id), id)
}

func containerBlock(config map[string]any, container, id string) (map[string]any, bool) {
	providers, ok := config[container].(map[string]any)
	if !ok {
		return nil, false
	}
	block, ok := providers[id].(map[string]any)
	return block, ok
}

// providerBlockShapeError reports the resolved container or its provider entry
// taking a non-object shape. An explicit null is treated as absent because it
// carries no content, but a scalar or array cannot be reconciled field-wise, so
// it stays a typed refusal; an object entry is adopted instead.
func providerBlockShapeError(config map[string]any, container, id string) error {
	raw, present := config[container]
	if !present || raw == nil {
		return nil
	}
	providers, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: %q is not an object", ErrProviderUnmanaged, container)
	}
	entry, present := providers[id]
	if !present || entry == nil {
		return nil
	}
	if _, isObject := entry.(map[string]any); !isObject {
		return fmt.Errorf("%w: provider %q in %q is not an object", ErrProviderUnmanaged, id, container)
	}
	return nil
}

// providerModelsObject projects the models map of an existing provider block so
// the overlay can distinguish a model that already exists from one it must add.
func providerModelsObject(block map[string]any) map[string]any {
	models, _ := block[modelsKey].(map[string]any)
	return models
}

// coveringProviderRecord reports whether an agreed managed record accredits
// provider.<id> in exactly this winner file. A missing record marks a
// hand-authored block, which install adopts and records rather than refuses.
func coveringProviderRecord(meta state.MetadataV2, id, winnerRel string) bool {
	for _, record := range meta.Providers {
		if record.Name == id && record.ConfigPath == winnerRel && record.Ownership == state.OwnershipManaged {
			return true
		}
	}
	return false
}

// providerEntryForCatalog builds the catalog-authored provider entry for one
// container shape. Variants are authored only for a model the existing block
// does not already define, so a deep merge refreshes catalog-owned fields
// (name, limit, modalities/capabilities, npm/package options) while leaving
// every user-added per-model field, model, and variant untouched. It returns
// the entry and the number of variant objects the overlay authors.
func providerEntryForCatalog(provider providermgr.Provider, token, container string, existingModels map[string]any) (map[string]any, int) {
	plural := container == providersKey
	models := make(map[string]any, len(provider.Models()))
	variants := 0
	for _, model := range provider.Models() {
		entry := catalogModelEntry(model, plural)
		if _, present := existingModels[model.ID()]; !present {
			if authored := modelVariants(model); authored != nil {
				entry[variantsKey] = authored
				variants += len(authored)
			}
		}
		models[model.ID()] = entry
	}
	member, key := providerRuntime(provider, container)
	entry := map[string]any{
		nameKey:   provider.Name(),
		modelsKey: models,
		member:    key,
	}
	if plural {
		entry[settingsKey] = map[string]any{
			baseURLKey: provider.BaseURL(),
			apiKeyKey:  token,
		}
	} else {
		entry[optionsKey] = map[string]any{
			baseURLKey: provider.BaseURL(),
			apiKeyKey:  token,
		}
	}
	return entry, variants
}

// catalogModelEntry projects the catalog-owned per-model fields for one
// container shape. The v2 plural container names media support capabilities
// with a tools flag; the singular container names it modalities.
func catalogModelEntry(model providermgr.ModelDef, plural bool) map[string]any {
	entry := map[string]any{nameKey: model.Name()}
	if limit, ok := model.Limit(); ok {
		entry[limitKey] = map[string]any{contextKey: limit.Context, outputKey: limit.Output}
	}
	if media, ok := model.Modalities(); ok {
		if plural {
			entry[capabilitiesKey] = map[string]any{
				toolsKey:  true,
				inputKey:  media.Input,
				outputKey: media.Output,
			}
		} else {
			entry[modalitiesKey] = map[string]any{
				inputKey:  media.Input,
				outputKey: media.Output,
			}
		}
	}
	return entry
}

func providerPackage(provider providermgr.Provider) string {
	if pkg := provider.Package(); pkg != "" {
		return pkg
	}
	return provider.NPM()
}

// providerRuntime selects the runtime package member and value a container
// shape writes: the singular container always writes npm, while the plural v2
// container writes package and falls back to npm when the catalog declares
// none. The installer displays this same member and value, so the preview and
// the emitted entry never diverge.
func providerRuntime(provider providermgr.Provider, container string) (member, key string) {
	if container == providersKey {
		return packageKey, providerPackage(provider)
	}
	return npmKey, provider.NPM()
}

// modelVariants materializes the {id, settings.reasoningEffort} variant shape.
// A model with no enumerable effort vocabulary gets no variants key at all.
func modelVariants(model providermgr.ModelDef) []any {
	efforts := model.Efforts()
	if len(efforts) == 0 {
		return nil
	}
	variants := make([]any, 0, len(efforts))
	for _, effort := range efforts {
		variants = append(variants, map[string]any{
			idKey: effort,
			settingsKey: map[string]any{
				reasoningEffortKey: effort,
			},
		})
	}
	return variants
}

func encodeProviderOverlay(container, id string, entry map[string]any) ([]byte, error) {
	overlay := map[string]any{container: map[string]any{id: entry}}
	data, err := json.Marshal(overlay)
	if err != nil {
		return nil, fmt.Errorf("encode provider overlay: %w", err)
	}
	return data, nil
}

// decodeOverlayEntry re-decodes the overlay through the shared JSON boundary so
// the containment comparison uses exactly the value types the merge boundary
// will observe (float64 numbers, []any arrays).
func decodeOverlayEntry(overlay []byte, container, id string) (map[string]any, error) {
	decoded, err := filemerge.DecodeJSONObject(overlay)
	if err != nil {
		return nil, err
	}
	containerObject, ok := decoded[container].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("provider overlay has no %q container", container)
	}
	entry, ok := containerObject[id].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("provider overlay has no %q entry", id)
	}
	return entry, nil
}

// objectContains reports whether container already carries every member of
// subset with an equal value. Nested objects may carry extra members because
// the field-wise merge preserves them. Numbers and arrays are compared after
// JSON decoding, so both sides share the same value types.
func objectContains(container, subset map[string]any) bool {
	for key, want := range subset {
		got, present := container[key]
		if !present {
			return false
		}
		wantObject, isObject := want.(map[string]any)
		if !isObject {
			if !reflect.DeepEqual(got, want) {
				return false
			}
			continue
		}
		gotObject, ok := got.(map[string]any)
		if !ok || !objectContains(gotObject, wantObject) {
			return false
		}
	}
	return true
}

func providerModelIDs(provider providermgr.Provider) []string {
	models := provider.Models()
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID())
	}
	return ids
}

// providerIdentityDigest is the versioned, identity-only digest that accredits
// provider ownership. It covers the catalog identity and every model effort
// (the variant ids) and is computed over the catalog, never over the written
// block, so options.apiKey and headers are not representable and token
// rotation leaves the digest unchanged.
func providerIdentityDigest(provider providermgr.Provider) (string, error) {
	type canonicalModel struct {
		ID      string   `json:"id"`
		Efforts []string `json:"efforts"`
	}
	type canonicalProvider struct {
		Version int              `json:"version"`
		ID      string           `json:"id"`
		Name    string           `json:"name"`
		NPM     string           `json:"npm"`
		BaseURL string           `json:"baseURL"`
		Models  []canonicalModel `json:"models"`
	}
	canonical := canonicalProvider{
		Version: providerDigestVersion,
		ID:      provider.ID(),
		Name:    provider.Name(),
		NPM:     provider.NPM(),
		BaseURL: provider.BaseURL(),
	}
	for _, model := range provider.Models() {
		canonical.Models = append(canonical.Models, canonicalModel{ID: model.ID(), Efforts: model.Efforts()})
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("encode provider identity: %w", err)
	}
	sum := sha256.Sum256(append([]byte(providerDigestDomain), payload...))
	return providerDigestPrefix + strconv.Itoa(providerDigestVersion) + ":" + hex.EncodeToString(sum[:]), nil
}

func (p *providerInstallPlan) ownershipRecord() (state.ProviderV2, error) {
	digest, err := providerIdentityDigest(p.provider)
	if err != nil {
		return state.ProviderV2{}, err
	}
	return state.ProviderV2{
		Name:           p.provider.ID(),
		ConfigPath:     p.winnerRel,
		Models:         p.models,
		SemanticDigest: digest,
		Ownership:      state.OwnershipManaged,
	}, nil
}

func (p *providerInstallPlan) receipt(dryRun bool) *ProviderInstallReceipt {
	entry := catalogEntryProjection(p.provider)
	receipt := &ProviderInstallReceipt{
		Action:          "installed",
		Entry:           &entry,
		Container:       p.container,
		RuntimeMember:   p.runtimeMember,
		RuntimeKey:      p.runtimeKey,
		Provider:        p.provider.ID(),
		ConfigPath:      p.winnerPath,
		ModelsWritten:   len(p.models),
		VariantsWritten: p.variants,
		DryRun:          dryRun,
	}
	switch {
	case p.adopt:
		receipt.Action = "adopted"
	case !p.winnerNeeds && !p.reconcile:
		receipt.Action = "already-present"
	}
	if p.reconcile {
		receipt.ReconciledTwinPath = p.twinPath
	}
	receipt.Detail = p.disclosure()
	return receipt
}

func (p *providerInstallPlan) disclosure() []string {
	if !p.winnerNeeds && !p.reconcile && !p.adopt {
		return []string{fmt.Sprintf("provider.%s in %s already matches the catalog definition; no changes planned", p.provider.ID(), p.winnerRel)}
	}
	lines := []string{fmt.Sprintf("backup first: capture one verified snapshot of %s and %s before any edit", p.winnerRel, p.twinRel)}
	if p.adopt {
		lines = append(lines, fmt.Sprintf("adopt: record ownership of the existing provider.%s block in %s and merge catalog fields in place", p.provider.ID(), p.winnerRel))
	}
	if p.reconcile {
		lines = append(lines, fmt.Sprintf("twin cleanup: remove the stale provider.%s block from non-winning %s", p.provider.ID(), p.twinRel))
	}
	if p.winnerNeeds {
		lines = append(lines, fmt.Sprintf("winner write: materialize provider.%s with %d models and %d variants into %s", p.provider.ID(), len(p.models), p.variants, p.winnerRel))
	}
	return lines
}

func upsertProviderRecord(meta state.MetadataV2, record state.ProviderV2) []state.ProviderV2 {
	records := make([]state.ProviderV2, 0, len(meta.Providers)+1)
	replaced := false
	for _, existing := range meta.Providers {
		if existing.Name == record.Name {
			records = append(records, record)
			replaced = true
			continue
		}
		records = append(records, existing)
	}
	if !replaced {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })
	return records
}

// commitProviderState persists the Providers record set as agreeing v2 state
// and lock documents, journaling both writes so a failure restores the exact
// pre-transaction metadata bytes. Providers are state-only evidence and are
// deliberately absent from the lock view, matching ValidateLockV2.
func (t *serviceTxn) commitProviderState(meta state.MetadataV2, records []state.ProviderV2, now time.Time) error {
	meta.Providers = records
	meta.UpdatedAt = now
	if err := t.run(state.StatePath(t.homeDir), func() error {
		return commitStateV2(t.homeDir, meta)
	}); err != nil {
		return err
	}
	lock := state.NewLockFromMetadataV2(meta)
	return t.run(state.LockPath(t.homeDir), func() error {
		return commitLockV2(t.homeDir, lock)
	})
}
