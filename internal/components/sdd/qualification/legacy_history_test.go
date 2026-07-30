package qualification

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type diskHistoryArtifact struct {
	SourcePayload        json.RawMessage `json:"source_payload"`
	BundlePayload        json.RawMessage `json:"bundle_payload"`
	SourceSnapshotDigest string          `json:"source_snapshot_digest"`
	BundleDigest         string          `json:"bundle_digest"`
}

func TestLegacyHistoryEvidenceDiskArtifactReconstructsInSeparateProcess(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "evidence", "legacy-history", "legacy-history-evidence-v4.json")
	if os.Getenv("LEGACY_HISTORY_RECONSTRUCTION_CHILD") == "1" {
		verifyDiskHistoryArtifact(t, os.Getenv("LEGACY_HISTORY_ARTIFACT"))
		return
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestLegacyHistoryEvidenceDiskArtifactReconstructsInSeparateProcess", "-test.v")
	cmd.Env = append(os.Environ(), "LEGACY_HISTORY_RECONSTRUCTION_CHILD=1", "LEGACY_HISTORY_ARTIFACT="+path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("independent reconstruction process failed: %v\n%s", err, output)
	}
}

func verifyDiskHistoryArtifact(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var artifact diskHistoryArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	var source struct {
		RawEvidence       []RawEvidenceFile  `json:"raw_evidence"`
		LineageReferences []LineageReference `json:"lineage_references"`
	}
	if err := json.Unmarshal(artifact.SourcePayload, &source); err != nil {
		t.Fatal(err)
	}
	var bundle struct {
		Source struct {
			RawEvidence []RawEvidenceFile `json:"raw_evidence"`
		} `json:"source_payload"`
	}
	if err := json.Unmarshal(artifact.BundlePayload, &bundle); err != nil {
		t.Fatal(err)
	}
	if len(source.RawEvidence) == 0 || len(source.RawEvidence) != len(bundle.Source.RawEvidence) {
		t.Fatal("disk artifact does not embed matching raw evidence")
	}
	if len(source.LineageReferences) != 4 {
		t.Fatalf("lineage reference count = %d, want 4", len(source.LineageReferences))
	}
	for _, ref := range source.LineageReferences {
		if !sha256DigestPattern.MatchString(ref.Digest) || digestString(ref.Payload) != ref.Digest {
			t.Fatalf("lineage reference %s is not canonically resolved", ref.ID)
		}
	}
	for _, raw := range source.RawEvidence {
		bytes, err := raw.Bytes()
		if err != nil || !sha256DigestPattern.MatchString(raw.Digest) || digestBytes(bytes) != raw.Digest {
			t.Fatalf("raw evidence %s is not byte-accurate", raw.Path)
		}
	}
	sourceValue := map[string]any{}
	bundleValue := map[string]any{}
	if err := json.Unmarshal(artifact.SourcePayload, &sourceValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(artifact.BundlePayload, &bundleValue); err != nil {
		t.Fatal(err)
	}
	sourceBytes, err := canonicalJSONBytes(sourceValue)
	if err != nil {
		t.Fatal(err)
	}
	bundleBytes, err := canonicalJSONBytes(bundleValue)
	if err != nil {
		t.Fatal(err)
	}
	if got := digestString(string(sourceBytes)); got != artifact.SourceSnapshotDigest {
		t.Fatalf("source digest = %s, want %s", got, artifact.SourceSnapshotDigest)
	}
	if got := digestString(string(bundleBytes)); got != artifact.BundleDigest {
		t.Fatalf("bundle digest = %s, want %s", got, artifact.BundleDigest)
	}
}

func TestLegacyHistoryEvidenceFinalizesAndValidates(t *testing.T) {
	bundle := validLegacyHistoryEvidence()
	if err := bundle.Finalize(); err != nil {
		t.Fatal(err)
	}
	if bundle.SourceSnapshotDigest == "" || bundle.BundleDigest == "" {
		t.Fatal("finalized bundle must contain source and bundle digests")
	}
	if err := bundle.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyHistoryEvidenceIsIndependentlyReconstructible(t *testing.T) {
	bundle := validLegacyHistoryEvidence()
	if err := bundle.Finalize(); err != nil {
		t.Fatal(err)
	}
	if len(bundle.SourcePayload.Tasks) != len(bundle.Tasks) || len(bundle.BundlePayload.Source.Tasks) != len(bundle.Tasks) {
		t.Fatal("artifact does not embed complete source and bundle payloads")
	}
	sourceDigest, bundleDigest, err := ReconstructDigests(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if sourceDigest != bundle.SourceSnapshotDigest || bundleDigest != bundle.BundleDigest {
		t.Fatalf("independent reconstruction = %s/%s, want %s/%s", sourceDigest, bundleDigest, bundle.SourceSnapshotDigest, bundle.BundleDigest)
	}
}

func TestLegacyHistoryEvidenceRejectsPayloadReferenceAndRawEvidenceTampering(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*LegacyHistoryEvidence)
		want   string
	}{
		{"source payload mismatch", func(b *LegacyHistoryEvidence) { b.SourcePayload.Tasks[0].Notes = "rewritten" }, "source payload"},
		{"bundle payload mismatch", func(b *LegacyHistoryEvidence) { b.BundlePayload.ImmutableEventLog.Reason = "rewritten" }, "bundle payload"},
		{"raw evidence content mismatch", func(b *LegacyHistoryEvidence) { b.RawEvidence[0].Content = "rewritten" }, "raw evidence digest"},
		{"raw evidence placeholder digest", func(b *LegacyHistoryEvidence) { b.RawEvidence[0].Digest = "sha256:reference-placeholder" }, "canonical SHA-256 hex"},
		{"raw evidence non-hex digest", func(b *LegacyHistoryEvidence) { b.RawEvidence[0].Digest = "sha256:zzzz" }, "canonical SHA-256 hex"},
		{"lineage non-hex digest", func(b *LegacyHistoryEvidence) { b.LineageReferences[0].Digest = "sha256:reference-1487" }, "canonical SHA-256 hex"},
		{"lineage payload mismatch", func(b *LegacyHistoryEvidence) { b.LineageReferences[0].Payload = `{"id":1487,"type":"tampered"}` }, "lineage reference digest"},
		{"missing retrieval contract", func(b *LegacyHistoryEvidence) { b.Retrieval.Method = "" }, "retrieval contract"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := validLegacyHistoryEvidence()
			if err := bundle.Finalize(); err != nil {
				t.Fatal(err)
			}
			tt.mutate(&bundle)
			if err := bundle.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestLegacyHistoryEvidenceCanonicalDigestIsOrderIndependent(t *testing.T) {
	left := validLegacyHistoryEvidence()
	right := validLegacyHistoryEvidence()
	right.Tasks[0].Dependencies = []string{"task-b", "task-a"}
	left.Tasks[0].Dependencies = []string{"task-a", "task-b"}
	if err := left.Finalize(); err != nil {
		t.Fatal(err)
	}
	if err := right.Finalize(); err != nil {
		t.Fatal(err)
	}
	if left.SourceSnapshotDigest != right.SourceSnapshotDigest || left.BundleDigest != right.BundleDigest {
		t.Fatalf("canonical digests differ: %s/%s vs %s/%s", left.SourceSnapshotDigest, left.BundleDigest, right.SourceSnapshotDigest, right.BundleDigest)
	}
}

func TestLegacyHistoryEvidenceRejectsTamperingAndMissingEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*LegacyHistoryEvidence)
		want   string
	}{
		{"tampered source", func(b *LegacyHistoryEvidence) { b.Tasks[0].Notes = "changed" }, "source snapshot digest"},
		{"missing notes", func(b *LegacyHistoryEvidence) { b.Tasks[0].Notes = "" }, "notes"},
		{"missing contract", func(b *LegacyHistoryEvidence) { b.Contracts = nil }, "contract"},
		{"mode mismatch", func(b *LegacyHistoryEvidence) { b.LegacyBoard.Mode = "direct-v1" }, "legacy mode"},
		{"cardinality drift", func(b *LegacyHistoryEvidence) { b.Completeness.TaskCount++ }, "cardinality"},
		{"unavailable log hidden", func(b *LegacyHistoryEvidence) { b.ImmutableEventLog.Status = "available" }, "immutable_event_log"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := validLegacyHistoryEvidence()
			if err := bundle.Finalize(); err != nil {
				t.Fatal(err)
			}
			tt.mutate(&bundle)
			if err := bundle.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func validLegacyHistoryEvidence() LegacyHistoryEvidence {
	return LegacyHistoryEvidence{
		SchemaVersion:     "1.0.0",
		ChangeName:        "improve-agent-phase-workflows",
		LegacyBoard:       LegacyBoardSnapshot{BoardID: "board-24ab96032185490ca3ef2eb918b55837", Mode: "legacy", Revision: 1},
		Tasks:             []LegacyTaskEvidence{{TaskID: "task-1", Status: "done", Dependencies: []string{}, Notes: "completed", CreatedAt: time.Date(2026, 7, 29, 1, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 29, 2, 0, 0, 0, time.UTC)}},
		Contracts:         []ContractEvidence{{ID: "sdd-contract-1", Digest: "sha256:contract", ObservationID: 1}},
		Cortex:            []CortexLineageEvidence{{ObservationID: 1, ArtifactDigest: "sha256:artifact", RevisionHistory: []string{"rev-1"}, GraphEdges: []string{"edge-1"}}},
		Completeness:      EvidenceCompleteness{TaskCount: 1, ContractCount: 1, CortexCount: 1, StatusConsistent: true, Complete: true},
		ImmutableEventLog: ImmutableEventLogDisclosure{Status: "unavailable", Reason: "legacy board exposes no authorized immutable event stream", Limitations: []string{"notes and Cortex lineage are not immutable ForgeSpec events", "SHA-256 does not prove source authenticity or completeness"}},
		Capture:           CaptureMetadata{CapturedAt: time.Date(2026, 7, 29, 3, 0, 0, 0, time.UTC), Actor: "cortex-ia-orchestrator", Source: "tb_status+authorized Cortex reads"},
		Retrieval:         EvidenceRetrievalContract{Method: "authorized snapshot read", Authorization: "cortex-ia-orchestrator", RedactionRule: "none; no secrets captured", Complete: true},
		RawEvidence:       rawEvidence("legacy/tb_status.json", "canonical legacy snapshot", "cortex/lineage.json", "canonical Cortex lineage"),
		LineageReferences: testLineageReferences(),
	}
}

func testLineageReferences() []LineageReference {
	refs := []struct{ id, typ, payload string }{
		{"cortex-observation-1487", "observation", `{"id":1487,"topic":"sdd/improve-agent-phase-workflows/p3.3-quality-history-report","type":"bugfix"}`},
		{"cortex-revision-history-1487", "revision-history", `{"observation_id":1487,"revisions":[]}`},
		{"cortex-graph-1487", "graph-edges", `{"edges":[1493,1410,1401,1400,1397,1387,1385,1384,1383,1380,1379,1378,1377,1374,1369,1366,1365,1362,1361,1360,1359,1357,1122],"observation_id":1487}`},
		{"cortex-related-payloads-1487", "related-observations", `{"observation_ids":[1493,1410,1401,1400,1397,1387,1385,1384,1383,1380,1379,1378,1377,1374,1369,1366,1365,1362,1361,1360,1359,1357,1122]}`},
	}
	result := make([]LineageReference, 0, len(refs))
	for _, ref := range refs {
		result = append(result, LineageReference{ID: ref.id, Type: ref.typ, Payload: ref.payload, Digest: digestString(ref.payload)})
	}
	return result
}

func rawEvidence(path, content, otherPath, otherContent string) []RawEvidenceFile {
	return []RawEvidenceFile{
		{Path: path, Content: content, Digest: digestString(content)},
		{Path: otherPath, Content: otherContent, Digest: digestString(otherContent)},
	}
}
