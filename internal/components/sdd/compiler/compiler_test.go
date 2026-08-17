package compiler

import (
	"encoding/json"
	"errors"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/prompt"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/quality"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/registry"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// testAdapterContract mirrors a minimal valid adapter prompt contract: safe
// roots and a traversal-rejecting path expander, matching the shape production
// qualification produces.
func testAdapterContract() prompt.AdapterPromptContract {
	return prompt.AdapterPromptContract{
		Target:      "opencode",
		RootPath:    ".config/opencode",
		AgentPath:   func(id ir.SemanticID) string { return path.Join(".config/opencode/agent", string(id)) },
		SkillRoot:   "skills",
		CommandRoot: "commands",
		ExpandPath: func(root, relative string) (string, error) {
			if strings.Contains(relative, "..") {
				return "", errors.New("relative path contains a traversal segment")
			}
			return path.Join(root, relative), nil
		},
	}
}

// baseAsset builds a required, fingerprinted catalog asset spec.
func baseAsset(id ir.SemanticID, class ir.AssetClass, source string, maxTokens int) ir.AssetSpec {
	content := "# " + string(id) + "\n"
	return ir.AssetSpec{
		ID: id, Class: class, SourcePath: source, Required: true,
		MaxTokens: maxTokens, SHA256: ir.FingerprintContent([]byte(content)),
	}
}

// baseInput builds the minimal deterministic Compile input that passes schema
// and semantic validation: a role-less workflow, an empty capability snapshot,
// and a composition-ready effective asset catalog (root index, shared
// contract, quality template, and the portable-flat profile overlay).
func baseInput(t *testing.T) Input {
	t.Helper()
	overlay := baseAsset("asset/profile-overlay/portable-flat", ir.AssetProfileOverlay, "profile-overlay/portable-flat.md", 800)
	overlay.Profiles = []ir.SemanticID{"portable-flat"}
	specs := []ir.AssetSpec{
		baseAsset("asset/root-index", ir.AssetRootIndex, "AGENTS.md", 1500),
		baseAsset("asset/shared-contract", ir.AssetSharedContract, "_shared/sdd-phase-contract.md", 1000),
		baseAsset("asset/quality-template", ir.AssetQualityTemplate, "generated/quality-plan-template.md", 800),
		overlay,
	}
	return Input{
		WorkflowDocument: []byte(`{"schema_version":"1.0.0","id":"workflow/test","version":"1.0.0"}`),
		CatalogDocument:  []byte(`{"schema_version":"1.0.0","version":"1.0.0","facts":[]}`),
		Target:           "opencode",
		Profile:          "portable-flat",
		Configuration:    json.RawMessage(`{}`),
		CompilerVersion:  ir.MustParseVersion("1.0.0"),
		EvaluationTime:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		AssetCatalog:     ir.AssetCatalog{SchemaVersion: ir.AssetCatalogSchema.Current, Assets: specs},
		Adapter:          testAdapterContract(),
		ProfilePlan:      quality.ProfilePlan{ProfileID: "portable-flat"},
	}
}

// testCustomSkill builds one registry-normalized custom skill record exactly
// as the registry merge delivers it: canonical UTF-8/LF content with the
// normalization-stage digest and custom origin.
func testCustomSkill(t *testing.T, id, document string) registry.Skill {
	t.Helper()
	canonical, digest, err := registry.NormalizeContent([]byte(document))
	if err != nil {
		t.Fatalf("normalize custom skill content: %v", err)
	}
	return registry.Skill{
		ID: model.SkillID(id), Content: canonical, ContentSHA256: digest, Origin: registry.OriginCustom,
	}
}

// withCustomSkill attaches custom skill declarations to the input together
// with their effective-catalog asset specs, mirroring the additive overlay the
// assets catalog produces for accepted custom skills.
func withCustomSkill(t *testing.T, input Input, skills ...registry.Skill) Input {
	t.Helper()
	updated := input
	updated.CustomSkills = append(slices.Clone(input.CustomSkills), skills...)
	for _, skill := range skills {
		updated.AssetCatalog.Assets = append(slices.Clone(updated.AssetCatalog.Assets), ir.AssetSpec{
			ID:         ir.SemanticID("asset/skill/" + string(skill.ID)),
			Class:      ir.AssetSkill,
			SourcePath: "skills/" + string(skill.ID) + "/SKILL.md",
			Required:   true,
			MaxTokens:  3500,
			SHA256:     skill.ContentSHA256,
		})
	}
	return updated
}

// requireValidationError asserts that err is a compiler ValidationError with
// the expected code.
func requireValidationError(t *testing.T, err error, code ErrorCode) *ValidationError {
	t.Helper()
	if err == nil {
		t.Fatalf("compilation was accepted, want rejection with code %q", code)
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error is %T (%v), want *compiler.ValidationError", err, err)
	}
	if validationErr.Code != code {
		t.Errorf("code = %q, want %q", validationErr.Code, code)
	}
	return validationErr
}

// TestCompiler_CustomValidAccepted verifies REQ-REG-001 (SC-REG1-H) at the
// compiler boundary: a valid registry-normalized custom skill with a new ID
// compiles into the effective catalog exactly once, preserving identity,
// custom origin, and the registry-normalized content digest only.
func TestCompiler_CustomValidAccepted(t *testing.T) {
	input := baseInput(t)
	skill := testCustomSkill(t, "custom-review",
		"---\nname: custom-review\ndescription: Extra review pass for registry slices.\n---\n\n# custom-review\n\nExtra review pass for registry slices.\n")
	input = withCustomSkill(t, input, skill)

	result, err := Compile(input)
	if err != nil {
		t.Fatalf("compile effective catalog with one valid custom skill: %v", err)
	}

	if len(result.Normalized.CustomSkills) != 1 {
		t.Fatalf("normalized custom skills = %d, want exactly 1", len(result.Normalized.CustomSkills))
	}
	record := result.Normalized.CustomSkills[0]
	if record.ID != "custom-review" {
		t.Errorf("record ID = %q, want %q", record.ID, "custom-review")
	}
	// The digest is preserved from registry normalization, not recomputed
	// from a different canonicalization.
	_, wantDigest, digestErr := registry.NormalizeContent(skill.Content)
	if digestErr != nil {
		t.Fatalf("recompute registry-normalized digest: %v", digestErr)
	}
	if record.ContentSHA256 != wantDigest {
		t.Errorf("record digest = %q, want registry-normalized digest %q", record.ContentSHA256, wantDigest)
	}
	if record.ContentSHA256 != skill.ContentSHA256 {
		t.Errorf("record digest = %q, want the declared registry digest %q", record.ContentSHA256, skill.ContentSHA256)
	}
	if record.Origin != "custom" {
		t.Errorf("record origin = %q, want %q", record.Origin, "custom")
	}

	// The compiled record carries no authority fields: its JSON projection
	// contains exactly the three canonical fields.
	encoded, marshalErr := json.Marshal(record)
	if marshalErr != nil {
		t.Fatalf("marshal custom skill record: %v", marshalErr)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal custom skill record: %v", err)
	}
	for _, key := range []string{"id", "content_sha256", "origin"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("custom skill record is missing canonical key %q", key)
		}
	}
	if len(fields) != 3 {
		t.Errorf("custom skill record has %d JSON fields, want exactly 3 canonical fields", len(fields))
	}

	// The custom skill compiles into the effective catalog exactly once,
	// as a skill-class asset with the same digest.
	count := 0
	for _, spec := range result.Normalized.AssetCatalog.Assets {
		if spec.ID != "asset/skill/custom-review" {
			continue
		}
		count++
		if spec.Class != ir.AssetSkill {
			t.Errorf("effective catalog class = %q, want %q", spec.Class, ir.AssetSkill)
		}
		if spec.SHA256 != wantDigest {
			t.Errorf("effective catalog digest = %q, want %q", spec.SHA256, wantDigest)
		}
	}
	if count != 1 {
		t.Errorf("custom skill appears %d times in the compiled effective catalog, want 1", count)
	}

	// The canonical evidence includes the accepted custom skill.
	if !strings.Contains(string(result.Canonical), `"custom_skills"`) ||
		!strings.Contains(string(result.Canonical), `custom-review`) {
		t.Error("canonical compiler evidence does not include the accepted custom skill")
	}
}

// TestCompiler_AuthorityFieldsRejected verifies REQ-ADAPT-001 (SC-ADAPT-F)
// and REQ-REG-001 (SC-REG1-F): a custom skill declaring agents, tools,
// permissions, or bindings in its frontmatter is rejected pre-write with an
// actionable cause, while non-top-level occurrences are values, not
// declarations, and must pass.
func TestCompiler_AuthorityFieldsRejected(t *testing.T) {
	cases := []struct {
		name string
		// document is the SKILL.md body whose frontmatter carries the defect.
		document string
		// field is the expected offending frontmatter field; empty means the
		// defect is fail-closed invalid frontmatter, not an authority field.
		field   string
		invalid bool
	}{
		{name: "AgentsDeclared", document: "---\nname: hostile\nagents: [reviewer]\n---\n\n# hostile\n", field: "agents"},
		{name: "ToolsDeclared", document: "---\nname: hostile\ntools: [bash]\n---\n\n# hostile\n", field: "tools"},
		{name: "PermissionsDeclared", document: "---\nname: hostile\npermissions: [fs/write]\n---\n\n# hostile\n", field: "permissions"},
		{name: "BindingsDeclared", document: "---\nname: hostile\nbindings: [role/implement]\n---\n\n# hostile\n", field: "bindings"},
		{name: "UppercaseVariantRejected", document: "---\nname: hostile\nTools: [bash]\n---\n\n# hostile\n", field: "tools"},
		{name: "MalformedFrontmatterFailsClosed", document: "---\nname: [unclosed\n---\n\n# hostile\n", invalid: true},
		{name: "NonMappingFrontmatterFailsClosed", document: "---\n- just\n- a\n  list\n---\n\n# hostile\n", invalid: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := baseInput(t)
			skill := testCustomSkill(t, "hostile-skill", tc.document)
			input = withCustomSkill(t, input, skill)

			result, err := Compile(input)
			if err == nil {
				t.Fatalf("authority-bearing declaration was accepted: %+v", result.Normalized.CustomSkills)
			}
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error is %T (%v), want *compiler.ValidationError", err, err)
			}
			if tc.invalid {
				if validationErr.Code != ErrorInvalidInput {
					t.Errorf("code = %q, want %q", validationErr.Code, ErrorInvalidInput)
				}
				if !strings.HasPrefix(validationErr.Path, "$.custom_skills[0]") {
					t.Errorf("path = %q, want it to identify the offending declaration", validationErr.Path)
				}
				return
			}
			if validationErr.Code != ErrorUnsupportedDeclaration {
				t.Errorf("code = %q, want %q", validationErr.Code, ErrorUnsupportedDeclaration)
			}
			if want := "$.custom_skills[0].frontmatter." + tc.field; validationErr.Path != want {
				t.Errorf("path = %q, want %q", validationErr.Path, want)
			}
			if validationErr.Remediation == "" {
				t.Error("authority rejection must cite a safe remediation")
			}
			if len(result.Normalized.CustomSkills) != 0 {
				t.Errorf("rejected declaration must not produce compiled records; got %d", len(result.Normalized.CustomSkills))
			}
		})
	}

	// An indented occurrence under another key is a value, not a top-level
	// authority declaration, and must not be rejected.
	t.Run("NestedKeyIsNotTopLevelDeclaration", func(t *testing.T) {
		input := baseInput(t)
		skill := testCustomSkill(t, "nested-ok", "---\nname: nested-ok\nmetadata:\n  tools: [grep]\n---\n\n# nested-ok\n")
		input = withCustomSkill(t, input, skill)
		if _, err := Compile(input); err != nil {
			t.Fatalf("nested non-top-level key was rejected: %v", err)
		}
	})
}

// TestCompiler_CustomSkillBoundary covers the remaining compiler-boundary
// invariants: declared digests must match the declared content, the custom
// skill must be present exactly once in the effective catalog with the same
// digest, duplicate declarations are rejected, only custom-origin records are
// accepted, and an input without an overlay compiles unchanged.
func TestCompiler_CustomSkillBoundary(t *testing.T) {
	validDocument := "---\nname: boundary-skill\ndescription: Boundary case.\n---\n\n# boundary-skill\n"

	t.Run("DigestMismatchRejected", func(t *testing.T) {
		input := baseInput(t)
		input = withCustomSkill(t, input, testCustomSkill(t, "boundary-skill", validDocument))
		input.CustomSkills[0].ContentSHA256 = strings.Repeat("0", 64)

		validationErr := requireValidationError(t, mustCompileErr(t, input), ErrorInvalidInput)
		if want := "$.custom_skills[0].content_sha256"; validationErr.Path != want {
			t.Errorf("path = %q, want %q", validationErr.Path, want)
		}
	})

	t.Run("SkillMissingFromEffectiveCatalogRejected", func(t *testing.T) {
		input := baseInput(t)
		// Declare the skill without attaching its effective-catalog spec.
		input.CustomSkills = []registry.Skill{testCustomSkill(t, "boundary-skill", validDocument)}

		validationErr := requireValidationError(t, mustCompileErr(t, input), ErrorUnresolvedReference)
		if want := "$.custom_skills[0]"; validationErr.Path != want {
			t.Errorf("path = %q, want %q", validationErr.Path, want)
		}
	})

	t.Run("CatalogDigestDisagreementRejected", func(t *testing.T) {
		input := baseInput(t)
		input = withCustomSkill(t, input, testCustomSkill(t, "boundary-skill", validDocument))
		for i, spec := range input.AssetCatalog.Assets {
			if spec.ID == "asset/skill/boundary-skill" {
				input.AssetCatalog.Assets[i].SHA256 = ir.FingerprintContent([]byte("# other content\n"))
			}
		}

		requireValidationError(t, mustCompileErr(t, input), ErrorInvalidInput)
	})

	t.Run("DuplicateCustomDeclarationRejected", func(t *testing.T) {
		input := baseInput(t)
		skill := testCustomSkill(t, "boundary-skill", validDocument)
		input = withCustomSkill(t, input, skill)
		// The same declaration arrives twice; the effective catalog still
		// carries it exactly once.
		input.CustomSkills = append(input.CustomSkills, skill)

		validationErr := requireValidationError(t, mustCompileErr(t, input), ErrorDuplicateReference)
		if want := "$.custom_skills[1].id"; validationErr.Path != want {
			t.Errorf("path = %q, want %q", validationErr.Path, want)
		}
	})

	t.Run("EmbeddedOriginRejected", func(t *testing.T) {
		input := baseInput(t)
		input = withCustomSkill(t, input, testCustomSkill(t, "boundary-skill", validDocument))
		input.CustomSkills[0].Origin = registry.OriginEmbedded

		validationErr := requireValidationError(t, mustCompileErr(t, input), ErrorInvalidInput)
		if want := "$.custom_skills[0].origin"; validationErr.Path != want {
			t.Errorf("path = %q, want %q", validationErr.Path, want)
		}
	})

	t.Run("NoOverlayCompilesUnchanged", func(t *testing.T) {
		result, err := Compile(baseInput(t))
		if err != nil {
			t.Fatalf("compile baseline input without overlay: %v", err)
		}
		if len(result.Normalized.CustomSkills) != 0 {
			t.Errorf("baseline compile produced %d custom skill records, want 0", len(result.Normalized.CustomSkills))
		}
		if strings.Contains(string(result.Canonical), `"custom_skills"`) {
			t.Error("baseline canonical evidence must not carry an empty custom_skills field")
		}
	})
}

// mustCompileErr runs Compile and fails the test when it succeeds.
func mustCompileErr(t *testing.T, input Input) error {
	t.Helper()
	result, err := Compile(input)
	if err == nil {
		t.Fatalf("compilation was accepted, want rejection (records: %+v)", result.Normalized.CustomSkills)
	}
	return err
}
