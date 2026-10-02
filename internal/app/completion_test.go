package app

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunCompletion(t *testing.T) {
	shells := []string{"bash", "zsh", "fish"}
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := runCompletion([]string{shell})

			_ = w.Close()
			os.Stdout = oldStdout

			if err != nil {
				t.Fatalf("runCompletion(%q) failed: %v", shell, err)
			}
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			out := buf.String()
			if !strings.Contains(out, "cortex-ia") {
				t.Fatalf("output for %s does not mention cortex-ia", shell)
			}
			if !strings.Contains(out, "work") || !strings.Contains(out, "board") {
				t.Fatalf("output for %s missing key subcommands", shell)
			}
		})
	}

	t.Run("unsupported shell returns error", func(t *testing.T) {
		err := runCompletion([]string{"powershell"})
		if err == nil {
			t.Fatal("expected error for unsupported shell, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported shell") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}
