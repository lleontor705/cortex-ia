package cortexiaweb

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
	"github.com/lleontor705/cortex-ia/internal/herdr"
)

//go:embed static
var staticFiles embed.FS

type API struct{ store *delegation.Store }

func NewHandler(store *delegation.Store) (http.Handler, error) {
	if store == nil {
		return nil, errors.New("cortex-ia web store is required")
	}
	assets, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, fmt.Errorf("load embedded cortex-ia web: %w", err)
	}
	api := &API{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/overview", api.overview)
	mux.HandleFunc("GET /api/boards", api.listBoards)
	mux.HandleFunc("POST /api/boards", api.createBoard)
	mux.HandleFunc("GET /api/boards/{id}", api.boardSnapshot)
	mux.HandleFunc("POST /api/boards/{id}/archive", api.archiveBoard)
	mux.HandleFunc("POST /api/boards/{id}/unarchive", api.unarchiveBoard)
	mux.HandleFunc("DELETE /api/boards/{id}", api.deleteBoard)
	mux.HandleFunc("POST /api/tasks", api.createTask)
	mux.HandleFunc("GET /api/config", api.getConfig)
	mux.HandleFunc("GET /api/delegations/{id}", api.getDelegation)
	mux.Handle("GET /", http.FileServer(http.FS(assets)))
	return securityHeaders(mux), nil
}

func Serve(ctx context.Context, store *delegation.Store, address string, ready chan<- string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid cortex-ia web address: %w", err)
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return errors.New("cortex-ia web address must use localhost or a loopback IP")
		}
	}
	handler, err := NewHandler(store)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen for cortex-ia web: %w", err)
	}
	defer func() { _ = listener.Close() }()
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	actual := listener.Addr().String()
	if strings.HasPrefix(actual, "[::1]") {
		actual = "localhost" + strings.TrimPrefix(actual, "[::1]")
	}
	select {
	case ready <- "http://" + actual:
	case <-ctx.Done():
		return nil
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (a *API) overview(w http.ResponseWriter, r *http.Request) {
	dashboard, err := a.store.Dashboard(r.Context())
	writeResult(w, dashboard, err)
}

func (a *API) listBoards(w http.ResponseWriter, r *http.Request) {
	boards, err := a.store.ListBoards(r.Context())
	writeResult(w, boards, err)
}

func (a *API) boardSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot, err := a.store.BoardSnapshot(r.Context(), r.PathValue("id"))
	writeResult(w, snapshot, err)
}

func (a *API) createBoard(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, errors.New("cross-origin writes are not allowed"))
		return
	}
	var input struct {
		ID          string `json:"board_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	board, err := a.store.CreateBoard(r.Context(), input.ID, input.Title, input.Description)
	writeCreated(w, board, err)
}

func (a *API) archiveBoard(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, errors.New("cross-origin writes are not allowed"))
		return
	}
	id := r.PathValue("id")
	board, err := a.store.ArchiveBoard(r.Context(), id)
	writeResult(w, board, err)
}

func (a *API) unarchiveBoard(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, errors.New("cross-origin writes are not allowed"))
		return
	}
	id := r.PathValue("id")
	board, err := a.store.UnarchiveBoard(r.Context(), id)
	writeResult(w, board, err)
}

func (a *API) deleteBoard(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, errors.New("cross-origin writes are not allowed"))
		return
	}
	id := r.PathValue("id")
	err := a.store.DeleteBoard(r.Context(), id)
	if err != nil {
		writeResult(w, nil, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "board_id": id})
}

func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, errors.New("cross-origin writes are not allowed"))
		return
	}
	var input struct {
		BoardID      string   `json:"board_id"`
		ID           string   `json:"task_id"`
		Title        string   `json:"title"`
		Objective    string   `json:"objective"`
		Acceptance   string   `json:"acceptance_criteria"`
		Verification string   `json:"verification"`
		AllowedFiles []string `json:"allowed_files"`
		Dependencies []string `json:"dependencies"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := a.store.CreateWorkInBoardWithDefinition(r.Context(), input.BoardID, input.ID, input.Title, input.Dependencies, delegation.WorkDefinition{
		Objective: input.Objective, Acceptance: input.Acceptance, Verification: input.Verification, AllowedFiles: input.AllowedFiles,
	})
	writeCreated(w, item, err)
}

func (a *API) getConfig(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	cfg, err := delegation.Load(filepath.Join(home, ".config", "opencode"))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"delegation_config":            map[string]any{"delegation_enabled": false, "roles": map[string]any{}},
			"herdr_installed":              false,
			"herdr_active":                 false,
			"background_subagents_enabled": false,
		})
		return
	}
	herdrExe, _ := herdr.ResolveHerdr()
	herdrActive := herdr.HerdrRunning(herdrExe)
	bgEnv := os.Getenv("OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS") == "true"

	writeJSON(w, http.StatusOK, map[string]any{
		"delegation_config":            cfg,
		"herdr_installed":              herdrExe != "",
		"herdr_active":                 herdrActive,
		"background_subagents_enabled": bgEnv,
	})
}

func (a *API) getDelegation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := a.store.Get(r.Context(), id)
	if err != nil {
		writeResult(w, nil, err)
		return
	}
	writeResult(w, job, nil)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return errors.New("Content-Type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, r.Host) && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func writeCreated(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeResult(w, nil, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func writeResult(w http.ResponseWriter, value any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, value)
		return
	}
	status := http.StatusBadRequest
	if errors.Is(err, delegation.ErrBoardNotFound) || errors.Is(err, delegation.ErrWorkNotFound) {
		status = http.StatusNotFound
	}
	writeError(w, status, err)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
