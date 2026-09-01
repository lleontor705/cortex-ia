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
	"runtime"
	"strings"
	"time"
)

// SystemDiagnostics captures environment metadata for debugging.
type SystemDiagnostics struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	GoVersion string `json:"go_version"`
	Version   string `json:"version"`
	Hostname  string `json:"hostname,omitempty"`
}

// ErrorReport represents a cryptographically signed operational error report.
type ErrorReport struct {
	ID           string            `json:"report_id"`
	Timestamp    string            `json:"timestamp"`
	Source       string            `json:"source"`
	TaskID       string            `json:"task_id,omitempty"`
	JobID        string            `json:"job_id,omitempty"`
	BoardID      string            `json:"board_id,omitempty"`
	Workspace    string            `json:"workspace,omitempty"`
	ErrorCode    string            `json:"error_code"`
	ErrorMessage string            `json:"error_message"`
	Details      string            `json:"details,omitempty"`
	SystemInfo   SystemDiagnostics `json:"system_info"`
	Signature    string            `json:"signature"`
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
	if endpoint := strings.TrimSpace(os.Getenv("CORTEX_REPORT_ENDPOINT")); endpoint != "" {
		return Config{
			Endpoint: endpoint,
			Secret:   strings.TrimSpace(os.Getenv("CORTEX_REPORT_SECRET")),
			Enabled:  true,
		}, nil
	}
	p := ConfigPath(homeDir)
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{Enabled: false}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func SaveConfig(homeDir string, cfg Config) error {
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

func NewReportID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func CanonicalSignaturePayload(r *ErrorReport) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s",
		r.ID, r.Timestamp, r.Source, r.TaskID, r.JobID, r.ErrorCode, r.ErrorMessage, r.Details, r.Workspace)
}

func SignReport(report *ErrorReport, secret string) {
	if secret == "" {
		secret = "cortex-ia-default-unsigned-secret"
	}
	payload := CanonicalSignaturePayload(report)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	report.Signature = hex.EncodeToString(h.Sum(nil))
}

func VerifyReport(report *ErrorReport, secret string) bool {
	if secret == "" {
		secret = "cortex-ia-default-unsigned-secret"
	}
	expectedHmac := hmac.New(sha256.New, []byte(secret))
	expectedHmac.Write([]byte(CanonicalSignaturePayload(report)))
	expectedSig := hex.EncodeToString(expectedHmac.Sum(nil))
	return hmac.Equal([]byte(report.Signature), []byte(expectedSig))
}

func CreateReport(source, errorCode, errorMessage, details, taskID, jobID, boardID, workspace, version, secret string) *ErrorReport {
	host, _ := os.Hostname()
	r := &ErrorReport{
		ID:           NewReportID(),
		Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
		Source:       source,
		TaskID:       taskID,
		JobID:        jobID,
		BoardID:      boardID,
		Workspace:    workspace,
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
		Details:      details,
		SystemInfo: SystemDiagnostics{
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
			GoVersion: runtime.Version(),
			Version:   version,
			Hostname:  host,
		},
	}
	SignReport(r, secret)
	return r
}

type ReportResponse struct {
	Status   string `json:"status"`
	ReportID string `json:"report_id"`
	Message  string `json:"message,omitempty"`
}

func SendReport(ctx context.Context, endpoint string, report *ErrorReport) (*ReportResponse, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, errors.New("report endpoint is empty")
	}
	body, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("marshal error report: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create report request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "cortex-ia/"+report.SystemInfo.Version)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send report to %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("endpoint returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var res ReportResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return &ReportResponse{Status: "accepted", ReportID: report.ID}, nil
	}
	return &res, nil
}
