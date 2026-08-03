package conformance

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var activeVendorTierPattern = regexp.MustCompile(`(?i)\b(?:` + strings.Join([]string{
	string([]byte{'s', 'o', 'n', 'n', 'e', 't'}),
	string([]byte{'o', 'p', 'u', 's'}),
	string([]byte{'h', 'a', 'i', 'k', 'u'}),
}, "|") + `)\b`)

var activeAssignmentTablePattern = regexp.MustCompile(`(?im)^\s*\|[^\n]*(?:phase|role)[^\n]*\|[^\n]*(?:model|tier|assignment)[^\n]*\|`)

func TestActiveDocumentationAndEmbeddedAssetsAreProviderNeutral(t *testing.T) {
	root := repositoryRoot(t)
	paths := []string{
		"README.md", "docs/configuration.md", "docs/architecture.md", "docs/agents.md",
		"docs/sdd-workflow.md", "llms-full.txt", "internal/assets/generic/sdd-root/model-routing.md",
	}
	for _, skill := range canonicalSkillDirs {
		paths = append(paths, filepath.ToSlash(filepath.Join(canonicalSkillBase, skill, "SKILL.md")))
	}
	for _, relative := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		text := string(content)
		if matches := activeVendorTierPattern.FindAllStringIndex(text, -1); len(matches) != 0 {
			t.Errorf("%s contains %d active provider-tier aliases", relative, len(matches))
		}
		if matches := activeAssignmentTablePattern.FindAllStringIndex(text, -1); len(matches) != 0 {
			t.Errorf("%s contains %d active phase/model assignment tables", relative, len(matches))
		}
	}
}

func TestActiveAssetCorpusBudgetsRemainBounded(t *testing.T) {
	root := repositoryRoot(t)
	paths := []string{rootIndexRel, sharedContractRel}
	for _, module := range rootModuleFiles {
		paths = append(paths, filepath.ToSlash(filepath.Join(rootModuleDirRel, module+".md")))
	}
	for _, skill := range canonicalSkillDirs {
		paths = append(paths, filepath.ToSlash(filepath.Join(canonicalSkillBase, skill, "SKILL.md")))
	}
	for _, relative := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		tokens := estimateTokens(string(content))
		limit := 3500
		if relative == rootIndexRel {
			limit = 1500
		} else if relative == sharedContractRel {
			limit = 1000
		} else if strings.HasPrefix(relative, rootModuleDirRel+"/") {
			limit = 300
		}
		if tokens > limit {
			t.Errorf("%s is %d tokens, exceeds budget %d", relative, tokens, limit)
		}
	}
}
