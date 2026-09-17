package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestCortexSnapshotExactAndLateFailures(t *testing.T) {
	content := "line\r\n雪😀\n"
	b, _ := json.Marshal(map[string]any{"id": 7, "project": "p", "content": content})
	digest := sha256.Sum256([]byte(content))
	request := []CortexSnapshotRequest{{7, hex.EncodeToString(digest[:])}}
	valid := "[" + string(b) + "]"
	got, err := decodeCortexSnapshots(strings.NewReader(valid), "p", request)
	if err != nil || len(got) != 1 || got[0].Content != content || got[0].ByteLength != len(content) {
		t.Fatalf("exact snapshot: %+v %v", got, err)
	}
	for name, input := range map[string]string{
		"late-duplicate": "[" + string(b) + "," + string(b) + "]",
		"truncated":      "[" + string(b), "trailing": valid + "{}", "not-array": string(b),
		"project": strings.Replace(valid, `"project":"p"`, `"project":"other"`, 1),
		"missing": `[]`, "null": `[{"id":7,"project":"p","content":null}]`,
		"utf8":           "[{\"id\":7,\"project\":\"p\",\"content\":\"\xff\"}]",
		"surrogate":      `[{"id":7,"project":"p","content":"\ud800"}]`,
		"low-surrogate":  `[{"id":7,"project":"p","content":"\udfff"}]`,
		"late-malformed": "[" + string(b) + ",invalid]",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeCortexSnapshots(strings.NewReader(input), "p", request); err == nil {
				t.Fatal("accepted invalid export")
			}
		})
	}
	if _, err := decodeCortexSnapshots(strings.NewReader(valid), "p", []CortexSnapshotRequest{{7, strings.Repeat("0", 64)}}); err == nil {
		t.Fatal("accepted changed pin")
	}
	for _, text := range []string{`\ud83d\ude00`, `\\ud800`, `�`} {
		input := `[{"id":7,"project":"p","content":"` + text + `"}]`
		if _, err := decodeCortexSnapshots(strings.NewReader(input), "p", []CortexSnapshotRequest{{ID: 7}}); err != nil {
			t.Fatalf("valid Unicode %q: %v", text, err)
		}
	}
}

func TestCortexSnapshotStreamBounds(t *testing.T) {
	row := `{"id":8,"project":"p","content":"` + strings.Repeat("x", 900000) + `"},`
	selected := `{"id":7,"project":"p","content":"selected"}]`
	readers := []io.Reader{strings.NewReader("[")}
	for i := 0; i < 12; i++ {
		readers = append(readers, strings.NewReader(row))
	}
	readers = append(readers, strings.NewReader(selected))
	got, err := decodeCortexSnapshots(io.MultiReader(readers...), "p", []CortexSnapshotRequest{{ID: 7}})
	if err != nil || len(got) != 1 || got[0].Content != "selected" {
		t.Fatalf("stream beyond former 8MiB cap: %v", err)
	}
	for _, size := range []int{cortexContentLimit + 1, cortexRecordLimit + 1} {
		input := `[{"id":7,"project":"p","content":"` + strings.Repeat("x", size) + `"}]`
		if _, err := decodeCortexSnapshots(strings.NewReader(input), "p", []CortexSnapshotRequest{{ID: 7}}); err == nil {
			t.Fatal("accepted oversized content/record")
		}
	}
	readers = []io.Reader{strings.NewReader("[")}
	for i := 0; i < 80; i++ {
		readers = append(readers, strings.NewReader(row))
	}
	readers = append(readers, strings.NewReader(selected))
	if _, err := decodeCortexSnapshots(io.MultiReader(readers...), "p", []CortexSnapshotRequest{{ID: 7}}); err == nil {
		t.Fatal("accepted oversized total export")
	}
}

func TestCortexSnapshotProducerCompletion(t *testing.T) {
	for _, mode := range []string{"ok", "fail", "stderr", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			deadline := 5 * time.Second
			if mode == "timeout" {
				deadline = 150 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), deadline)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCortexSnapshotProducer$")
			cmd.Env = append(os.Environ(), "CORTEX_SNAPSHOT_TEST_PRODUCER="+mode)
			cmd.WaitDelay = time.Second
			got, err := readCortexSnapshotCommand(cmd, cancel, "p", []CortexSnapshotRequest{{ID: 7}})
			if mode == "ok" {
				if err != nil || len(got) != 1 {
					t.Fatalf("%v %v", got, err)
				}
			} else if err == nil {
				t.Fatal("accepted unsuccessful producer")
			}
		})
	}
}

func TestCortexSnapshotProducer(t *testing.T) {
	mode := os.Getenv("CORTEX_SNAPSHOT_TEST_PRODUCER")
	if mode == "" {
		return
	}
	fmt.Print(`[{"id":7,"project":"p","content":"exact"}]`)
	switch mode {
	case "fail":
		os.Exit(7)
	case "stderr":
		fmt.Fprint(os.Stderr, strings.Repeat("x", 128*1024))
	case "timeout":
		time.Sleep(5 * time.Second)
	}
	os.Exit(0)
}
