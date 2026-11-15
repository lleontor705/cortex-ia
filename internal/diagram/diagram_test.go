package diagram

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleArchitectureJSON = `{
  "schema_version": 1,
  "diagram_type": "architecture",
  "meta": {
    "title": "Sample Web Architecture",
    "quality_profile": "standard"
  },
  "components": [
    { "id": "web", "type": "frontend", "label": "Web App" },
    { "id": "api", "type": "backend", "label": "API Gateway" },
    { "id": "db", "type": "database", "label": "Postgres DB" }
  ],
  "connections": [
    { "from": "web", "to": "api", "label": "HTTPS" },
    { "from": "api", "to": "db", "label": "SQL" }
  ]
}`

func TestValidate(t *testing.T) {
	res, err := Validate([]byte(sampleArchitectureJSON), QualityStandard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid=true, got errors: %+v", res.Errors)
	}
	if res.NodeCount != 3 {
		t.Errorf("expected 3 nodes, got %d", res.NodeCount)
	}
	if res.EdgeCount != 2 {
		t.Errorf("expected 2 edges, got %d", res.EdgeCount)
	}

	// Invalid connection target
	invalidJSON := strings.Replace(sampleArchitectureJSON, `"to": "db"`, `"to": "unknown"`, 1)
	resInvalid, err := Validate([]byte(invalidJSON), QualityStandard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resInvalid.Valid {
		t.Errorf("expected valid=false for missing connection target")
	}
}

func TestCompare(t *testing.T) {
	baseJSON := sampleArchitectureJSON
	headJSON := `{
  "schema_version": 1,
  "diagram_type": "architecture",
  "meta": { "title": "Updated Architecture" },
  "components": [
    { "id": "web", "type": "frontend", "label": "Web App Modernized" },
    { "id": "api", "type": "backend", "label": "API Gateway" },
    { "id": "cache", "type": "database", "label": "Redis Cache" }
  ],
  "connections": [
    { "from": "web", "to": "api", "label": "gRPC" },
    { "from": "api", "to": "cache", "label": "TCP" }
  ]
}`

	tmpDir := t.TempDir()
	outHtml := filepath.Join(tmpDir, "delta.html")

	report, err := Compare([]byte(baseJSON), []byte(headJSON), outHtml)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(report.ComponentsAdded) != 1 || report.ComponentsAdded[0].ID != "cache" {
		t.Errorf("expected component 'cache' added, got %+v", report.ComponentsAdded)
	}
	if len(report.ComponentsRemoved) != 1 || report.ComponentsRemoved[0].ID != "db" {
		t.Errorf("expected component 'db' removed, got %+v", report.ComponentsRemoved)
	}
	if len(report.ComponentsModified) != 1 || report.ComponentsModified[0].ID != "web" {
		t.Errorf("expected component 'web' modified, got %+v", report.ComponentsModified)
	}

	info, err := os.Stat(outHtml)
	if err != nil || info.Size() == 0 {
		t.Errorf("expected non-empty delta html output")
	}
}

func TestReach(t *testing.T) {
	res, err := Reach([]byte(sampleArchitectureJSON), "web", DirectionDownstream)
	if err != nil {
		t.Fatalf("Reach failed: %v", err)
	}
	if res.ReachableCount != 2 {
		t.Errorf("expected 2 reachable nodes from web, got %d (%+v)", res.ReachableCount, res.ReachableNodes)
	}

	path, err := ShortestPath([]byte(sampleArchitectureJSON), "web", "db")
	if err != nil {
		t.Fatalf("ShortestPath failed: %v", err)
	}
	if len(path) != 3 || path[0] != "web" || path[1] != "api" || path[2] != "db" {
		t.Errorf("expected [web api db], got %+v", path)
	}
}

func TestRender(t *testing.T) {
	tmpDir := t.TempDir()
	outHtml := filepath.Join(tmpDir, "diagram.html")

	res, err := Render(context.Background(), []byte(sampleArchitectureJSON), outHtml, RenderOptions{
		Quality: QualityStandard,
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	info, err := os.Stat(outHtml)
	if err != nil || info.Size() == 0 {
		t.Errorf("expected rendered HTML file to exist and have bytes, got size %d", res.ByteCount)
	}
}
