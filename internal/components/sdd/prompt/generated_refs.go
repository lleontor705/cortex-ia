package prompt

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/assets"
	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// GeneratedReference is receipt-visible metadata for one executable contract
// reference. Phase is intentionally separate and canonical; aliases are
// accepted only by compatibility boundaries and are never persisted here.
type GeneratedReference struct {
	ID                ir.SemanticID
	Version           string
	SourceFingerprint string
	SHA256            string
	Phase             ir.SemanticID
}

// GeneratedReferences exposes the deterministic generated contract inventory
// carried by the operational catalog exactly once.
func GeneratedReferences(catalog assets.MaterializedCatalog) ([]GeneratedReference, error) {
	refs := make([]GeneratedReference, 0, len(catalog.Generated))
	seen := make(map[ir.SemanticID]struct{}, len(catalog.Generated))
	for _, generated := range catalog.Generated {
		if generated.SemanticID == "" || generated.Version == "" || generated.SourceFingerprint == "" || generated.SHA256 == "" {
			return nil, fmt.Errorf("generated reference %q has incomplete receipt metadata", generated.SemanticID)
		}
		if strings.Contains(string(generated.SemanticID), "bootstrap") || strings.Contains(string(generated.SemanticID), "investigate") || strings.Contains(string(generated.SemanticID), "validate") {
			return nil, fmt.Errorf("generated reference %q contains a compatibility alias", generated.SemanticID)
		}
		if _, ok := seen[generated.SemanticID]; ok {
			return nil, fmt.Errorf("duplicate generated reference %q", generated.SemanticID)
		}
		seen[generated.SemanticID] = struct{}{}
		refs = append(refs, GeneratedReference{ID: generated.SemanticID, Version: generated.Version, SourceFingerprint: generated.SourceFingerprint, SHA256: generated.SHA256})
	}
	slices.SortFunc(refs, func(a, b GeneratedReference) int { return strings.Compare(string(a.ID), string(b.ID)) })
	return refs, nil
}
