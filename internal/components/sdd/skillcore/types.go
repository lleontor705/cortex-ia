// Package skillcore declares the shared declarative skill core contracts:
// Skill, SkillSet, and OriginKind. It is a leaf package in the component
// graph: it imports no SDD component and no agent package, so registries,
// adapters, and renderers can share these types without creating import
// cycles. It owns data shape only; provenance, policy, merge, and host
// representation stay in their own policy owners, and the type never grows
// agents, tools, permissions, or binding fields.
package skillcore

import (
	"github.com/lleontor705/cortex-ia/internal/model"
)

// OriginKind distinguishes where a skill's bytes come from.
type OriginKind uint8

const (
	// OriginEmbedded marks a skill from the embedded canonical baseline.
	OriginEmbedded OriginKind = iota
	// OriginCustom marks a skill declared by the local overlay config.
	OriginCustom
)

// Skill is one verified, normalized skill record. It intentionally carries
// only identity and content: agents, tools, permissions, and bindings are
// never fields of a declarative skill and are rejected as unsupported.
type Skill struct {
	// ID is the normalized lowercase ASCII skill identifier.
	ID model.SkillID
	// Content is the canonical UTF-8/LF skill body used for hashing.
	Content []byte
	// ContentSHA256 is the SHA-256 digest of Content.
	ContentSHA256 string
	// Origin reports whether the skill is embedded or custom.
	Origin OriginKind
}

// SkillSet is the deterministically ordered effective skill collection.
type SkillSet struct {
	// Ordered lists skills sorted by normalized ID.
	Ordered []Skill
	// Fingerprint is the stable digest of the ordered set.
	Fingerprint string
}
