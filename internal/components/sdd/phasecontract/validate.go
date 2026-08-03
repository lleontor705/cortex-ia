package phasecontract

import "fmt"

// ReasonError gives callers a stable machine-readable rejection identifier while
// retaining a useful human message for logs and acceptance evidence.
type ReasonError struct {
	Reason  string
	Message string
}

func (e *ReasonError) Error() string { return e.Reason + ": " + e.Message }

func reject(reason, message string) error { return &ReasonError{Reason: reason, Message: message} }

func ValidatePhaseStatus(status PhaseStatus) error {
	for _, allowed := range CanonicalPhaseStatuses() {
		if status == allowed {
			return nil
		}
	}
	return reject("contract/status-invalid", fmt.Sprintf("%q is not a canonical phase status", status))
}

func ValidateVerificationVerdict(verdict VerificationVerdict) error {
	for _, allowed := range CanonicalVerificationVerdicts() {
		if verdict == allowed {
			return nil
		}
	}
	return reject("contract/verdict-invalid", fmt.Sprintf("%q is not a canonical verification verdict", verdict))
}

// ValidateEnvelope rejects aliases and unversioned/free-form terminal terms at
// the canonical boundary. Compatibility aliases must be decoded first.
func ValidateEnvelope(e CanonicalEnvelope) error {
	if e.SchemaVersion == "" {
		return reject("contract/schema-version-required", "schema version is required")
	}
	if e.ContractVersion == "" {
		return reject("contract/version-required", "contract version is required")
	}
	if err := ValidatePhaseID(e.Phase); err != nil {
		return reject("contract/phase-invalid", err.Error())
	}
	if e.ChangeName == "" {
		return reject("contract/change-required", "change name is required")
	}
	if e.Project == "" {
		return reject("contract/project-required", "project is required")
	}
	if e.Objective == "" {
		return reject("contract/objective-required", "objective is required")
	}
	if err := ValidatePhaseStatus(e.Status); err != nil {
		return err
	}
	if e.Confidence < 0 || e.Confidence > 1 {
		return reject("contract/confidence-invalid", "confidence must be within [0,1]")
	}
	if len(e.TerminalStates) == 0 {
		return reject("contract/terminal-states-required", "terminal states are required")
	}
	for _, state := range e.TerminalStates {
		if err := ValidatePhaseStatus(PhaseStatus(state)); err != nil {
			return reject("contract/terminal-status-invalid", err.Error())
		}
	}
	if len(e.Stops.Completion) == 0 {
		return reject("contract/completion-stop-required", "at least one completion stop is required")
	}
	if err := e.OutputSchema.Validate(); err != nil {
		return reject("contract/output-schema-invalid", err.Error())
	}
	if err := e.Authority.Validate(); err != nil {
		return reject("contract/authority-invalid", err.Error())
	}
	if e.Verdict != nil {
		if err := ValidateVerificationVerdict(*e.Verdict); err != nil {
			return err
		}
	}
	for _, ref := range append(append([]ArtifactRef{}, e.Artifacts...), e.Evidence...) {
		if err := ref.Validate(); err != nil {
			return reject("contract/reference-invalid", err.Error())
		}
	}
	for _, ref := range e.Handoff {
		if err := ref.Validate(); err != nil {
			return reject("contract/handoff-invalid", err.Error())
		}
	}
	return nil
}

type CompatibilityEvidence struct {
	Alias   string `json:"alias"`
	Version string `json:"version"`
}

// DecodeCompatibilityAlias is the only supported alias-to-canonical lowering
// operation. It records evidence so persistence can retain only the PhaseID.
func DecodeCompatibilityAlias(alias, version string) (PhaseID, CompatibilityEvidence, error) {
	if version != CompatibilityVersion {
		return "", CompatibilityEvidence{}, reject("contract/compatibility-version-unsupported", fmt.Sprintf("version %q is unsupported", version))
	}
	phase, ok := compatibilityAliases()[alias]
	if !ok {
		return "", CompatibilityEvidence{}, reject("contract/compatibility-alias-unknown", fmt.Sprintf("alias %q is unknown", alias))
	}
	return phase, CompatibilityEvidence{Alias: alias, Version: version}, nil
}
