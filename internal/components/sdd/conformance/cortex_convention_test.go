package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func cortexAssetRoot(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(root, "..", "..", "..", ".."))
}

func cortexTokens(content string) int { return (utf8.RuneCountInString(content) + 2) / 3 }

func normalizedParagraphHashes(content string) map[string]string {
	hashes := make(map[string]string)
	for _, paragraph := range strings.Split(content, "\n\n") {
		normalized := strings.Join(strings.Fields(paragraph), " ")
		if normalized == "" {
			continue
		}
		sum := sha256.Sum256([]byte(normalized))
		hashes[hex.EncodeToString(sum[:])] = normalized
	}
	return hashes
}

func TestCortexConventionHasOneCommonAuthorityAndProgressiveModule(t *testing.T) {
	root := cortexAssetRoot(t)
	sharedDir := filepath.Join(root, "internal", "assets", "skills", "_shared")
	entries, err := os.ReadDir(sharedDir)
	if err != nil {
		t.Fatal(err)
	}
	var conventions, advanced []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.Contains(name, "cortex-convention") {
			conventions = append(conventions, entry.Name())
		}
		if strings.Contains(name, "cortex-advanced") {
			advanced = append(advanced, entry.Name())
		}
	}
	if len(conventions) != 1 {
		t.Fatalf("want exactly one Cortex convention, got %v", conventions)
	}
	if len(advanced) > 1 {
		t.Fatalf("want at most one progressive Cortex module, got %v", advanced)
	}
	convention, err := os.ReadFile(filepath.Join(sharedDir, conventions[0]))
	if err != nil {
		t.Fatal(err)
	}
	if got := cortexTokens(string(convention)); got < 700 || got > 1000 {
		t.Fatalf("Cortex convention budget: got %d tokens, want 700..1000", got)
	}
	text := string(convention)
	for _, required := range []string{"cortex_save", "cortex_search", "cortex_get_observation", "cortex_session_summary", "transport"} {
		if !strings.Contains(strings.ToLower(text), required) {
			t.Errorf("Cortex convention missing current contract marker %q", required)
		}
	}
	if strings.Contains(text, "mem_") {
		t.Error("Cortex convention contains legacy mem_* namespace")
	}
	if len(advanced) == 1 {
		module, err := os.ReadFile(filepath.Join(sharedDir, advanced[0]))
		if err != nil {
			t.Fatal(err)
		}
		if got := cortexTokens(string(module)); got < 150 || got > 300 {
			t.Fatalf("Cortex advanced budget: got %d tokens, want 150..300", got)
		}
	}
}

func TestCortexPolicyIsNotDuplicatedAcrossRootAssets(t *testing.T) {
	root := cortexAssetRoot(t)
	paths := []string{
		filepath.Join(root, "internal", "assets", "generic", "sdd-orchestrator-root-index.md"),
		filepath.Join(root, "internal", "assets", "generic", "sdd-root"),
	}
	var contents []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.IsDir() {
			err = filepath.Walk(path, func(file string, info os.FileInfo, walkErr error) error {
				if walkErr != nil || info.IsDir() || !strings.HasSuffix(file, ".md") {
					return walkErr
				}
				data, readErr := os.ReadFile(file)
				if readErr != nil {
					return readErr
				}
				contents = append(contents, string(data))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		contents = append(contents, string(data))
	}
	seen := map[string]int{}
	for _, content := range contents {
		for hash := range normalizedParagraphHashes(content) {
			seen[hash]++
		}
	}
	for hash, count := range seen {
		if count > 1 {
			t.Fatalf("duplicated normalized root policy paragraph %s appears %d times", hash, count)
		}
	}
}
