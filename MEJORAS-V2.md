# Mejoras Propuestas v2 — Post Paridad con GENTLE-AI

> Con 14 de 15 gaps cerrados, estas mejoras diferencian cortex-ia hacia adelante.

---

## Resumen de Prioridades

| # | Mejora | Prioridad | Esfuerzo | Impacto |
|:-:|--------|:---------:|:--------:|:-------:|
| 1 | Ejecucion paralela de componentes | P0 | 3h | MUY ALTO |
| 2 | Configuracion por proyecto (.cortex-ia.yaml) | P0 | 4h | MUY ALTO |
| 3 | Skills dinamicos (community/custom) | P0 | 4h | ALTO |
| 4 | Cost tracking + budget alerts en SDD | P1 | 3h | ALTO |
| 5 | Orchestrator adaptativo | P1 | 3h | ALTO |
| 6 | Backup rotation + cleanup | P1 | 2h | MEDIO |
| 7 | Logging estructurado (slog) | P1 | 3h | MEDIO |
| 8 | TUI progress bar + component picker | P2 | 4h | MEDIO |
| 9 | PowerShell installer (Windows) | P2 | 2h | MEDIO |
| 10 | Single-agent resilience (auto-recovery) | P2 | 2h | MEDIO |
| 11 | Uninstall command | P3 | 3h | BAJO |
| 12 | Custom agent plugins (.yaml) | P3 | 4h | BAJO |

---

## P0 — Alto Impacto, Diferenciadores Reales

### 1. Ejecucion Paralela de Componentes

**Problema**: `RunOrchestrator` ejecuta apply steps secuencialmente. `TopoSort` ya calcula `ParallelGroups` pero el pipeline no las usa. Instalacion de 7 componentes toma ~20s cuando podria tomar ~5s.

**Solucion**: Nuevo `RunParallelGroups` que ejecuta componentes del mismo nivel en goroutines.

**Cambios**:
```
pipeline/runner.go  → agregar RunParallelGroups(groups [][]Step) StageResult
pipeline/pipeline.go → usar TopoSort + ParallelGroups en vez de flat list
```

**Diseño**:
```go
func RunParallelGroups(groups [][]Step) StageResult {
    for _, group := range groups {
        var wg sync.WaitGroup
        errs := make(chan error, len(group))
        for _, step := range group {
            wg.Add(1)
            go func(s Step) {
                defer wg.Done()
                if err := s.Run(); err != nil {
                    errs <- fmt.Errorf("%s: %w", s.Name(), err)
                }
            }(step)
        }
        wg.Wait()
        close(errs)
        // Collect errors from this level before proceeding to next
    }
}
```

**Resultado**: Nivel 0 (cortex, forgespec, mailbox, context7, cli-orch) → paralelo. Nivel 1 (conventions) → secuencial. Nivel 2 (sdd) → secuencial. **~4x speedup**.

---

### 2. Configuracion por Proyecto (.cortex-ia.yaml)

**Problema**: Solo existe configuracion global en `~/.cortex-ia/`. Equipos no pueden estandarizar SDD per-repo. Cada developer configura manualmente.

**Solucion**: Archivo `.cortex-ia.yaml` en la raiz del proyecto (git-tracked).

**Formato**:
```yaml
# .cortex-ia.yaml
preset: minimal
persona: mentor
model-preset: economy
agents:
  - claude-code
  - opencode
disabled-components:
  - mailbox      # Este repo no usa P2P messaging
custom-skills:
  - path: ./skills/domain-validator
  - path: ./skills/api-reviewer
```

**Cambios**:
```
internal/config/
├── config.go       → Load, Merge (project + global)
├── config_test.go
internal/app/app.go → cortex-ia init (crea .cortex-ia.yaml)
                    → cortex-ia install --local (aplica config del proyecto)
```

**Busqueda**: CWD → parent dirs (walk up hasta `.git`) → `~/.cortex-ia/state.json`

**Impacto**: Estandarizacion de SDD en equipos. Un `git clone` + `cortex-ia install --local` configura todo.

---

### 3. Skills Dinamicos (Community/Custom)

**Problema**: Skills son solo assets embebidos. No hay forma de agregar skills sin recompilar cortex-ia. Comunidad no puede contribuir.

**Solucion**: Sistema de 3 capas de skills.

| Capa | Ubicacion | Prioridad |
|------|-----------|:---------:|
| **Embebidos** | Binary (go:embed) | Baja (fallback) |
| **Globales** | `~/.cortex-ia/skills-community/` | Media |
| **Proyecto** | `.cortex-ia.yaml` → `custom-skills` | Alta (override) |

**Cambios**:
```
internal/components/sdd/inject.go → buscar en 3 capas
internal/components/sdd/registry.go → NUEVO: SkillRegistry con merge de capas
```

**CLI**:
```bash
cortex-ia skill add <path-or-url>    # Copia skill a ~/.cortex-ia/skills-community/
cortex-ia skill list                  # Lista skills de las 3 capas
cortex-ia skill remove <id>           # Remueve skill custom
```

**Formato de skill custom**:
```yaml
# ~/.cortex-ia/skills-community/api-reviewer/SKILL.md
---
name: api-reviewer
description: Validates API contracts against OpenAPI specs
version: "1.0.0"
requires:
  - validate    # Depende del skill validate de cortex-ia
---
# API Reviewer
<role>You review API implementations against OpenAPI specifications...</role>
...
```

---

## P1 — Calidad del SDD Workflow

### 4. Cost Tracking + Budget Alerts

**Problema**: El orchestrator delega sin limite. Un cambio complejo puede consumir 500K tokens sin aviso. No hay visibilidad de costo por fase.

**Solucion**: Campo `tokens_used` en contratos SDD + budget alerts en orchestrator.

**Cambios en orchestrator prompt**:
```markdown
## Cost Control

After EACH sub-agent returns, check the token count:
- If cumulative tokens > 50K for this change: warn user
- If any single phase > 30K tokens: log as expensive
- If budget specified in delegation: enforce and stop at limit

Include in each contract:
{
  "tokens_used": 8500,
  "phase_cost": "moderate"
}
```

**Cambios en `model/types.go`**:
```go
type CostTracking struct {
    TokensUsed    int    `json:"tokens_used"`
    PhaseCost     string `json:"phase_cost"` // low/moderate/high
    CumulativeTokens int `json:"cumulative_tokens"`
}
```

---

### 5. Orchestrator Adaptativo

**Problema**: El fast-track decision es estatico. Si `implement` encuentra complejidad inesperada, no escala automaticamente a pipeline mas profundo.

**Solucion**: Reglas de escalation/de-escalation en el orchestrator.

**Reglas propuestas**:
```markdown
## Adaptive Pipeline

ESCALATION TRIGGERS:
- implement returns confidence < 0.6 → escalate to validate+finalize
- validate finds 3+ spec violations → re-run architect+implement
- team-lead reports 30%+ task failures → halt, ask user

DE-ESCALATION:
- If all phases return confidence > 0.9 and 0 errors → skip finalize retrospective
- If change touches 1 file only → suggest fast-track for next similar change

CHECKPOINT STRATEGY:
- Every 4 tasks completed: mem_save incremental progress
- Enables recovery without re-doing completed work
```

---

### 6. Backup Rotation + Cleanup

**Problema**: Backups se acumulan indefinidamente. ~1MB × 50 installs = 50MB+ sin cleanup.

**Solucion**:
```bash
cortex-ia cleanup [--keep 5] [--older-than 30d] [--dry-run]
```

**Cambios**:
```
internal/backup/cleanup.go → NUEVO: CleanupOldBackups(homeDir, keep, maxAge)
internal/app/app.go        → cortex-ia cleanup command
```

---

### 7. Logging Estructurado (slog)

**Problema**: Solo `fmt.Printf` y `log.Printf` sin niveles ni output estructurado. Debugging en produccion es imposible.

**Solucion**: Migrar a `log/slog` (stdlib Go 1.21+).

```bash
cortex-ia install --verbose          # DEBUG level
cortex-ia install --quiet            # Solo errores
cortex-ia doctor --json              # Output JSON para CI/CD
```

**Cambios**: Crear `internal/logging/logging.go` con setup de slog. Reemplazar `fmt.Printf` en pipeline, components, app.

---

## P2 — UX y Plataforma

### 8. TUI Progress Bar + Component Picker

**Problema**: Screen de instalacion muestra texto estatico "This may take a moment". No hay indicacion de progreso ni seleccion granular de componentes.

**Solucion**:
- **Progress bar**: Usar `github.com/charmbracelet/bubbles/progress`
- **Component picker**: Nuevo screen entre Preset y Persona para deseleccionar componentes opcionales

**Flujo TUI actualizado**:
```
Welcome → Detection → Agents → Preset → [Components] → Persona → Review → Installing (progress) → Complete
```

---

### 9. PowerShell Installer (Windows)

**Problema**: Windows users deben compilar desde source o usar Go install.

**Solucion**: `scripts/install.ps1`

```powershell
# Uso: irm https://raw.githubusercontent.com/.../install.ps1 | iex
$version = (Invoke-RestMethod "https://api.github.com/repos/.../releases/latest").tag_name
$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
$url = "https://github.com/.../releases/download/$version/cortex-ia_${version}_windows_${arch}.zip"
# Download, verify checksum, extract, add to PATH
```

---

### 10. Single-Agent Resilience (Auto-Recovery)

**Problema**: El orchestrator single-agent no detecta compaction ni tiene recovery automatico. El usuario debe hacer `mem_context` manualmente.

**Solucion**: Agregar seccion de recovery al `sdd-orchestrator-single.md`:

```markdown
## Automatic Context Recovery

AFTER EVERY PHASE:
1. Check if you still have the change context (change name, current phase, last artifact)
2. If context seems lost (variables undefined, change name unknown):
   a. Call mem_context to recover session state
   b. Call mem_search(query: "sdd/{change}/state") for pipeline progress
   c. Resume from last completed phase
3. NEVER restart from scratch — always check for existing progress first
```

---

## P3 — Nice-to-Have

### 11. Uninstall Command

```bash
cortex-ia uninstall [--agent X] [--keep-backups] [--dry-run]
```

Reverso de inject: remueve markers, MCP configs, skills. Complejo pero necesario para clean uninstall.

---

### 12. Custom Agent Plugins

Permitir agregar agentes sin modificar codigo via manifest YAML:
```yaml
# ~/.cortex-ia/agents/my-agent/agent.yaml
name: my-custom-agent
binary: my-agent-cli
config-dir: ~/.my-agent
system-prompt: ~/.my-agent/AGENTS.md
prompt-strategy: markdown-sections
mcp-strategy: merge-into-settings
```

---

## Roadmap Visual

```
Sprint 1 (P0)                     Sprint 2 (P1)
┌──────────────────────────┐      ┌──────────────────────────┐
│ Ejecucion paralela       │      │ Cost tracking SDD        │
│ Configuracion por proyecto│      │ Orchestrator adaptativo  │
│ Skills dinamicos         │      │ Backup cleanup           │
│                          │      │ Logging (slog)           │
└──────────────────────────┘      └──────────────────────────┘
           ↓                                ↓
Sprint 3 (P2)                     Sprint 4+ (P3)
┌──────────────────────────┐      ┌──────────────────────────┐
│ TUI progress + components│      │ Uninstall command        │
│ PowerShell installer     │      │ Custom agent plugins     │
│ Single-agent recovery    │      │                          │
└──────────────────────────┘      └──────────────────────────┘
```

---

## Diferenciacion Post-Mejoras

Tras implementar P0+P1, cortex-ia tendria:

| Aspecto | cortex-ia | gentle-ai | Quien lidera |
|---------|-----------|-----------|:------------:|
| MCP ecosystem | 5 servers, ~52 tools | 2 servers, ~20 tools | **cortex-ia** |
| Install speed | **Paralelo** (~5s) | Secuencial (~20s) | **cortex-ia** |
| Per-repo config | **.cortex-ia.yaml** | No | **cortex-ia** |
| Community skills | **3 capas** | Repo separado | **cortex-ia** |
| Cost tracking SDD | **Si** (tokens per phase) | No | **cortex-ia** |
| Adaptive pipeline | **Si** (escalate/de-escalate) | No | **cortex-ia** |
| Backup management | **Rotation + cleanup** | Manual | **cortex-ia** |
| Structured logging | **slog** | No | **cortex-ia** |

**Resultado**: cortex-ia pasaria de "paridad con gentle-ai" a **lider claro** en performance, configurabilidad, observabilidad, y extensibilidad del SDD workflow.
