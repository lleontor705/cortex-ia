package theme

import (
	"testing"

	"github.com/lleontor705/cortex-ia/internal/agents/opencode"
)

func TestInject_OpenCode(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := opencode.NewAdapter()

	result, err := Inject(tmpDir, adapter, ThemeCortex)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}
}

func TestInject_DefaultTheme(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := opencode.NewAdapter()

	// Empty theme → defaults to cortex.
	result, err := Inject(tmpDir, adapter, "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Error("expected Changed=true with default theme")
	}
}

func TestInject_InvalidTheme(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := opencode.NewAdapter()

	_, err := Inject(tmpDir, adapter, "nonexistent")
	if err == nil {
		t.Fatal("expected error for invalid theme")
	}
}

func TestInject_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	adapter := opencode.NewAdapter()

	if _, err := Inject(tmpDir, adapter, ThemeCortex); err != nil {
		t.Fatal(err)
	}
	second, err := Inject(tmpDir, adapter, ThemeCortex)
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed {
		t.Error("expected idempotent")
	}
}
