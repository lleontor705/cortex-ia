package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

func (m model) updateAgentStudio(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "esc", "b", "B":
		if m.studioStep == 0 || m.studioStep == 2 {
			m.screen = screenHome
			m.cursor = 3
			return m, nil
		}
		m.studioStep--
		return m, nil
	case "up", "k":
		if m.studioStep == 0 && m.studioArchIdx > 0 {
			m.studioArchIdx--
		}
	case "down", "j":
		if m.studioStep == 0 && m.studioArchIdx < len(studioArchetypes)-1 {
			m.studioArchIdx++
		}
	case "enter", " ":
		switch m.studioStep {
		case 0:
			m.studioStep = 1 // Advance to Preview & Confirmation
		case 1:
			// Write the agent file to disk
			arch := studioArchetypes[m.studioArchIdx]
			targetDir := filepath.Join(m.homeDir, ".config", "opencode", "agents")
			if err := os.MkdirAll(targetDir, 0o755); err != nil {
				m.studioResultMsg = fmt.Sprintf("Error creando directorio: %v", err)
				m.studioStep = 2
				return m, nil
			}
			targetPath := filepath.Join(targetDir, arch.ID+".md")
			content := generateAgentMarkdown(arch)
			if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
				m.studioResultMsg = fmt.Sprintf("Error guardando agente: %v", err)
			} else {
				m.studioResultMsg = fmt.Sprintf("Agente creado exitosamente en: %s", targetPath)
			}
			m.studioStep = 2 // Advance to Result
		case 2:
			m.screen = screenHome
			m.cursor = 3
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
	}

	currentTitle := stepTitles[0]
	if m.studioStep < len(stepTitles) {
		currentTitle = stepTitles[m.studioStep]
	}

	top = append(top, truncate(m.header(currentTitle), width), "")

	var content []string

	switch m.studioStep {
	case 0:
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
	case 1:
		arch := studioArchetypes[m.studioArchIdx]
		targetPath := filepath.Join(m.homeDir, ".config", "opencode", "agents", arch.ID+".md")
		top = append(top,
			styleSubtitle.Render("Previsualización del Subagente"),
			"",
			fmt.Sprintf("  • Archivo Destino: %s", styleSelected.Render(targetPath)),
			fmt.Sprintf("  • Arquetipo:       %s", arch.Name),
			fmt.Sprintf("  • Categoría:       %s", stylePass.Render(arch.Category)),
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
	case 2:
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
	}

	var bottom []string
	switch m.studioStep {
	case 0:
		bottom = append(bottom, m.footer("enter continuar · ↑/↓ seleccionar · esc / b volver"))
	case 1:
		bottom = append(bottom, m.footer("enter crear agente en disco · esc / b volver"))
	default:
		bottom = append(bottom, m.footer("enter / esc volver al menú inicio"))
	}

	return strings.Join(clampScreen(top, content, bottom, m.bodyHeight(), 0, "up/down"), "\n")
}
