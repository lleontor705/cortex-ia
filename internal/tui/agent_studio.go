package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lleontor705/cortex-ia/internal/tui/styles"
)

const (
	StudioStepSelect     = 0
	StudioStepPreview    = 1
	StudioStepResult     = 2
	StudioStepOverwrite  = 3
	studioExistingTarget = "existing_target"
)

type StudioArchetype struct {
	ID          string
	Name        string
	Category    string
	Description string
	ToolsYaml   string
	PromptBody  string
}

var studioArchetypes = []StudioArchetype{
	{
		ID:          "db-migrator",
		Name:        "Database Migration Specialist",
		Category:    "Worker (Mutating with Leases)",
		Description: "Owns schema migrations, SQL scripts, rollback verification, and SQLite/Postgres DDL.",
		ToolsYaml: `tools:
  task: false
  write: true
  read: true
  grep: true
  glob: true
  list: true
  edit: true
  bash: true
  skill: true
  cortex_*: true
  cortex_ia_*: true
permission:
  bash:
    "*": allow`,
		PromptBody: `# role/db-migrator [STATIC_PREFIX_V2]

You are an expert database migration and schema engineering subagent.
- You must ALWAYS reserve an exclusive file lease via cortex_ia_file_reserve before creating or modifying migration scripts.
- Verify migrations are idempotent, forward-compatible, and include safe rollbacks.
- Query code symbols and AST relations with cortex_* tools before executing schema migrations.
- Persist discovered gotchas, connection issues, or schema invariants in Cortex via cortex_save.`,
	},
	{
		ID:          "sec-auditor",
		Name:        "Security & Vulnerability Auditor",
		Category:    "Auditor (Read-Only)",
		Description: "Read-only auditor for authentication, token leaks, secret exposures, and injection risks.",
		ToolsYaml: `tools:
  task: false
  write: false
  read: true
  grep: true
  glob: true
  list: true
  edit: false
  bash: true
  skill: true
  cortex_*: true
  cortex_ia_*: true
permission:
  bash:
    "*": allow`,
		PromptBody: `# role/sec-auditor [STATIC_PREFIX_V2]

You are a security and vulnerability auditor subagent.
- Strictly read-only: NEVER edit or modify repository files directly.
- Audit authentication flows, cryptographic hygiene, boundary checks, and input sanitization.
- Ensure no credentials, tokens (claim_token, lease_token), or environment secrets are exposed in logs or diffs.
- Save confirmed vulnerabilities as structured gotchas in Cortex via cortex_save.`,
	},
	{
		ID:          "api-validator",
		Name:        "API & Contract Validator",
		Category:    "Auditor (Read-Only)",
		Description: "Audits REST/gRPC endpoints, JSON-Schema boundaries, and OpenSpec delta contracts.",
		ToolsYaml: `tools:
  task: false
  write: false
  read: true
  grep: true
  glob: true
  list: true
  edit: false
  bash: true
  skill: true
  cortex_*: true
  cortex_ia_*: true
permission:
  bash:
    "*": allow`,
		PromptBody: `# role/api-validator [STATIC_PREFIX_V2]

You are an API contract and endpoint validation subagent.
- Compare delivered endpoint implementations against OpenSpec specifications.
- Verify HTTP response status codes, payload structures, and error envelopes.
- Check backward compatibility and breaking changes across endpoints.`,
	},
	{
		ID:          "perf-profiler",
		Name:        "Performance & Benchmark Profiler",
		Category:    "Investigator (Diagnostics)",
		Description: "Diagnoses memory allocations, hot loops, CPU profiles, and benchmark regressions.",
		ToolsYaml: `tools:
  task: false
  write: false
  read: true
  grep: true
  glob: true
  list: true
  edit: false
  bash: true
  skill: true
  cortex_*: true
  cortex_ia_*: true
permission:
  bash:
    "*": allow`,
		PromptBody: `# role/perf-profiler [STATIC_PREFIX_V2]

You are a performance profiling and benchmark analysis subagent.
- Run targeted benchmarks (e.g. go test -bench, memory profiles).
- Analyze allocation hot paths and algorithmic complexity.
- Inspect AST communities with cortex_analyze_architecture to detect performance bottlenecks.`,
	},
}

func generateAgentMarkdown(arch StudioArchetype) string {
	return fmt.Sprintf(`---
description: "%s"
mode: subagent
temperature: 0.1
steps: 40
color: "#8B5CF6"
%s
---

%s
`, arch.Description, arch.ToolsYaml, arch.PromptBody)
}

func writeAgentFile(targetDir, targetPath, content string, allowOverwrite bool) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creando directorio: %w", err)
	}

	if !allowOverwrite {
		// New target creation: atomic open with O_EXCL prevents clobbering existing files.
		f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		if _, err := f.Write([]byte(content)); err != nil {
			_ = os.Remove(targetPath)
			return err
		}
		if err := f.Sync(); err != nil {
			_ = os.Remove(targetPath)
			return err
		}
		return nil
	}

	// Confirmed replacement: atomic replacement using temporary file in targetDir.
	tmpFile, err := os.CreateTemp(targetDir, "agent-studio-*.tmp")
	if err != nil {
		return fmt.Errorf("creando archivo temporal: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("escribiendo archivo temporal: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sincronizando archivo temporal: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("cerrando archivo temporal: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("reemplazando archivo destino: %w", err)
	}
	return nil
}

func (m model) updateAgentStudio(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc", "b", "B":
		if m.studioStep == StudioStepOverwrite {
			m.studioStep = StudioStepPreview
			return m, nil
		}
		if m.studioStep == StudioStepSelect || m.studioStep == StudioStepResult {
			m.screen = screenHome
			m.cursor = 4
			return m, homeTick()
		}
		m.studioStep--
		m.studioResultMsg = ""
		return m, nil
	case "n", "N":
		if m.studioStep == StudioStepOverwrite {
			m.studioStep = StudioStepPreview
			return m, nil
		}
	case "y", "Y":
		if m.studioStep == StudioStepOverwrite {
			arch := studioArchetypes[m.studioArchIdx]
			targetDir := filepath.Join(m.homeDir, ".config", "opencode", "agents")
			targetPath := filepath.Join(targetDir, arch.ID+".md")
			content := generateAgentMarkdown(arch)

			if err := writeAgentFile(targetDir, targetPath, content, true); err != nil {
				m.studioResultMsg = fmt.Sprintf("Error guardando agente: %v", err)
			} else {
				m.studioResultMsg = fmt.Sprintf("Agente creado exitosamente en: %s", targetPath)
			}
			m.studioStep = StudioStepResult
			return m, nil
		}
	case "up", "k":
		if m.studioStep == StudioStepSelect && m.studioArchIdx > 0 {
			m.studioArchIdx--
		}
	case "down", "j":
		if m.studioStep == StudioStepSelect && m.studioArchIdx < len(studioArchetypes)-1 {
			m.studioArchIdx++
		}
	case "enter", " ":
		switch m.studioStep {
		case StudioStepSelect:
			arch := studioArchetypes[m.studioArchIdx]
			targetPath := filepath.Join(m.homeDir, ".config", "opencode", "agents", arch.ID+".md")
			if _, err := os.Stat(targetPath); err == nil {
				m.studioResultMsg = studioExistingTarget
			} else {
				m.studioResultMsg = ""
			}
			m.studioStep = StudioStepPreview // Advance to Preview & Confirmation
		case StudioStepPreview:
			arch := studioArchetypes[m.studioArchIdx]
			targetDir := filepath.Join(m.homeDir, ".config", "opencode", "agents")
			targetPath := filepath.Join(targetDir, arch.ID+".md")
			content := generateAgentMarkdown(arch)

			// Overwrite-protection: existing target demands explicit bound confirmation.
			if m.studioResultMsg == studioExistingTarget {
				m.studioStep = StudioStepOverwrite
				return m, nil
			}

			// Atomic new target creation with race prevention (O_EXCL).
			if err := writeAgentFile(targetDir, targetPath, content, false); err != nil {
				m.studioResultMsg = fmt.Sprintf("Error guardando agente: %v", err)
			} else {
				m.studioResultMsg = fmt.Sprintf("Agente creado exitosamente en: %s", targetPath)
			}
			m.studioStep = StudioStepResult
		case StudioStepResult:
			m.screen = screenHome
			m.cursor = 4
			return m, homeTick()
		case StudioStepOverwrite:
			// Explicit 'y' required to proceed with overwrite, or 'n'/'esc' to cancel.
			return m, nil
		}
	}
	return m, nil
}

func (m model) viewAgentStudio() string {
	width := m.contentWidth()
	var top []string

	stepTitles := []string{
		"Agent Studio — Paso 1 de 2: Seleccionar Arquetipo",
		"Agent Studio — Paso 2 de 2: Previsualizar y Confirmar",
		"Agent Studio — Resultado",
		"Agent Studio — Confirmar Sobrescritura",
	}

	currentTitle := stepTitles[0]
	if m.studioStep < len(stepTitles) {
		currentTitle = stepTitles[m.studioStep]
	}

	top = append(top, truncate(m.header(currentTitle), width), "")

	var content []string

	switch m.studioStep {
	case StudioStepSelect:
		top = append(top,
			styleSubtitle.Render("Crea subagentes personalizados con guardas de seguridad y contratos Cortex-IA"),
			"",
			styleDim.Render("Selecciona el arquetipo base que deseas instalar en tu entorno OpenCode:"),
			"",
		)
		for i, arch := range studioArchetypes {
			cursorMark := "  "
			title := fmt.Sprintf("[%s] %s (%s)", " ", arch.Name, arch.ID)
			if m.studioArchIdx == i {
				cursorMark = "> "
				title = styleSelected.Render(fmt.Sprintf("[%s] %s (%s)", "x", arch.Name, arch.ID))
			}
			content = append(content, truncate(cursorMark+title, width))
			content = append(content, truncate(fmt.Sprintf("    Tipo: %s", stylePass.Render(arch.Category)), width))
			content = append(content, truncate(fmt.Sprintf("    %s", styleDim.Render(arch.Description)), width))
			content = append(content, "")
		}
	case StudioStepPreview:
		arch := studioArchetypes[m.studioArchIdx]
		targetPath := filepath.Join(m.homeDir, ".config", "opencode", "agents", arch.ID+".md")
		top = append(top,
			styleSubtitle.Render("Previsualización del Subagente"),
			"",
			fmt.Sprintf("  • Archivo Destino: %s", styleSelected.Render(targetPath)),
			fmt.Sprintf("  • Arquetipo:       %s", arch.Name),
			fmt.Sprintf("  • Categoría:       %s", stylePass.Render(arch.Category)),
		)
		if m.studioResultMsg == studioExistingTarget {
			top = append(top, styleWarn.Render("  • Estado:          ⚠ El archivo de destino ya existe (requiere confirmación)"))
		}
		top = append(top,
			"",
			styleDim.Render("Contenido del archivo markdown que se escribirá en disco:"),
			"",
		)
		content = append(content, "---")
		content = append(content, fmt.Sprintf("description: \"%s\"", arch.Description))
		content = append(content, "mode: subagent")
		content = append(content, arch.ToolsYaml)
		content = append(content, "---")
		content = append(content, "")
		for _, line := range strings.Split(arch.PromptBody, "\n") {
			content = append(content, truncate(line, width))
		}
	case StudioStepResult:
		top = append(top,
			styleSubtitle.Render("Operación Finalizada"),
			"",
		)
		if strings.HasPrefix(m.studioResultMsg, "Error") {
			content = append(content, styleFail.Render("❌ "+m.studioResultMsg))
		} else {
			arch := studioArchetypes[m.studioArchIdx]
			content = append(content, stylePass.Render("✅ "+m.studioResultMsg))
			content = append(content, "")
			content = append(content, styleSubtitle.Render("¿Cómo invocar este subagente?"))
			content = append(content, "  Desde el chat de OpenCode o desde un prompt orquestador, usa:")
			content = append(content, styleSelected.Render(fmt.Sprintf("  task({ subagent: \"%s\", prompt: \"...\" })", arch.ID)))
			content = append(content, "")
			content = append(content, styleDim.Render("El subagente respetará automáticamente los leases de archivos y la autoridad SQLite."))
		}
	case StudioStepOverwrite:
		arch := studioArchetypes[m.studioArchIdx]
		targetPath := filepath.Join(m.homeDir, ".config", "opencode", "agents", arch.ID+".md")
		top = append(top,
			styleSubtitle.Render("Confirmación de Sobrescritura"),
			"",
			fmt.Sprintf("  • Archivo Destino: %s", styleSelected.Render(targetPath)),
			fmt.Sprintf("  • Arquetipo:       %s", arch.Name),
			fmt.Sprintf("  • Categoría:       %s", stylePass.Render(arch.Category)),
			"",
		)
		confirmBox := []string{
			styleWarn.Render("⚠ Confirm overwrite"),
			truncate(fmt.Sprintf("El archivo destino %q ya existe en disco.", targetPath), width-4),
			"Sobrescribir reemplazará permanentemente el subagente existente en esta ruta.",
			"",
			styleDim.Render("[y] yes, proceed   [n]/esc no, cancel"),
		}
		content = append(content, styleFrame.BorderForeground(styles.Warning).Render(strings.Join(confirmBox, "\n")))
	}

	var bottom []string
	switch m.studioStep {
	case StudioStepSelect:
		bottom = append(bottom, m.footer("enter continuar · ↑/↓ seleccionar · esc / b volver"))
	case StudioStepPreview:
		if m.studioResultMsg == studioExistingTarget {
			bottom = append(bottom, m.footer("enter solicitar confirmación de sobrescritura · esc / b volver"))
		} else {
			bottom = append(bottom, m.footer("enter crear agente en disco · esc / b volver"))
		}
	case StudioStepOverwrite:
		bottom = append(bottom, m.footer("y confirmar sobrescritura · n / esc cancelar"))
	default:
		bottom = append(bottom, m.footer("enter / esc volver al menú inicio"))
	}

	return strings.Join(clampScreen(top, content, bottom, m.bodyHeight(), 0, "up/down"), "\n")
}
