package prompt

import (
	"strings"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/components/sdd/ir"
)

func TestEstimateTokensCeilsRunesDividedByThree(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content string
		want    int
	}{
		{name: "empty", content: "", want: 0},
		{name: "one rune", content: "x", want: 1},
		{name: "three runes ceil to one", content: "abc", want: 1},
		{name: "four runes ceil to two", content: "abcd", want: 2},
		{name: "six runes ceil to two", content: "abcdef", want: 2},
		{name: "seven runes ceil to three", content: "abcdefg", want: 3},
		{name: "multibyte counted as one rune", content: "héllo", want: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens([]byte(tt.content))
			if got != tt.want {
				t.Fatalf("EstimateTokens(%q) = %d, want %d", tt.content, got, tt.want)
			}
		})
	}
}

func TestBudgetLimitsMatchDesignCeilings(t *testing.T) {
	if RootIndexMaxTokens != 1500 {
		t.Fatalf("RootIndexMaxTokens = %d, want 1500", RootIndexMaxTokens)
	}
	if RoleStubMaxTokens != 300 {
		t.Fatalf("RoleStubMaxTokens = %d, want 300", RoleStubMaxTokens)
	}
	if SharedContractMaxTokens != 1000 {
		t.Fatalf("SharedContractMaxTokens = %d, want 1000", SharedContractMaxTokens)
	}
	if ProfileOverlayMaxTokens != 800 {
		t.Fatalf("ProfileOverlayMaxTokens = %d, want 800", ProfileOverlayMaxTokens)
	}
}

func TestValidateAssetBudgetRejectsOverLimitRootIndex(t *testing.T) {
	overLimit := strings.Repeat("a", 1500*3+1) // 1501 tokens
	spec := ir.AssetSpec{
		ID: "asset/root-index", Class: ir.AssetRootIndex,
		SourcePath: "root.md", Required: true, MaxTokens: RootIndexMaxTokens, SHA256: "abc",
	}
	if err := ValidateAssetBudget(spec, []byte(overLimit)); err == nil {
		t.Fatal("ValidateAssetBudget over-limit root-index returned nil, want rejection")
	}
}

func TestValidateAssetBudgetAcceptsAtLimitRootIndex(t *testing.T) {
	atLimit := strings.Repeat("a", 1500*3) // exactly 1500 tokens
	spec := ir.AssetSpec{
		ID: "asset/root-index", Class: ir.AssetRootIndex,
		SourcePath: "root.md", Required: true, MaxTokens: RootIndexMaxTokens, SHA256: "abc",
	}
	if err := ValidateAssetBudget(spec, []byte(atLimit)); err != nil {
		t.Fatalf("ValidateAssetBudget at-limit root-index error = %v", err)
	}
}

func TestValidateAssetBudgetRejectsOverLimitRoleStub(t *testing.T) {
	overLimit := strings.Repeat("b", 300*3+1) // 301 tokens
	spec := ir.AssetSpec{
		ID: "asset/role-stub", Class: ir.AssetRoleStub,
		SourcePath: "stub.md", Required: true, MaxTokens: RoleStubMaxTokens, SHA256: "def",
	}
	if err := ValidateAssetBudget(spec, []byte(overLimit)); err == nil {
		t.Fatal("ValidateAssetBudget over-limit role-stub returned nil, want rejection")
	}
}

func TestValidateAssetBudgetRejectsOverLimitSharedContract(t *testing.T) {
	overLimit := strings.Repeat("c", 1000*3+1) // 1001 tokens
	spec := ir.AssetSpec{
		ID: "asset/shared-contract", Class: ir.AssetSharedContract,
		SourcePath: "contract.md", Required: true, MaxTokens: SharedContractMaxTokens, SHA256: "ghi",
	}
	if err := ValidateAssetBudget(spec, []byte(overLimit)); err == nil {
		t.Fatal("ValidateAssetBudget over-limit shared-contract returned nil, want rejection")
	}
}

func TestValidateAssetBudgetRejectsOverLimitProfileOverlay(t *testing.T) {
	overLimit := strings.Repeat("d", 800*3+1) // 801 tokens
	spec := ir.AssetSpec{
		ID: "asset/profile-overlay", Class: ir.AssetProfileOverlay,
		SourcePath: "overlay.md", Required: true, MaxTokens: ProfileOverlayMaxTokens, SHA256: "jkl",
	}
	if err := ValidateAssetBudget(spec, []byte(overLimit)); err == nil {
		t.Fatal("ValidateAssetBudget over-limit profile-overlay returned nil, want rejection")
	}
}

func TestValidateAssetBudgetAcceptsUncappedAssetClasses(t *testing.T) {
	spec := ir.AssetSpec{
		ID: "asset/skill", Class: ir.AssetSkill,
		SourcePath: "skill.md", Required: true, MaxTokens: 0, SHA256: "mno",
	}
	if err := ValidateAssetBudget(spec, []byte(strings.Repeat("e", 9999))); err != nil {
		t.Fatalf("ValidateAssetBudget uncapped skill error = %v", err)
	}
}
