package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignAndVerifyReport(t *testing.T) {
	secret := "super-secret-key-123"
	report := CreateReport("orchestrator", "ERR_TEST", "Test error message", "Detailed stack info", "task-99", "job-88", "board-1", "/test/dir", "3.0.0", secret)

	// 1. Signature must verify
	if !VerifyReport(report, secret) {
		t.Errorf("expected signature to be valid")
	}

	// 2. Tampering payload must invalidate signature
	tampered := *report
	tampered.ErrorMessage = "Tampered error message"
	if VerifyReport(&tampered, secret) {
		t.Errorf("expected tampered report to fail verification")
	}

	// 3. Wrong secret must fail verification
	if VerifyReport(report, "wrong-secret") {
		t.Errorf("expected wrong secret to fail verification")
	}
}

func TestConfigLoadAndSave(t *testing.T) {
	tempHome := t.TempDir()
	cfg := Config{
		Endpoint: "https://my-railway-app.up.railway.app/api/v1/reports",
		Secret:   "my-railway-secret",
		Enabled:  true,
	}

	if err := SaveConfig(tempHome, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig(tempHome)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.Endpoint != cfg.Endpoint || loaded.Secret != cfg.Secret || !loaded.Enabled {
		t.Errorf("loaded config mismatch: %+v", loaded)
	}
}

func TestSendReport(t *testing.T) {
	var receivedReport ErrorReport
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedReport); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ReportResponse{
			Status:   "accepted",
			ReportID: receivedReport.ID,
			Message:  "persisted",
		})
	}))
	defer ts.Close()

	secret := "test-secret"
	report := CreateReport("worker", "ERR_WORKER_DIED", "worker process crashed", "exit 1", "t-1", "j-1", "b-1", "/ws", "3.0.0", secret)

	res, err := SendReport(context.Background(), ts.URL, report)
	if err != nil {
		t.Fatalf("SendReport failed: %v", err)
	}
	if res.ReportID != report.ID {
		t.Errorf("expected report ID %s, got %s", report.ID, res.ReportID)
	}
	if receivedReport.ErrorCode != "ERR_WORKER_DIED" {
		t.Errorf("expected received code ERR_WORKER_DIED, got %s", receivedReport.ErrorCode)
	}
}
