package registry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/lleontor705/cortex-ia/internal/model"
)

// Normalize-stage rule names. Diagnostics cite these instead of restating
// thresholds or grammar so reports stay stable across policy revisions.
const (
	// RuleSkillIDGrammar guards the strict lowercase ASCII skill ID grammar.
	RuleSkillIDGrammar = "skill-id-grammar"
	// RuleContentEncoding guards UTF-8 validity of skill content.
	RuleContentEncoding = "content-encoding"
	// RulePathResolution guards canonical path resolution.
	RulePathResolution = "path-resolution"
)

// NormalizeSkillID validates raw against the strict lowercase ASCII skill ID
// grammar and returns it typed. The grammar accepts one or more [a-z0-9]
// segments separated by single hyphens, matching every canonical embedded
// skill ID. Normalization is identity on valid input: there is no case
// folding, Unicode normalization, trimming, or other silent rewriting, so
// equivalent IDs are byte-identical IDs and every deviation is rejected.
func NormalizeSkillID(raw string) (model.SkillID, error) {
	invalid := func(reason string) (model.SkillID, error) {
		return "", fmt.Errorf("rule %s: skill id %q %s", RuleSkillIDGrammar, raw, reason)
	}
	if raw == "" {
		return invalid("is empty")
	}
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			continue
		case c == '-':
			if i == 0 {
				return invalid("must not start with a hyphen")
			}
			if raw[i-1] == '-' {
				return invalid("must not contain consecutive hyphens")
			}
			if i == len(raw)-1 {
				return invalid("must not end with a hyphen")
			}
		default:
			return invalid(fmt.Sprintf("contains byte 0x%02x outside the lowercase ASCII grammar", c))
		}
	}
	return model.SkillID(raw), nil
}

// CanonicalPath resolves path to its canonical filesystem identity: an
// absolute, symlink-resolved, cleaned path. Canonicalization goes through
// real filesystem resolution (Abs then EvalSymlinks), never lexical
// rewriting, so equivalent spellings and links to one source converge to a
// single canonical path and absent paths fail closed. It makes no trust,
// containment, or regularity decision; those belong to the load stage.
func CanonicalPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("rule %s: path is empty", RulePathResolution)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("rule %s: make %q absolute: %w", RulePathResolution, path, err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("rule %s: resolve %q: %w", RulePathResolution, absolute, err)
	}
	return filepath.Clean(resolved), nil
}

// NormalizeContent canonicalizes skill bytes and returns them with their
// SHA-256 digest as a bare lowercase hex string. Content must already be
// valid UTF-8; line endings are normalized from CRLF to LF before hashing so
// the digest is deterministic across platforms. No other transformation is
// applied: case, Unicode form, and spacing are preserved byte-for-byte so
// canonicalization can never silently change skill identity.
func NormalizeContent(content []byte) ([]byte, string, error) {
	if !utf8.Valid(content) {
		return nil, "", fmt.Errorf("rule %s: content is not valid UTF-8", RuleContentEncoding)
	}
	canonical := content
	if bytes.Contains(canonical, []byte("\r\n")) {
		canonical = bytes.ReplaceAll(canonical, []byte("\r\n"), []byte("\n"))
	}
	sum := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(sum[:]), nil
}

// BuildSkillSet returns the deterministically ordered effective skill set.
// Skills are sorted by normalized ID (then digest, then content, so ordering
// is total even while a duplicate ID is pending its merge-stage collision
// diagnostic) and the ordered ID/digest pairs are fingerprinted, removing
// source-order dependence: equivalent inputs yield identical Ordered slices
// and an identical Fingerprint. Duplicate IDs are preserved, never silently
// deduplicated, because collision policy belongs to the merge stage.
func BuildSkillSet(skills []Skill) SkillSet {
	ordered := make([]Skill, len(skills))
	copy(ordered, skills)
	slices.SortFunc(ordered, func(a, b Skill) int {
		if c := strings.Compare(string(a.ID), string(b.ID)); c != 0 {
			return c
		}
		if c := strings.Compare(a.ContentSHA256, b.ContentSHA256); c != 0 {
			return c
		}
		return bytes.Compare(a.Content, b.Content)
	})
	var record strings.Builder
	for _, skill := range ordered {
		record.WriteString(string(skill.ID))
		record.WriteByte(':')
		record.WriteString(skill.ContentSHA256)
		record.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(record.String()))
	return SkillSet{Ordered: ordered, Fingerprint: hex.EncodeToString(sum[:])}
}
