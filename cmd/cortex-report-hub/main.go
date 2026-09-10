package main

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/telemetry"
	_ "modernc.org/sqlite"
)

type HubServer struct {
	db           *sql.DB
	secret       string
	authUser     string
	authPassword string
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	secret := strings.TrimSpace(os.Getenv("CORTEX_REPORT_SECRET"))
	authUser := strings.TrimSpace(os.Getenv("DASHBOARD_USER"))
	if authUser == "" {
		authUser = "admin"
	}
	authPassword := strings.TrimSpace(os.Getenv("DASHBOARD_PASSWORD"))
	if authPassword == "" {
		authPassword = secret // Default to CORTEX_REPORT_SECRET if DASHBOARD_PASSWORD is not set
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "."
	}
	_ = os.MkdirAll(dataDir, 0o755)
	dbPath := filepath.Join(dataDir, "reports.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open SQLite database at %s: %v", dbPath, err)
	}
	defer func() { _ = db.Close() }()

	// Initialize tables
	initSQL := `
	CREATE TABLE IF NOT EXISTS reports (
		id TEXT PRIMARY KEY,
		timestamp TEXT NOT NULL,
		source TEXT NOT NULL,
		task_id TEXT NOT NULL DEFAULT '',
		job_id TEXT NOT NULL DEFAULT '',
		board_id TEXT NOT NULL DEFAULT '',
		workspace TEXT NOT NULL DEFAULT '',
		error_code TEXT NOT NULL,
		error_message TEXT NOT NULL,
		details TEXT NOT NULL DEFAULT '',
		os TEXT NOT NULL DEFAULT '',
		arch TEXT NOT NULL DEFAULT '',
		go_version TEXT NOT NULL DEFAULT '',
		version TEXT NOT NULL DEFAULT '',
		hostname TEXT NOT NULL DEFAULT '',
		num_cpu INTEGER DEFAULT 0,
		mem_alloc_mb INTEGER DEFAULT 0,
		mem_sys_mb INTEGER DEFAULT 0,
		goroutines INTEGER DEFAULT 0,
		signature TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS reports_created_idx ON reports(created_at DESC);
	CREATE INDEX IF NOT EXISTS reports_code_idx ON reports(error_code);
	CREATE INDEX IF NOT EXISTS reports_task_idx ON reports(task_id);
	`
	if _, err := db.Exec(initSQL); err != nil {
		log.Fatalf("Failed to initialize reports schema: %v", err)
	}

	// Schema migrations for backward-compatible addition of enriched telemetry columns
	for _, migration := range []string{
		"ALTER TABLE reports ADD COLUMN num_cpu INTEGER DEFAULT 0",
		"ALTER TABLE reports ADD COLUMN mem_alloc_mb INTEGER DEFAULT 0",
		"ALTER TABLE reports ADD COLUMN mem_sys_mb INTEGER DEFAULT 0",
		"ALTER TABLE reports ADD COLUMN goroutines INTEGER DEFAULT 0",
	} {
		_, _ = db.Exec(migration)
	}

	hub := &HubServer{
		db:           db,
		secret:       secret,
		authUser:     authUser,
		authPassword: authPassword,
	}

	mux := http.NewServeMux()
	// Public healthcheck for Railway
	mux.HandleFunc("GET /health", hub.handleHealth)

	// Ingestion endpoint (protected by HMAC-SHA256 signature)
	mux.HandleFunc("POST /api/v1/reports", hub.handleCreateReport)

	// Protected read endpoints & dashboard (protected by HTTP Basic Auth & Token)
	mux.HandleFunc("GET /api/v1/reports", hub.requireAuth(hub.handleListReports))
	mux.HandleFunc("GET /api/v1/reports/{id}", hub.requireAuth(hub.handleGetReport))
	mux.HandleFunc("GET /", hub.requireAuth(hub.handleDashboard))

	addr := ":" + port
	log.Printf("🚀 Cortex Report Hub started on %s (Auth Protected: %v, User: %s)", addr, authPassword != "", authUser)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func (h *HubServer) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.authPassword == "" {
			next(w, r)
			return
		}

		// 1. HTTP Basic Auth
		user, pass, ok := r.BasicAuth()
		if ok && user == h.authUser && pass == h.authPassword {
			next(w, r)
			return
		}

		// 2. Bearer Token or X-Cortex-Key Header
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") && strings.TrimPrefix(authHeader, "Bearer ") == h.authPassword {
			next(w, r)
			return
		}
		if r.Header.Get("X-Cortex-Key") == h.authPassword {
			next(w, r)
			return
		}

		// 3. Query Param (?token=...)
		if r.URL.Query().Get("token") == h.authPassword {
			next(w, r)
			return
		}

		// Challenge with Basic Auth prompt
		w.Header().Set("WWW-Authenticate", `Basic realm="Cortex-IA Protected Hub"`)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Acceso no autorizado: Se requiere usuario y contraseña para ver los reportes y el panel de control.",
		})
	}
}

func (h *HubServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "cortex-report-hub",
	})
}

func (h *HubServer) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var report telemetry.ErrorReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON body: " + err.Error()})
		return
	}

	if report.ID == "" {
		report.ID = telemetry.NewReportID()
	}
	if report.Timestamp == "" {
		report.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	// Verify cryptographic signature if secret is configured
	if h.secret != "" {
		if !telemetry.VerifyReport(&report, h.secret) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "cryptographic HMAC signature verification failed"})
			return
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	insertSQL := `
	INSERT INTO reports (
		id, timestamp, source, task_id, job_id, board_id, workspace,
		error_code, error_message, details, os, arch, go_version, version, hostname,
		num_cpu, mem_alloc_mb, mem_sys_mb, goroutines,
		signature, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := h.db.ExecContext(r.Context(), insertSQL,
		report.ID, report.Timestamp, report.Source, report.TaskID, report.JobID, report.BoardID, report.Workspace,
		report.ErrorCode, report.ErrorMessage, report.Details,
		report.SystemInfo.OS, report.SystemInfo.Arch, report.SystemInfo.GoVersion, report.SystemInfo.Version, report.SystemInfo.Hostname,
		report.SystemInfo.NumCPU, report.SystemInfo.MemoryAllocMB, report.SystemInfo.MemorySysMB, report.SystemInfo.Goroutines,
		report.Signature, now,
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "database insert failed: " + err.Error()})
		return
	}

	log.Printf("📥 [Report Received] ID=%s Code=%s Source=%s Task=%s CPU=%d RAM=%dMB Goroutines=%d",
		report.ID, report.ErrorCode, report.Source, report.TaskID,
		report.SystemInfo.NumCPU, report.SystemInfo.MemoryAllocMB, report.SystemInfo.Goroutines)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":    "accepted",
		"report_id": report.ID,
		"message":   "Error report verified and recorded successfully",
	})
}

func (h *HubServer) handleListReports(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
		limit = l
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, timestamp, source, task_id, job_id, board_id, workspace,
		       error_code, error_message, details, os, arch, go_version, version, hostname,
		       COALESCE(num_cpu, 0), COALESCE(mem_alloc_mb, 0), COALESCE(mem_sys_mb, 0), COALESCE(goroutines, 0),
		       signature, created_at
		FROM reports
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer func() { _ = rows.Close() }()

	var results []telemetry.ErrorReport
	for rows.Next() {
		var rep telemetry.ErrorReport
		var host, createdAt string
		err := rows.Scan(
			&rep.ID, &rep.Timestamp, &rep.Source, &rep.TaskID, &rep.JobID, &rep.BoardID, &rep.Workspace,
			&rep.ErrorCode, &rep.ErrorMessage, &rep.Details,
			&rep.SystemInfo.OS, &rep.SystemInfo.Arch, &rep.SystemInfo.GoVersion, &rep.SystemInfo.Version, &host,
			&rep.SystemInfo.NumCPU, &rep.SystemInfo.MemoryAllocMB, &rep.SystemInfo.MemorySysMB, &rep.SystemInfo.Goroutines,
			&rep.Signature, &createdAt,
		)
		if err != nil {
			continue
		}
		rep.SystemInfo.Hostname = host
		results = append(results, rep)
	}

	if results == nil {
		results = []telemetry.ErrorReport{}
	}
	_ = json.NewEncoder(w).Encode(results)
}

func (h *HubServer) handleGetReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	id := r.PathValue("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing report id"})
		return
	}

	row := h.db.QueryRowContext(r.Context(), `
		SELECT id, timestamp, source, task_id, job_id, board_id, workspace,
		       error_code, error_message, details, os, arch, go_version, version, hostname,
		       COALESCE(num_cpu, 0), COALESCE(mem_alloc_mb, 0), COALESCE(mem_sys_mb, 0), COALESCE(goroutines, 0),
		       signature, created_at
		FROM reports WHERE id=?
	`, id)

	var rep telemetry.ErrorReport
	var host, createdAt string
	err := row.Scan(
		&rep.ID, &rep.Timestamp, &rep.Source, &rep.TaskID, &rep.JobID, &rep.BoardID, &rep.Workspace,
		&rep.ErrorCode, &rep.ErrorMessage, &rep.Details,
		&rep.SystemInfo.OS, &rep.SystemInfo.Arch, &rep.SystemInfo.GoVersion, &rep.SystemInfo.Version, &host,
		&rep.SystemInfo.NumCPU, &rep.SystemInfo.MemoryAllocMB, &rep.SystemInfo.MemorySysMB, &rep.SystemInfo.Goroutines,
		&rep.Signature, &createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "report not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	rep.SystemInfo.Hostname = host
	_ = json.NewEncoder(w).Encode(rep)
}

type DashboardItem struct {
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp"`
	Source        string `json:"source"`
	TaskID        string `json:"task_id"`
	JobID         string `json:"job_id"`
	BoardID       string `json:"board_id"`
	Workspace     string `json:"workspace"`
	Code          string `json:"code"`
	Message       string `json:"message"`
	Details       string `json:"details"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	GoVersion     string `json:"go_version"`
	Version       string `json:"version"`
	Hostname      string `json:"hostname"`
	NumCPU        int    `json:"num_cpu"`
	MemoryAllocMB uint64 `json:"mem_alloc_mb"`
	MemorySysMB   uint64 `json:"mem_sys_mb"`
	Goroutines    int    `json:"goroutines"`
	CreatedAt     string `json:"created_at"`
}

func (h *HubServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, timestamp, source, task_id, job_id, board_id, workspace,
		       error_code, error_message, details,
		       os, arch, go_version, version, hostname,
		       COALESCE(num_cpu, 0), COALESCE(mem_alloc_mb, 0), COALESCE(mem_sys_mb, 0), COALESCE(goroutines, 0),
		       created_at
		FROM reports
		ORDER BY created_at DESC
		LIMIT 200
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	items := make([]DashboardItem, 0)
	codeCounts := make(map[string]int)
	for rows.Next() {
		var it DashboardItem
		if err := rows.Scan(
			&it.ID, &it.Timestamp, &it.Source, &it.TaskID, &it.JobID, &it.BoardID, &it.Workspace,
			&it.Code, &it.Message, &it.Details,
			&it.OS, &it.Arch, &it.GoVersion, &it.Version, &it.Hostname,
			&it.NumCPU, &it.MemoryAllocMB, &it.MemorySysMB, &it.Goroutines,
			&it.CreatedAt,
		); err == nil {
			items = append(items, it)
			codeCounts[it.Code]++
		}
	}

	rawJSON, _ := json.Marshal(items)

	tmplStr := `<!DOCTYPE html>
<html lang="es" class="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cortex-IA Telemetry & Error Hub</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <script>
    tailwind.config = {
      darkMode: 'class',
      theme: {
        extend: {
          colors: {
            brand: {
              50: '#eef2ff',
              100: '#e0e7ff',
              500: '#6366f1',
              600: '#4f46e5',
              700: '#4338ca',
            }
          }
        }
      }
    }
  </script>
</head>
<body class="bg-[#0b0f19] text-slate-100 min-h-screen font-sans selection:bg-indigo-500 selection:text-white">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
    <!-- Top Bar -->
    <header class="flex flex-col md:flex-row md:items-center justify-between pb-6 border-b border-slate-800/80 gap-4">
      <div class="flex items-center space-x-3.5">
        <div class="w-11 h-11 rounded-xl bg-gradient-to-br from-indigo-500 to-violet-700 flex items-center justify-center font-bold text-2xl shadow-lg shadow-indigo-500/20 ring-1 ring-white/10">
          🧠
        </div>
        <div>
          <div class="flex items-center space-x-2.5">
            <h1 class="text-xl font-bold tracking-tight text-white">Cortex-IA Telemetry Hub</h1>
            <span class="text-[11px] px-2 py-0.5 rounded-full bg-indigo-950/80 text-indigo-300 border border-indigo-700/60 font-medium">
              v2.0 Pro
            </span>
          </div>
          <p class="text-xs text-slate-400 mt-0.5">Control Centralizado de Falla, Auditoría Operacional y Trazas del Motor</p>
        </div>
      </div>
      <div class="flex flex-wrap items-center gap-2.5">
        <span class="inline-flex items-center px-3 py-1 rounded-lg text-xs font-medium bg-slate-900 text-slate-300 border border-slate-800">
          🔒 Auth: <span class="text-indigo-400 ml-1 font-semibold">{{ .User }}</span>
        </span>
        <span class="inline-flex items-center px-3 py-1 rounded-lg text-xs font-medium bg-emerald-950/60 text-emerald-400 border border-emerald-800/60">
          <span class="w-2 h-2 rounded-full bg-emerald-400 mr-2 animate-pulse"></span> Railway Live
        </span>
        <button onclick="location.reload()" class="px-3.5 py-1 rounded-lg text-xs font-medium bg-indigo-600 hover:bg-indigo-500 text-white transition shadow-sm flex items-center gap-1.5">
          <span>🔄</span> Actualizar
        </button>
      </div>
    </header>

    <!-- Metrics Cards -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mt-6">
      <div class="bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <div class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Total Incidentes</div>
        <div class="mt-1 text-2xl font-black text-slate-100" id="metricTotal">{{ len .Items }}</div>
        <div class="text-[11px] text-slate-500 mt-1">Últimos 200 en buffer</div>
      </div>
      <div class="bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <div class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Códigos Distintos</div>
        <div class="mt-1 text-2xl font-black text-indigo-400">{{ .UniqueCodes }}</div>
        <div class="text-[11px] text-slate-500 mt-1">Categorías de excepción</div>
      </div>
      <div class="bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <div class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Integridad de Firma</div>
        <div class="mt-1 text-2xl font-black text-emerald-400">HMAC-256</div>
        <div class="text-[11px] text-slate-500 mt-1">Criptográficamente auditado</div>
      </div>
      <div class="bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <div class="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">Trazas Enriquecidas</div>
        <div class="mt-1 text-2xl font-black text-amber-400">CPU + RAM + DAG</div>
        <div class="text-[11px] text-slate-500 mt-1">Auto-correlación SQLite activa</div>
      </div>
    </div>

    <!-- Filter & Search Toolbar -->
    <div class="mt-6 flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3 bg-slate-900/80 p-3.5 rounded-xl border border-slate-800">
      <div class="relative flex-1">
        <span class="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-slate-500 text-sm">
          🔍
        </span>
        <input
          type="text"
          id="searchInput"
          placeholder="Buscar por código, tarea, job, workspace, mensaje o traza... (Presiona '/' para buscar)"
          class="w-full pl-9 pr-4 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-500 transition"
          oninput="handleSearch(this.value)"
        />
      </div>
      <div class="flex items-center gap-2 self-end md:self-auto text-xs text-slate-400">
        <span>Mostrando <span id="filteredCount" class="font-bold text-slate-200">{{ len .Items }}</span> de <span class="font-bold text-slate-200">{{ len .Items }}</span></span>
        <button onclick="clearSearch()" class="text-xs text-indigo-400 hover:text-indigo-300 underline ml-2">Limpiar</button>
      </div>
    </div>

    <!-- Incidents Table -->
    <main class="mt-4 bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-2xl">
      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead class="bg-slate-950/70 text-slate-400 uppercase text-[10px] tracking-wider border-b border-slate-800">
            <tr>
              <th class="px-5 py-3.5">Timestamp</th>
              <th class="px-5 py-3.5">Código</th>
              <th class="px-5 py-3.5">Origen / Host</th>
              <th class="px-5 py-3.5">Tarea / Job</th>
              <th class="px-5 py-3.5">Diagnósticos</th>
              <th class="px-5 py-3.5">Mensaje de Error</th>
              <th class="px-5 py-3.5 text-right">Acción</th>
            </tr>
          </thead>
          <tbody id="reportsTableBody" class="divide-y divide-slate-800/60 font-mono">
            {{ if eq (len .Items) 0 }}
            <tr id="emptyRow">
              <td colspan="7" class="px-6 py-12 text-center text-slate-500 font-sans">
                ✨ No hay errores reportados en el sistema. Todo marcha en orden.
              </td>
            </tr>
            {{ end }}
            {{ range .Items }}
            <tr class="report-row hover:bg-slate-800/50 transition cursor-pointer"
                data-id="{{ .ID }}"
                data-search="{{ .Code }} {{ .Source }} {{ .TaskID }} {{ .JobID }} {{ .BoardID }} {{ .Workspace }} {{ .Message }} {{ .Details }} {{ .Hostname }}"
                onclick="openModal('{{ .ID }}')">
              <td class="px-5 py-3 text-slate-400 whitespace-nowrap text-[11px]">{{ .CreatedAt }}</td>
              <td class="px-5 py-3 whitespace-nowrap">
                <span class="px-2 py-0.5 rounded-md bg-rose-950/80 text-rose-300 border border-rose-800 font-semibold text-[11px]">
                  {{ .Code }}
                </span>
              </td>
              <td class="px-5 py-3 font-sans">
                <div class="text-indigo-300 font-medium text-[11px]">{{ .Source }}</div>
                <div class="text-slate-500 text-[10px] font-mono">{{ .OS }}/{{ .Arch }}</div>
              </td>
              <td class="px-5 py-3">
                {{ if .TaskID }}
                <div class="flex items-center gap-1 text-[11px] text-slate-300">
                  <span class="text-indigo-400">📋</span> {{ .TaskID }}
                </div>
                {{ end }}
                {{ if .JobID }}
                <div class="flex items-center gap-1 text-[10px] text-slate-500">
                  <span class="text-violet-400">🤖</span> {{ .JobID }}
                </div>
                {{ end }}
                {{ if and (not .TaskID) (not .JobID) }}
                <span class="text-slate-600">-</span>
                {{ end }}
              </td>
              <td class="px-5 py-3 whitespace-nowrap font-sans">
                <div class="flex items-center gap-1.5 text-[10px]">
                  {{ if gt .NumCPU 0 }}
                  <span class="px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700 font-mono">
                    {{ .NumCPU }} CPU
                  </span>
                  {{ end }}
                  {{ if gt .MemoryAllocMB 0 }}
                  <span class="px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700 font-mono">
                    {{ .MemoryAllocMB }}MB
                  </span>
                  {{ end }}
                  {{ if gt .Goroutines 0 }}
                  <span class="px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700 font-mono">
                    {{ .Goroutines }} gr
                  </span>
                  {{ end }}
                  {{ if and (eq .NumCPU 0) (eq .MemoryAllocMB 0) }}
                  <span class="text-slate-600">-</span>
                  {{ end }}
                </div>
              </td>
              <td class="px-5 py-3 text-slate-200 max-w-sm truncate font-sans text-xs">
                {{ .Message }}
              </td>
              <td class="px-5 py-3 text-right font-sans whitespace-nowrap">
                <button
                  onclick="event.stopPropagation(); openModal('{{ .ID }}')"
                  class="px-2.5 py-1 rounded bg-indigo-900/60 hover:bg-indigo-800 text-indigo-300 hover:text-white border border-indigo-700/60 transition text-xs font-medium">
                  Inspeccionar
                </button>
              </td>
            </tr>
            {{ end }}
            <tr id="noMatchesRow" style="display: none;">
              <td colspan="7" class="px-6 py-12 text-center text-slate-500 font-sans">
                🔍 No se encontraron incidentes que coincidan con la búsqueda.
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>
  </div>

  <!-- Detail / Trace Inspection Modal -->
  <div id="reportModal" class="fixed inset-0 z-50 bg-black/80 backdrop-blur-sm hidden items-center justify-center p-4">
    <div class="bg-[#0e1424] border border-slate-700/80 rounded-2xl max-w-4xl w-full max-h-[90vh] flex flex-col shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-150">
      
      <!-- Modal Header -->
      <div class="px-6 py-4 border-b border-slate-800 flex items-center justify-between bg-slate-900/60">
        <div class="flex items-center space-x-3">
          <span id="modalCodeBadge" class="px-2.5 py-1 rounded-md bg-rose-950 text-rose-300 border border-rose-800 font-mono font-bold text-xs">
            ERROR_CODE
          </span>
          <div>
            <div class="flex items-center space-x-2">
              <span class="text-xs font-mono text-slate-400">ID:</span>
              <span id="modalReportID" class="text-xs font-mono text-slate-200 select-all font-semibold"></span>
              <button onclick="copyCurrentID()" class="text-xs text-indigo-400 hover:text-indigo-300 ml-1" title="Copiar ID">
                📋
              </button>
            </div>
            <div id="modalTimestamp" class="text-[11px] text-slate-500"></div>
          </div>
        </div>
        <button onclick="closeModal()" class="w-8 h-8 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white flex items-center justify-center transition">
          ✕
        </button>
      </div>

      <!-- Modal Body (Scrollable) -->
      <div class="p-6 overflow-y-auto space-y-5 text-xs">
        <!-- Error Message Card -->
        <div class="bg-rose-950/20 border border-rose-900/40 rounded-xl p-4">
          <div class="text-[11px] font-semibold text-rose-400 uppercase tracking-wider mb-1">Mensaje de Excepción</div>
          <div id="modalErrorMessage" class="text-sm font-semibold text-rose-200 leading-snug font-sans"></div>
        </div>

        <!-- Metadata Grid -->
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 bg-slate-900/70 p-4 rounded-xl border border-slate-800">
          <div>
            <div class="text-[10px] text-slate-400 uppercase">Tarea ID</div>
            <div id="modalTaskID" class="font-mono text-slate-200 font-medium truncate mt-0.5">-</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase">Job ID</div>
            <div id="modalJobID" class="font-mono text-slate-200 font-medium truncate mt-0.5">-</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase">Board ID</div>
            <div id="modalBoardID" class="font-mono text-slate-200 font-medium truncate mt-0.5">-</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase">Origen</div>
            <div id="modalSource" class="font-medium text-indigo-300 truncate mt-0.5">-</div>
          </div>
        </div>

        <!-- Environment & System Diagnostics -->
        <div class="bg-slate-900/70 p-4 rounded-xl border border-slate-800 space-y-2.5">
          <div class="text-[11px] font-semibold text-slate-300 uppercase tracking-wider flex items-center justify-between">
            <span>⚙️ Entorno y Diagnósticos del Sistema</span>
            <span id="modalHostBadge" class="text-[10px] text-slate-500 font-mono"></span>
          </div>
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 text-slate-300 font-mono text-[11px]">
            <div class="bg-slate-950/60 p-2 rounded border border-slate-800">
              <span class="text-slate-500 text-[10px] block">OS / ARCH</span>
              <span id="modalOSArch">-</span>
            </div>
            <div class="bg-slate-950/60 p-2 rounded border border-slate-800">
              <span class="text-slate-500 text-[10px] block">VERSION MOTOR</span>
              <span id="modalVersion">-</span>
            </div>
            <div class="bg-slate-950/60 p-2 rounded border border-slate-800">
              <span class="text-slate-500 text-[10px] block">RECURSOS</span>
              <span id="modalResources">-</span>
            </div>
            <div class="bg-slate-950/60 p-2 rounded border border-slate-800">
              <span class="text-slate-500 text-[10px] block">WORKSPACE</span>
              <span id="modalWorkspace" class="truncate block" title="">-</span>
            </div>
          </div>
        </div>

        <!-- Full Operational Trace & Breadcrumbs -->
        <div class="space-y-1.5">
          <div class="flex items-center justify-between">
            <span class="text-[11px] font-semibold text-slate-300 uppercase tracking-wider">
              📜 Traza Operacional & Breadcrumbs del Motor
            </span>
            <button
              id="copyTraceBtn"
              onclick="copyCurrentTrace()"
              class="text-xs px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-indigo-300 border border-slate-700 transition flex items-center gap-1">
              <span>📋</span> Copiar Traza
            </button>
          </div>
          <pre id="modalDetails" class="bg-slate-950 p-4 rounded-xl border border-slate-800 font-mono text-xs leading-relaxed text-emerald-400 overflow-x-auto select-all max-h-72 whitespace-pre-wrap"></pre>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="px-6 py-3.5 border-t border-slate-800 bg-slate-900/60 flex items-center justify-between">
        <button
          id="copyJSONBtn"
          onclick="copyCurrentJSON()"
          class="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white border border-slate-700 transition text-xs font-medium">
          📦 Copiar JSON Completo
        </button>
        <button
          onclick="closeModal()"
          class="px-4 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white transition text-xs font-medium">
          Cerrar (Esc)
        </button>
      </div>

    </div>
  </div>

  <!-- Embedded Reports Data -->
  <script id="reports-data" type="application/json">{{ .ItemsJSON }}</script>

  <!-- Client-side logic for search, inspection modal, and clipboard -->
  <script>
    let reports = [];
    try {
      const parsed = JSON.parse(document.getElementById('reports-data').textContent || '[]');
      if (Array.isArray(parsed)) reports = parsed;
    } catch (e) {
      console.error('Failed to parse reports JSON:', e);
    }
    const reportsMap = {};
    reports.forEach(r => { if (r && r.id) reportsMap[r.id] = r; });

    let activeReport = null;

    function handleSearch(query) {
      const q = (query || '').toLowerCase().trim();
      let count = 0;
      const rows = document.querySelectorAll('.report-row');
      rows.forEach(row => {
        const text = (row.getAttribute('data-search') || '').toLowerCase();
        if (!q || text.includes(q)) {
          row.style.display = '';
          count++;
        } else {
          row.style.display = 'none';
        }
      });
      document.getElementById('filteredCount').textContent = count;
      const noMatches = document.getElementById('noMatchesRow');
      if (noMatches) {
        noMatches.style.display = (count === 0 && rows.length > 0) ? '' : 'none';
      }
    }

    function clearSearch() {
      const input = document.getElementById('searchInput');
      if (input) {
        input.value = '';
        handleSearch('');
        input.focus();
      }
    }

    function openModal(id) {
      const report = reportsMap[id];
      if (!report) return;
      activeReport = report;

      document.getElementById('modalCodeBadge').textContent = report.code || 'UNKNOWN';
      document.getElementById('modalReportID').textContent = report.id;
      document.getElementById('modalTimestamp').textContent = report.created_at + ' (UTC: ' + report.timestamp + ')';
      document.getElementById('modalErrorMessage').textContent = report.message || 'Sin mensaje especificado';

      document.getElementById('modalTaskID').textContent = report.task_id || 'No vinculada';
      document.getElementById('modalJobID').textContent = report.job_id || 'No vinculado';
      document.getElementById('modalBoardID').textContent = report.board_id || 'default';
      document.getElementById('modalSource').textContent = report.source || 'orchestrator';

      document.getElementById('modalHostBadge').textContent = report.hostname ? 'Host: ' + report.hostname : '';
      document.getElementById('modalOSArch').textContent = (report.os || 'unknown') + ' / ' + (report.arch || 'unknown');
      document.getElementById('modalVersion').textContent = (report.version || 'unknown') + ' (' + (report.go_version || 'go') + ')';
      
      const resParts = [];
      if (report.num_cpu) resParts.push(report.num_cpu + ' CPU');
      if (report.mem_alloc_mb) resParts.push(report.mem_alloc_mb + 'MB RAM');
      if (report.goroutines) resParts.push(report.goroutines + ' gr');
      document.getElementById('modalResources').textContent = resParts.length > 0 ? resParts.join(' · ') : 'No reportado';

      const wsEl = document.getElementById('modalWorkspace');
      wsEl.textContent = report.workspace || '-';
      wsEl.title = report.workspace || '';

      document.getElementById('modalDetails').textContent = report.details || 'Sin traza adicional.';

      const modal = document.getElementById('reportModal');
      modal.classList.remove('hidden');
      modal.classList.add('flex');
    }

    function closeModal() {
      const modal = document.getElementById('reportModal');
      modal.classList.add('hidden');
      modal.classList.remove('flex');
      activeReport = null;
    }

    // Close on click outside modal
    document.getElementById('reportModal').addEventListener('click', function(e) {
      if (e.target === this) {
        closeModal();
      }
    });

    // Keyboard shortcuts: Esc to close modal, / to focus search
    document.addEventListener('keydown', function(e) {
      if (e.key === 'Escape') {
        closeModal();
      } else if (e.key === '/' && document.activeElement !== document.getElementById('searchInput')) {
        const modal = document.getElementById('reportModal');
        if (modal.classList.contains('hidden')) {
          e.preventDefault();
          document.getElementById('searchInput').focus();
        }
      }
    });

    async function copyCurrentID() {
      if (!activeReport) return;
      await navigator.clipboard.writeText(activeReport.id);
      alert('Report ID copiado al portapapeles: ' + activeReport.id);
    }

    async function copyCurrentTrace() {
      if (!activeReport) return;
      const trace = '=== ERROR REPORT ' + activeReport.id + ' ===\n' +
        'Code: ' + activeReport.code + '\n' +
        'Message: ' + activeReport.message + '\n' +
        'Task: ' + activeReport.task_id + ' | Job: ' + activeReport.job_id + '\n\n' +
        (activeReport.details || '');
      await navigator.clipboard.writeText(trace);
      const btn = document.getElementById('copyTraceBtn');
      const orig = btn.innerHTML;
      btn.innerHTML = '<span>✓</span> ¡Copiado!';
      setTimeout(() => { btn.innerHTML = orig; }, 2000);
    }

    async function copyCurrentJSON() {
      if (!activeReport) return;
      await navigator.clipboard.writeText(JSON.stringify(activeReport, null, 2));
      const btn = document.getElementById('copyJSONBtn');
      const orig = btn.innerHTML;
      btn.innerHTML = '✓ ¡JSON Copiado!';
      setTimeout(() => { btn.innerHTML = orig; }, 2000);
    }
  </script>
</body>
</html>`

	t, err := template.New("dashboard").Parse(tmplStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.Execute(w, struct {
		User        string
		Items       []DashboardItem
		ItemsJSON   template.JS
		UniqueCodes int
	}{
		User:        h.authUser,
		Items:       items,
		ItemsJSON:   template.JS(rawJSON),
		UniqueCodes: len(codeCounts),
	})
}
