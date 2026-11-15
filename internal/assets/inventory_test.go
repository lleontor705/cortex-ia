package assets

import (
	"path"
	"strings"
	"testing"
	"testing/fstest"
)

func TestInventoryFSSkipsAppleDouble(t *testing.T) {
	fsys := fstest.MapFS{
		"AGENTS.md":            {Data: []byte("agents")},
		"._AGENTS.md":          {Data: []byte("junk")},
		"opencode.jsonc":       {Data: []byte("{}")},
		"agents/build.md":      {Data: []byte("agent")},
		"agents/._build.md":    {Data: []byte("junk")},
		"tui/status.js":        {Data: []byte("tui")},
		"tui/._status.js":      {Data: []byte("junk")},
		"themes/cortex.json":   {Data: []byte("{}")},
		"themes/._cortex.json": {Data: []byte("junk")},
	}

	files, err := InventoryFS(fsys)
	if err != nil {
		t.Fatalf("InventoryFS must skip AppleDouble entries, got error: %v", err)
	}

	seen := make(map[string]bool, len(files))
	for _, file := range files {
		if strings.HasPrefix(path.Base(file.Path), "._") {
			t.Errorf("inventory included AppleDouble entry %q", file.Path)
		}
		seen[file.Path] = true
	}

	for _, want := range []string{"AGENTS.md", "opencode.jsonc", "agents/build.md", "tui/status.js", "themes/cortex.json"} {
		if !seen[want] {
			t.Errorf("inventory is missing real asset %q", want)
		}
	}
	if len(files) != 5 {
		t.Errorf("expected exactly 5 assets, got %d: %v", len(files), files)
	}
}

func TestIsAppleDouble(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"._AGENTS.md", true},
		{"themes/._cortex.json", true},
		{"AGENTS.md", false},
		{"agents/_shared.md", false},
	} {
		if got := isAppleDouble(tc.path); got != tc.want {
			t.Errorf("isAppleDouble(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
