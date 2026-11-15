package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lleontor705/cortex-ia/internal/installmeta"
)

const (
	testMCPRecordName        = "ctx7"
	testAgentModelRecordName = "agent-model/build"
	testFingerprintRelPath   = "opencode.jsonc"
)

func newFingerprintTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, stateDir), 0o700); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	return home
}

func testMCPDigest(t *testing.T) string {
	t.Helper()
	digest, err := installmeta.MCPPostImageDigest(installmeta.MCPServerPostImage{
		Identity:   installmeta.MCPServerIdentity{Name: testMCPRecordName, Type: "local", Command: "npx"},
		ConfigPath: testFingerprintRelPath,
	}, []byte("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"))
	if err != nil {
		t.Fatalf("compute mcpv2 digest: %v", err)
	}
	return digest
}

func testAgentModelDigest(t *testing.T) string {
	t.Helper()
	digest, err := installmeta.AgentModelIdentityDigest(installmeta.AgentModelIdentity{
		Agent:    "build",
		Provider: "anthropic",
		Model:    "claude-sonnet",
		Variant:  "high",
	})
	if err != nil {
		t.Fatalf("compute amdv1 digest: %v", err)
	}
	return digest
}

func testFingerprintDocument(salt string, records ...FingerprintRecord) FingerprintDocument {
	return FingerprintDocument{SchemaVersion: FingerprintSchemaV1, Salt: salt, Records: records}
}

func TestFingerprintDocument_AgentModelRoundTrip(t *testing.T) {
	home := newFingerprintTestHome(t)
	doc := testFingerprintDocument(DeterministicFingerprintSalt(home), FingerprintRecord{
		Name:            testAgentModelRecordName,
		ConfigPath:      testFingerprintRelPath,
		PostImageDigest: testAgentModelDigest(t),
	})

	if err := SaveFingerprintDocument(home, doc); err != nil {
		t.Fatalf("save amdv1 record: %v", err)
	}
	loaded, present, err := LoadFingerprintDocument(home)
	if err != nil {
		t.Fatalf("load amdv1 record: %v", err)
	}
	if !present {
		t.Fatal("loaded document is not present")
	}
	if len(loaded.Records) != 1 || loaded.Records[0].PostImageDigest != doc.Records[0].PostImageDigest {
		t.Fatalf("amdv1 record did not round-trip: %+v", loaded.Records)
	}
}

func TestFingerprintDocument_MPCRoundTripUnchanged(t *testing.T) {
	home := newFingerprintTestHome(t)
	doc := testFingerprintDocument(DeterministicFingerprintSalt(home), FingerprintRecord{
		Name:            testMCPRecordName,
		ConfigPath:      testFingerprintRelPath,
		PostImageDigest: testMCPDigest(t),
	})

	if err := SaveFingerprintDocument(home, doc); err != nil {
		t.Fatalf("save mcpv2 record: %v", err)
	}
	loaded, present, err := LoadFingerprintDocument(home)
	if err != nil || !present {
		t.Fatalf("load mcpv2 record: present=%v err=%v", present, err)
	}
	if len(loaded.Records) != 1 || loaded.Records[0].PostImageDigest != doc.Records[0].PostImageDigest {
		t.Fatalf("mcpv2 record did not round-trip: %+v", loaded.Records)
	}
}

func TestFingerprintDocument_MixedNamespacesRoundTrip(t *testing.T) {
	home := newFingerprintTestHome(t)
	mcpDigest := testMCPDigest(t)
	amdvDigest := testAgentModelDigest(t)
	doc := testFingerprintDocument(DeterministicFingerprintSalt(home),
		FingerprintRecord{Name: testAgentModelRecordName, ConfigPath: testFingerprintRelPath, PostImageDigest: amdvDigest},
		FingerprintRecord{Name: testMCPRecordName, ConfigPath: testFingerprintRelPath, PostImageDigest: mcpDigest},
	)

	if err := SaveFingerprintDocument(home, doc); err != nil {
		t.Fatalf("save mixed document: %v", err)
	}
	loaded, present, err := LoadFingerprintDocument(home)
	if err != nil || !present {
		t.Fatalf("load mixed document: present=%v err=%v", present, err)
	}
	if len(loaded.Records) != 2 {
		t.Fatalf("expected both records, got %+v", loaded.Records)
	}
}

func TestFingerprintDocument_AgentModelRejectsInvalidDigests(t *testing.T) {
	salt := DeterministicFingerprintSalt(t.TempDir())
	longHex := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

	cases := map[string]string{
		"empty":                  "",
		"malformed prefix":       "amdv:1234",
		"malformed sum":          "amdv1:" + longHex[:63],
		"unknown version":        "amdv9:" + longHex,
		"legacy raw hex":         longHex,
		"mcpv2 encoding":         testMCPDigest(t),
		"unversioned amd string": "amdv1:" + longHex + "0",
	}
	for name, digest := range cases {
		t.Run(name, func(t *testing.T) {
			doc := testFingerprintDocument(salt, FingerprintRecord{
				Name:            testAgentModelRecordName,
				ConfigPath:      testFingerprintRelPath,
				PostImageDigest: digest,
			})
			if err := validateFingerprintDocument(doc); err == nil {
				t.Fatalf("agent-model record with invalid digest %q was accepted", digest)
			}
		})
	}
}

func TestFingerprintDocument_OtherNamespacesRejectAgentModelDigest(t *testing.T) {
	salt := DeterministicFingerprintSalt(t.TempDir())
	amdvDigest := testAgentModelDigest(t)

	for _, name := range []string{testMCPRecordName, "agent-model", "agent-modelX/build", "mcp/agent-model"} {
		t.Run(name, func(t *testing.T) {
			doc := testFingerprintDocument(salt, FingerprintRecord{
				Name:            name,
				ConfigPath:      testFingerprintRelPath,
				PostImageDigest: amdvDigest,
			})
			if err := validateFingerprintDocument(doc); err == nil {
				t.Fatalf("non-agent-model record %q accepted an amdv1 digest", name)
			}
		})
	}
}

func TestLoadFingerprintDocument_RejectsTamperedAgentModelDigest(t *testing.T) {
	home := newFingerprintTestHome(t)
	raw := `{"schema_version":1,"salt":"` + DeterministicFingerprintSalt(home) +
		`","records":[{"name":"agent-model/build","config_path":"opencode.jsonc","postimage_digest":"amdv1:zz"}]}`
	if err := os.WriteFile(FingerprintPath(home), []byte(raw), 0o600); err != nil {
		t.Fatalf("write tampered sidecar: %v", err)
	}
	_, present, err := LoadFingerprintDocument(home)
	if err == nil || !present {
		t.Fatalf("tampered agent-model digest must fail closed: present=%v err=%v", present, err)
	}
}
