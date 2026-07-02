package skillregistry

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/assets"
	"gopkg.in/yaml.v3"
)

// RegistryVersion is the schema version for registry output.
const RegistryVersion = "1.0.0"

// --- Public API ---

// Scan scans all three skill tiers (embedded, project-level, community) and
// returns a deterministic registry sorted by skill name.
func Scan() (RegistryOutput, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return RegistryOutput{}, fmt.Errorf("get working directory: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return RegistryOutput{}, fmt.Errorf("get home directory: %w", err)
	}
	return scanSources(cwd, home)
}

// ScanCached scans with SHA1-based content cache invalidation. If the cache
// hash matches the current skill file contents, the cached result is returned
// without rescanning. Corrupt cache is silently invalidated and regenerated.
func ScanCached(ttl time.Duration) (RegistryOutput, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return RegistryOutput{}, fmt.Errorf("get working directory: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return RegistryOutput{}, fmt.Errorf("get home directory: %w", err)
	}
	cachePath := filepath.Join(cwd, ".sdd", ".skill-registry-cache")
	return scanCachedAt(cachePath, cwd, home, ttl)
}

// --- Internal scanning ---

// scanSources scans all tiers from the given project root and home directory.
// This is the testable core used by both Scan() and ScanCached().
func scanSources(projectRoot, homeDir string) (RegistryOutput, error) {
	var skills []SkillEntry

	// Tier 1: embedded assets.
	// Embedded scan errors are non-fatal — we still scan filesystem tiers.
	embedded, _ := scanEmbedded()
	skills = append(skills, embedded...)

	// Tier 2: project-level skills (filesystem).
	projectDir := filepath.Join(projectRoot, "skills")
	project := scanDir(projectDir, "project", "skills")
	skills = append(skills, project...)

	// Tier 3: community skills (filesystem, user home).
	communityDir := filepath.Join(homeDir, ".cortex-ia", "skills-community")
	community := scanDir(communityDir, "community", "~/.cortex-ia/skills-community")
	skills = append(skills, community...)

	// Deduplicate by name: project > community > embedded.
	skills = deduplicate(skills)

	// Sort by name for deterministic output.
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	return RegistryOutput{
		Skills:  skills,
		Version: RegistryVersion,
	}, nil
}

// scanCachedAt is the testable version of ScanCached with an explicit cache path.
func scanCachedAt(cachePath, projectRoot, homeDir string, ttl time.Duration) (RegistryOutput, error) {
	// Compute the current content hash across all tiers.
	currentHash, err := computeContentHash(projectRoot, homeDir)
	if err != nil {
		// Hash failure is non-fatal — fall back to full scan.
		out, scanErr := scanSources(projectRoot, homeDir)
		if scanErr != nil {
			return RegistryOutput{}, scanErr
		}
		_ = writeCache(cachePath, currentHash, out)
		return out, nil
	}

	// Attempt to read and validate the cache.
	cached, cacheHash, cacheOK := readCache(cachePath)
	if cacheOK && cacheHash == currentHash {
		// Cache hit — SHA1 matches, skip rescan.
		return cached, nil
	}

	// Cache miss (missing, corrupt, or hash mismatch) — rescan.
	out, err := scanSources(projectRoot, homeDir)
	if err != nil {
		return RegistryOutput{}, err
	}

	// Persist the cache for next time.
	_ = writeCache(cachePath, currentHash, out)

	return out, nil
}

// --- Tier scanners ---

// scanEmbedded reads skill entries from the embedded assets.FS.
func scanEmbedded() ([]SkillEntry, error) {
	entries, err := fs.ReadDir(assets.FS, "skills")
	if err != nil {
		return nil, fmt.Errorf("read embedded skills dir: %w", err)
	}

	var skills []SkillEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		if isSkippedDir(dirName) {
			continue
		}
		skillPath := "skills/" + dirName + "/SKILL.md"
		data, err := fs.ReadFile(assets.FS, skillPath)
		if err != nil {
			continue // no SKILL.md in this directory
		}
		fm := parseFrontmatter(string(data))
		name := fm.Name
		if name == "" {
			name = dirName
		}
		skills = append(skills, SkillEntry{
			ID:          dirName,
			Name:        name,
			Path:        "internal/assets/skills/" + dirName + "/SKILL.md",
			Trigger:     extractTrigger(fm.Description),
			Category:    "embedded",
			Description: fm.Description,
		})
	}
	return skills, nil
}

// scanDir reads skill entries from a filesystem directory.
// displayPrefix is used to construct the human-readable path.
func scanDir(dir, sourceName, displayPrefix string) []SkillEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // missing directory is not an error
	}

	var skills []SkillEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		if isSkippedDir(dirName) {
			continue
		}
		skillPath := filepath.Join(dir, dirName, "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue // no SKILL.md in this directory
		}
		fm := parseFrontmatter(string(data))
		name := fm.Name
		if name == "" {
			name = dirName
		}
		skills = append(skills, SkillEntry{
			ID:          dirName,
			Name:        name,
			Path:        displayPrefix + "/" + dirName + "/SKILL.md",
			Trigger:     extractTrigger(fm.Description),
			Category:    sourceName,
			Description: fm.Description,
		})
	}
	return skills
}

// --- Deduplication ---

// deduplicate removes skills with duplicate names, keeping the highest-priority source.
// Priority order: project (3) > community (2) > embedded (1).
func deduplicate(skills []SkillEntry) []SkillEntry {
	sourcePriority := map[string]int{
		"project":   3,
		"community": 2,
		"embedded":  1,
	}

	best := make(map[string]SkillEntry)
	for _, s := range skills {
		existing, ok := best[s.Name]
		if !ok {
			best[s.Name] = s
			continue
		}
		if sourcePriority[s.Category] > sourcePriority[existing.Category] {
			best[s.Name] = s
		}
	}

	result := make([]SkillEntry, 0, len(best))
	for _, s := range best {
		result = append(result, s)
	}
	return result
}

// --- Frontmatter parsing ---

// skillFrontmatter holds only the YAML fields we extract from SKILL.md headers.
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// parseFrontmatter extracts YAML frontmatter (between --- delimiters) from
// SKILL.md content and returns the parsed name and description fields.
func parseFrontmatter(content string) skillFrontmatter {
	var fm skillFrontmatter

	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return fm
	}

	// Skip the opening --- line.
	rest := trimmed[3:]
	// Find the closing --- delimiter on its own line.
	endIdx := strings.Index(rest, "\n---")
	if endIdx < 0 {
		return fm
	}

	yamlContent := strings.TrimLeft(rest[:endIdx], "\n")
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return skillFrontmatter{}
	}
	return fm
}

// extractTrigger pulls the "Trigger: ..." text out of a description string.
// Returns an empty string if no trigger is found.
func extractTrigger(description string) string {
	idx := strings.Index(description, "Trigger:")
	if idx < 0 {
		return ""
	}
	trigger := description[idx+len("Trigger:"):]
	trigger = strings.TrimSpace(trigger)
	return trigger
}

// --- SHA1 content hashing ---

// fileContent pairs a skill file's path with its raw content for hashing.
type fileContent struct {
	Path    string
	Content string
}

// computeContentHash computes a deterministic SHA1 hash of all SKILL.md file
// contents across all three tiers. Files are sorted by path before hashing
// to ensure determinism.
func computeContentHash(projectRoot, homeDir string) (string, error) {
	contents, err := collectFileContents(projectRoot, homeDir)
	if err != nil {
		return "", err
	}

	sort.Slice(contents, func(i, j int) bool {
		return contents[i].Path < contents[j].Path
	})

	h := sha1.New()
	for _, c := range contents {
		h.Write([]byte(c.Path))
		h.Write([]byte{0})
		h.Write([]byte(c.Content))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// collectFileContents reads all SKILL.md file contents from all tiers.
func collectFileContents(projectRoot, homeDir string) ([]fileContent, error) {
	var contents []fileContent

	// Embedded assets.
	entries, err := fs.ReadDir(assets.FS, "skills")
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || isSkippedDir(entry.Name()) {
				continue
			}
			skillPath := "skills/" + entry.Name() + "/SKILL.md"
			data, err := fs.ReadFile(assets.FS, skillPath)
			if err != nil {
				continue
			}
			contents = append(contents, fileContent{
				Path:    "embedded:" + skillPath,
				Content: string(data),
			})
		}
	}

	// Project-level skills.
	projectDir := filepath.Join(projectRoot, "skills")
	collectDirContents(projectDir, "project", &contents)

	// Community skills.
	communityDir := filepath.Join(homeDir, ".cortex-ia", "skills-community")
	collectDirContents(communityDir, "community", &contents)

	return contents, nil
}

// collectDirContents reads all SKILL.md files from a filesystem directory.
func collectDirContents(dir, prefix string, contents *[]fileContent) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || isSkippedDir(entry.Name()) {
			continue
		}
		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}
		*contents = append(*contents, fileContent{
			Path:    prefix + ":" + entry.Name(),
			Content: string(data),
		})
	}
}

// --- Cache persistence ---

// cacheData is the on-disk cache structure.
type cacheData struct {
	Hash   string         `json:"hash"`
	Output RegistryOutput `json:"output"`
}

// readCache reads and parses the cache file. Returns (output, hash, ok).
// On any error (missing file, corrupt JSON, etc.) returns ok=false.
func readCache(path string) (RegistryOutput, string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RegistryOutput{}, "", false
	}
	var cd cacheData
	if err := json.Unmarshal(data, &cd); err != nil {
		return RegistryOutput{}, "", false // corrupt cache
	}
	return cd.Output, cd.Hash, true
}

// writeCache persists the cache file atomically-ish.
func writeCache(path, hash string, output RegistryOutput) error {
	cd := cacheData{Hash: hash, Output: output}
	data, err := json.MarshalIndent(cd, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// --- Utilities ---

// isSkippedDir returns true for directories that should not be treated as skills.
func isSkippedDir(name string) bool {
	return name == "_shared" || name == "old"
}
