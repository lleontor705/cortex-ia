package delegation

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecutionEnvironment(t *testing.T) {
	parent := []string{"CORTEX_IA_AGY_AUTH=gemini", "GEMINI_API_KEY=synthetic-key", "PATH=" + os.Getenv("PATH"), "SYSTEMROOT=" + os.Getenv("SYSTEMROOT"), "HOME=forbidden-home", "CORTEX_IA_HOME=forbidden-authority", "AWS_SECRET_ACCESS_KEY=forbidden-secret", "NODE_OPTIONS=forbidden-hook"}
	home, env, err := executionEnvironment(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	settings, err := os.ReadFile(filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"))
	if err != nil || strings.TrimSpace(string(settings)) != `{"modelProvider":"gemini"}` {
		t.Fatalf("minimal provider settings: %s, %v", settings, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestEnvironmentChild$")
	cmd.Env = append(env, "CORTEX_TEST_ENV_CHILD=1")
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var actual []string
	if err := json.Unmarshal(output, &actual); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(actual, "\n")
	for _, forbidden := range []string{"forbidden", "CORTEX_IA_AGY_AUTH=", "AWS_SECRET", "NODE_OPTIONS"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("inherited forbidden environment %s", forbidden)
		}
	}
	for _, expected := range []string{"HOME=" + home, "USERPROFILE=" + home, "GEMINI_API_KEY=synthetic-key"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing expected environment %s", expected)
		}
	}
}

func TestEnvironmentChild(t *testing.T) {
	if os.Getenv("CORTEX_TEST_ENV_CHILD") != "1" {
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(os.Environ())
	os.Exit(0)
}

func TestExecutionAuthenticationMissing(t *testing.T) {
	for _, parent := range [][]string{{"CORTEX_IA_AGY_AUTH=gemini"}, {"CORTEX_IA_AGY_AUTH=gemini", "GEMINI_API_KEY=bad\nkey"}} {
		home, _, err := executionEnvironment(parent)
		if err == nil || !strings.Contains(err.Error(), "AGY_AUTH_REQUIRED") || home != "" {
			t.Fatalf("expected actionable prelaunch rejection: %q %v", home, err)
		}
	}
	t.Setenv("CORTEX_IA_AGY_AUTH", "gemini")
	_, _, err := runAGY(context.Background(), nil, Request{}, RoleConfig{Model: "claude-sonnet"}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "AGY_AUTH_MODEL_UNSUPPORTED") {
		t.Fatalf("expected model rejection: %v", err)
	}
}

func TestExecutionAccountEnvironment(t *testing.T) {
	for _, mode := range []string{"", "account"} {
		t.Run("mode="+mode, func(t *testing.T) {
			home, env, err := executionEnvironment([]string{"CORTEX_IA_AGY_AUTH=" + mode, "GEMINI_API_KEY=unrequested-secret", "HOME=forbidden-home", "USERPROFILE=forbidden-profile", "APPDATA=forbidden-appdata", "XDG_CONFIG_HOME=forbidden-config", "CORTEX_IA_HOME=forbidden-authority", "AWS_SECRET_ACCESS_KEY=forbidden-secret", "NODE_OPTIONS=forbidden-hook"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(home); err != nil {
					t.Error(err)
				}
			})
			if _, err := os.Stat(filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")); !os.IsNotExist(err) {
				t.Fatalf("account mode must not force provider settings: %v", err)
			}
			joined := strings.Join(env, "\n")
			for _, forbidden := range []string{"forbidden", "GEMINI_API_KEY=", "CORTEX_IA_AGY_AUTH=", "CORTEX_IA_HOME=", "AWS_SECRET", "NODE_OPTIONS"} {
				if strings.Contains(joined, forbidden) {
					t.Fatalf("unexpected inherited field: %s", forbidden)
				}
			}
			for _, required := range []string{"HOME=" + home, "USERPROFILE=" + home, "APPDATA=" + filepath.Join(home, "appdata"), "XDG_CONFIG_HOME=" + filepath.Join(home, "config")} {
				if !strings.Contains(joined, required) {
					t.Fatalf("missing isolated environment: %s", required)
				}
			}
		})
	}
	if home, _, err := executionEnvironment([]string{"CORTEX_IA_AGY_AUTH=unknown"}); err == nil || home != "" {
		t.Fatal("unknown explicit mode must fail before creating a home")
	}
}

func TestExecutionAccountPreservesModelAndPermissions(t *testing.T) {
	t.Setenv("CORTEX_IA_AGY_AUTH", "")
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	for _, name := range []string{"HOME", "USERPROFILE", "LOCALAPPDATA"} {
		t.Setenv(name, home)
	}
	skip := true
	role := RoleConfig{Model: "claude-sonnet", SkipPermissions: skip}
	args := buildAGYArgs(Request{}, role, "1s", home, skip)
	if !strings.Contains(strings.Join(args, " "), "--model claude-sonnet") || !strings.Contains(strings.Join(args, " "), "--dangerously-skip-permissions") {
		t.Fatalf("user model or permission preference changed: %v", args)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Never launch a real AGY process, including system fallback paths.
	_, _, err := runAGY(ctx, nil, Request{Workspace: home}, role, time.Second)
	if err == nil || strings.Contains(err.Error(), "AGY_AUTH_") {
		t.Fatalf("account model must pass authentication checks: %v", err)
	}
}
