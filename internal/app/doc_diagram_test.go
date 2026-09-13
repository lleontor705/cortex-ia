package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCLIDoc(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte("ColA,ColB\nVal1,Val2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Inspect
	if err := runDoc([]string{"inspect", csvFile, "--json"}); err != nil {
		t.Errorf("runDoc inspect failed: %v", err)
	}

	// 2. Convert to file
	outFile := filepath.Join(tmpDir, "out.md")
	if err := runDoc([]string{"convert", csvFile, "-o", outFile, "--json"}); err != nil {
		t.Errorf("runDoc convert failed: %v", err)
	}

	info, err := os.Stat(outFile)
	if err != nil || info.Size() == 0 {
		t.Errorf("expected non-empty output markdown file")
	}
}

func TestCLIDiagram(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "arch.json")
	specJSON := `{
  "schema_version": 1,
  "diagram_type": "architecture",
  "meta": { "title": "Test Arch" },
  "components": [
    { "id": "svc1", "type": "backend", "label": "Service 1" },
    { "id": "db1", "type": "database", "label": "Database 1" }
  ],
  "connections": [
    { "from": "svc1", "to": "db1", "label": "reads" }
  ]
}`
	if err := os.WriteFile(specFile, []byte(specJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Validate
	if err := runDiagram([]string{"validate", "architecture", specFile, "--json"}); err != nil {
		t.Errorf("runDiagram validate failed: %v", err)
	}

	// 2. Render
	htmlFile := filepath.Join(tmpDir, "arch.html")
	if err := runDiagram([]string{"render", "architecture", specFile, htmlFile, "--json"}); err != nil {
		t.Errorf("runDiagram render failed: %v", err)
	}
	info, err := os.Stat(htmlFile)
	if err != nil || info.Size() == 0 {
		t.Errorf("expected non-empty html file")
	}

	// 3. Reach
	if err := runDiagram([]string{"reach", specFile, "--from", "svc1", "--json"}); err != nil {
		t.Errorf("runDiagram reach failed: %v", err)
	}

	// 4. Compare
	specFile2 := filepath.Join(tmpDir, "arch2.json")
	specJSON2 := `{
  "schema_version": 1,
  "diagram_type": "architecture",
  "meta": { "title": "Test Arch 2" },
  "components": [
    { "id": "svc1", "type": "backend", "label": "Service 1 (v2)" },
    { "id": "svc2", "type": "backend", "label": "Service 2" }
  ],
  "connections": [
    { "from": "svc1", "to": "svc2", "label": "calls" }
  ]
}`
	if err := os.WriteFile(specFile2, []byte(specJSON2), 0644); err != nil {
		t.Fatal(err)
	}

	deltaHtml := filepath.Join(tmpDir, "delta.html")
	if err := runDiagram([]string{"compare", specFile, specFile2, deltaHtml, "--json"}); err != nil {
		t.Errorf("runDiagram compare failed: %v", err)
	}
	infoDelta, err := os.Stat(deltaHtml)
	if err != nil || infoDelta.Size() == 0 {
		t.Errorf("expected non-empty delta html file")
	}
}
