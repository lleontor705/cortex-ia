// Package contractgen emits deterministic references from executable SDD
// contract and policy definitions. It owns no independent workflow policy.
package contractgen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/phasecontract"
)

type GeneratorInput struct {
	Definitions       phasecontract.Definitions
	Version           string
	SourceFingerprint string
}

type GeneratedAsset struct {
	SemanticID        ir.SemanticID
	Class             ir.AssetClass
	RelativePath      string
	Content           []byte
	SHA256            string
	Version           string
	SourceFingerprint string
}

func GenerateReferences(in GeneratorInput) ([]GeneratedAsset, error) {
	if in.Version == "" || in.SourceFingerprint == "" {
		return nil, fmt.Errorf("generator version and source fingerprint are required")
	}
	if len(in.Definitions.Phases) == 0 {
		in.Definitions = phasecontract.CanonicalDefinitions()
	}
	snapshot := phasecontract.PolicySnapshot()
	assets := []GeneratedAsset{
		makeAsset(in, "asset/contract/phase-envelope", ir.AssetContractSchema, "contracts/phase-envelope.json", envelopeReference()),
		makeAsset(in, "asset/contract/status-vocabulary", ir.AssetContractSchema, "contracts/status-vocabulary.json", statusReference(in.Definitions)),
		makeAsset(in, "asset/contract/phase-schema", ir.AssetContractSchema, "contracts/phase-schema.json", schemaReference(in.Definitions)),
		// Policy is emitted as a generated contract reference; the shared
		// Cortex contract remains the sole shared-contract authority.
		makeAsset(in, "asset/contract/policy", ir.AssetContractSchema, "contracts/policy.json", snapshot),
		// The generated policy table is a supplemental root module, not the
		// always-on root index. Keeping that ownership distinct prevents the
		// installer from treating it as a second root authority.
		makeTextAsset(in, "asset/root/policy", ir.AssetRootModule, "root/policy.md", rootReference(snapshot)),
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].SemanticID < assets[j].SemanticID })
	return assets, nil
}

func makeAsset(in GeneratorInput, id ir.SemanticID, class ir.AssetClass, path string, value any) GeneratedAsset {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	content = append(content, '\n')
	sum := sha256.Sum256(content)
	return GeneratedAsset{SemanticID: id, Class: class, RelativePath: path, Content: content, SHA256: hex.EncodeToString(sum[:]), Version: in.Version, SourceFingerprint: in.SourceFingerprint}
}

func makeTextAsset(in GeneratorInput, id ir.SemanticID, class ir.AssetClass, path, value string) GeneratedAsset {
	content := []byte(strings.TrimRight(value, "\r\n") + "\n")
	sum := sha256.Sum256(content)
	return GeneratedAsset{SemanticID: id, Class: class, RelativePath: path, Content: content, SHA256: hex.EncodeToString(sum[:]), Version: in.Version, SourceFingerprint: in.SourceFingerprint}
}

func envelopeReference() map[string]any {
	t := reflect.TypeOf(phasecontract.PhaseEnvelope{})
	fields := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		fields = append(fields, t.Field(i).Name)
	}
	sort.Strings(fields)
	return map[string]any{"type": "phase-envelope", "fields": fields}
}

func statusReference(definitions phasecontract.Definitions) map[string]any {
	phases := make([]string, 0, len(definitions.Phases))
	for _, phase := range definitions.Phases {
		phases = append(phases, string(phase))
	}
	sort.Strings(phases)
	statuses := append([]phasecontract.PhaseStatus(nil), definitions.Statuses...)
	verdicts := append([]phasecontract.VerificationVerdict(nil), definitions.Verdicts...)
	sort.Slice(statuses, func(i, j int) bool { return statuses[i] < statuses[j] })
	sort.Slice(verdicts, func(i, j int) bool { return verdicts[i] < verdicts[j] })
	return map[string]any{"phase_ids": phases, "phase_statuses": statuses, "verification_verdicts": verdicts}
}

func schemaReference(definitions phasecontract.Definitions) map[string]any {
	return map[string]any{
		"schema_version": definitions.SchemaVersion,
		"envelope":       envelopeReference(),
	}
}

func rootReference(snapshot phasecontract.PolicySnapshotData) string {
	keys := make([]string, 0, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		keys = append(keys, entry.RetryProfile, entry.RouteKey, entry.ConfidenceKey,
			entry.ModelRouteKey, entry.ReasonID, entry.HumanGateKey)
	}
	sort.Strings(keys)
	return fmt.Sprintf("# Generated SDD policy references\n\n- Version: `%s`\n- Source fingerprint: `%s`\n- Keys: `%s`\n\nUse executable policy keys from `contracts/policy.json`; prompts own no policy tables.\n", snapshot.Version, snapshot.SourceFingerprint, joinKeys(keys))
}

func joinKeys(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	result := keys[0]
	for _, key := range keys[1:] {
		result += "`, `" + key
	}
	return "`" + result + "`"
}
