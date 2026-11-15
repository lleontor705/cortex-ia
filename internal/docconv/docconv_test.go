package docconv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectFormat(t *testing.T) {
	// PDF magic bytes
	pdfBytes := []byte("%PDF-1.4\n%somecontent")
	if f := DetectFormat("sample.dat", pdfBytes); f != FormatPdf {
		t.Errorf("expected FormatPdf, got %s", f)
	}

	// RTF magic bytes
	rtfBytes := []byte("{\\rtf1\\ansi\\deff0...")
	if f := DetectFormat("sample.dat", rtfBytes); f != FormatRtf {
		t.Errorf("expected FormatRtf, got %s", f)
	}

	// Extension fallback
	if f := DetectFormat("data.csv", nil); f != FormatCsv {
		t.Errorf("expected FormatCsv, got %s", f)
	}
	if f := DetectFormat("report.docx", nil); f != FormatDocx {
		t.Errorf("expected FormatDocx, got %s", f)
	}
	if f := DetectFormat("presentation.pptx", nil); f != FormatPptx {
		t.Errorf("expected FormatPptx, got %s", f)
	}
	if f := DetectFormat("sheets.xlsx", nil); f != FormatXlsx {
		t.Errorf("expected FormatXlsx, got %s", f)
	}
}

func TestConvertCsv(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test.csv")
	csvContent := "Name,Role,Status\nAlice,Developer,Active\nBob,Designer,Pending\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Convert(context.Background(), ConvertOptions{
		FilePath: csvPath,
	})
	if err != nil {
		t.Fatalf("Convert CSV failed: %v", err)
	}

	if res.Format != "csv" {
		t.Errorf("expected format csv, got %s", res.Format)
	}
	if !strings.Contains(res.Markdown, "| Name | Role | Status |") {
		t.Errorf("expected markdown table header, got:\n%s", res.Markdown)
	}
	if !strings.Contains(res.Markdown, "| Alice | Developer | Active |") {
		t.Errorf("expected table row for Alice, got:\n%s", res.Markdown)
	}
}

func TestConvertMaxLines(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "long.txt")
	lines := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n"
	if err := os.WriteFile(txtPath, []byte(lines), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Convert(context.Background(), ConvertOptions{
		FilePath: txtPath,
		MaxLines: 4,
	})
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	if !res.Truncated {
		t.Errorf("expected truncated=true, got false")
	}
	if !strings.Contains(res.Markdown, "Content truncated") {
		t.Errorf("expected truncation notice in markdown")
	}
}

func TestInspect(t *testing.T) {
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "sample.csv")
	if err := os.WriteFile(csvPath, []byte("A,B\n1,2\n3,4\n"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Inspect(csvPath)
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	if res.Format != "csv" {
		t.Errorf("expected format csv, got %s", res.Format)
	}
	if res.EstimatedUnits < 3 {
		t.Errorf("expected at least 3 rows, got %d", res.EstimatedUnits)
	}
}
