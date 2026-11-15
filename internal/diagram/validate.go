package diagram

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var idRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

// Diagnostic represents a validation finding.
type Diagnostic struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
	Severity string `json:"severity"` // "error" or "warning"
}

// ValidationResult records the findings of diagram specification validation.
type ValidationResult struct {
	Valid       bool         `json:"valid"`
	DiagramType string       `json:"diagram_type"`
	Title       string       `json:"title"`
	NodeCount   int          `json:"node_count"`
	EdgeCount   int          `json:"edge_count"`
	Errors      []Diagnostic `json:"errors"`
	Warnings    []Diagnostic `json:"warnings"`
}

// ValidateFile reads and validates a diagram JSON file.
func ValidateFile(filePath string, quality QualityProfile) (*ValidationResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read diagram file %s: %w", filePath, err)
	}
	return Validate(data, quality)
}

// Validate checks the diagram specification against topological and structural rules.
func Validate(data []byte, quality QualityProfile) (*ValidationResult, error) {
	var spec DiagramSpecification
	if err := json.Unmarshal(data, &spec); err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []Diagnostic{{
				Code:     "json/syntax-error",
				Message:  fmt.Sprintf("invalid JSON: %v", err),
				Severity: "error",
			}},
		}, nil
	}

	res := &ValidationResult{
		Valid:       true,
		DiagramType: string(spec.DiagramType),
		Title:       spec.Meta.Title,
		Errors:      []Diagnostic{},
		Warnings:    []Diagnostic{},
	}

	addErr := func(code, msg, path string) {
		res.Valid = false
		res.Errors = append(res.Errors, Diagnostic{Code: code, Message: msg, Path: path, Severity: "error"})
	}
	addWarn := func(code, msg, path string) {
		res.Warnings = append(res.Warnings, Diagnostic{Code: code, Message: msg, Path: path, Severity: "warning"})
	}

	// 1. Core Schema and Type Checks
	if spec.SchemaVersion < 1 || spec.SchemaVersion > 2 {
		addErr("schema/invalid-version", fmt.Sprintf("unsupported schema_version %d (must be 1 or 2)", spec.SchemaVersion), "/schema_version")
	}

	validTypes := map[DiagramType]bool{
		TypeArchitecture: true,
		TypeWorkflow:     true,
		TypeSequence:     true,
		TypeDataflow:     true,
		TypeLifecycle:    true,
	}
	if !validTypes[spec.DiagramType] {
		addErr("schema/invalid-diagram-type", fmt.Sprintf("unknown diagram_type %q", spec.DiagramType), "/diagram_type")
	}

	if strings.TrimSpace(spec.Meta.Title) == "" {
		addErr("meta/title-required", "meta.title is required and cannot be empty", "/meta/title")
	}

	// 2. Nodes validation (Components, Steps, Participants)
	nodeMap := make(map[string]bool)
	nodes := spec.Components
	if len(nodes) == 0 && len(spec.Steps) > 0 {
		for _, s := range spec.Steps {
			nodes = append(nodes, Component{ID: s.ID, Label: s.Label, Type: s.Type})
		}
	}
	if len(nodes) == 0 && len(spec.Participants) > 0 {
		nodes = spec.Participants
	}

	if len(nodes) == 0 {
		addErr("topology/empty-diagram", "diagram must define at least one component, step, or participant", "/components")
	}

	for idx, node := range nodes {
		path := fmt.Sprintf("/components/%d", idx)
		if strings.TrimSpace(node.ID) == "" {
			addErr("topology/node-id-empty", "node ID cannot be empty", path+"/id")
			continue
		}
		if !idRegex.MatchString(node.ID) {
			addErr("topology/node-id-invalid", fmt.Sprintf("node ID %q does not match identifier pattern ^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$", node.ID), path+"/id")
		}
		if nodeMap[node.ID] {
			addErr("topology/node-id-duplicate", fmt.Sprintf("duplicate node ID %q", node.ID), path+"/id")
		}
		nodeMap[node.ID] = true

		if strings.TrimSpace(node.Label) == "" {
			addErr("topology/node-label-empty", fmt.Sprintf("node %q has empty label", node.ID), path+"/label")
		}
	}
	res.NodeCount = len(nodeMap)

	// 3. Connections & Edges validation
	edgeCount := 0
	for idx, conn := range spec.Connections {
		path := fmt.Sprintf("/connections/%d", idx)
		edgeCount++
		if !nodeMap[conn.From] {
			addErr("topology/edge-from-missing", fmt.Sprintf("connection from %q does not match any existing component", conn.From), path+"/from")
		}
		if !nodeMap[conn.To] {
			addErr("topology/edge-to-missing", fmt.Sprintf("connection to %q does not match any existing component", conn.To), path+"/to")
		}
		if conn.From == conn.To && spec.DiagramType != TypeLifecycle {
			addWarn("topology/self-loop", fmt.Sprintf("self-loop connection on component %q", conn.From), path)
		}
	}

	// 4. Sequence messages validation
	for idx, msg := range spec.Messages {
		path := fmt.Sprintf("/messages/%d", idx)
		edgeCount++
		if !nodeMap[msg.From] {
			addErr("sequence/caller-missing", fmt.Sprintf("message caller %q not found in participants", msg.From), path+"/from")
		}
		if !nodeMap[msg.To] {
			addErr("sequence/callee-missing", fmt.Sprintf("message callee %q not found in participants", msg.To), path+"/to")
		}
	}
	res.EdgeCount = edgeCount

	// 5. Boundaries validation
	boundaryMap := make(map[string]bool)
	for idx, b := range spec.Boundaries {
		path := fmt.Sprintf("/boundaries/%d", idx)
		bID := strings.TrimSpace(b.ID)
		if bID == "" {
			bID = strings.TrimSpace(b.Label)
		}
		if bID == "" {
			bID = fmt.Sprintf("boundary-%d", idx)
		}
		if boundaryMap[bID] {
			addErr("boundary/id-duplicate", fmt.Sprintf("duplicate boundary ID %q", bID), path+"/id")
		}
		boundaryMap[bID] = true

		if len(b.Wraps) == 0 {
			addWarn("boundary/empty-wraps", fmt.Sprintf("boundary %q wraps 0 components", bID), path+"/wraps")
		}
		for _, w := range b.Wraps {
			if !nodeMap[w] {
				addErr("boundary/wrapped-node-missing", fmt.Sprintf("boundary %q wraps unknown component %q", b.ID, w), path+"/wraps")
			}
		}
	}

	// 6. Quality Profile specific checks
	if quality == QualityShowcase || spec.Meta.QualityProfile == QualityShowcase {
		if res.NodeCount > 16 {
			addWarn("quality/dense-showcase", fmt.Sprintf("showcase diagrams recommended <= 12-16 primary nodes; found %d", res.NodeCount), "/components")
		}
		if res.EdgeCount == 0 && res.NodeCount > 1 {
			addWarn("quality/disconnected-nodes", "showcase diagram has multiple nodes but zero connections", "/connections")
		}
	}

	return res, nil
}

// ParseSpecification reads and unmarshals raw JSON into DiagramSpecification.
func ParseSpecification(data []byte) (*DiagramSpecification, error) {
	var spec DiagramSpecification
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, errors.New("invalid diagram JSON specification: " + err.Error())
	}
	return &spec, nil
}
