package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	cortexExportLimit  = 64 * 1024 * 1024
	cortexRecordLimit  = 2 * 1024 * 1024
	cortexContentLimit = 1024 * 1024
)

type CortexSnapshotRequest struct {
	ID             int64
	ExpectedSHA256 string
}

type CortexSnapshot struct {
	Transport     string `json:"transport"`
	Project       string `json:"project"`
	ObservationID int64  `json:"observation_id"`
	Content       string `json:"content"`
	SHA256        string `json:"sha256"`
	ByteLength    int    `json:"byte_length"`
}

// ReadCortexSnapshots streams the existing project export, not a per-ID query.
// Success requires the complete export AND successful producer termination.
func ReadCortexSnapshots(ctx context.Context, project string, requests []CortexSnapshotRequest) ([]CortexSnapshot, error) {
	if err := validateSnapshotRequests(project, requests); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "cortex", "export", "--project", project)
	command.WaitDelay = time.Second
	return readCortexSnapshotCommand(command, cancel, project, requests)
}

func validateSnapshotRequests(project string, requests []CortexSnapshotRequest) error {
	if strings.TrimSpace(project) == "" || len(project) > 512 || !utf8.ValidString(project) || len(requests) == 0 || len(requests) > 128 {
		return errors.New("snapshot requires a valid project and 1..128 observations")
	}
	seen := make(map[int64]bool)
	for _, request := range requests {
		if request.ID < 1 || request.ID > 9007199254740991 || seen[request.ID] {
			return errors.New("snapshot IDs must be unique positive safe integers")
		}
		seen[request.ID] = true
		if request.ExpectedSHA256 != "" {
			b, err := hex.DecodeString(request.ExpectedSHA256)
			if err != nil || len(b) != sha256.Size || strings.ToLower(request.ExpectedSHA256) != request.ExpectedSHA256 {
				return errors.New("snapshot digest must be lowercase SHA-256")
			}
		}
	}
	return nil
}

func readCortexSnapshotCommand(command *exec.Cmd, cancel context.CancelFunc, project string, requests []CortexSnapshotRequest) ([]CortexSnapshot, error) {
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// Stderr is drained but never retained or reflected into errors (it may contain private data).
	stderr := &cortexStderr{cancel: cancel}
	command.Stderr = stderr
	if err = command.Start(); err != nil {
		return nil, errors.New("local Cortex export could not start")
	}
	snapshots, readErr := decodeCortexSnapshots(stdout, project, requests)
	if readErr != nil {
		cancel()
		_ = stdout.Close()
	}
	waitErr := command.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if waitErr != nil || stderr.bytes > 64*1024 {
		return nil, errors.New("local Cortex export failed, timed out or exceeded stderr bounds; snapshot remains unverified")
	}
	return snapshots, nil
}

type cortexStderr struct {
	bytes  int
	cancel context.CancelFunc
}

func (s *cortexStderr) Write(p []byte) (int, error) {
	s.bytes += len(p)
	if s.bytes > 64*1024 {
		s.cancel()
		return 0, errors.New("local Cortex stderr limit exceeded")
	}
	return len(p), nil
}

var jsonEscapePattern = regexp.MustCompile(`\\(?:u[0-9a-fA-F]{4}|.)`)

// encoding/json replaces lone UTF-16 surrogates with U+FFFD. Reject that lossy
// normalization before hashing, while allowing literal U+FFFD and valid pairs.
func exactJSONUnicode(raw []byte) bool {
	escapes := jsonEscapePattern.FindAllIndex(raw, -1)
	for i := 0; i < len(escapes); i++ {
		start, end := escapes[i][0], escapes[i][1]
		if end-start != 6 || raw[start+1] != 'u' {
			continue
		}
		value, _ := strconv.ParseUint(string(raw[start+2:end]), 16, 16)
		if value >= 0xdc00 && value <= 0xdfff {
			return false
		}
		if value >= 0xd800 && value <= 0xdbff {
			if i+1 >= len(escapes) || escapes[i+1][0] != end || escapes[i+1][1]-end != 6 || raw[end+1] != 'u' {
				return false
			}
			low, _ := strconv.ParseUint(string(raw[end+2:end+6]), 16, 16)
			if low < 0xdc00 || low > 0xdfff {
				return false
			}
			i++
		}
	}
	return true
}

// Bound decoder read-ahead as well as each record, so a giant unselected record
// cannot force a project-sized allocation before Decode returns.
type cortexExportReader struct {
	source    io.Reader
	read, end int64
}

func (r *cortexExportReader) Read(p []byte) (int, error) {
	remaining := min(int64(cortexExportLimit), r.end) - r.read
	if remaining <= 0 {
		return 0, errors.New("local Cortex export exceeds total or record byte limit")
	}
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := r.source.Read(p)
	r.read += int64(n)
	return n, err
}

func decodeCortexSnapshots(source io.Reader, project string, requests []CortexSnapshotRequest) ([]CortexSnapshot, error) {
	if err := validateSnapshotRequests(project, requests); err != nil {
		return nil, err
	}
	input := &cortexExportReader{source: source, end: cortexRecordLimit}
	decoder := json.NewDecoder(input)
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return nil, errors.New("local Cortex export must be a JSON array")
	}
	wanted := make(map[int64]string, len(requests))
	for _, request := range requests {
		wanted[request.ID] = request.ExpectedSHA256
	}
	found := make(map[int64]CortexSnapshot, len(requests))
	for {
		input.end = decoder.InputOffset() + cortexRecordLimit
		if !decoder.More() {
			break
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, errors.New("local Cortex export has malformed, truncated or oversized records")
		}
		if len(raw) > cortexRecordLimit || !utf8.Valid(raw) || !exactJSONUnicode(raw) {
			return nil, errors.New("local Cortex export record exceeds bounds or has invalid Unicode")
		}
		var observation struct {
			ID      int64   `json:"id"`
			Project *string `json:"project"`
			Content *string `json:"content"`
		}
		if err := json.Unmarshal(raw, &observation); err != nil || observation.ID < 1 {
			return nil, errors.New("local Cortex export has an invalid observation identity")
		}
		expected, selected := wanted[observation.ID]
		if !selected {
			continue
		}
		if _, duplicate := found[observation.ID]; duplicate {
			return nil, errors.New("local Cortex export contains a duplicate requested observation")
		}
		if observation.Project == nil || *observation.Project != project || observation.Content == nil || len(*observation.Content) > cortexContentLimit {
			return nil, errors.New("local Cortex snapshot project or content does not match requested identity and bounds")
		}
		digest := sha256.Sum256([]byte(*observation.Content))
		encoded := hex.EncodeToString(digest[:])
		if expected != "" && expected != encoded {
			return nil, errors.New("local Cortex snapshot digest mismatch; contract validation failed")
		}
		found[observation.ID] = CortexSnapshot{"local_cortex_cli", project, observation.ID, *observation.Content, encoded, len(*observation.Content)}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim(']') {
		return nil, errors.New("local Cortex export array is truncated")
	}
	input.end = decoder.InputOffset() + cortexRecordLimit
	if err = decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return nil, errors.New("local Cortex export has trailing data or exceeds bounds")
	}
	results := make([]CortexSnapshot, 0, len(requests))
	for _, request := range requests {
		snapshot, exists := found[request.ID]
		if !exists {
			return nil, fmt.Errorf("local Cortex observation %s missing from export", strconv.FormatInt(request.ID, 10))
		}
		results = append(results, snapshot)
	}
	return results, nil
}
