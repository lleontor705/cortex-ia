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
		signature, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := h.db.ExecContext(r.Context(), insertSQL,
		report.ID, report.Timestamp, report.Source, report.TaskID, report.JobID, report.BoardID, report.Workspace,
		report.ErrorCode, report.ErrorMessage, report.Details,
		report.SystemInfo.OS, report.SystemInfo.Arch, report.SystemInfo.GoVersion, report.SystemInfo.Version, report.SystemInfo.Hostname,
		report.Signature, now,
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "database insert failed: " + err.Error()})
		return
	}

	log.Printf("📥 [Report Received] ID=%s Code=%s Source=%s Task=%s", report.ID, report.ErrorCode, report.Source, report.TaskID)
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
		       signature, created_at
		FROM reports WHERE id=?
	`, id)

	var rep telemetry.ErrorReport
	var host, createdAt string
	err := row.Scan(
		&rep.ID, &rep.Timestamp, &rep.Source, &rep.TaskID, &rep.JobID, &rep.BoardID, &rep.Workspace,
		&rep.ErrorCode, &rep.ErrorMessage, &rep.Details,
		&rep.SystemInfo.OS, &rep.SystemInfo.Arch, &rep.SystemInfo.GoVersion, &rep.SystemInfo.Version, &host,
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

func (h *HubServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, timestamp, source, task_id, job_id, error_code, error_message, details, os, version, created_at
		FROM reports
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	type Item struct {
		ID        string
		Timestamp string
		Source    string
		TaskID    string
		JobID     string
		Code      string
		Message   string
		Details   string
		OS        string
		Version   string
		CreatedAt string
	}
	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Timestamp, &it.Source, &it.TaskID, &it.JobID, &it.Code, &it.Message, &it.Details, &it.OS, &it.Version, &it.CreatedAt); err == nil {
			items = append(items, it)
		}
	}

	tmplStr := `<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cortex-IA Error & Telemetry Hub</title>
  <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-slate-950 text-slate-100 min-h-screen">
  <div class="max-w-7xl mx-auto px-4 py-8">
    <header class="flex items-center justify-between pb-6 border-b border-slate-800">
      <div class="flex items-center space-x-3">
        <div class="w-10 h-10 rounded-lg bg-indigo-600 flex items-center justify-center font-bold text-xl">🧠</div>
        <div>
          <h1 class="text-2xl font-bold tracking-tight">Cortex-IA Error & Telemetry Hub</h1>
          <p class="text-xs text-slate-400">Panel Centralizado de Reportes y Auditoría de Fallas</p>
        </div>
      </div>
      <div class="flex items-center space-x-4">
        <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-indigo-950 text-indigo-300 border border-indigo-800">
          🔒 Auth Active ({{ .User }})
        </span>
        <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-emerald-950 text-emerald-400 border border-emerald-800">
          ● Railway Online
        </span>
        <button onclick="location.reload()" class="px-3 py-1.5 text-xs bg-slate-800 hover:bg-slate-700 rounded border border-slate-700 transition">
          🔄 Actualizar
        </button>
      </div>
    </header>

    <main class="mt-8">
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <div class="bg-slate-900 border border-slate-800 rounded-xl p-5">
          <div class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Total de Reportes</div>
          <div class="mt-2 text-3xl font-bold text-slate-100">{{ len .Items }}</div>
        </div>
        <div class="bg-slate-900 border border-slate-800 rounded-xl p-5">
          <div class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Integridad Criptográfica</div>
          <div class="mt-2 text-3xl font-bold text-indigo-400">HMAC-SHA256</div>
        </div>
        <div class="bg-slate-900 border border-slate-800 rounded-xl p-5">
          <div class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Origen Principal</div>
          <div class="mt-2 text-3xl font-bold text-amber-400">Orquestador / Workers</div>
        </div>
      </div>

      <div class="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-2xl">
        <div class="px-6 py-4 border-b border-slate-800 flex justify-between items-center">
          <h2 class="font-semibold text-sm text-slate-200">Incidentes Recientes</h2>
          <span class="text-xs text-slate-500">Mostrando últimos {{ len .Items }} registros</span>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs">
            <thead class="bg-slate-950/50 text-slate-400 uppercase text-[10px] tracking-wider border-b border-slate-800">
              <tr>
                <th class="px-6 py-3">Timestamp</th>
                <th class="px-6 py-3">Código</th>
                <th class="px-6 py-3">Origen</th>
                <th class="px-6 py-3">Tarea / Job</th>
                <th class="px-6 py-3">Mensaje</th>
                <th class="px-6 py-3 text-right">Detalle</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono">
              {{ if eq (len .Items) 0 }}
              <tr>
                <td colspan="6" class="px-6 py-12 text-center text-slate-500 font-sans">
                  ✨ No hay errores reportados en el sistema. Todo marcha en orden.
                </td>
              </tr>
              {{ end }}
              {{ range .Items }}
              <tr class="hover:bg-slate-800/40 transition">
                <td class="px-6 py-3.5 text-slate-400 whitespace-nowrap">{{ .CreatedAt }}</td>
                <td class="px-6 py-3.5">
                  <span class="px-2 py-0.5 rounded bg-rose-950 text-rose-300 border border-rose-800 font-semibold">{{ .Code }}</span>
                </td>
                <td class="px-6 py-3.5 text-indigo-300 font-medium">{{ .Source }}</td>
                <td class="px-6 py-3.5 text-slate-400">
                  {{ if .TaskID }}Task: <span class="text-slate-200">{{ .TaskID }}</span>{{ else }}-{{ end }}
                </td>
                <td class="px-6 py-3.5 text-slate-200 max-w-xs truncate font-sans">{{ .Message }}</td>
                <td class="px-6 py-3.5 text-right font-sans">
                  <button onclick="alert('ID: {{.ID}}\n\nCódigo: {{.Code}}\nMensaje: {{.Message}}\n\nDetalles:\n{{.Details}}')" class="text-indigo-400 hover:text-indigo-300 hover:underline">
                    Ver Traza
                  </button>
                </td>
              </tr>
              {{ end }}
            </tbody>
          </table>
        </div>
      </div>
    </main>
  </div>
</body>
</html>`

	t, err := template.New("dashboard").Parse(tmplStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.Execute(w, struct {
		User  string
		Items []Item
	}{
		User:  h.authUser,
		Items: items,
	})
}
