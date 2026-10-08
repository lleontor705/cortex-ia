package delegation

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

// HighRiskKeywords are paths or keywords that trigger mandatory security reviews.
var HighRiskKeywords = []string{
	"auth", "oauth", "jwt", "session", "password", "credential",
	"crypto", "cipher", "tls", "ssl", "cert",
	"sql", "migration", "schema", "secrets", "vault",
}

// DetectHighRiskPaths evaluates whether any assigned file paths touch security-sensitive surfaces.
func DetectHighRiskPaths(paths []string) bool {
	for _, p := range paths {
		lower := strings.ToLower(filepath.ToSlash(p))
		for _, kw := range HighRiskKeywords {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}
	return false
}

// ErrSpecKitRetired reports the retired Spec Kit plane. Contract encoding rejects
// `speckit` before any acceptance path is reachable, so this guard only backstops
// callers that still hold a historically persisted spec_plane.
var ErrSpecKitRetired = errors.New("speckit specification plane is retired; use openspec, cortex, or hybrid")

// ProjectSpecKitState is the retired Spec Kit projector retained as a fail-closed
// stub so a historically persisted speckit contract cannot silently re-enter the
// projection path. New contracts cannot carry the plane: encodeContract rejects it.
func (s *Store) ProjectSpecKitState(context.Context, string, string, string) error {
	return ErrSpecKitRetired
}
