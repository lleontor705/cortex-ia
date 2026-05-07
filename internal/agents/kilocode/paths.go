package kilocode

import "path/filepath"

// ConfigPath returns the configuration directory path for Kilocode.
func ConfigPath(homeDir string) string {
	return filepath.Join(homeDir, ".config", "kilo")
}
