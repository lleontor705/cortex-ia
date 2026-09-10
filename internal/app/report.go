package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/lleontor705/cortex-ia/internal/delegation"
	"github.com/lleontor705/cortex-ia/internal/telemetry"
)

func runReport(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println("Usage: cortex-ia report <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  error --code <code> --message <msg> [options]    Generate and send a signed error report")
		fmt.Println("  config --endpoint <url> [--secret <key>]         Configure error reporting endpoint")
		fmt.Println("  status                                           Show current reporting configuration")
		return nil
	}

	home, err := cortexStateHome()
	if err != nil {
		return err
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "config":
		return runReportConfig(home, args[1:])
	case "status":
		return runReportStatus(home)
	case "error", "send":
		return runReportError(home, args[1:])
	default:
		return fmt.Errorf("unknown report subcommand %q (see 'cortex-ia report --help')", args[0])
	}
}

func runReportConfig(home string, args []string) error {
	enableFlag := false
	disableFlag := false
	filteredArgs := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--enable":
			enableFlag = true
		case "--disable":
			disableFlag = true
		default:
			filteredArgs = append(filteredArgs, arg)
		}
	}

	opts, _, err := workOptions(filteredArgs, map[string]bool{"--endpoint": false, "--secret": false})
	if err != nil {
		return fmt.Errorf("usage: cortex-ia report config [--endpoint <url>] [--secret <key>] [--enable|--disable]")
	}

	cfg, _ := telemetry.LoadConfig(home)
	if ep := oneOption(opts, "--endpoint"); ep != "" {
		cfg.Endpoint = telemetry.NormalizeEndpoint(ep)
		cfg.Enabled = true
	}
	if sec := oneOption(opts, "--secret"); sec != "" {
		cfg.Secret = sec
	}
	if enableFlag {
		cfg.Enabled = true
	}
	if disableFlag {
		cfg.Enabled = false
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = telemetry.CanonicalDefaultEndpoint
	}
	if err := telemetry.SaveConfig(home, cfg); err != nil {
		return fmt.Errorf("save telemetry config: %w", err)
	}
	fmt.Printf("✅ Configuración de reporte actualizada: Endpoint=%s, Enabled=%v\n", cfg.Endpoint, cfg.Enabled)
	return nil
}

func runReportStatus(home string) error {
	cfg, err := telemetry.LoadConfig(home)
	if err != nil {
		return err
	}
	fmt.Printf("📊 Estado de Reporte y Telemetría de Errores:\n")
	fmt.Printf("  • Endpoint: %s\n", cfg.Endpoint)
	fmt.Printf("  • Habilitado: %v\n", cfg.Enabled)
	hasSecret := cfg.Secret != ""
	fmt.Printf("  • Firma Secreta: %v\n", hasSecret)
	return nil
}

func runReportError(home string, args []string) error {
	opts, _, err := workOptions(args, map[string]bool{
		"--code": false, "--message": false, "--details": false,
		"--task": false, "--job": false, "--board": false,
		"--source": false, "--workspace": false,
	})
	if err != nil {
		return fmt.Errorf("usage: cortex-ia report error --code <code> --message <msg> [--details <text|@stdin>] [--task <id>] [--job <id>] [--source <source>]")
	}
	code := oneOption(opts, "--code")
	msg := oneOption(opts, "--message")
	if code == "" || msg == "" {
		return errors.New("report error requires --code and --message")
	}

	details := oneOption(opts, "--details")
	if details == "@stdin" {
		data, readErr := io.ReadAll(io.LimitReader(os.Stdin, 256*1024))
		if readErr == nil {
			details = string(data)
		}
	}

	source := oneOption(opts, "--source")
	if source == "" {
		source = "orchestrator"
	}
	taskID := oneOption(opts, "--task")
	jobID := oneOption(opts, "--job")
	boardID := oneOption(opts, "--board")
	ws := oneOption(opts, "--workspace")
	if ws == "" {
		ws, _ = os.Getwd()
	}

	taskID, boardID, details = enrichReportMetadata(home, jobID, taskID, boardID, details)

	cfg, _ := telemetry.LoadConfig(home)
	report := telemetry.CreateReport(source, code, msg, details, taskID, jobID, boardID, ws, Version, cfg.Secret)

	// Print JSON report locally
	if err := printJSON(report); err != nil {
		return err
	}
	if cfg.Secret == "" {
		fmt.Fprintf(os.Stderr, "⚠️ Nota: Sin secreto de autenticación configurado (el reporte se envió sin firma HMAC oficial).\n")
	}

	// If endpoint is configured, dispatch HTTP
	if cfg.Enabled && cfg.Endpoint != "" {
		res, err := telemetry.SendReport(context.Background(), cfg.Endpoint, report)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ Error al enviar reporte al backend (%s): %v\n", cfg.Endpoint, err)
		} else {
			fmt.Fprintf(os.Stderr, "🚀 Reporte enviado exitosamente a Railway backend [ID: %s]\n", res.ReportID)
		}
	}
	return nil
}

func enrichReportMetadata(home, jobID, taskID, boardID, details string) (string, string, string) {
	if jobID == "" && taskID == "" {
		return taskID, boardID, details
	}
	dbPath := delegation.DefaultDBPath(home)
	if _, err := os.Stat(dbPath); err != nil {
		return taskID, boardID, details
	}
	store, err := delegation.OpenStore(dbPath)
	if err != nil {
		return taskID, boardID, details
	}
	defer func() { _ = store.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var diag strings.Builder
	diag.WriteString("=== OPERATIONAL TRACE & DIAGNOSTICS ===\n")

	if jobID != "" {
		if job, err := store.Get(ctx, jobID); err == nil {
			if taskID == "" && job.TaskID != "" {
				taskID = job.TaskID
			}
			diag.WriteString(fmt.Sprintf("• Job: %s | Role: %s | Status: %s | Transport: %s | Pane: %s | PID: %d | Attempt: #%d\n",
				job.ID, job.Role, job.Status, job.Transport, job.PaneID, job.PID, job.Attempt))
			if job.StartedAt != "" {
				diag.WriteString(fmt.Sprintf("  Timing: Started=%s, Updated=%s\n", job.StartedAt, job.UpdatedAt))
			}
		}
	}

	if taskID != "" {
		if work, err := store.GetWork(ctx, taskID); err == nil {
			if boardID == "" && work.BoardID != "" {
				boardID = work.BoardID
			}
			diag.WriteString(fmt.Sprintf("• Task: %s | Board: %s | Status: %s | Title: %q\n",
				work.ID, work.BoardID, work.Status, work.Title))
		}
	}

	// Fetch recent events for this job or task
	if events, err := store.ListActivity(ctx, 40); err == nil {
		var matched []string
		for _, e := range events {
			if (jobID != "" && e.EntityID == jobID) || (taskID != "" && e.EntityID == taskID) {
				tStr := e.CreatedAt
				if t, parseErr := time.Parse(time.RFC3339Nano, e.CreatedAt); parseErr == nil {
					tStr = t.Format("15:04:05")
				}
				matched = append(matched, fmt.Sprintf("  • %s [%s] %s -> %s %s", tStr, e.Kind, e.From, e.To, e.Detail))
				if len(matched) >= 8 {
					break
				}
			}
		}
		if len(matched) > 0 {
			diag.WriteString("• Lifecycle Breadcrumbs:\n")
			for i := len(matched) - 1; i >= 0; i-- {
				diag.WriteString(matched[i] + "\n")
			}
		}
	}

	diag.WriteString("=======================================\n\n")
	if strings.TrimSpace(details) != "" {
		diag.WriteString(details)
	} else {
		diag.WriteString("No additional details provided.")
	}

	return taskID, boardID, diag.String()
}
