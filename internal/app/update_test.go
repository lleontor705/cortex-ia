package app

import (
	"testing"
)

func TestRunUpdateHelp(t *testing.T) {
	if err := runUpdate([]string{"--help"}); err != nil {
		t.Errorf("expected nil error on --help, got %v", err)
	}
	if err := runUpdate([]string{"-h"}); err != nil {
		t.Errorf("expected nil error on -h, got %v", err)
	}
}

func TestRunUpdateUnknownFlag(t *testing.T) {
	err := runUpdate([]string{"--invalid-flag"})
	if err == nil {
		t.Error("expected error for unknown flag, got nil")
	}
}
