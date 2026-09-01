package cortexiaweb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
)

func TestCortexIAWebAPI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := delegation.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	_, _ = store.CreateBoard(ctx, "b-web", "Web Board", "Description")

	handler, err := NewHandler(store)
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	// 1. GET /api/overview
	reqOverview := httptest.NewRequest("GET", "/api/overview", nil)
	wOverview := httptest.NewRecorder()
	handler.ServeHTTP(wOverview, reqOverview)
	if wOverview.Code != http.StatusOK {
		t.Errorf("GET /api/overview expected 200, got %d", wOverview.Code)
	}

	// 2. GET /api/boards
	reqBoards := httptest.NewRequest("GET", "/api/boards", nil)
	wBoards := httptest.NewRecorder()
	handler.ServeHTTP(wBoards, reqBoards)
	if wBoards.Code != http.StatusOK {
		t.Errorf("GET /api/boards expected 200, got %d", wBoards.Code)
	}

	// 3. GET /api/config
	reqConfig := httptest.NewRequest("GET", "/api/config", nil)
	wConfig := httptest.NewRecorder()
	handler.ServeHTTP(wConfig, reqConfig)
	if wConfig.Code != http.StatusOK {
		t.Errorf("GET /api/config expected 200, got %d", wConfig.Code)
	}

	// 4. GET / (static index.html)
	reqStatic := httptest.NewRequest("GET", "/", nil)
	wStatic := httptest.NewRecorder()
	handler.ServeHTTP(wStatic, reqStatic)
	if wStatic.Code != http.StatusOK {
		t.Errorf("GET / expected 200, got %d", wStatic.Code)
	}
	if !strings.Contains(wStatic.Body.String(), "Cortex-IA") {
		t.Errorf("static response missing Cortex-IA title:\n%s", wStatic.Body.String())
	}
}

func TestServeLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "delegation.db")

	store, err := delegation.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)

	go func() {
		// Use port 0 for dynamic ephemeral port allocation in test
		_ = Serve(ctx, store, "127.0.0.1:0", ready)
	}()

	select {
	case addr := <-ready:
		if !strings.HasPrefix(addr, "http://127.0.0.1:") {
			t.Errorf("expected 127.0.0.1 address, got %s", addr)
		}
		cancel()
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("Serve timed out waiting for ready signal")
	}
}
