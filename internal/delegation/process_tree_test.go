package delegation

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessTreeDeadlineStopsDescendant(t *testing.T) {
	home := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessTreeChild$")
	cmd.Env = []string{"SYSTEMROOT=" + os.Getenv("SYSTEMROOT"), "CORTEX_TEST_TREE=parent", "CORTEX_TEST_MARKER=" + filepath.Join(home, "heartbeat")}
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stop, err := startProcessTree(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	scanner := bufio.NewScanner(pipe)
	if !scanner.Scan() || scanner.Text() != "ready" {
		t.Fatal("descendant not ready")
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("expected deadline termination")
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(home, "heartbeat")
	before, err := os.ReadFile(marker)
	if err != nil || len(before) == 0 {
		t.Fatalf("descendant heartbeat absent: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	after, err := os.ReadFile(marker)
	if err != nil || string(before) != string(after) {
		t.Fatalf("descendant still writing after tree stop: %v", err)
	}
	// The descendant keeps this file open; Windows refuses removal until exit.
	if err := os.RemoveAll(home); err != nil {
		t.Fatalf("cleanup before descendants exited: %v", err)
	}
}

func TestProcessTreeChild(t *testing.T) {
	mode := os.Getenv("CORTEX_TEST_TREE")
	if mode == "" {
		return
	}
	if mode == "parent" {
		child := exec.Command(os.Args[0], "-test.run=^TestProcessTreeChild$")
		child.Env = append(os.Environ(), "CORTEX_TEST_TREE=descendant")
		child.Stdout = os.Stdout
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		_ = child.Wait()
		os.Exit(0)
	}
	file, err := os.Create(os.Getenv("CORTEX_TEST_MARKER"))
	if err != nil {
		os.Exit(3)
	}
	_, _ = file.WriteString("alive\n")
	fmt.Println("ready")
	for {
		_, _ = file.WriteString("alive\n")
		time.Sleep(10 * time.Millisecond)
	}
}
