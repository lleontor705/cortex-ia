package manifest

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEmitPreservesMetadataSentinel(t *testing.T) {
	sentinel := json.RawMessage(`{"profile":"profile-sentinel","quality":"quality-sentinel","observability":"trace-sentinel"}`)
	input := validInput()
	input.Metadata = sentinel
	output, err := Emit(input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output.SecurityJSON), "profile-sentinel") || !strings.Contains(string(output.DegradationJSON), "trace-sentinel") {
		t.Fatalf("manifest dropped metadata: security=%s degradation=%s", output.SecurityJSON, output.DegradationJSON)
	}
}
