package compiler

import (
	"encoding/json"
	"testing"
)

func TestCompilePreservesImmutableMetadataThroughComposition(t *testing.T) {
	sentinel := json.RawMessage(`{"contract":"contract-sentinel","profile":"profile-sentinel","primary":"primary-sentinel","fallback":"fallback-sentinel","quality":"quality-sentinel","trust":"trust-sentinel","permissions":["filesystem/read"],"gate":"gate-sentinel","observability":"trace-sentinel"}`)
	input := validInput(t)
	input.Metadata = sentinel
	result, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Normalized.Metadata) != string(sentinel) {
		t.Fatalf("metadata lost across compiler: normalized=%s", result.Normalized.Metadata)
	}
}
