package qualification

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

// LegacyHistoryEvidence is a credential-free, tamper-evident description of
// work observed on the pre-bridge legacy board. It is evidence, not an event
// log, and never creates or implies historical ForgeSpec events.
type LegacyHistoryEvidence struct {
	SchemaVersion        string                      `json:"schema_version"`
	ChangeName           string                      `json:"change_name"`
	LegacyBoard          LegacyBoardSnapshot         `json:"legacy_board"`
	Tasks                []LegacyTaskEvidence        `json:"tasks"`
	Contracts            []ContractEvidence          `json:"contracts"`
	Cortex               []CortexLineageEvidence     `json:"cortex"`
	Completeness         EvidenceCompleteness        `json:"completeness"`
	ImmutableEventLog    ImmutableEventLogDisclosure `json:"immutable_event_log"`
	Capture              CaptureMetadata             `json:"capture"`
	Retrieval            EvidenceRetrievalContract   `json:"retrieval"`
	RawEvidence          []RawEvidenceFile           `json:"raw_evidence"`
	LineageReferences    []LineageReference          `json:"lineage_references"`
	SourcePayload        LegacySourcePayload         `json:"source_payload"`
	BundlePayload        LegacyBundlePayload         `json:"bundle_payload"`
	ArtifactLinks        []string                    `json:"artifact_links,omitempty"`
	SourceSnapshotDigest string                      `json:"source_snapshot_digest"`
	BundleDigest         string                      `json:"bundle_digest"`
}

// LegacySourcePayload is the complete normalized pre-bridge input embedded in
// the artifact for independent digest reconstruction.
type LegacySourcePayload struct {
	LegacyBoard       LegacyBoardSnapshot       `json:"legacy_board"`
	Tasks             []LegacyTaskEvidence      `json:"tasks"`
	Contracts         []ContractEvidence        `json:"contracts"`
	Cortex            []CortexLineageEvidence   `json:"cortex"`
	Completeness      EvidenceCompleteness      `json:"completeness"`
	Retrieval         EvidenceRetrievalContract `json:"retrieval"`
	RawEvidence       []RawEvidenceFile         `json:"raw_evidence"`
	LineageReferences []LineageReference        `json:"lineage_references"`
}

// LegacyBundlePayload is the complete normalized envelope hashed for the
// bundle digest. Source is duplicated intentionally and must match.
type LegacyBundlePayload struct {
	SchemaVersion     string                      `json:"schema_version"`
	ChangeName        string                      `json:"change_name"`
	Source            LegacySourcePayload         `json:"source"`
	ImmutableEventLog ImmutableEventLogDisclosure `json:"immutable_event_log"`
	Capture           CaptureMetadata             `json:"capture"`
	ArtifactLinks     []string                    `json:"artifact_links,omitempty"`
}

type EvidenceRetrievalContract struct {
	Method        string `json:"method"`
	Authorization string `json:"authorization"`
	RedactionRule string `json:"redaction_rule"`
	Complete      bool   `json:"complete"`
}

type RawEvidenceFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Digest   string `json:"digest"`
	Encoding string `json:"encoding,omitempty"`
}

func (r RawEvidenceFile) Bytes() ([]byte, error) {
	if r.Encoding == "" || r.Encoding == "utf-8" {
		return []byte(r.Content), nil
	}
	if r.Encoding == "base64" {
		return base64.StdEncoding.Strict().DecodeString(r.Content)
	}
	return nil, fmt.Errorf("unsupported raw evidence encoding %q", r.Encoding)
}

type LineageReference struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Digest  string `json:"digest"`
}

var sha256DigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type LegacyBoardSnapshot struct {
	BoardID  string `json:"board_id"`
	Mode     string `json:"mode"`
	Revision int64  `json:"revision"`
}
type LegacyTaskEvidence struct {
	TaskID       string    `json:"task_id"`
	Status       string    `json:"status"`
	Dependencies []string  `json:"dependencies"`
	Notes        string    `json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type ContractEvidence struct {
	ID            string `json:"id"`
	Digest        string `json:"digest"`
	ObservationID int64  `json:"observation_id"`
}
type CortexLineageEvidence struct {
	ObservationID   int64    `json:"observation_id"`
	ArtifactDigest  string   `json:"artifact_digest"`
	RevisionHistory []string `json:"revision_history"`
	GraphEdges      []string `json:"graph_edges"`
}
type EvidenceCompleteness struct {
	TaskCount        int  `json:"task_count"`
	ContractCount    int  `json:"contract_count"`
	CortexCount      int  `json:"cortex_count"`
	StatusConsistent bool `json:"status_consistent"`
	Complete         bool `json:"complete"`
}
type ImmutableEventLogDisclosure struct {
	Status      string   `json:"status"`
	Reason      string   `json:"reason"`
	Limitations []string `json:"limitations"`
}
type CaptureMetadata struct {
	CapturedAt time.Time `json:"captured_at"`
	Actor      string    `json:"actor"`
	Source     string    `json:"source"`
}

func (b *LegacyHistoryEvidence) Finalize() error {
	if err := b.validateShape(); err != nil {
		return err
	}
	b.SourcePayload = b.canonicalSourcePayload()
	b.BundlePayload = b.canonicalBundlePayload()
	b.SourceSnapshotDigest = b.digest(b.SourcePayload)
	b.BundleDigest = b.digest(b.BundlePayload)
	return nil
}

func (b LegacyHistoryEvidence) Validate() error {
	if err := b.validateShape(); err != nil {
		return err
	}
	if b.SourceSnapshotDigest == "" || b.BundleDigest == "" {
		return fmt.Errorf("missing source snapshot or bundle digest")
	}
	if !reflect.DeepEqual(b.SourcePayload, b.canonicalSourcePayload()) {
		return fmt.Errorf("source payload mismatch (source snapshot digest cannot be recomputed)")
	}
	if !reflect.DeepEqual(b.BundlePayload, b.canonicalBundlePayload()) {
		return fmt.Errorf("bundle payload reference mismatch")
	}
	if got := b.digest(b.SourcePayload); got != b.SourceSnapshotDigest {
		return fmt.Errorf("source snapshot digest mismatch: got %s want %s", got, b.SourceSnapshotDigest)
	}
	if got := b.digest(b.BundlePayload); got != b.BundleDigest {
		return fmt.Errorf("bundle digest mismatch: got %s want %s", got, b.BundleDigest)
	}
	return nil
}

func (b LegacyHistoryEvidence) validateShape() error {
	if b.SchemaVersion == "" || b.ChangeName == "" {
		return fmt.Errorf("schema version and change name are required")
	}
	if b.LegacyBoard.BoardID == "" || b.LegacyBoard.Mode != "legacy" {
		return fmt.Errorf("legacy mode and board ID are required")
	}
	if b.ImmutableEventLog.Status != "unavailable" {
		return fmt.Errorf("immutable_event_log must be unavailable")
	}
	if b.ImmutableEventLog.Reason == "" || len(b.ImmutableEventLog.Limitations) == 0 {
		return fmt.Errorf("immutable_event_log reason and limitations are required")
	}
	if b.Capture.CapturedAt.IsZero() || b.Capture.Actor == "" || b.Capture.Source == "" {
		return fmt.Errorf("capture metadata is required")
	}
	if b.Retrieval.Method == "" || b.Retrieval.Authorization == "" || b.Retrieval.RedactionRule == "" || !b.Retrieval.Complete {
		return fmt.Errorf("retrieval contract is incomplete")
	}
	if len(b.RawEvidence) == 0 {
		return fmt.Errorf("raw evidence is required for independent reconstruction")
	}
	seenRaw := map[string]bool{}
	for _, raw := range b.RawEvidence {
		if raw.Path == "" || raw.Content == "" || raw.Digest == "" {
			return fmt.Errorf("raw evidence is incomplete")
		}
		if seenRaw[raw.Path] {
			return fmt.Errorf("duplicate raw evidence path %s", raw.Path)
		}
		seenRaw[raw.Path] = true
		if !sha256DigestPattern.MatchString(raw.Digest) {
			return fmt.Errorf("raw evidence digest is not a canonical SHA-256 hex value for %s", raw.Path)
		}
		bytes, err := raw.Bytes()
		if err != nil {
			return fmt.Errorf("raw evidence encoding invalid for %s: %w", raw.Path, err)
		}
		if got := digestBytes(bytes); got != raw.Digest {
			return fmt.Errorf("raw evidence digest mismatch for %s", raw.Path)
		}
	}
	if len(b.LineageReferences) == 0 {
		return fmt.Errorf("lineage references are required")
	}
	seenLineage := map[string]bool{}
	for _, ref := range b.LineageReferences {
		if ref.ID == "" || ref.Type == "" || ref.Payload == "" || ref.Digest == "" || seenLineage[ref.ID] {
			return fmt.Errorf("lineage reference is incomplete or duplicated")
		}
		seenLineage[ref.ID] = true
		if !sha256DigestPattern.MatchString(ref.Digest) {
			return fmt.Errorf("lineage reference digest is not canonical SHA-256 hex for %s", ref.ID)
		}
		var payload any
		if err := json.Unmarshal([]byte(ref.Payload), &payload); err != nil {
			return fmt.Errorf("lineage reference payload is invalid for %s", ref.ID)
		}
		canonical, err := canonicalJSONBytes(payload)
		if err != nil || string(canonical) != ref.Payload {
			return fmt.Errorf("lineage reference payload is not canonical for %s", ref.ID)
		}
		if got := digestString(ref.Payload); got != ref.Digest {
			return fmt.Errorf("lineage reference digest mismatch for %s", ref.ID)
		}
	}
	if !b.Completeness.Complete || !b.Completeness.StatusConsistent {
		return fmt.Errorf("completeness/status consistency is not sufficient")
	}
	if b.Completeness.TaskCount != len(b.Tasks) {
		return fmt.Errorf("cardinality mismatch for tasks")
	}
	if b.Completeness.ContractCount != len(b.Contracts) {
		return fmt.Errorf("cardinality mismatch for contract evidence")
	}
	if b.Completeness.CortexCount != len(b.Cortex) {
		return fmt.Errorf("cardinality mismatch for Cortex evidence")
	}
	if len(b.Tasks) == 0 || len(b.Contracts) == 0 || len(b.Cortex) == 0 {
		return fmt.Errorf("missing task, contract, or Cortex evidence")
	}
	seen := map[string]bool{}
	for _, task := range b.Tasks {
		if task.TaskID == "" {
			return fmt.Errorf("task ID is required")
		}
		if task.Notes == "" {
			return fmt.Errorf("task %s notes are required", task.TaskID)
		}
		if seen[task.TaskID] {
			return fmt.Errorf("duplicate task ID %s", task.TaskID)
		}
		seen[task.TaskID] = true
	}
	for _, c := range b.Contracts {
		if c.ID == "" || c.Digest == "" || c.ObservationID == 0 {
			return fmt.Errorf("contract evidence is incomplete")
		}
	}
	for _, c := range b.Cortex {
		if c.ObservationID == 0 || c.ArtifactDigest == "" || len(c.RevisionHistory) == 0 || len(c.GraphEdges) == 0 {
			return fmt.Errorf("cortex lineage evidence is incomplete")
		}
	}
	return nil
}

func (b LegacyHistoryEvidence) digest(v any) string {
	data, _ := canonicalJSONBytes(v)
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func canonicalJSONBytes(v any) ([]byte, error) { return json.Marshal(v) }

func digestString(value string) string {
	return digestBytes([]byte(value))
}

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ReconstructDigests hashes only the embedded payloads, with no privileged
// ForgeSpec or Cortex access.
func ReconstructDigests(b LegacyHistoryEvidence) (source, bundle string, err error) {
	if b.SourcePayload.LegacyBoard.BoardID == "" || b.BundlePayload.Source.LegacyBoard.BoardID == "" {
		return "", "", fmt.Errorf("embedded source and bundle payloads are required")
	}
	return b.digest(b.SourcePayload), b.digest(b.BundlePayload), nil
}

func (b LegacyHistoryEvidence) canonicalSourcePayload() LegacySourcePayload {
	return canonicalSource(LegacySourcePayload{LegacyBoard: b.LegacyBoard, Tasks: b.Tasks, Contracts: b.Contracts, Cortex: b.Cortex, Completeness: b.Completeness, Retrieval: b.Retrieval, RawEvidence: b.RawEvidence, LineageReferences: b.LineageReferences})
}

func (b LegacyHistoryEvidence) canonicalBundlePayload() LegacyBundlePayload {
	c := LegacyBundlePayload{SchemaVersion: b.SchemaVersion, ChangeName: b.ChangeName, Source: b.canonicalSourcePayload(), ImmutableEventLog: b.ImmutableEventLog, Capture: b.Capture, ArtifactLinks: append([]string(nil), b.ArtifactLinks...)}
	sort.Strings(c.ArtifactLinks)
	return c
}

func canonicalSource(c LegacySourcePayload) LegacySourcePayload {
	c.Tasks = append([]LegacyTaskEvidence(nil), c.Tasks...)
	c.Contracts = append([]ContractEvidence(nil), c.Contracts...)
	c.Cortex = append([]CortexLineageEvidence(nil), c.Cortex...)
	c.RawEvidence = append([]RawEvidenceFile(nil), c.RawEvidence...)
	c.LineageReferences = append([]LineageReference(nil), c.LineageReferences...)
	sort.Slice(c.Tasks, func(i, j int) bool { return c.Tasks[i].TaskID < c.Tasks[j].TaskID })
	sort.Slice(c.Contracts, func(i, j int) bool { return c.Contracts[i].ID < c.Contracts[j].ID })
	sort.Slice(c.Cortex, func(i, j int) bool { return c.Cortex[i].ObservationID < c.Cortex[j].ObservationID })
	sort.Slice(c.RawEvidence, func(i, j int) bool { return c.RawEvidence[i].Path < c.RawEvidence[j].Path })
	sort.Slice(c.LineageReferences, func(i, j int) bool { return c.LineageReferences[i].ID < c.LineageReferences[j].ID })
	for i := range c.Tasks {
		c.Tasks[i].Dependencies = append([]string(nil), c.Tasks[i].Dependencies...)
		sort.Strings(c.Tasks[i].Dependencies)
	}
	for i := range c.Cortex {
		c.Cortex[i].RevisionHistory = append([]string(nil), c.Cortex[i].RevisionHistory...)
		c.Cortex[i].GraphEdges = append([]string(nil), c.Cortex[i].GraphEdges...)
		sort.Strings(c.Cortex[i].RevisionHistory)
		sort.Strings(c.Cortex[i].GraphEdges)
	}
	return c
}

func (b LegacyHistoryEvidence) String() string { return strings.TrimSpace(b.BundleDigest) }
