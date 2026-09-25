package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lleontor705/cortex-ia/internal/clidetect"
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

const (
	TUIPluginPath       = "./tui-plugins/cortex-ia-tui.js"
	TUIPluginDirV2      = "./tui-plugins/cortex-ia"
	LegacyTUIPluginPath = "./plugins/cortex-ia-tui.js"

	cliConfigName      = "cli.json"
	tuiConfigName      = "tui.jsonc"
	tuiJSONName        = "tui.json"
	opencodeConfigName = "opencode.jsonc"
	opencodeJSONName   = "opencode.json"
	tuiSchemaV1        = "https://opencode.ai/tui.json"
	cliSchemaV2        = "https://opencode.ai/v2/cli.json"
	opencodeSchemaV2   = "https://opencode.ai/config.json"
	cortexThemeName    = "cortex"
)

// OpenCode configuration generations the installer distinguishes. V2 loads
// cli.json plus the managed opencode.jsonc plugins array; v1 loads tui.json(c)
// only.
const (
	opencodeMajorV1 = 1
	opencodeMajorV2 = 2
)

// builtinAgentsDisabled lists the OpenCode built-in agents cortex-ia replaces
// with its orchestrator-led agent set. Disabling them at the config point
// removes them from the merged agent catalog; the 'general' built-in is left
// untouched because cortex-ia does not supersede it.
var builtinAgentsDisabled = []string{"build", "plan", "explore"}

// opencodeVersionDetector resolves the installed OpenCode CLI's major version.
// It is a seam so tests never execute a host binary.
var opencodeVersionDetector = detectOpenCodeCLIMajor

// SetOpenCodeVersionDetectorForTesting substitutes the OpenCode CLI
// major-version probe and returns a restoration function.
func SetOpenCodeVersionDetectorForTesting(detector func(homeDir string) (int, bool)) func() {
	previous := opencodeVersionDetector
	opencodeVersionDetector = detector
	return func() { opencodeVersionDetector = previous }
}

func detectOpenCodeCLIMajor(homeDir string) (int, bool) {
	info := clidetect.DetectOpenCode(homeDir)
	if !info.Found {
		return 0, false
	}
	return parseMajorVersion(info.Version)
}

// parseMajorVersion extracts the leading numeric major from a version string
// such as "2.0.16" or "v1.4.2".
func parseMajorVersion(version string) (int, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(version), "v")
	digits := trimmed
	if index := strings.IndexFunc(trimmed, func(r rune) bool { return r < '0' || r > '9' }); index >= 0 {
		digits = trimmed[:index]
	}
	if digits == "" {
		return 0, false
	}
	major, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return major, true
}

// detectOpenCodeConfigMajor returns the OpenCode generation that owns the
// configuration directory. Durable v2 (cli.json) and v1 (tui.json[c]) artifacts
// win because OpenCode itself wrote them; an otherwise empty home falls back to
// the installed CLI version, and the product default is v2.
func detectOpenCodeConfigMajor(homeDir, configDir string) int {
	if fileExists(filepath.Join(configDir, cliConfigName)) {
		return opencodeMajorV2
	}
	if fileExists(filepath.Join(configDir, tuiConfigName)) || fileExists(filepath.Join(configDir, tuiJSONName)) {
		return opencodeMajorV1
	}
	if major, ok := opencodeVersionDetector(homeDir); ok && major < opencodeMajorV2 {
		return opencodeMajorV1
	}
	return opencodeMajorV2
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ThemeApplyOutcome reports how the cortex theme was resolved by one
// configuration pass.
type ThemeApplyOutcome string

const (
	// ThemeOutcomeApplied reports the cortex theme entry was written or refreshed.
	ThemeOutcomeApplied ThemeApplyOutcome = "applied"
	// ThemeOutcomeSkipped reports the TUI plugin effect never ran, so the theme
	// key was left untouched.
	ThemeOutcomeSkipped ThemeApplyOutcome = "skipped-not-requested"
	// ThemeOutcomeError reports the theme could not be written.
	ThemeOutcomeError ThemeApplyOutcome = "error"
)

// ConfigureTUIPlugin ensures OpenCode's configuration contains the cortex-ia TUI
// plugin entry and applies the bundled cortex theme.
func ConfigureTUIPlugin(homeDir string) (string, error) {
	path, _, _, err := ConfigureTUIPluginWithResult(homeDir, false)
	return path, err
}

// ConfigureTUIPluginWithResult ensures the configuration file the detected
// OpenCode generation actually loads contains the cortex-ia TUI plugin entry
// and the cortex theme, ensures the OpenCode v2 bridge directory exists, and
// reports whether anything changed.
//
// V2 (cli.json present, or the installed CLI reports v2+) registers the plugin
// in cli.json and in the managed opencode.jsonc plugins array, disables the
// superseded OpenCode built-in agents there, and never writes tui.json(c). V1
// keeps the tui.jsonc/tui.json target and leaves opencode.jsonc alone.
//
// The theme is applied unconditionally: OpenCode only loads a theme named in
// the configuration, so a skipped write let an external client rewrite drop the
// key and silently revert the cortex theme. applyTheme is retained for call-site
// compatibility but no longer gates the write.
func ConfigureTUIPluginWithResult(homeDir string, applyTheme bool) (string, bool, ThemeApplyOutcome, error) {
	_ = applyTheme
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", false, ThemeOutcomeError, err
		}
	}
	configDir := filepath.Join(homeDir, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", false, ThemeOutcomeError, err
	}

	var primaryPath string
	var changed bool
	var err error
	if detectOpenCodeConfigMajor(homeDir, configDir) >= opencodeMajorV2 {
		primaryPath, changed, err = configureV2TUI(configDir)
	} else {
		primaryPath, changed, err = configureV1TUI(configDir)
	}
	if err != nil {
		return "", false, ThemeOutcomeError, err
	}

	_ = ensureTUIBridge(configDir)
	_ = CleanupLegacyFlatFiles(homeDir)
	return primaryPath, changed, ThemeOutcomeApplied, nil
}

// configureV2TUI writes the plugin and theme to cli.json and registers the
// plugin plus the built-in agent disablement in the managed opencode.jsonc.
func configureV2TUI(configDir string) (string, bool, error) {
	cliPath := filepath.Join(configDir, cliConfigName)
	cliChanged, err := configureSingleTUIFile(cliPath)
	if err != nil {
		return "", false, err
	}
	managedChanged, err := configureManagedOpenCodeConfig(configDir)
	if err != nil {
		return "", false, err
	}
	return cliPath, cliChanged || managedChanged, nil
}

// configureV1TUI preserves the v1 tui.jsonc/tui.json target cascade and never
// touches the managed opencode.jsonc.
func configureV1TUI(configDir string) (string, bool, error) {
	primaryPath := v1TUIConfigPath(configDir)
	secondaryPaths := []string{}
	if primaryPath == filepath.Join(configDir, tuiConfigName) {
		if secondary := filepath.Join(configDir, tuiJSONName); fileExists(secondary) {
			secondaryPaths = append(secondaryPaths, secondary)
		}
	}
	primaryChanged, err := configureSingleTUIFile(primaryPath)
	if err != nil {
		return "", false, err
	}
	anyChanged := primaryChanged
	for _, secondary := range secondaryPaths {
		secondaryChanged, secondaryErr := configureSingleTUIFile(secondary)
		if secondaryErr == nil && secondaryChanged {
			anyChanged = true
		}
	}
	return primaryPath, anyChanged, nil
}

// v1TUIConfigPath mirrors the v1 install cascade: an existing tui.jsonc wins,
// then tui.json, and a fresh v1 home materializes tui.jsonc.
func v1TUIConfigPath(configDir string) string {
	tuiC := filepath.Join(configDir, tuiConfigName)
	if fileExists(tuiC) {
		return tuiC
	}
	tui := filepath.Join(configDir, tuiJSONName)
	if fileExists(tui) {
		return tui
	}
	return tuiC
}

// managedOpenCodeConfigPath follows the MCP manager's load precedence: JSONC is
// loaded after JSON and therefore owns conflicting keys when both exist.
func managedOpenCodeConfigPath(configDir string) string {
	jsonc := filepath.Join(configDir, opencodeConfigName)
	if fileExists(jsonc) {
		return jsonc
	}
	jsonFile := filepath.Join(configDir, opencodeJSONName)
	if fileExists(jsonFile) {
		return jsonFile
	}
	return jsonc
}

// configureManagedOpenCodeConfig applies the durable OpenCode v2 channel: it
// registers the TUI plugin in the managed opencode.jsonc plugins array and
// disables the built-in agents cortex-ia supersedes. The plugins array is
// rebuilt as a union so third-party entries survive and exactly one cortex-ia
// entry remains; the agents overlay only sets the disabled flag, so unrelated
// agent entries and any keys already on the built-ins are preserved by the
// value-level merge.
func configureManagedOpenCodeConfig(configDir string) (bool, error) {
	path := managedOpenCodeConfigPath(configDir)
	current := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		decoded, decodeErr := filemerge.DecodeJSONObject(raw)
		if decodeErr != nil {
			return false, decodeErr
		}
		current = decoded
	} else if !os.IsNotExist(err) {
		return false, err
	}

	plugins, err := cortexPluginUnion(current["plugins"])
	if err != nil {
		return false, err
	}

	overlay, err := json.Marshal(map[string]any{
		"$schema": opencodeSchemaV2,
		"plugins": plugins,
		"agents":  builtinAgentDisableOverlay(),
	})
	if err != nil {
		return false, err
	}
	mutated, err := filemerge.MutateJSONFile(path, filemerge.JSONMutation{Overlay: overlay})
	if err != nil {
		return false, err
	}
	return mutated.Changed || mutated.Created, nil
}

// cortexPluginUnion rebuilds the plugins array so every third-party entry is
// preserved and the v2 cortex-ia entry appears exactly once at the end.
func cortexPluginUnion(configured any) ([]any, error) {
	plugins := []any{}
	if configured != nil {
		values, ok := configured.([]any)
		if !ok {
			return nil, errors.New("OpenCode config plugins must be an array")
		}
		plugins = append(plugins, values...)
	}
	union := make([]any, 0, len(plugins)+1)
	for _, entry := range plugins {
		if value, ok := entry.(string); ok && isCortexPluginPath(value) {
			continue
		}
		union = append(union, entry)
	}
	return append(union, TUIPluginDirV2), nil
}

func isCortexPluginPath(value string) bool {
	return value == TUIPluginDirV2 || value == TUIPluginPath || value == LegacyTUIPluginPath
}

func builtinAgentDisableOverlay() map[string]any {
	agents := make(map[string]any, len(builtinAgentsDisabled))
	for _, name := range builtinAgentsDisabled {
		agents[name] = map[string]any{"disabled": true}
	}
	return agents
}

func ensureTUIBridge(configDir string) error {
	bridgeDir := filepath.Join(configDir, "tui-plugins", "cortex-ia")
	if err := os.MkdirAll(bridgeDir, 0o755); err != nil {
		return err
	}
	bridgeFile := filepath.Join(bridgeDir, "tui.js")
	bridgeContent := []byte("// OpenCode v2 directory bridge for cortex-ia TUI plugin\nexport * from \"../cortex-ia-tui.js\";\nexport { default } from \"../cortex-ia-tui.js\";\n")
	if err := writeFileIfChanged(bridgeFile, bridgeContent); err != nil {
		return err
	}
	// OpenCode v2 imports the "./tui" export of the configured package, so the bridge
	// directory must expose a module manifest or the cli.json entry resolves to nothing.
	manifestFile := filepath.Join(bridgeDir, "package.json")
	manifestContent := []byte("{\n  \"name\": \"cortex-ia-tui\",\n  \"private\": true,\n  \"type\": \"module\",\n  \"exports\": {\n    \".\": \"./tui.js\",\n    \"./tui\": \"./tui.js\"\n  }\n}\n")
	return writeFileIfChanged(manifestFile, manifestContent)
}

func writeFileIfChanged(path string, content []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == string(content) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func configureSingleTUIFile(tuiPath string) (bool, error) {
	plugins := []any{}
	current := map[string]any{}
	isCLI := filepath.Base(tuiPath) == cliConfigName
	targetPlugin := TUIPluginPath
	if isCLI {
		targetPlugin = TUIPluginDirV2
	}

	if raw, readErr := os.ReadFile(tuiPath); readErr == nil {
		var decodeErr error
		current, decodeErr = filemerge.DecodeJSONObject(raw)
		if decodeErr != nil {
			return false, decodeErr
		}
		if configured, exists := current["plugin"]; exists {
			values, ok := configured.([]any)
			if !ok {
				return false, errors.New("OpenCode config plugin must be an array")
			}
			plugins = append(plugins, values...)
		}
		if configured, exists := current["plugins"]; exists {
			values, ok := configured.([]any)
			if !ok {
				return false, errors.New("OpenCode config plugins must be an array")
			}
			plugins = append(plugins, values...)
		}
	} else if !os.IsNotExist(readErr) {
		return false, readErr
	}

	filtered := make([]any, 0, len(plugins))
	for _, configured := range plugins {
		if value, ok := configured.(string); ok {
			if value == LegacyTUIPluginPath {
				continue
			}
			if isCLI && value == TUIPluginPath {
				continue
			}
		}
		filtered = append(filtered, configured)
	}
	plugins = filtered
	found := false
	for _, configured := range plugins {
		if value, ok := configured.(string); ok && value == targetPlugin {
			found = true
			break
		}
	}
	if !found {
		plugins = append(plugins, targetPlugin)
	}

	schemaURL := tuiSchemaV1
	pluginKey := "plugin"
	if isCLI {
		schemaURL = cliSchemaV2
		pluginKey = "plugins"
	} else if _, exists := current["plugins"]; exists {
		pluginKey = "plugins"
	}

	overlayMap := map[string]any{
		"$schema": schemaURL,
		pluginKey: plugins,
		"theme":   cortexThemeOverlay(current),
	}

	overlay, err := json.Marshal(overlayMap)
	if err != nil {
		return false, err
	}
	mutated, err := filemerge.MutateJSONFile(tuiPath, filemerge.JSONMutation{Overlay: overlay})
	if err != nil {
		return false, err
	}
	return mutated.Changed || mutated.Created, nil
}

// cortexThemeOverlay builds the cortex theme entry. An explicit light/dark
// preference is preserved because it encodes the user's terminal contrast
// choice, which is unrelated to the theme name. Anything else — including a
// theme name left behind by an earlier install — is refreshed to the cortex
// theme, since a stale name is what silently prevented the cortex theme from
// ever loading.
func cortexThemeOverlay(current map[string]any) map[string]any {
	mode := "dark"
	if existing, ok := current["theme"].(map[string]any); ok {
		if value, ok := existing["mode"].(string); ok && (value == "light" || value == "dark") {
			mode = value
		}
	}
	return map[string]any{
		"name": cortexThemeName,
		"mode": mode,
	}
}

// CleanupLegacyFlatFiles cleans up orphaned flat files like cortex-authority-state-*.json,
// cortex-delegation-panes-*.json, and cortex-delegation-events.jsonl in ~/.config/opencode/.
func CleanupLegacyFlatFiles(homeDir string) error {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return err
		}
	}
	configDir := filepath.Join(homeDir, ".config", "opencode")
	entries, err := os.ReadDir(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if (len(name) > 23 && name[:23] == "cortex-authority-state-" && len(name) > 5 && name[len(name)-5:] == ".json") ||
			(len(name) > 22 && name[:22] == "cortex-delegation-panes-" && len(name) > 5 && name[len(name)-5:] == ".json") ||
			name == "cortex-delegation-events.jsonl" {
			_ = os.Remove(filepath.Join(configDir, name))
		}
	}
	return nil
}
