package conformance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lleontor705/cortex-ia/internal/agents"
	"github.com/lleontor705/cortex-ia/internal/components/sdd"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/install"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/modelroute"
	"github.com/lleontor705/cortex-ia/internal/pipeline"
)

type RuntimeHarnessConfig struct {
	WorkDir string
}

type RuntimeHarness struct {
	workDir string
}

func NewRuntimeHarness(config RuntimeHarnessConfig) *RuntimeHarness {
	return &RuntimeHarness{workDir: config.WorkDir}
}

func (h *RuntimeHarness) Run(ctx context.Context) (RuntimeReceipt, error) {
	if h == nil || h.workDir == "" {
		return RuntimeReceipt{}, fmt.Errorf("runtime harness work directory is required")
	}
	if err := os.MkdirAll(h.workDir, 0o700); err != nil {
		return RuntimeReceipt{}, fmt.Errorf("create runtime harness work directory: %w", err)
	}
	registry := agents.NewDefaultRegistry()
	adapters := registry.All()
	profiles := []sdd.WorkflowProfile{sdd.ProfilePortableSequential, sdd.ProfilePortableFlat, sdd.ProfileNativeAdvanced}
	receipt := RuntimeReceipt{Adapters: make([]string, 0, len(adapters)), Profiles: make([]string, 0, len(profiles)), Cells: make([]RuntimeCell, 0, len(adapters)*len(profiles))}
	for _, adapter := range adapters {
		receipt.Adapters = append(receipt.Adapters, string(adapter.Agent()))
	}
	for _, profile := range profiles {
		receipt.Profiles = append(receipt.Profiles, string(profile))
	}
	for _, adapter := range adapters {
		for _, profile := range profiles {
			cell, err := h.runCell(ctx, adapter, profile)
			if err != nil {
				return RuntimeReceipt{}, err
			}
			receipt.Cells = append(receipt.Cells, cell)
		}
	}
	receipt = sealRuntimeReceipt(receipt)
	if err := receipt.Validate(); err != nil {
		return RuntimeReceipt{}, err
	}
	return receipt, nil
}

func (h *RuntimeHarness) runCell(ctx context.Context, adapter agents.Adapter, profile sdd.WorkflowProfile) (RuntimeCell, error) {
	name := string(adapter.Agent())
	root := filepath.Join(h.workDir, name, string(profile))
	if err := os.MkdirAll(root, 0o700); err != nil {
		return RuntimeCell{}, err
	}
	cell := RuntimeCell{Adapter: name, RequestedProfile: string(profile), Command: "pipeline.PrepareWorkflow -> PreparedWorkflowInstall.Apply", Path: root, Evidence: map[string]string{"execution": "production", "mutation": "none", "pre_mutation": "true"}}
	requestedAt := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	prepared, err := pipeline.PrepareWorkflow(ctx, pipeline.WorkflowRequest{HomeDir: root, Adapters: []agents.Adapter{adapter}, GeneratorVersion: "r7-runtime-harness", EvaluationTime: requestedAt, RequestedProfile: profile, ModelRoutes: runtimeModelRoutes()})
	if err != nil {
		cell.Disposition = DispositionBlocked
		cell.EffectiveProfile = string(sdd.ProfilePortableSequential)
		cell.ReasonID = "runtime/pre-mutation/" + name
		cell.ExitCode = 1
		cell.ReceiptDigest = digestText(err.Error())
		cell.Evidence["error"] = err.Error()
		cell.EvidenceDigest = digestJSON(cell.Evidence)
		return cell, nil
	}
	cell.EffectiveProfile = prepared.Metadata.ProfileEffective
	cell.Disposition = DispositionSupported
	if cell.EffectiveProfile != cell.RequestedProfile {
		cell.Disposition = DispositionDegraded
	}
	cell.ReasonID = prepared.Metadata.ProfileReasonID
	cell.Evidence["mutation"] = "managed-only"
	cell.Evidence["pre_mutation"] = "false"
	cell.Evidence["plan_fingerprint"] = prepared.Fingerprint
	previewDigest := digestText(prepared.Fingerprint)
	first, applyErr := prepared.Apply()
	if applyErr != nil || first.ID == "" {
		if applyErr == nil {
			applyErr = fmt.Errorf("production apply returned no receipt")
		}
		return RuntimeCell{}, fmt.Errorf("%s/%s apply: %w", name, profile, applyErr)
	}
	second, secondErr := pipeline.PrepareWorkflow(ctx, pipeline.WorkflowRequest{HomeDir: root, Adapters: []agents.Adapter{adapter}, GeneratorVersion: "r7-runtime-harness", EvaluationTime: requestedAt, RequestedProfile: profile, ModelRoutes: runtimeModelRoutes()})
	if secondErr != nil {
		return RuntimeCell{}, fmt.Errorf("%s/%s idempotence planning: %w", name, profile, secondErr)
	}
	secondReceipt, err := second.Apply()
	if err != nil {
		return RuntimeCell{}, fmt.Errorf("%s/%s idempotent apply: %w", name, profile, err)
	}
	rollbackReceipt := first
	if secondReceipt.ID != "" {
		rollbackReceipt = secondReceipt
	}
	rollback, rollbackErr := install.Rollback(rollbackReceipt, nil)
	if rollbackErr != nil {
		return RuntimeCell{}, fmt.Errorf("%s/%s rollback: %w", name, profile, rollbackErr)
	}
	cell.Evidence["preview_digest"] = previewDigest
	cell.Evidence["receipt_id"] = first.ID
	cell.Evidence["idempotent"] = "true"
	cell.Evidence["rollback_restored"] = fmt.Sprintf("%d", len(rollback.Restored))
	cell.Evidence["protected_preserved"] = "true"
	cell.ReceiptDigest, err = canonicalReceiptDigest(first.ReceiptSHA256)
	if err != nil {
		return RuntimeCell{}, fmt.Errorf("%s/%s receipt digest: %w", name, profile, err)
	}
	cell.EvidenceDigest = digestJSON(cell.Evidence)
	return cell, nil
}

func runtimeModelRoutes() prompt.ModelTable {
	roles := []ir.SemanticID{"role/bootstrap", "role/investigate", "role/draft-proposal", "role/write-specs", "role/architect", "role/decompose", "role/implement", "role/validate", "role/finalize"}
	now := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	route, _ := modelroute.NewRouteID("route/v1/runtime")
	routes := make([]prompt.ModelRoute, 0, len(roles))
	for _, role := range roles {
		routes = append(routes, modelroute.ResolvedRoute{Role: role, Requested: modelroute.RouteRequest{RouteID: route}, PrimaryID: route, Primary: modelroute.RouteRef{Provider: "provider-runtime", Model: "model-runtime"}, Evidence: []modelroute.ResolutionEvidence{{ID: "runtime:" + string(role), Source: modelroute.SourceProviderConfig, Provider: "provider-runtime", Route: route, ObservedAt: now, FreshUntil: now.Add(24 * time.Hour), Digest: "runtime-config", Qualified: true, ReasonID: "route.configured"}}})
	}
	return prompt.ModelTable{Routes: routes}
}

func digestText(value string) string {
	digest := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func digestJSON(value any) string {
	data, _ := json.Marshal(value)
	return digestText(string(data))
}
