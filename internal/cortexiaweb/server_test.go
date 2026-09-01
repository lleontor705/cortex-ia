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

	// 4. POST /api/boards/b-web/archive
	reqArchive := httptest.NewRequest("POST", "/api/boards/b-web/archive", nil)
	wArchive := httptest.NewRecorder()
	handler.ServeHTTP(wArchive, reqArchive)
	if wArchive.Code != http.StatusOK {
		t.Errorf("POST /api/boards/b-web/archive expected 200, got %d: %s", wArchive.Code, wArchive.Body.String())
	}

	// 5. POST /api/boards/b-web/unarchive
	reqUnarchive := httptest.NewRequest("POST", "/api/boards/b-web/unarchive", nil)
	wUnarchive := httptest.NewRecorder()
	handler.ServeHTTP(wUnarchive, reqUnarchive)
	if wUnarchive.Code != http.StatusOK {
		t.Errorf("POST /api/boards/b-web/unarchive expected 200, got %d: %s", wUnarchive.Code, wUnarchive.Body.String())
	}

	// 6. Archive again and DELETE /api/boards/b-web
	reqArchive2 := httptest.NewRequest("POST", "/api/boards/b-web/archive", nil)
	wArchive2 := httptest.NewRecorder()
	handler.ServeHTTP(wArchive2, reqArchive2)
	if wArchive2.Code != http.StatusOK {
		t.Errorf("POST /api/boards/b-web/archive (2) expected 200, got %d", wArchive2.Code)
	}

	reqDelete := httptest.NewRequest("DELETE", "/api/boards/b-web", nil)
	wDelete := httptest.NewRecorder()
	handler.ServeHTTP(wDelete, reqDelete)
	if wDelete.Code != http.StatusOK {
		t.Errorf("DELETE /api/boards/b-web expected 200, got %d: %s", wDelete.Code, wDelete.Body.String())
	}

	// 7. GET / (static index.html)
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
