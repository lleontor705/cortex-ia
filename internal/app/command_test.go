package app

import (
	"strings"
	"testing"
)

func TestRunCLI_RemovedGGACommand(t *testing.T) {
	err := runCLI([]string{"gga", "--list"})
	if err == nil {
		t.Fatal("runCLI(gga) returned nil; removed commands must be rejected")
	}
	if !strings.Contains(err.Error(), "unknown command: gga") {
		t.Fatalf("runCLI(gga) error = %q, want unknown-command error", err)
	}
}
