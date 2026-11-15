package install

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// CortexBinaryName is the executable the installer probes for.
	CortexBinaryName = "cortex"
	// CortexModulePath is the canonical module the preflight installs.
	CortexModulePath = "github.com/lleontor705/cortex/v2/cmd/cortex@latest"
	// CortexManualCommand is the remedy operators can run by hand when the
	// automatic preflight cannot complete.
	CortexManualCommand = "go install " + CortexModulePath
	// cortexInstallTimeout bounds the automatic install so a stalled toolchain
	// or network can never block the caller's install flow indefinitely.
	cortexInstallTimeout = 5 * time.Minute
	// cortexOutputLimit caps retained go-install output; it is never echoed
	// beyond this bound.
	cortexOutputLimit = 2 * 1024
)

// CortexInstallOutcome classifies one preflight resolution.
type CortexInstallOutcome string

const (
	// CortexPresent reports the binary was already resolvable.
	CortexPresent CortexInstallOutcome = "present"
	// CortexInstalled reports the automatic install succeeded and the binary
	// became resolvable.
	CortexInstalled CortexInstallOutcome = "installed"
	// CortexFailed reports the binary is still unresolvable; the caller
	// degrades to the manual command instead of failing.
	CortexFailed CortexInstallOutcome = "failed"
)

// CortexInstallResult is the bounded outcome of the cortex binary preflight.
type CortexInstallResult struct {
	Outcome CortexInstallOutcome
	// Path is the resolved executable path when the outcome is present or
	// installed.
	Path string
	// Detail explains a failure without echoing unbounded command output.
	Detail string
	// Manual is the command operators run when the automatic path fails.
	Manual string
}

// cortexBinaryLocator and cortexInstallRunner are substitution seams: tests
// must never depend on the host toolchain or reach the network.
var (
	cortexBinaryLocator = locateCortexBinary
	cortexInstallRunner = runCortexInstall
	// cortexNoticeSink receives human-facing preflight notices. It defaults to
	// stdout; tests redirect it to keep assertions deterministic.
	cortexNoticeSink io.Writer = os.Stdout
)

// SetCortexNoticeSinkForTesting redirects preflight notices during tests and
// returns a restore function. A nil sink is ignored.
func SetCortexNoticeSinkForTesting(sink io.Writer) func() {
	previous := cortexNoticeSink
	if sink != nil {
		cortexNoticeSink = sink
	}
	return func() { cortexNoticeSink = previous }
}

func cortexNoticef(format string, args ...any) {
	if cortexNoticeSink == nil {
		return
	}
	_, _ = fmt.Fprintf(cortexNoticeSink, format, args...)
}

// SetCortexBinaryLocatorForTesting substitutes the resolver during tests and
// returns a restore function. A nil locator is ignored.
func SetCortexBinaryLocatorForTesting(locator func() (string, bool)) func() {
	previous := cortexBinaryLocator
	if locator != nil {
		cortexBinaryLocator = locator
	}
	return func() { cortexBinaryLocator = previous }
}

// SetCortexInstallRunnerForTesting substitutes the installer during tests and
// returns a restore function. A nil runner is ignored.
func SetCortexInstallRunnerForTesting(runner func(context.Context) error) func() {
	previous := cortexInstallRunner
	if runner != nil {
		cortexInstallRunner = runner
	}
	return func() { cortexInstallRunner = previous }
}

// CortexBinaryPath resolves the cortex executable: PATH first, then the Go bin
// directory. The second step matters on Windows, where a shell never refreshes
// its inherited PATH after `go install` writes the binary.
func CortexBinaryPath() (string, bool) {
	return cortexBinaryLocator()
}

// CortexBinaryMissing reports whether the cortex executable cannot be resolved.
func CortexBinaryMissing() bool {
	_, ok := cortexBinaryLocator()
	return !ok
}

func locateCortexBinary() (string, bool) {
	if found, err := exec.LookPath(CortexBinaryName); err == nil {
		return found, true
	}
	for _, candidate := range cortexBinCandidates() {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

// cortexBinCandidates lists the Go bin paths a fresh `go install` targets,
// derived from the process environment plus the default GOPATH layout.
func cortexBinCandidates() []string {
	name := cortexExecutableName()
	var candidates []string
	if gobin := strings.TrimSpace(os.Getenv("GOBIN")); gobin != "" {
		candidates = append(candidates, filepath.Join(gobin, name))
	}
	for _, entry := range filepath.SplitList(strings.TrimSpace(os.Getenv("GOPATH"))) {
		if entry != "" {
			candidates = append(candidates, filepath.Join(entry, "bin", name))
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "go", "bin", name))
	}
	return candidates
}

func cortexExecutableName() string {
	if runtime.GOOS == "windows" {
		return CortexBinaryName + ".exe"
	}
	return CortexBinaryName
}

// cortexInstallArgv is the shell-free argv the preflight executes.
func cortexInstallArgv() []string {
	return []string{"go", "install", CortexModulePath}
}

// EnsureCortexBinary resolves the cortex executable, running `go install` when
// it is absent. Failure is a value, never an error: the caller reports the
// manual remedy and continues.
func EnsureCortexBinary(ctx context.Context) CortexInstallResult {
	if path, ok := cortexBinaryLocator(); ok {
		return CortexInstallResult{Outcome: CortexPresent, Path: path}
	}
	ctx, cancel := context.WithTimeout(ctx, cortexInstallTimeout)
	defer cancel()
	if err := cortexInstallRunner(ctx); err != nil {
		return CortexInstallResult{Outcome: CortexFailed, Detail: err.Error(), Manual: CortexManualCommand}
	}
	if path, ok := cortexBinaryLocator(); ok {
		return CortexInstallResult{Outcome: CortexInstalled, Path: path}
	}
	return CortexInstallResult{
		Outcome: CortexFailed,
		Detail:  "the binary was installed but is not resolvable yet; open a new shell so PATH picks up the Go bin directory",
		Manual:  CortexManualCommand,
	}
}

func runCortexInstall(ctx context.Context) error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("the Go toolchain ('go') is not available on PATH")
	}
	argv := cortexInstallArgv()
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.WaitDelay = time.Second
	command.Stdout = io.Discard
	output := &cortexOutput{limit: cortexOutputLimit}
	command.Stderr = output
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("go install timed out: %w", ctx.Err())
		}
		if tail := strings.TrimSpace(output.String()); tail != "" {
			return fmt.Errorf("go install failed: %w: %s", err, tail)
		}
		return fmt.Errorf("go install failed: %w", err)
	}
	return nil
}

// cortexOutput retains at most limit bytes of command output.
type cortexOutput struct {
	limit int
	buf   []byte
}

func (o *cortexOutput) Write(p []byte) (int, error) {
	if remaining := o.limit - len(o.buf); remaining > 0 {
		o.buf = append(o.buf, p[:min(len(p), remaining)]...)
	}
	return len(p), nil
}

func (o *cortexOutput) String() string {
	return string(o.buf)
}
