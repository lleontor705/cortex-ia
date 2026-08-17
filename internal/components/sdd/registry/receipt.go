package registry

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
	"github.com/lleontor705/cortex-ia/internal/model"
)

// ReceiptSchemaVersion is the current canonical receipt schema version. The
// schema version participates in the receipt fingerprint, so any change to
// the canonical encoding must bump this version.
const ReceiptSchemaVersion = "1.0.0"

// canonicalReceipt is the canonical JSON projection of a Receipt. Fields are
// declared in alphabetical tag order and every collection is sorted before
// encoding, so two receipts with identical effective content always encode
// to identical bytes regardless of declaration or execution order. Volatile
// fields (timestamps, absolute paths, inode/mtime facts, backup IDs, changed
// flags) have no projection fields by construction.
type canonicalReceipt struct {
	BaselineDigest      string           `json:"baseline_digest"`
	EffectiveComponents []string         `json:"effective_components"`
	EffectiveSkills     []canonicalSkill `json:"effective_skills"`
	HostOutputs         []string         `json:"host_outputs"`
	PolicyDigest        string           `json:"policy_digest"`
	SchemaVersion       string           `json:"schema_version"`
}

// canonicalSkill is the stable semantic projection of one effective skill:
// identity, origin, and content digest. Raw Content is intentionally absent
// because ContentSHA256 already carries it.
type canonicalSkill struct {
	ContentSHA256 string `json:"content_sha256"`
	ID            string `json:"id"`
	Origin        string `json:"origin"`
}

// canonicalPolicy is the canonical JSON projection of a Policy. Component
// classes are encoded as a string-keyed map so encoding/json emits them in
// sorted key order.
type canonicalPolicy struct {
	ComponentClasses map[string]string `json:"component_classes"`
	PolicyVersion    string            `json:"policy_version"`
	SchemaVersion    string            `json:"schema_version"`
}

// FingerprintPolicy returns the canonical digest of the applied disable
// policy. The digest covers the schema and policy versions plus every
// component classification, so any policy change is visible to the receipt
// fingerprint.
func FingerprintPolicy(policy Policy) string {
	classes := make(map[string]string, len(policy.ComponentClasses))
	for id, class := range policy.ComponentClasses {
		classes[string(id)] = disableClassLabel(class)
	}
	return ir.FingerprintContent(mustMarshalJSON(canonicalPolicy{
		ComponentClasses: classes,
		PolicyVersion:    policy.PolicyVersion,
		SchemaVersion:    policy.SchemaVersion,
	}))
}

// CanonicalReceiptJSON returns the canonical JSON encoding of the receipt's
// stable semantic content: schema/policy/baseline digests, effective
// components and skills, and relative host outputs. The receipt Fingerprint
// and the skill-set Fingerprint are derived values and are excluded; skill
// content is carried by ContentSHA256; and every collection is sorted, so
// the encoding depends only on the effective inputs. Timestamps, absolute
// paths, inode/mtime facts, backup IDs, changed flags, and execution order
// are never part of the projection (design D9).
func CanonicalReceiptJSON(receipt Receipt) []byte {
	skills := make([]canonicalSkill, 0, len(receipt.EffectiveSkills.Ordered))
	for _, skill := range receipt.EffectiveSkills.Ordered {
		skills = append(skills, canonicalSkill{
			ContentSHA256: skill.ContentSHA256,
			ID:            string(skill.ID),
			Origin:        originLabel(skill.Origin),
		})
	}
	slices.SortFunc(skills, compareCanonicalSkills)
	return mustMarshalJSON(canonicalReceipt{
		BaselineDigest:      receipt.BaselineDigest,
		EffectiveComponents: sortedComponentIDs(receipt.EffectiveComponents),
		EffectiveSkills:     skills,
		HostOutputs:         sortedStrings(receipt.HostOutputs),
		PolicyDigest:        receipt.PolicyDigest,
		SchemaVersion:       receipt.SchemaVersion,
	})
}

// FingerprintReceipt returns the SHA-256 of the canonical JSON encoding,
// computed without the receipt's own Fingerprint field. Identical effective
// inputs always yield the identical fingerprint.
func FingerprintReceipt(receipt Receipt) string {
	return ir.FingerprintContent(CanonicalReceiptJSON(receipt))
}

// SealReceipt returns the receipt sealed with its canonical fingerprint. The
// returned copy is first ordered (components, skills, and host outputs
// sorted) so the committed evidence is the versioned ordered struct itself.
func SealReceipt(receipt Receipt) Receipt {
	receipt.EffectiveComponents = slices.Clone(receipt.EffectiveComponents)
	slices.Sort(receipt.EffectiveComponents)
	receipt.HostOutputs = slices.Clone(receipt.HostOutputs)
	slices.Sort(receipt.HostOutputs)
	receipt.EffectiveSkills.Ordered = slices.Clone(receipt.EffectiveSkills.Ordered)
	slices.SortFunc(receipt.EffectiveSkills.Ordered, compareSkills)
	receipt.Fingerprint = FingerprintReceipt(receipt)
	return receipt
}

// ValidateReceipt verifies that a sealed receipt still matches its canonical
// content. It rejects unsealed receipts and fingerprint mismatches so stale
// or tampered evidence can never be treated as current.
func ValidateReceipt(receipt Receipt) error {
	want := receipt.Fingerprint
	if want == "" {
		return fmt.Errorf("registry receipt is not sealed")
	}
	if got := FingerprintReceipt(receipt); got != want {
		return fmt.Errorf("registry receipt fingerprint mismatch: sealed %s, canonical %s", want, got)
	}
	return nil
}

func compareSkills(first, second Skill) int {
	if diff := strings.Compare(string(first.ID), string(second.ID)); diff != 0 {
		return diff
	}
	if diff := strings.Compare(first.ContentSHA256, second.ContentSHA256); diff != 0 {
		return diff
	}
	return strings.Compare(originLabel(first.Origin), originLabel(second.Origin))
}

func compareCanonicalSkills(first, second canonicalSkill) int {
	if diff := strings.Compare(first.ID, second.ID); diff != 0 {
		return diff
	}
	if diff := strings.Compare(first.ContentSHA256, second.ContentSHA256); diff != 0 {
		return diff
	}
	return strings.Compare(first.Origin, second.Origin)
}

func sortedComponentIDs(ids []model.ComponentID) []string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, string(id))
	}
	slices.Sort(values)
	return values
}

func sortedStrings(values []string) []string {
	sorted := make([]string, 0, len(values))
	sorted = append(sorted, values...)
	slices.Sort(sorted)
	return sorted
}

func originLabel(origin OriginKind) string {
	switch origin {
	case OriginEmbedded:
		return "embedded"
	case OriginCustom:
		return "custom"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(origin))
	}
}

func disableClassLabel(class DisableClass) string {
	switch class {
	case Optional:
		return "optional"
	case ProtectedAuthority:
		return "protected_authority"
	case ProtectedWorkflow:
		return "protected_workflow"
	case ProtectedRequired:
		return "protected_required"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(class))
	}
}

func mustMarshalJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal canonical registry receipt data: %v", err))
	}
	return encoded
}
