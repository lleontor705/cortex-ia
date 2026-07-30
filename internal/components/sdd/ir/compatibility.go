package ir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ErrorCode provides stable machine-readable compatibility failure classes.
type ErrorCode string

const (
	ErrorUnsupportedVersion   ErrorCode = "unsupported_version"
	ErrorMissingRequired      ErrorCode = "missing_required"
	ErrorUnknownField         ErrorCode = "unknown_field"
	ErrorUnsupportedExtension ErrorCode = "unsupported_extension"
	ErrorInvalidValue         ErrorCode = "invalid_value"
)

// CompatibilityError contains the diagnostics required to repair an input.
type CompatibilityError struct {
	Code        ErrorCode
	Schema      string
	Observed    string
	Supported   string
	Path        string
	Remediation string
	Cause       error
}

func (e *CompatibilityError) Error() string {
	return fmt.Sprintf("%s at %s: observed %s; supported %s; %s", e.Schema, e.Path, e.Observed, e.Supported, e.Remediation)
}

func (e *CompatibilityError) Unwrap() error { return e.Cause }

// Degradation records an optional namespaced extension ignored by this compiler.
type Degradation struct {
	SemanticID SemanticID `json:"semantic_id"`
	Reason     string     `json:"reason"`
}

// DecodeResult contains a validated workflow and all visible degradations.
type DecodeResult struct {
	Workflow     WorkflowIR
	Degradations []Degradation
}

type extensionDeclaration struct {
	Optional bool            `json:"optional"`
	Value    json.RawMessage `json:"value,omitempty"`
}

var workflowFields = map[string]struct{}{
	"schema_version": {}, "id": {}, "version": {}, "roles": {}, "phases": {},
	"tools": {}, "context": {}, "services": {}, "profiles": {}, "extensions": {},
}

// DecodeWorkflow validates schema compatibility before decoding canonical IR.
func DecodeWorkflow(data []byte) (DecodeResult, error) {
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&fields); err != nil {
		return DecodeResult{}, compatibilityError(ErrorInvalidValue, "$", string(data), "valid JSON object", "provide a valid workflow object", err)
	}

	for _, required := range []string{"schema_version", "id", "version"} {
		if raw, found := fields[required]; !found || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return DecodeResult{}, compatibilityError(ErrorMissingRequired, "$."+required, "<missing>", WorkflowSchema.Supported.String(), "add the required field", nil)
		}
	}

	var observed Version
	if err := json.Unmarshal(fields["schema_version"], &observed); err != nil {
		return DecodeResult{}, compatibilityError(ErrorInvalidValue, "$.schema_version", string(fields["schema_version"]), WorkflowSchema.Supported.String(), "use a major.minor.patch version", err)
	}
	if observed.Major != WorkflowSchema.Supported.Minimum.Major {
		return DecodeResult{}, compatibilityError(ErrorUnsupportedVersion, "$.schema_version", observed.String(), WorkflowSchema.Supported.String(), "regenerate the workflow with a supported schema major", nil)
	}

	unknown := make([]string, 0)
	for name := range fields {
		if _, known := workflowFields[name]; !known {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)

	result := DecodeResult{}
	for _, name := range unknown {
		if !strings.Contains(name, "/") {
			return DecodeResult{}, compatibilityError(ErrorUnknownField, "$."+name, name, WorkflowSchema.Supported.String(), "remove the field or use a declared namespaced extension", nil)
		}
		var extension extensionDeclaration
		if err := json.Unmarshal(fields[name], &extension); err != nil {
			return DecodeResult{}, compatibilityError(ErrorInvalidValue, "$."+name, string(fields[name]), WorkflowSchema.Supported.String(), "declare the extension as an object with optional=true", err)
		}
		if !extension.Optional {
			return DecodeResult{}, compatibilityError(ErrorUnsupportedExtension, "$."+name, name, WorkflowSchema.Supported.String(), "remove the extension or mark it optional after confirming degradation is safe", nil)
		}
		result.Degradations = append(result.Degradations, Degradation{
			SemanticID: SemanticID(name),
			Reason:     "optional namespaced extension is not supported and was ignored",
		})
		delete(fields, name)
	}

	canonical, err := json.Marshal(fields)
	if err != nil {
		return DecodeResult{}, err
	}
	if err := json.Unmarshal(canonical, &result.Workflow); err != nil {
		return DecodeResult{}, compatibilityError(ErrorInvalidValue, "$", "invalid workflow value", WorkflowSchema.Supported.String(), "correct the field type reported by the decoder", err)
	}
	if err := ValidateSemanticID(result.Workflow.ID); err != nil {
		return DecodeResult{}, compatibilityError(ErrorInvalidValue, "$.id", string(result.Workflow.ID), WorkflowSchema.Supported.String(), "use a canonical lower-case namespaced semantic ID", err)
	}
	for index, extension := range result.Workflow.Extensions {
		if err := extension.Validate(); err != nil {
			return DecodeResult{}, compatibilityError(ErrorInvalidValue, fmt.Sprintf("$.extensions[%d]", index), string(extension.ID), WorkflowSchema.Supported.String(), "use the unsupported and unbound provider-neutral extension contract", err)
		}
	}
	return result, nil
}

func compatibilityError(code ErrorCode, path, observed, supported, remediation string, cause error) *CompatibilityError {
	return &CompatibilityError{
		Code:        code,
		Schema:      string(WorkflowSchema.ID),
		Observed:    observed,
		Supported:   supported,
		Path:        path,
		Remediation: remediation,
		Cause:       cause,
	}
}
