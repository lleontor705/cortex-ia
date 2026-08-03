package prompt

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestNormalizeParagraphCollapsesWhitespaceAndLowercases(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{name: "trims leading/trailing", in: "  hello world  ", want: "hello world"},
		{name: "collapses internal spaces", in: "hello   world", want: "hello world"},
		{name: "collapses tabs and newlines", in: "hello\t\nworld", want: "hello world"},
		{name: "lowercases uppercase", in: "HELLO World", want: "hello world"},
		{name: "collapses mixed whitespace", in: "\t  Hello \r\n World  \t", want: "hello world"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeParagraph(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeParagraph(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHashParagraphIsDeterministic(t *testing.T) {
	h1 := hashParagraph("hello world")
	h2 := hashParagraph("hello world")
	if h1 != h2 {
		t.Fatalf("hashParagraph is not deterministic: %q != %q", h1, h2)
	}
	if h1 == "" {
		t.Fatal("hashParagraph returned empty hash")
	}
}

func TestHashParagraphIgnoresWhitespaceAndCase(t *testing.T) {
	h1 := hashParagraph("Hello World")
	h2 := hashParagraph("  hello   world  ")
	if h1 != h2 {
		t.Fatalf("normalized hashes differ: %q != %q", h1, h2)
	}
}

func TestScanForDuplicatesDetectsDuplicatedParagraphs(t *testing.T) {
	// Two assets sharing a paragraph above 40 tokens.
	longText := strings.Repeat("word ", 50) // ~50 tokens
	assets := []AssetContent{
		{Spec: ir.AssetSpec{ID: "asset/root", Class: ir.AssetRootIndex, SourcePath: "a.md", Required: true, MaxTokens: 1500, SHA256: "x"}, Content: []byte(longText)},
		{Spec: ir.AssetSpec{ID: "asset/module", Class: ir.AssetRootModule, SourcePath: "b.md", Required: true, MaxTokens: 0, SHA256: "y"}, Content: []byte(longText)},
	}
	findings, err := ScanForDuplicates(assets)
	if err != nil {
		t.Fatalf("ScanForDuplicates error = %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("ScanForDuplicates returned no findings for duplicated paragraphs")
	}
}

func TestScanForDuplicatesIgnoresShortDuplicates(t *testing.T) {
	// Two assets sharing a paragraph below 40 tokens — permitted.
	shortText := strings.Repeat("word ", 10) // ~10 tokens
	assets := []AssetContent{
		{Spec: ir.AssetSpec{ID: "asset/root", Class: ir.AssetRootIndex, SourcePath: "a.md", Required: true, MaxTokens: 1500, SHA256: "x"}, Content: []byte(shortText)},
		{Spec: ir.AssetSpec{ID: "asset/module", Class: ir.AssetRootModule, SourcePath: "b.md", Required: true, MaxTokens: 0, SHA256: "y"}, Content: []byte(shortText)},
	}
	findings, err := ScanForDuplicates(assets)
	if err != nil {
		t.Fatalf("ScanForDuplicates error = %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("ScanForDuplicates returned %d findings for short duplicates, want 0", len(findings))
	}
}

func TestScanForDuplicatesPassesUniqueContent(t *testing.T) {
	assets := []AssetContent{
		{Spec: ir.AssetSpec{ID: "asset/root", Class: ir.AssetRootIndex, SourcePath: "a.md", Required: true, MaxTokens: 1500, SHA256: "x"}, Content: []byte(strings.Repeat("alpha ", 50))},
		{Spec: ir.AssetSpec{ID: "asset/module", Class: ir.AssetRootModule, SourcePath: "b.md", Required: true, MaxTokens: 0, SHA256: "y"}, Content: []byte(strings.Repeat("beta ", 50))},
	}
	findings, err := ScanForDuplicates(assets)
	if err != nil {
		t.Fatalf("ScanForDuplicates error = %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("ScanForDuplicates returned %d findings for unique content, want 0", len(findings))
	}
}

func TestScanForDuplicatesDetectsAcrossManyAssets(t *testing.T) {
	longText := strings.Repeat("shared ", 50)
	assets := make([]AssetContent, 5)
	for i := range assets {
		assets[i] = AssetContent{
			Spec:    ir.AssetSpec{ID: ir.SemanticID("asset/" + string(rune('a'+i))), Class: ir.AssetRootModule, SourcePath: "f.md", Required: true, SHA256: "h"},
			Content: []byte(longText),
		}
	}
	findings, err := ScanForDuplicates(assets)
	if err != nil {
		t.Fatalf("ScanForDuplicates error = %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("ScanForDuplicates found %d duplicate findings, want 1 (one paragraph shared by 5 assets)", len(findings))
	}
	if len(findings[0].AssetIDs) != 5 {
		t.Fatalf("finding lists %d asset IDs, want 5", len(findings[0].AssetIDs))
	}
}
