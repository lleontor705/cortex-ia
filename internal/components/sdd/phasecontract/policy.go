package phasecontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// PolicyEntry is the stable, non-numeric policy reference emitted to prompts
// and generated assets. Numeric retry ceilings remain executable data behind
// RetryPolicyForKey.
type PolicyEntry struct {
	Phase         PhaseID `json:"phase"`
	RetryProfile  string  `json:"retry_profile"`
	RouteKey      string  `json:"route_key"`
	ConfidenceKey string  `json:"confidence_key"`
	ModelRouteKey string  `json:"model_route_key"`
	ReasonID      string  `json:"reason_id"`
	HumanGateKey  string  `json:"human_gate_key"`
}

// PolicySnapshotData is the deterministic generated-policy input.
type PolicySnapshotData struct {
	Version           string        `json:"version"`
	SourceFingerprint string        `json:"source_fingerprint"`
	Entries           []PolicyEntry `json:"entries"`
}

const policyVersion = "1.0.0"

// RetryProfileKeys returns the stable retry keys in lexical order.
func RetryProfileKeys() []string {
	keys := make([]string, 0, len(CanonicalPhaseIDs()))
	for _, phase := range CanonicalPhaseIDs() {
		keys = append(keys, retryKey(phase))
	}
	sort.Strings(keys)
	return keys
}

// RetryPolicyForKey resolves a prompt-visible key to executable retry policy.
func RetryPolicyForKey(key string) (RetryPolicy, error) {
	const prefix = "retry/"
	if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
		return RetryPolicy{}, fmt.Errorf("unknown retry profile key %q", key)
	}
	phase := PhaseID(key[len(prefix):])
	if err := ValidatePhaseID(phase); err != nil {
		return RetryPolicy{}, fmt.Errorf("unknown retry profile key %q", key)
	}
	return PolicyFor(phase)
}

func policyEntries() []PolicyEntry {
	entries := make([]PolicyEntry, 0, len(CanonicalPhaseIDs()))
	for _, key := range RetryProfileKeys() {
		phase := PhaseID(key[len("retry/"):])
		entries = append(entries, PolicyEntry{Phase: phase, RetryProfile: key,
			RouteKey: routeKey(phase), ConfidenceKey: confidenceKey(phase),
			ModelRouteKey: modelRouteKey(phase), ReasonID: reasonID(phase),
			HumanGateKey: humanGateKey(phase)})
	}
	return entries
}

func retryKey(phase PhaseID) string      { return "retry/" + string(phase) }
func routeKey(phase PhaseID) string      { return "route/phase/" + string(phase) }
func confidenceKey(phase PhaseID) string { return "confidence/phase/" + string(phase) }
func modelRouteKey(phase PhaseID) string { return "model/phase/" + string(phase) }
func reasonID(phase PhaseID) string      { return "reason/phase/" + string(phase) }
func humanGateKey(phase PhaseID) string  { return "gate/phase/" + string(phase) }

// PolicySnapshot returns the sole executable policy inventory used by codegen.
func PolicySnapshot() PolicySnapshotData {
	entries := policyEntries()
	payload, _ := json.Marshal(struct {
		Version string        `json:"version"`
		Entries []PolicyEntry `json:"entries"`
	}{policyVersion, entries})
	sum := sha256.Sum256(payload)
	return PolicySnapshotData{Version: policyVersion, SourceFingerprint: hex.EncodeToString(sum[:]), Entries: entries}
}
