package telemetry

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// SystemDiagnostics captures environment and runtime health metadata for debugging.
type SystemDiagnostics struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	GoVersion     string `json:"go_version"`
	Version       string `json:"version"`
	Hostname      string `json:"hostname,omitempty"`
	NumCPU        int    `json:"num_cpu,omitempty"`
	MemoryAllocMB uint64 `json:"mem_alloc_mb,omitempty"`
	MemorySysMB   uint64 `json:"mem_sys_mb,omitempty"`
	Goroutines    int    `json:"goroutines,omitempty"`
}

// ErrorReport represents a cryptographically signed operational error report.
type ErrorReport struct {
	SchemaVersion int               `json:"schema_version,omitempty"`
	ID            string            `json:"report_id"`
	Timestamp     string            `json:"timestamp"`
	Source        string            `json:"source"`
	TaskID        string            `json:"task_id,omitempty"`
	JobID         string            `json:"job_id,omitempty"`
	BoardID       string            `json:"board_id,omitempty"`
	Workspace     string            `json:"workspace,omitempty"`
	SessionID     string            `json:"session_id,omitempty"`
	SubagentRole  string            `json:"subagent_role,omitempty"`
	TargetPath    string            `json:"target_path,omitempty"`
	ModelID       string            `json:"model_id,omitempty"`
	ErrorCode     string            `json:"error_code"`
	ErrorMessage  string            `json:"error_message"`
	Details       string            `json:"details,omitempty"`
	SystemInfo    SystemDiagnostics `json:"system_info"`
	Signature     string            `json:"signature"`
}

var (
	// CanonicalDefaultEndpoint is the default production error telemetry ingestion endpoint.
	// This can be overridden at compile time via Go linker flags (-ldflags -X).
	CanonicalDefaultEndpoint = "https://cortex-report-hub-production.up.railway.app/api/v1/reports"

	// CanonicalDefaultSecret is the HMAC signing secret injected into release binaries
	// at compile time via Go linker flags (-ldflags -X). It is intentionally not hardcoded
	// in the public source repository for maximum security.
	CanonicalDefaultSecret = ""
)

// NormalizeEndpoint ensures the endpoint has a scheme and points to the reports ingestion path.
func NormalizeEndpoint(raw string) string {
	ep := strings.TrimSpace(raw)
	if ep == "" {
		return CanonicalDefaultEndpoint
	}
	if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
		ep = "https://" + ep
	}
	ep = strings.TrimRight(ep, "/")
	if !strings.Contains(ep, "/api/") {
		ep = ep + "/api/v1/reports"
	}
	return ep
}

// Config stores client telemetry settings.
type Config struct {
	Endpoint string `json:"endpoint"`
	Secret   string `json:"secret,omitempty"`
	Enabled  bool   `json:"enabled"`
}

func ConfigPath(homeDir string) string {
	return filepath.Join(homeDir, ".cortex-ia", "telemetry.json")
}

func LoadConfig(homeDir string) (Config, error) {
	secret := strings.TrimSpace(os.Getenv("CORTEX_REPORT_SECRET"))
	if secret == "" {
		secret = CanonicalDefaultSecret
	}
	if endpoint := strings.TrimSpace(os.Getenv("CORTEX_REPORT_ENDPOINT")); endpoint != "" {
		return Config{
			Endpoint: NormalizeEndpoint(endpoint),
			Secret:   secret,
			Enabled:  true,
		}, nil
	}
	p := ConfigPath(homeDir)
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{
				Endpoint: CanonicalDefaultEndpoint,
				Secret:   secret,
				Enabled:  true,
			}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = CanonicalDefaultEndpoint
	} else {
		cfg.Endpoint = NormalizeEndpoint(cfg.Endpoint)
	}
	if cfg.Secret == "" {
		cfg.Secret = secret
	}
	return cfg, nil
}

func SaveConfig(homeDir string, cfg Config) error {
	cfg.Endpoint = NormalizeEndpoint(cfg.Endpoint)
	// For security, if the configured secret matches the built-in binary secret,
	// do not persist it to disk in plaintext.
	if cfg.Secret != "" && cfg.Secret == CanonicalDefaultSecret {
		cfg.Secret = ""
	}
	p := ConfigPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// AutoReport persists before returning; delivery may finish in a later process.
func AutoReport(homeDir, source, code, message, details, taskID, jobID, boardID, workspace, version string) {
	cfg, err := LoadConfig(homeDir)
	if err != nil || !cfg.Enabled || cfg.Endpoint == "" || cfg.Secret == "" {
		return
	}
	report := CreateReport(source, code, message, details, taskID, jobID, boardID, workspace, version, cfg.Secret)
	if err := EnqueueReport(homeDir, report, cfg.Secret); err != nil {
		fmt.Fprintln(os.Stderr, "telemetry queue failed:", err)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_ = FlushReports(ctx, homeDir, cfg)
	}()
}

func NewReportID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func CanonicalSignaturePayload(r *ErrorReport) string {
	if r.SchemaVersion == 2 {
		copy := *r
		copy.Signature = ""
		data, _ := json.Marshal(copy)
		return string(data)
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s",
		r.ID, r.Timestamp, r.Source, r.TaskID, r.JobID, r.ErrorCode, r.ErrorMessage, r.Details, r.Workspace)
}

func SignReport(report *ErrorReport, secret string) {
	if secret == "" {
		secret = CanonicalDefaultSecret
	}
	if secret == "" {
		report.Signature = ""
		return
	}
	payload := CanonicalSignaturePayload(report)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	report.Signature = hex.EncodeToString(h.Sum(nil))
}

func VerifyReport(report *ErrorReport, secret string) bool {
	if report == nil || (report.SchemaVersion != 0 && report.SchemaVersion != 2) {
		return false
	}
	if secret == "" {
		secret = CanonicalDefaultSecret
	}
	if secret == "" || report.Signature == "" {
		return false
	}
	expectedHmac := hmac.New(sha256.New, []byte(secret))
	expectedHmac.Write([]byte(CanonicalSignaturePayload(report)))
	expectedSig := hex.EncodeToString(expectedHmac.Sum(nil))
	return hmac.Equal([]byte(report.Signature), []byte(expectedSig))
}

func CreateReport(source, errorCode, errorMessage, details, taskID, jobID, boardID, workspace, version, secret string) *ErrorReport {
	host, _ := os.Hostname()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	r := &ErrorReport{
		SchemaVersion: 2,
		ID:            NewReportID(),
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Source:        source,
		TaskID:        taskID,
		JobID:         jobID,
		BoardID:       boardID,
		Workspace:     workspace,
		ErrorCode:     errorCode,
		ErrorMessage:  errorMessage,
		Details:       details,
		SystemInfo: SystemDiagnostics{
			OS:            runtime.GOOS,
			Arch:          runtime.GOARCH,
			GoVersion:     runtime.Version(),
			Version:       version,
			Hostname:      host,
			NumCPU:        runtime.NumCPU(),
			MemoryAllocMB: m.Alloc / 1024 / 1024,
			MemorySysMB:   m.Sys / 1024 / 1024,
			Goroutines:    runtime.NumGoroutine(),
		},
	}
	SanitizeReport(r, secret)
	SignReport(r, secret)
	return r
}

type ReportResponse struct {
	Status   string `json:"status"`
	ReportID string `json:"report_id"`
	Message  string `json:"message,omitempty"`
}

func SendReport(ctx context.Context, endpoint string, report *ErrorReport) (*ReportResponse, error) {
	if err := ValidateReport(report); err != nil {
		return nil, err
	}
	clean := *report
	SanitizeReport(&clean, "")
	if clean != *report {
		return nil, errors.New("report must be sanitized and signed before transmission")
	}
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("report endpoint is empty")
	}
	body, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("marshal error report: %w", err)
	}
	if len(body) > MaxReportBytes {
		return nil, errors.New("report exceeds payload limit")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create report request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "cortex-ia/"+report.SystemInfo.Version)

	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("report delivery failed; retained for retry")
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024+1))
	if err != nil || len(respBody) > 16*1024 {
		return nil, errors.New("invalid report acknowledgement")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("report endpoint returned status %d", resp.StatusCode)
	}

	var res ReportResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, errors.New("report endpoint returned an invalid acknowledgement")
	}
	if res.ReportID != report.ID || res.Status != "accepted" {
		return nil, errors.New("report acknowledgement does not match queued report")
	}
	return &res, nil
}

const MaxReportBytes = 128 * 1024

var reportIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var reportCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,95}$`)
var reportCredentials = regexp.MustCompile(`(?i)(["']?(?:[a-z0-9_]*(?:token|password|secret|api_key|apikey)|authorization)["']?\s*[:=]\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;]+)`)
var reportBearer = regexp.MustCompile(`(?i)\bBearer\s+[^\s,;"']+`)
var reportURLCredentials = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^/\s:@]+:[^/\s@]+@`)
var reportCLISecret = regexp.MustCompile(`(?i)(--(?:claim-token|lease-token|password|secret|api-key|token)\s+)[^\s]+`)
var reportAPIKey = regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{16,}|AIza[A-Za-z0-9_-]{20,})\b`)

func boundedReportText(value string, limit int, secret string) string {
	if secret != "" {
		value = strings.ReplaceAll(value, secret, "[REDACTED]")
	}
	value = reportBearer.ReplaceAllString(value, "Bearer [REDACTED]")
	value = reportCredentials.ReplaceAllString(value, "${1}[REDACTED]")
	value = reportURLCredentials.ReplaceAllString(value, "${1}[REDACTED]@")
	value = reportCLISecret.ReplaceAllString(value, "${1}[REDACTED]")
	value = reportAPIKey.ReplaceAllString(value, "[REDACTED]")
	value = strings.TrimSpace(strings.ToValidUTF8(value, ""))
	if value == "undefined" || value == "null" {
		return ""
	}
	if len(value) > limit {
		value = strings.ToValidUTF8(value[:limit], "")
	}
	return value
}

// SanitizeReport touches only the incident payload, never code/AST structures.
func SanitizeReport(r *ErrorReport, secret string) {
	for _, field := range []*string{&r.Source, &r.TaskID, &r.JobID, &r.BoardID, &r.SessionID, &r.SubagentRole, &r.ModelID,
		&r.SystemInfo.OS, &r.SystemInfo.Arch, &r.SystemInfo.GoVersion, &r.SystemInfo.Version, &r.SystemInfo.Hostname} {
		*field = boundedReportText(*field, 256, secret)
	}
	r.Workspace = boundedReportText(r.Workspace, 2048, secret)
	r.TargetPath = boundedReportText(r.TargetPath, 2048, secret)
	r.ErrorCode = boundedReportText(r.ErrorCode, 96, secret)
	r.ErrorMessage = boundedReportText(r.ErrorMessage, 4096, secret)
	r.Details = boundedReportText(r.Details, 64*1024, secret)
}

func ValidateReport(r *ErrorReport) error {
	if r == nil || !reportIDPattern.MatchString(r.ID) || !reportCodePattern.MatchString(r.ErrorCode) ||
		strings.TrimSpace(r.Source) == "" || strings.TrimSpace(r.ErrorMessage) == "" || r.ErrorMessage == "undefined" || r.ErrorMessage == "null" {
		return errors.New("report requires valid identity, source, error code and message")
	}
	if r.SchemaVersion != 0 && r.SchemaVersion != 2 {
		return errors.New("unsupported report schema")
	}
	if _, err := time.Parse(time.RFC3339Nano, r.Timestamp); err != nil {
		return errors.New("report timestamp is invalid")
	}
	data, err := json.Marshal(r)
	if err != nil || len(data) > MaxReportBytes {
		return errors.New("report exceeds payload limit")
	}
	return nil
}
