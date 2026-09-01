package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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
	opts, _, err := workOptions(args, map[string]bool{"--endpoint": false, "--secret": false, "--enable": false, "--disable": false})
	if err != nil {
		return fmt.Errorf("usage: cortex-ia report config --endpoint <url> [--secret <key>] [--enable|--disable]")
	}
	cfg, _ := telemetry.LoadConfig(home)
	if ep := oneOption(opts, "--endpoint"); ep != "" {
		cfg.Endpoint = ep
		cfg.Enabled = true
	}
	if sec := oneOption(opts, "--secret"); sec != "" {
		cfg.Secret = sec
	}
	if _, ok := opts["--enable"]; ok {
		cfg.Enabled = true
	}
	if _, ok := opts["--disable"]; ok {
		cfg.Enabled = false
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

	cfg, _ := telemetry.LoadConfig(home)
	report := telemetry.CreateReport(source, code, msg, details, taskID, jobID, boardID, ws, Version, cfg.Secret)

	// Print JSON report locally
	if err := printJSON(report); err != nil {
		return err
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
