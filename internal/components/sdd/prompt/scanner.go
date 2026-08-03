package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// AssetContent pairs an asset spec with its resolved content so that the
// scanner can evaluate duplication across the complete installed bundle.
type AssetContent struct {
	Spec    ir.AssetSpec
	Content []byte
}

// DuplicateFinding records one paragraph that appears in more than one asset
// and exceeds the DuplicateParagraphTokenThreshold. Duplicated paragraphs above
// the threshold are forbidden by the duplicate-hash gate (design §5).
type DuplicateFinding struct {
	AssetIDs []ir.SemanticID
	Hash     string
	Preview  string
	Tokens   int
}

// normalizeParagraph collapses all whitespace runs to single spaces, trims
// leading/trailing space, and lowercases so that whitespace and case variants
// hash identically.
func normalizeParagraph(text string) string {
	fields := strings.Fields(text)
	return strings.ToLower(strings.Join(fields, " "))
}

// hashParagraph returns the deterministic SHA-256 hex digest of the normalized
// paragraph text. Identical normalized paragraphs always produce identical
// hashes.
func hashParagraph(text string) string {
	sum := sha256.Sum256([]byte(normalizeParagraph(text)))
	return hex.EncodeToString(sum[:])
}

// extractParagraphs splits content into paragraphs on blank-line boundaries,
// trims each, and returns only non-empty paragraphs.
func extractParagraphs(content []byte) []string {
	text := string(content)
	// Split on one or more blank lines (two or more consecutive newlines).
	raw := strings.Split(text, "\n\n")
	result := make([]string, 0, len(raw))
	for _, p := range raw {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// ScanForDuplicates detects paragraphs that appear in more than one asset and
// exceed the DuplicateParagraphTokenThreshold. Each finding names the asset IDs
// sharing the duplicated paragraph. The scanner is deterministic: identical
// content always produces identical findings in stable order.
func ScanForDuplicates(assets []AssetContent) ([]DuplicateFinding, error) {
	// Map from paragraph hash to the assets containing it and its token count.
	type occurrence struct {
		assetIDs []ir.SemanticID
		tokens   int
		preview  string
	}
	seen := make(map[string]*occurrence)

	for _, ac := range assets {
		paragraphs := extractParagraphs(ac.Content)
		for _, p := range paragraphs {
			h := hashParagraph(p)
			tokens := EstimateTokens([]byte(p))
			occ, exists := seen[h]
			if !exists {
				occ = &occurrence{
					assetIDs: []ir.SemanticID{ac.Spec.ID},
					tokens:   tokens,
					preview:  truncatePreview(p),
				}
				seen[h] = occ
				continue
			}
			// Avoid recording the same asset twice for the same paragraph.
			alreadyListed := false
			for _, id := range occ.assetIDs {
				if id == ac.Spec.ID {
					alreadyListed = true
					break
				}
			}
			if !alreadyListed {
				occ.assetIDs = append(occ.assetIDs, ac.Spec.ID)
			}
		}
	}

	var findings []DuplicateFinding
	for h, occ := range seen {
		if len(occ.assetIDs) < 2 {
			continue
		}
		if occ.tokens <= DuplicateParagraphTokenThreshold {
			continue
		}
		findings = append(findings, DuplicateFinding{
			AssetIDs: occ.assetIDs,
			Hash:     h,
			Preview:  occ.preview,
			Tokens:   occ.tokens,
		})
	}
	return findings, nil
}

// truncatePreview limits the preview to 120 runes for diagnostics.
func truncatePreview(text string) string {
	runes := []rune(text)
	if len(runes) > 120 {
		return string(runes[:120]) + "..."
	}
	return text
}
