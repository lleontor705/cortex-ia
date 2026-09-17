package delegation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Default authentication uses AGY's account/keyring without copying profiles.
// Explicit Gemini authentication uses the provider setting documented at
// https://antigravity.google/docs/cli/install/#using-a-gemini-api-key .
// This does not isolate the OS user, keyring, network or workspace configuration.
func executionEnvironment(parent []string) (home string, env []string, err error) {
	values := make(map[string]string)
	for _, item := range parent {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[strings.ToUpper(key)] = value
		}
	}
	mode := values["CORTEX_IA_AGY_AUTH"]
	if mode != "" && mode != "account" && mode != "gemini" {
		return "", nil, fmt.Errorf("AGY_AUTH_UNSUPPORTED: select account authentication or explicit gemini authentication")
	}
	key := values["GEMINI_API_KEY"]
	if mode == "gemini" && (strings.TrimSpace(key) == "" || len(key) > 8192 || strings.ContainsAny(key, "\x00\r\n")) {
		return "", nil, fmt.Errorf("AGY_AUTH_REQUIRED: provide a non-empty bounded GEMINI_API_KEY for explicitly selected Gemini authentication")
	}
	home, err = os.MkdirTemp("", "cortex-ia-execution-*")
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(home)
		}
	}()
	for _, dir := range []string{"tmp", "config", "cache", "data", "appdata", "localappdata", ".gemini/antigravity-cli"} {
		if err = os.MkdirAll(filepath.Join(home, dir), 0o700); err != nil {
			return home, nil, err
		}
	}
	if mode == "gemini" {
		if err = os.WriteFile(filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"), []byte("{\"modelProvider\":\"gemini\"}\n"), 0o600); err != nil {
			return home, nil, err
		}
		env = append(env, "GEMINI_API_KEY="+key)
	}
	// Keep executable discovery and essential OS/locale values only. In particular
	// no Cortex authority, cloud credentials, shell hooks or runtime preload flags.
	for _, name := range []string{"PATH", "PATHEXT", "SYSTEMROOT", "WINDIR", "COMSPEC", "LANG", "LC_ALL", "TERM"} {
		if value := values[name]; value != "" {
			env = append(env, name+"="+value)
		}
	}
	for _, pair := range [][2]string{{"HOME", home}, {"USERPROFILE", home}, {"APPDATA", filepath.Join(home, "appdata")}, {"LOCALAPPDATA", filepath.Join(home, "localappdata")}, {"XDG_CONFIG_HOME", filepath.Join(home, "config")}, {"XDG_CACHE_HOME", filepath.Join(home, "cache")}, {"XDG_DATA_HOME", filepath.Join(home, "data")}, {"TMPDIR", filepath.Join(home, "tmp")}, {"TMP", filepath.Join(home, "tmp")}, {"TEMP", filepath.Join(home, "tmp")}} {
		env = append(env, pair[0]+"="+pair[1])
	}
	return home, append(env, "AGY_CLI_DISABLE_AUTO_UPDATE=true"), nil
}
