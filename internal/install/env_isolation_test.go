package install

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestEnvironmentIsolation(t *testing.T) {
	origEnv := os.Getenv(EnvBackgroundSubagentsKey)
	defer func() {
		if origEnv == "" {
			_ = os.Unsetenv(EnvBackgroundSubagentsKey)
		} else {
			_ = os.Setenv(EnvBackgroundSubagentsKey, origEnv)
		}
	}()

	t.Run("runner is replaced, records intended argv, and prevents real execution", func(t *testing.T) {
		type call struct {
			command string
			args    []string
		}
		var calls []call

		cleanup := SetWindowsEnvRunnerForTesting(func(cmd string, args ...string) ([]byte, error) {
			calls = append(calls, call{command: cmd, args: args})
			return []byte("success"), nil
		})
		defer cleanup()

		if err := configureWindowsEnv(); err != nil {
			t.Fatalf("configureWindowsEnv failed: %v", err)
		}

		if len(calls) != 1 {
			t.Fatalf("expected 1 call to WindowsEnvRunner, got %d", len(calls))
		}

		c := calls[0]
		if c.command != "powershell" {
			t.Errorf("expected command 'powershell', got %q", c.command)
		}

		joinedArgs := strings.Join(c.args, " ")
		if !strings.Contains(joinedArgs, "-NoProfile") || !strings.Contains(joinedArgs, "-NonInteractive") {
			t.Errorf("expected non-interactive profile flags in args: %v", c.args)
		}

		expectedSubstr := `[Environment]::SetEnvironmentVariable("OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS", "true", "User")`
		if !strings.Contains(joinedArgs, expectedSubstr) {
			t.Errorf("expected PowerShell command to contain %q, got %q", expectedSubstr, joinedArgs)
		}
	})

	t.Run("runner error is visibly surfaced with output", func(t *testing.T) {
		cleanup := SetWindowsEnvRunnerForTesting(func(cmd string, args ...string) ([]byte, error) {
			return []byte("PermissionDenied"), errors.New("exit status 1")
		})
		defer cleanup()

		err := configureWindowsEnv()
		if err == nil {
			t.Fatal("expected error from failing runner, got nil")
		}

		errMsg := err.Error()
		if !strings.Contains(errMsg, "set Windows user environment variable") {
			t.Errorf("expected error to describe operation, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "PermissionDenied") {
			t.Errorf("expected error to include command output, got: %s", errMsg)
		}
	})

	t.Run("missing substitute blocks execution", func(t *testing.T) {
		cleanup := SetWindowsEnvRunnerForTesting(nil)
		defer cleanup()

		err := configureWindowsEnv()
		if err == nil {
			t.Fatal("expected error when runner is nil, got nil")
		}
		if !strings.Contains(err.Error(), "windows environment runner substitute is missing") {
			t.Errorf("expected missing substitute error, got: %v", err)
		}
	})

	t.Run("cleanup restores previous runner", func(t *testing.T) {
		dummyRunner := func(cmd string, args ...string) ([]byte, error) {
			return []byte("dummy"), nil
		}

		cleanup1 := SetWindowsEnvRunnerForTesting(dummyRunner)
		cleanup2 := SetWindowsEnvRunnerForTesting(func(cmd string, args ...string) ([]byte, error) {
			return []byte("inner"), nil
		})

		cleanup2()
		envRunnerMu.Lock()
		active := currentWindowsEnvRunner
		envRunnerMu.Unlock()

		out, _ := active("test")
		if string(out) != "dummy" {
			t.Errorf("expected restored runner to output 'dummy', got %q", string(out))
		}

		cleanup1()
	})
}
