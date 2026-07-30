package prompt

import (
	"fmt"
	"unicode/utf8"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

// Installed prompt budget ceilings from design §5. These are Go compile-time
// constants enforced by ValidateAssetBudget so that no over-limit asset can
// reach installation. The estimator is deterministic: ceil(UTF-8 runes/3).
const (
	RootIndexMaxTokens      = 1500
	RoleStubMaxTokens       = 300
	SharedContractMaxTokens = 1000
	ProfileOverlayMaxTokens = 800
	// DuplicateParagraphTokenThreshold is the maximum normalized token count
	// at which a duplicated paragraph is still permitted. Any duplicate
	// exceeding this is forbidden by the duplicate-hash gate.
	DuplicateParagraphTokenThreshold = 40
)

// budgetCeilingForClass returns the deterministic token ceiling for a budgeted
// asset class, or zero to signal that the class is uncapped. Only installed
// prompt assets (root index, role stub, shared contract, profile overlay) are
// bounded; skills, commands, schemas, and other assets have no compile-time
// token limit because their size is governed by other quality gates.
func budgetCeilingForClass(class ir.AssetClass) int {
	switch class {
	case ir.AssetRootIndex:
		return RootIndexMaxTokens
	case ir.AssetRoleStub:
		return RoleStubMaxTokens
	case ir.AssetSharedContract:
		return SharedContractMaxTokens
	case ir.AssetProfileOverlay:
		return ProfileOverlayMaxTokens
	default:
		return 0
	}
}

// EstimateTokens returns the conservative deterministic token estimate:
// ceil(UTF-8 rune count / 3). No optional tokenizer may waive this estimate
// for installed assets per design §5.
func EstimateTokens(content []byte) int {
	runes := utf8.RuneCount(content)
	if runes == 0 {
		return 0
	}
	return (runes + 2) / 3
}

// BudgetViolation records one asset whose content exceeds its class ceiling.
type BudgetViolation struct {
	AssetID ir.SemanticID
	Class   ir.AssetClass
	Actual  int
	Limit   int
}

// Error implements the error interface with a structured message.
func (v BudgetViolation) Error() string {
	return fmt.Sprintf("asset %q (class %s) exceeds token budget: %d > %d", v.AssetID, v.Class, v.Actual, v.Limit)
}

// ValidateAssetBudget enforces the deterministic token ceiling for the asset's
// class. Uncapped classes pass without estimation. Capped classes are rejected
// if EstimateTokens(content) exceeds the class ceiling.
func ValidateAssetBudget(spec ir.AssetSpec, content []byte) error {
	limit := budgetCeilingForClass(spec.Class)
	if limit == 0 {
		return nil
	}
	actual := EstimateTokens(content)
	if actual > limit {
		return BudgetViolation{AssetID: spec.ID, Class: spec.Class, Actual: actual, Limit: limit}
	}
	return nil
}
