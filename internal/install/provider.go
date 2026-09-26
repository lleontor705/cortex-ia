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

// ErrProviderUnmanaged is returned when the winning config already defines the
// provider block without a covering managed ownership record. Reconciliation
// only ever touches the non-winning twin, so winner-side user content stays a
// typed refusal that mutates nothing.
var ErrProviderUnmanaged = errors.New("install service: provider block exists in the winning config without a covering managed ownership record; refusing to overwrite unmanaged configuration")

// The provider config member names mirror the shape internal/modelmgr reads
// (provider.<id>.models.<model>.variants) so the writer and the reader agree
// on exactly one encoding.
const (
	providerKey        = "provider"
	npmKey             = "npm"
	nameKey            = "name"
	optionsKey         = "options"
	baseURLKey         = "baseURL"
	apiKeyKey          = "apiKey"
	modelsKey          = "models"
	variantsKey        = "variants"
	settingsKey        = "settings"
	reasoningEffortKey = "reasoningEffort"
	replaceKey         = "__replace__"
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

// ProviderCatalogModel is one sanitized model projection of a catalog.
type ProviderCatalogModel struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Efforts []string `json:"efforts"`
}

// ProviderCatalogEntry is one sanitized provider projection; it never carries
// any endpoint secret.
type ProviderCatalogEntry struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Models []ProviderCatalogModel `json:"models"`
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
	Action             string   `json:"action"`
	Provider           string   `json:"provider"`
	ConfigPath         string   `json:"config_path,omitempty"`
	ReconciledTwinPath string   `json:"reconciled_twin_path,omitempty"`
	ModelsWritten      int      `json:"models_written"`
	VariantsWritten    int      `json:"variants_written"`
	DryRun             bool     `json:"dry_run"`
	BackupID           string   `json:"backup_id,omitempty"`
	RollbackCmd        string   `json:"rollback_cmd,omitempty"`
	Detail             []string `json:"detail,omitempty"`
}

// providerInstallPlan is the fully resolved decision for one call: the winner
// and twin paths, whether each needs mutation, and the exact overlay bytes.
type providerInstallPlan struct {
	meta        state.MetadataV2
	provider    providermgr.Provider
	winnerPath  string
	twinPath    string
	winnerRel   string
	twinRel     string
	winnerNeeds bool
	reconcile   bool
	overlay     []byte
	models      []string
	variants    int
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
		entry := ProviderCatalogEntry{ID: provider.ID(), Name: provider.Name()}
		for _, model := range provider.Models() {
			entry.Models = append(entry.Models, ProviderCatalogModel{
				ID:      model.ID(),
				Name:    model.Name(),
				Efforts: model.Efforts(),
			})
		}
		report.Providers = append(report.Providers, entry)
	}
	return report, nil
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
// strict order is: plan, ownership gate, verified backup protecting BOTH
// config files, twin RemovePaths cleanup, winner __replace__ overlay, ownership
// commit. An already-present provider performs no backup. Any failure after
// the backup restores both files from the verified backup and leaves state.json
// uncommitted.
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
	if !plan.winnerNeeds && !plan.reconcile {
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
				RemovePaths: [][]string{{providerKey, plan.provider.ID()}},
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

	if winnerPresent && !coveringProviderRecord(meta, id, winnerRel) {
		return nil, fmt.Errorf("%w: provider %q in %s", ErrProviderUnmanaged, id, winnerRel)
	}

	overlay, variants, err := buildProviderOverlay(provider, opts.Token, winnerPresent)
	if err != nil {
		return nil, err
	}
	return &providerInstallPlan{
		meta:        meta,
		provider:    provider,
		winnerPath:  winnerPath,
		twinPath:    twinPath,
		winnerRel:   winnerRel,
		twinRel:     twinRel,
		winnerNeeds: !reflect.DeepEqual(winnerBlock, buildProviderEntry(provider, opts.Token)),
		reconcile:   twinPresent,
		overlay:     overlay,
		models:      providerModelIDs(provider),
		variants:    variants,
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

// providerConfigBlock locates provider.<id> in a decoded config object.
func providerConfigBlock(config map[string]any, id string) (map[string]any, bool) {
	providers, ok := config[providerKey].(map[string]any)
	if !ok {
		return nil, false
	}
	block, ok := providers[id].(map[string]any)
	return block, ok
}

// coveringProviderRecord reports whether an agreed managed record accredits
// provider.<id> in exactly this winner file. Reconciliation never cleanses
// winner-side content, so an unmanaged winner stays refused.
func coveringProviderRecord(meta state.MetadataV2, id, winnerRel string) bool {
	for _, record := range meta.Providers {
		if record.Name == id && record.ConfigPath == winnerRel && record.Ownership == state.OwnershipManaged {
			return true
		}
	}
	return false
}

// buildProviderEntry materializes the full provider definition: the
// __replace__ payload guarantees convergence to the catalog with no orphan
// models. Variants carry exactly the {id, settings.reasoningEffort} shape and
// an empty effort vocabulary gets no variants key at all.
func buildProviderEntry(provider providermgr.Provider, token string) map[string]any {
	models := make(map[string]any, len(provider.Models()))
	for _, model := range provider.Models() {
		entry := map[string]any{nameKey: model.Name()}
		if efforts := model.Efforts(); len(efforts) > 0 {
			variants := make([]any, 0, len(efforts))
			for _, effort := range efforts {
				variants = append(variants, map[string]any{
					idKey: effort,
					settingsKey: map[string]any{
						reasoningEffortKey: effort,
					},
				})
			}
			entry[variantsKey] = variants
		}
		models[model.ID()] = entry
	}
	return map[string]any{
		npmKey:  provider.NPM(),
		nameKey: provider.Name(),
		optionsKey: map[string]any{
			baseURLKey: provider.BaseURL(),
			apiKeyKey:  token,
		},
		modelsKey: models,
	}
}

// buildProviderOverlay wraps the entry in the __replace__ sentinel only when
// the winner already defines the block: filemerge applies the sentinel to
// existing members, so an absent member takes the plain entry (a sentinel
// there would be written literally). Replacement is what converges a stale or
// rotated block with no orphan models.
func buildProviderOverlay(provider providermgr.Provider, token string, replace bool) ([]byte, int, error) {
	entry := buildProviderEntry(provider, token)
	if replace {
		entry = map[string]any{replaceKey: entry}
	}
	overlay := map[string]any{
		providerKey: map[string]any{provider.ID(): entry},
	}
	data, err := json.Marshal(overlay)
	if err != nil {
		return nil, 0, fmt.Errorf("encode provider overlay: %w", err)
	}
	return data, countProviderVariants(provider), nil
}

func countProviderVariants(provider providermgr.Provider) int {
	total := 0
	for _, model := range provider.Models() {
		total += len(model.Efforts())
	}
	return total
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
	receipt := &ProviderInstallReceipt{
		Action:          "installed",
		Provider:        p.provider.ID(),
		ConfigPath:      p.winnerPath,
		ModelsWritten:   len(p.models),
		VariantsWritten: p.variants,
		DryRun:          dryRun,
	}
	if !p.winnerNeeds && !p.reconcile {
		receipt.Action = "already-present"
	}
	if p.reconcile {
		receipt.ReconciledTwinPath = p.twinPath
	}
	receipt.Detail = p.disclosure()
	return receipt
}

func (p *providerInstallPlan) disclosure() []string {
	if !p.winnerNeeds && !p.reconcile {
		return []string{fmt.Sprintf("provider.%s in %s already matches the catalog definition; no changes planned", p.provider.ID(), p.winnerRel)}
	}
	lines := []string{fmt.Sprintf("backup first: capture one verified snapshot of %s and %s before any edit", p.winnerRel, p.twinRel)}
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
