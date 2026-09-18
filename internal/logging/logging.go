// Package logging provides the cortex-ia debug tracing sink. It is a strict
// no-op until Enable is called: disabled calls return after a single atomic
// load and write nothing. When enabled it writes timestamped, prefixed lines
// to stderr and, when possible, to an append-mode file under the caller
// resolved state home. It never writes to stdout, which stays reserved for
// machine-readable command receipts.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// prefix marks every debug line so it is distinguishable from normal
// command output on a shared stderr stream.
const prefix = "[debug] "

var (
	mu     sync.Mutex
	active atomic.Bool
	sinks  []io.Writer
)

// Enabled reports whether debug tracing is active. It is cheap and safe for
// hot paths that want to avoid building an expensive argument list.
func Enabled() bool {
	return active.Load()
}

// Enable activates debug tracing exactly once. It always attaches the stderr
// sink and, when stateHome is non-empty, appends to
// <stateHome>/logs/debug.log. stateHome is resolved by the caller (honoring
// CORTEX_IA_HOME, else the user home); this package performs no environment
// lookups. If the log directory or file cannot be opened, tracing stays
// stderr-only and the failure is reported once on stderr. Subsequent calls
// are no-ops.
func Enable(stateHome string) {
	mu.Lock()
	defer mu.Unlock()
	if active.Load() {
		return
	}

	writers := []io.Writer{os.Stderr}
	var openErr error
	logPath := ""
	if trimmed := stateHome; trimmed != "" {
		dir := filepath.Join(trimmed, "logs")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			openErr = err
			logPath = filepath.Join(dir, "debug.log")
		} else {
			logPath = filepath.Join(dir, "debug.log")
			file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				openErr = err
			} else {
				writers = append(writers, file)
			}
		}
	}

	sinks = writers
	active.Store(true)

	if openErr != nil {
		fmt.Fprintf(os.Stderr, "%sfile sink unavailable at %s: %v (stderr-only tracing)\n", prefix, logPath, openErr)
	}
}

// Debugf writes one formatted debug line when tracing is enabled. It is a
// no-op otherwise.
func Debugf(format string, args ...any) {
	if !active.Load() {
		return
	}
	emit(fmt.Sprintf(format, args...))
}

// Debug writes one debug line built from the arguments' default formatting
// when tracing is enabled. It is a no-op otherwise.
func Debug(args ...any) {
	if !active.Load() {
		return
	}
	emit(fmt.Sprint(args...))
}

// emit serializes one prefixed, timestamped line to every sink. The mutex
// keeps concurrent subsystem writes from interleaving mid-line.
func emit(message string) {
	line := time.Now().Format(time.RFC3339) + " " + prefix + message + "\n"
	mu.Lock()
	defer mu.Unlock()
	if !active.Load() {
		return
	}
	for _, sink := range sinks {
		_, _ = io.WriteString(sink, line)
	}
}
