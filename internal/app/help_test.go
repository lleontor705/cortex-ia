package app

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout captures os.Stdout during fn. printHelp writes to global
// stdout, so callers MUST NOT run t.Parallel while this helper is active.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	os.Stdout = orig

	return <-done
}

func TestPrintHelp_AdvertisesExactlySevenCurrentComponents(t *testing.T) {
	out := captureStdout(t, printHelp)

	if !strings.Contains(out, "All 7 components") {
		t.Errorf("help must advertise the current seven-component full preset; marker absent\n%s", out)
	}
	if strings.Contains(out, "All 8 components") {
		t.Errorf("help must not advertise the retired eight-component full preset\n%s", out)
	}
}

func TestPrintHelpOmitsRetiredSurfaces(t *testing.T) {
	out := captureStdout(t, printHelp)

	for _, retired := range []string{"agent-builder", "auto-install", "profiles", "--profile", "--model-preset"} {
		if strings.Contains(out, retired) {
			t.Errorf("help contains retired surface %q:\n%s", retired, out)
		}
	}
	for _, supported := range []string{"cortex-ia install", "cortex-ia sync", "cortex-ia doctor", "cortex-ia uninstall"} {
		if !strings.Contains(out, supported) {
			t.Errorf("help omits supported lifecycle command %q:\n%s", supported, out)
		}
	}
}
