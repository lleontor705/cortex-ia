package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/backup"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// Receipt is durable recovery evidence for one apply attempt. A failed apply
// deliberately leaves its verified backup available for explicit restoration.
type Receipt struct {
	SchemaVersion      string
	ID                 string
	PlanFingerprint    string
	ReceiptSHA256      string
	State              ReceiptState
	Backup             backup.Manifest
	BackupSHA256       string
	BackupVerified     bool
	RestoreAvailable   bool
	Applied            []string
	FailedPath         string
	Installed          []InstalledAsset
	Inventory          []AssetInventory
	OperationOutcomes  []OperationOutcome
	CapabilitySnapshot json.RawMessage
	PreDoctor          json.RawMessage
	PostDoctor         json.RawMessage
	Conflicts          []PlanConflict
	Retirements        []RetirementReceipt
	ProtectedPaths     []string
	RollbackEligible   bool
	Metadata           json.RawMessage
}

type ReceiptState string

const (
	ReceiptPrepared  ReceiptState = "prepared"
	ReceiptApplying  ReceiptState = "applying"
	ReceiptCommitted ReceiptState = "committed"
	ReceiptFailed    ReceiptState = "failed"
)

type OperationOutcome struct {
	Path         string
	BeforeSHA256 string
	AfterSHA256  string
	Status       string
	Error        string
}

type RetirementReceipt struct {
	SemanticID ir.SemanticID
	Path       string
	Decision   string
}

type WorkflowReceiptStore interface {
	Save(Receipt) error
	Load(string) (Receipt, error)
}

// InstalledAsset records the exact managed state written by an apply. Rollback
// uses it as the merge base so post-backup user edits can be distinguished from
// the prior managed bytes selected for restoration.
type InstalledAsset struct {
	Path         string
	OriginalPath string
	SemanticID   ir.SemanticID
	Content      []byte
	Existed      bool
}

// Applier consumes an exact Plan. It verifies the complete backup before the
// first target mutation and stops immediately after any mutation failure.
type Applier struct {
	root           string
	backupRoot     string
	now            func() time.Time
	beforeMutation func(Receipt, Effect) error
}

func NewApplier(root, backupRoot string) *Applier {
	return &Applier{root: root, backupRoot: backupRoot, now: time.Now}
}

func (a *Applier) Apply(plan Plan) (Receipt, error) {
	return a.ApplyWithStore(plan, fileReceiptStore{root: filepath.Join(a.root, ".cortex-ia", "receipts")})
}

func (a *Applier) ApplyWithStore(plan Plan, store WorkflowReceiptStore) (Receipt, error) {
	var receipt Receipt
	if strings.TrimSpace(a.root) == "" || strings.TrimSpace(a.backupRoot) == "" {
		return receipt, errors.New("apply target root and backup root are required")
	}
	if plan.HasBlockingConflicts() {
		return receipt, errors.New("apply blocked by install plan conflicts")
	}
	mutations := planMutations(plan)
	if len(mutations) == 0 {
		return receipt, nil
	}
	if err := validateBackupScope(plan.Backup, mutations); err != nil {
		return receipt, err
	}
	computedFingerprint := FingerprintPlan(plan)
	if plan.Fingerprint != "" && plan.Fingerprint != computedFingerprint {
		return receipt, fmt.Errorf("%w: plan fingerprint changed after preview", ErrStalePlan)
	}
	plan.Fingerprint = computedFingerprint
	if err := a.Preflight(plan); err != nil {
		return receipt, err
	}

	absolute := make([]string, len(plan.Backup.Paths))
	for index, path := range plan.Backup.Paths {
		if err := validateAssetPath(path); err != nil {
			return receipt, fmt.Errorf("backup path: %w", err)
		}
		absolute[index] = filepath.Join(a.root, filepath.FromSlash(path))
	}
	if err := os.MkdirAll(a.backupRoot, 0o700); err != nil {
		return receipt, fmt.Errorf("create backup root: %w", err)
	}
	backupDir, err := os.MkdirTemp(a.backupRoot, "install-"+a.now().UTC().Format("20060102T150405.000000000Z")+"-")
	if err != nil {
		return receipt, fmt.Errorf("allocate unique backup directory: %w", err)
	}
	manifest, err := backup.NewSnapshotter().Create(backupDir, absolute)
	if err != nil {
		return receipt, fmt.Errorf("create pre-apply backup: %w", err)
	}
	manifest.Source = backup.BackupSourceInstall
	if err := backup.WriteManifest(filepath.Join(backupDir, backup.ManifestFilename), manifest); err != nil {
		return receipt, fmt.Errorf("persist pre-apply backup: %w", err)
	}
	if err := backup.Verify(manifest); err != nil {
		return receipt, fmt.Errorf("verify pre-apply backup: %w", err)
	}
	receipt.Backup = manifest
	receipt.SchemaVersion = "1.0.0"
	receipt.ID = "receipt-" + manifest.ID
	receipt.PlanFingerprint = plan.Fingerprint
	receipt.BackupSHA256 = hashJSON(manifest)
	receipt.BackupVerified = true
	receipt.RestoreAvailable = true
	receipt.RollbackEligible = true
	receipt.State = ReceiptPrepared
	receipt.Conflicts = slices.Clone(plan.Conflicts)
	receipt.ProtectedPaths = slices.Clone(plan.ProtectedPaths)
	receipt.Inventory = slices.Clone(plan.Inventory)
	receipt.Metadata = slices.Clone(plan.Metadata)
	sealReceipt(&receipt)
	if err := saveReceipt(store, receipt); err != nil {
		return receipt, fmt.Errorf("persist PREPARED workflow receipt before mutation: %w", err)
	}

	for _, mutation := range mutations {
		if a.beforeMutation != nil {
			if err := a.beforeMutation(receipt, mutation.effect); err != nil {
				receipt.FailedPath = mutation.effect.Path
				receipt.State = ReceiptFailed
				receipt.OperationOutcomes = append(receipt.OperationOutcomes, failedOutcome(mutation.effect, err))
				sealReceipt(&receipt)
				_ = saveReceipt(store, receipt)
				return receipt, fmt.Errorf("apply %q stopped: %w", mutation.effect.Path, err)
			}
		}
		if err := a.checkMutation(mutation); err != nil {
			receipt.FailedPath = mutation.effect.Path
			receipt.State = ReceiptFailed
			receipt.OperationOutcomes = append(receipt.OperationOutcomes, failedOutcome(mutation.effect, err))
			sealReceipt(&receipt)
			_ = saveReceipt(store, receipt)
			return receipt, err
		}
		if err := a.mutate(mutation); err != nil {
			receipt.FailedPath = mutation.effect.Path
			receipt.State = ReceiptFailed
			receipt.OperationOutcomes = append(receipt.OperationOutcomes, failedOutcome(mutation.effect, err))
			sealReceipt(&receipt)
			_ = saveReceipt(store, receipt)
			return receipt, err
		}
		if plan.OwnershipMarkers {
			if err := a.writeOwnership(plan, mutation.effect); err != nil {
				receipt.FailedPath = mutation.effect.Path
				receipt.State = ReceiptFailed
				receipt.OperationOutcomes = append(receipt.OperationOutcomes, failedOutcome(mutation.effect, err))
				sealReceipt(&receipt)
				_ = saveReceipt(store, receipt)
				return receipt, err
			}
		}
		receipt.Applied = append(receipt.Applied, mutation.effect.Path)
		receipt.Installed = append(receipt.Installed, InstalledAsset{
			Path:         mutation.effect.Path,
			OriginalPath: filepath.Join(a.root, filepath.FromSlash(mutation.effect.Path)),
			SemanticID:   mutation.effect.SemanticID,
			Content:      bytes.Clone(mutation.effect.Content),
			Existed:      mutation.kind != mutationDelete,
		})
		receipt.State = ReceiptApplying
		receipt.OperationOutcomes = append(receipt.OperationOutcomes, OperationOutcome{
			Path: mutation.effect.Path, BeforeSHA256: mutation.effect.BeforeSHA256,
			AfterSHA256: mutation.effect.AfterSHA256, Status: "applied",
		})
		sealReceipt(&receipt)
		if err := saveReceipt(store, receipt); err != nil {
			return receipt, fmt.Errorf("checkpoint workflow receipt after %q: %w", mutation.effect.Path, err)
		}
	}
	receipt.State = ReceiptCommitted
	sealReceipt(&receipt)
	if err := saveReceipt(store, receipt); err != nil {
		return receipt, fmt.Errorf("persist COMMITTED workflow receipt: %w", err)
	}
	return receipt, nil
}

func (a *Applier) writeOwnership(plan Plan, effect Effect) error {
	generatorVersion := plan.GeneratorVersion
	if _, err := ir.ParseVersion(generatorVersion); err != nil {
		generatorVersion = "1.0.0"
	}
	metadata, err := NewOwnership(effect.Path, generatorVersion, effect.SemanticID, effect.Content, effect.Content)
	if err != nil {
		return fmt.Errorf("create ownership for %q: %w", effect.Path, err)
	}
	if err := NewOwnershipStore(a.root).Write(metadata, effect.Content); err != nil {
		return fmt.Errorf("persist ownership for %q: %w", effect.Path, err)
	}
	return nil
}

var ErrStalePlan = errors.New("immutable install plan is stale")

// Preflight validates every declared target before backup or mutation so a
// known-later stale target cannot produce a partial apply.
func (a *Applier) Preflight(plan Plan) error {
	for _, mutation := range planMutations(plan) {
		if err := a.checkMutation(mutation); err != nil {
			return err
		}
	}
	return nil
}

func (a *Applier) checkMutation(mutation plannedMutation) error {
	path := filepath.Join(a.root, filepath.FromSlash(mutation.effect.Path))
	ownershipPath := mutation.effect.OwnershipPath
	if ownershipPath == "" {
		ownershipPath = mutation.effect.Path + sidecarSuffix
	}
	basePath := mutation.effect.BasePath
	if basePath == "" {
		basePath = mutation.effect.Path + baseSuffix
	}
	if err := checkEvidenceHash(filepath.Join(a.root, filepath.FromSlash(ownershipPath)), mutation.effect.OwnershipSHA256, "ownership", mutation.effect.Path); err != nil {
		return err
	}
	if err := checkEvidenceHash(filepath.Join(a.root, filepath.FromSlash(basePath)), mutation.effect.BaseSHA256, "base", mutation.effect.Path); err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if mutation.kind == mutationCreate {
			return nil
		}
		return fmt.Errorf("%w: target %q no longer exists", ErrStalePlan, mutation.effect.Path)
	}
	if err != nil {
		return fmt.Errorf("preflight target %q: %w", mutation.effect.Path, err)
	}
	if mutation.kind == mutationCreate {
		return fmt.Errorf("%w: target %q was created after preview", ErrStalePlan, mutation.effect.Path)
	}
	if mutation.effect.BeforeSHA256 == "" {
		return nil // no prior bytes existed or historical plan declared no content CAS
	}
	if SHA256(content) != mutation.effect.BeforeSHA256 {
		return fmt.Errorf("%w: target %q before hash changed", ErrStalePlan, mutation.effect.Path)
	}
	return nil
}

func checkEvidenceHash(path, expected, kind, target string) error {
	if expected == "" {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%w: target %q %s evidence unavailable: %v", ErrStalePlan, target, kind, err)
	}
	if SHA256(content) != expected {
		return fmt.Errorf("%w: target %q %s evidence changed", ErrStalePlan, target, kind)
	}
	return nil
}

func failedOutcome(effect Effect, err error) OperationOutcome {
	return OperationOutcome{Path: effect.Path, BeforeSHA256: effect.BeforeSHA256, AfterSHA256: effect.AfterSHA256, Status: "failed", Error: err.Error()}
}

func saveReceipt(store WorkflowReceiptStore, receipt Receipt) error {
	if store == nil {
		return nil
	}
	return store.Save(receipt)
}

func sealReceipt(receipt *Receipt) {
	receipt.ReceiptSHA256 = ""
	receipt.ReceiptSHA256 = hashJSON(*receipt)
}

func ValidateReceipt(receipt Receipt) error {
	want := receipt.ReceiptSHA256
	receipt.ReceiptSHA256 = ""
	if want == "" || hashJSON(receipt) != want {
		return errors.New("workflow receipt checksum mismatch")
	}
	return nil
}

func SealReceipt(receipt Receipt) Receipt {
	sealReceipt(&receipt)
	return receipt
}

func hashJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal deterministic receipt data: %v", err))
	}
	return SHA256(encoded)
}

type fileReceiptStore struct{ root string }

func (s fileReceiptStore) Save(receipt Receipt) error {
	if receipt.ID == "" || strings.ContainsAny(receipt.ID, `/\\:`) {
		return fmt.Errorf("invalid workflow receipt ID %q", receipt.ID)
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	path := filepath.Join(s.root, receipt.ID+".json")
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, encoded, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func (s fileReceiptStore) Load(id string) (Receipt, error) {
	encoded, err := os.ReadFile(filepath.Join(s.root, id+".json"))
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	if err := json.Unmarshal(encoded, &receipt); err != nil {
		return Receipt{}, err
	}
	if err := ValidateReceipt(receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

type mutationKind uint8

const (
	mutationCreate mutationKind = iota
	mutationWrite
	mutationDelete
)

type plannedMutation struct {
	kind   mutationKind
	effect Effect
}

func planMutations(plan Plan) []plannedMutation {
	result := make([]plannedMutation, 0, len(plan.Creates)+len(plan.Updates)+len(plan.Deletes))
	for _, effect := range plan.Creates {
		result = append(result, plannedMutation{kind: mutationCreate, effect: effect})
	}
	for _, effect := range plan.Updates {
		result = append(result, plannedMutation{kind: mutationWrite, effect: effect})
	}
	for _, effect := range plan.Deletes {
		result = append(result, plannedMutation{kind: mutationDelete, effect: effect})
	}
	return result
}

func validateBackupScope(scope BackupScope, mutations []plannedMutation) error {
	if !scope.Required || len(scope.Paths) == 0 {
		return errors.New("managed mutations require a restorable backup scope")
	}
	paths := slices.Clone(scope.Paths)
	slices.Sort(paths)
	for _, mutation := range mutations {
		if _, found := slices.BinarySearch(paths, mutation.effect.Path); !found {
			return fmt.Errorf("backup scope omits managed target %q", mutation.effect.Path)
		}
	}
	return nil
}

func (a *Applier) mutate(mutation plannedMutation) error {
	fullPath := filepath.Join(a.root, filepath.FromSlash(mutation.effect.Path))
	if mutation.kind == mutationDelete {
		if err := os.Remove(fullPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("delete managed target %q: %w", mutation.effect.Path, err)
		}
		return nil
	}
	mode := mutation.effect.AfterMode.Perm()
	if mode == 0 {
		mode = 0o600
	}
	if _, err := filemerge.WriteFileAtomic(fullPath, mutation.effect.Content, mode); err != nil {
		return fmt.Errorf("write managed target %q: %w", mutation.effect.Path, err)
	}
	return nil
}
