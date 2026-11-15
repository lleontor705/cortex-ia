package mcpserve

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/hostbridge"
)

var (
	diagramTypes   = []string{"architecture", "workflow", "sequence", "dataflow", "lifecycle"}
	diagramQuality = []string{"standard", "showcase"}
)

func artifactTools() []tool {
	return []tool{
		{
			Name:        "cortex_ia_doc_convert",
			Description: "Convert an office document to Markdown text and metadata. Inline output is read-only. output_path writes a file and requires a live implement claim with a file lease on that path.",
			Schema: schemaObject(props(
				"file_path", schemaString("Document path under the workspace"),
				"output_path", schemaString("Optional Markdown output path, leased to the caller"),
				"format", schemaString("Optional output format"),
				"max_lines", schemaNumber("Optional maximum line count"),
				"ocr", schemaEnum("OCR policy", "reject", "hosted"),
			), "file_path"),
			Call: docConvert,
		},
		{
			Name:        "cortex_ia_diagram_validate",
			Description: "Validate a diagram JSON spec (architecture, workflow, sequence, dataflow, lifecycle). Read-only. Run before diagram_render; returns structural findings.",
			Schema: schemaObject(props(
				"spec_path", schemaString("Diagram specification path under the workspace"),
				"diagram_type", schemaEnum("Diagram type", diagramTypes...),
				"quality", schemaEnum("Quality gate", diagramQuality...),
			), "spec_path"),
			Call: diagramValidate,
		},
		{
			Name:        "cortex_ia_diagram_render",
			Description: "Render a validated diagram spec to interactive HTML at output_path (implement only). Requires a live claim and a file lease on output_path; reserve it with cortex_ia_file_reserve first.",
			Schema: schemaObject(props(
				"spec_path", schemaString("Diagram specification path under the workspace"),
				"output_path", schemaString("HTML output path, leased to the caller"),
				"diagram_type", schemaEnum("Diagram type (default architecture)", diagramTypes...),
				"quality", schemaEnum("Quality gate", diagramQuality...),
			), "spec_path", "output_path"),
			Call: diagramRender,
		},
	}
}

func docConvert(ctx context.Context, s *session, args map[string]any) (string, error) {
	file, err := reqString(args, "file_path")
	if err != nil {
		return "", err
	}
	argv := []string{"doc", "convert", s.absPath(file), "--json"}
	output := optString(args, "output_path")
	if output != "" {
		argv = append(argv, "-o", s.absPath(output))
	}
	if format := optString(args, "format"); format != "" {
		argv = append(argv, "--format", format)
	}
	maxLines, err := optInt(args, "max_lines")
	if err != nil {
		return "", err
	}
	if maxLines != "" {
		argv = append(argv, "--max-lines", maxLines)
	}
	if ocr := optString(args, "ocr"); ocr != "" {
		argv = append(argv, "--ocr", ocr)
	}
	if output != "" {
		return s.artifactCommand(ctx, argv, output)
	}
	return s.runClean(ctx, argv)
}

func diagramValidate(ctx context.Context, s *session, args map[string]any) (string, error) {
	spec, err := reqString(args, "spec_path")
	if err != nil {
		return "", err
	}
	argv := []string{"diagram", "validate"}
	if kind := optString(args, "diagram_type"); kind != "" {
		argv = append(argv, kind)
	}
	argv = append(argv, s.absPath(spec), "--json")
	if quality := optString(args, "quality"); quality != "" {
		argv = append(argv, "--quality="+quality)
	}
	return s.runClean(ctx, argv)
}

func diagramRender(ctx context.Context, s *session, args map[string]any) (string, error) {
	spec, err := reqString(args, "spec_path")
	if err != nil {
		return "", err
	}
	output, err := reqString(args, "output_path")
	if err != nil {
		return "", err
	}
	kind := optString(args, "diagram_type")
	if kind == "" {
		kind = "architecture"
	}
	argv := []string{"diagram", "render", kind, s.absPath(spec), s.absPath(output), "--json"}
	if quality := optString(args, "quality"); quality != "" {
		argv = append(argv, "--quality="+quality)
	}
	return s.artifactCommand(ctx, argv, output)
}

func (s *session) absPath(value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(s.workspace, value)
}

func (s *session) runClean(ctx context.Context, argv []string) (string, error) {
	stdout, err := s.runner.Run(ctx, argv, "")
	if err != nil {
		return "", err
	}
	return cleanOutput(stdout), nil
}

// artifactCommand runs a write-producing CLI verb whose output path must sit
// under the live lease of this session's single claimed task. The CLI
// re-checks both tokens; the tokens travel on stdin only.
func (s *session) artifactCommand(ctx context.Context, argv []string, output string) (string, error) {
	const denied = "BRIDGE_ROLE_DENIED: artifact output requires the host implement controller"
	key, err := s.sessionKeyFor("")
	if err != nil {
		return "", err
	}
	owned := s.vault.ownedBy(key)
	if len(owned) != 1 {
		return "", errors.New("LEASE_REQUIRED: artifact output requires one live task claim")
	}
	task := owned[0]
	caller, err := hostbridge.ResolveCaller(s.stateHome, task, s.now())
	if err != nil || caller.Role != "implement" || caller.SessionKey != key {
		return "", errors.New(denied)
	}
	relative, err := filepath.Rel(s.workspace, s.absPath(output))
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("LEASE_CHECK_FAILED: artifact output must be inside the workspace")
	}
	leasePath := workPath(relative)
	a, err := s.requireAuthority(ctx, task, key, true)
	if err != nil {
		return "", err
	}
	leaseToken := s.vault.leaseToken(a, leasePath)
	if leaseToken == "" {
		return "", errors.New("LEASE_REQUIRED: artifact output path is not reserved by this controller")
	}
	stdin, err := json.Marshal(map[string]any{
		"project": s.workspace, "session_id": key, "role": "implement", "task_id": task,
		"path": leasePath, "claim_token": a.claimToken, "lease_token": leaseToken,
	})
	if err != nil {
		return "", err
	}
	stdout, err := s.runner.Run(ctx, append(argv, "--artifact-authority=@stdin"), string(stdin))
	if err != nil {
		return "", err
	}
	return cleanOutput(stdout), nil
}
