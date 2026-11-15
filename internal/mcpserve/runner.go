package mcpserve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner executes one cortex-ia CLI verb. The stdout returned alongside a
// non-nil error lets callers read structured failures the CLI prints first.
type Runner interface {
	Run(ctx context.Context, argv []string, stdin string) (string, error)
}

const (
	commandTimeout = 120 * time.Second
	maxStderrBytes = 4 << 10
	maxStdoutBytes = 16 << 20
	agentShellEnv  = "CORTEX_IA_AGENT_SHELL"
)

type execRunner struct {
	dir string
}

func (r execRunner) Run(ctx context.Context, argv []string, stdin string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate cortex-ia executable: %w", err)
	}
	runCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, exe, argv...)
	cmd.Dir = r.dir
	cmd.Env = environmentWithout(agentShellEnv)
	cmd.Stdin = strings.NewReader(stdin)
	stdout := &boundedBuffer{limit: maxStdoutBytes}
	stderr := &boundedBuffer{limit: maxStderrBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if runErr := cmd.Run(); runErr != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return stdout.String(), fmt.Errorf("cortex-ia command timed out after %s", commandTimeout)
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = fmt.Sprintf("cortex-ia command failed (%v)", runErr)
		}
		return stdout.String(), errors.New(message)
	}
	return stdout.String(), nil
}

type boundedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.buf.Len(); room > 0 {
		if len(p) > room {
			p = p[:room]
		}
		b.buf.Write(p)
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string { return b.buf.String() }

func environmentWithout(name string) []string {
	prefix := name + "="
	env := os.Environ()
	kept := make([]string, 0, len(env))
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			kept = append(kept, entry)
		}
	}
	return kept
}
