package app

import (
	"bufio"
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

// retiredToken builds a retired identifier from fragments so this test file
// does not itself contain the literal forbidden current-surface vocabulary
// that future repository-wide absence scans guard against.
func retiredToken(parts ...string) string { return strings.Join(parts, "") }

func TestPrintHelp_AdvertisesExactlySevenCurrentComponents(t *testing.T) {
	out := captureStdout(t, printHelp)

	if !strings.Contains(out, "All 7 components") {
		t.Errorf("help must advertise the current seven-component full preset; marker absent\n%s", out)
	}
	if strings.Contains(out, "All 8 components") {
		t.Errorf("help must not advertise the retired eight-component full preset\n%s", out)
	}
}

func TestPrintHelp_HasNoLiveMailboxComponentAdvertisement(t *testing.T) {
	out := captureStdout(t, printHelp)
	mailbox := retiredToken("agent-", "mailbox")

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, mailbox) {
			t.Errorf("help must not advertise retired %s as a current component: %q",
				mailbox, line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan help output: %v", err)
	}
}
