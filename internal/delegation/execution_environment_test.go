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
	for _, parent := range [][]string{nil, {"CORTEX_IA_AGY_AUTH=keyring"}, {"CORTEX_IA_AGY_AUTH=gemini"}} {
		home, _, err := executionEnvironment(parent)
		if err == nil || !strings.Contains(err.Error(), "AGY_AUTH_REQUIRED") || home != "" {
			t.Fatalf("expected actionable prelaunch rejection: %q %v", home, err)
		}
	}
	_, _, err := runAGY(context.Background(), Request{}, RoleConfig{Model: "claude-sonnet"}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "AGY_AUTH_MODEL_UNSUPPORTED") {
		t.Fatalf("expected model rejection: %v", err)
	}
}
