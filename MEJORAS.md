# Mejoras Propuestas para CORTEX-IA

> Basado en la comparativa CORTEX-IA vs GENTLE-AI y analisis de gaps.

---

## Resumen de Prioridades

| # | Mejora | Prioridad | Esfuerzo | Impacto |
|:-:|--------|:---------:|:--------:|:-------:|
| 1 | Pipeline 2-stage con rollback | P0 | 2h | ALTO |
| 2 | Per-phase model assignments | P0 | 3h | ALTO |
| 3 | Health check framework (verify) | P0 | 2h | ALTO |
| 4 | System detection extendido | P1 | 4h | ALTO |
| 5 | Topological sort + parallel groups | P1 | 3h | MEDIO |
| 6 | Permissions & security guardrails | P1 | 3h | ALTO |
| 7 | Self-update mechanism | P1 | 6h | MEDIO |
| 8 | Sync command (refresh assets) | P2 | 4h | MEDIO |
| 9 | Persona system | P2 | 4h | MEDIO |
| 10 | CLI expansion (config, list, backup) | P2 | 6h | MEDIO |
| 11 | TUI screens (progress, skills, backups) | P3 | 8h | BAJO |
| 12 | E2E tests Docker | P3 | 8h | ALTO |

---

## P0 — Criticas (Sprint 1)

### 1. Pipeline 2-Stage con Rollback

**Problema**: `Install()` es single-pass. Si falla el componente 5 de 7, no hay recovery parcial. State y lock se guardan al final — crash intermedio = estado inconsistente.

**Solucion**: Adoptar patron de gentle-ai `orchestrator.go` — 2 stages (Prepare→Apply) con rollback policy.

**Cambios**:
```
internal/pipeline/
├── pipeline.go        → refactor: extraer Step interface
├── step.go            → NUEVO: Step + RollbackStep interfaces
├── orchestrator.go    → NUEVO: 2-stage executor con FailurePolicy
└── stages.go          → NUEVO: PrepareStage (backup+validate) + ApplyStage (inject)
```

**Diseño**:
```go
// step.go
type Step interface {
    ID() string
    Run() error
}
type RollbackStep interface {
    Step
    Rollback() error
}

// orchestrator.go
type FailurePolicy int
const (
    StopOnError FailurePolicy = iota
    ContinueOnError
)

type Orchestrator struct {
    policy FailurePolicy
    onProgress func(stepID string, status string)
}

func (o *Orchestrator) Execute(prepare []Step, apply []Step) ExecutionResult
```

**Impacto**: Rollback parcial, progress tracking, dry-run nativo, errores por stage.

---

### 2. Per-Phase Model Assignments

**Problema**: El orchestrator SDD usa un solo modelo para todas las fases. Fases de arquitectura (alta complejidad) usan el mismo modelo que archivado (baja complejidad). Desperdicio de tokens/costo.

**Solucion**: Adoptar `claude_model.go` de gentle-ai — tipos + presets.

**Cambios**:
```
internal/model/
├── types.go           → agregar ModelAssignment, ClaudeModelAlias
├── model_routing.go   → NUEVO: presets (balanced/performance/economy)
└── selection.go       → agregar ModelAssignments map
```

**Diseño**:
```go
// model_routing.go
type ClaudeModelAlias string
const (
    ModelOpus   ClaudeModelAlias = "opus"
    ModelSonnet ClaudeModelAlias = "sonnet"
    ModelHaiku  ClaudeModelAlias = "haiku"
)

type ModelPreset string
const (
    PresetBalanced    ModelPreset = "balanced"
    PresetPerformance ModelPreset = "performance"
    PresetEconomy     ModelPreset = "economy"
)

func ClaudePreset(preset ModelPreset) map[string]ClaudeModelAlias {
    // balanced: orchestrator=opus, design=opus, apply=sonnet, archive=haiku
    // performance: orchestrator=opus, design=opus, verify=opus, rest=sonnet
    // economy: all=sonnet, archive=haiku
}
```

**Inyeccion**: El orchestrator prompt recibe `{{MODEL_ASSIGNMENTS}}` con tabla fase→modelo. Sub-agentes usan el modelo asignado.

**Impacto**: Reduccion de costos 40-60% con preset economy sin perder calidad en fases criticas.

---

### 3. Health Check Framework

**Problema**: `doctor` solo verifica que archivos del lockfile existen. No valida contenido, MCP servers, ni dependencias runtime.

**Solucion**: Adoptar `verify/checks.go` de gentle-ai — framework de checks composable.

**Cambios**:
```
internal/verify/
├── verify.go      → NUEVO: Check interface + runner
├── checks.go      → NUEVO: checks concretos
└── report.go      → NUEVO: render de resultados
```

**Diseño**:
```go
type Check struct {
    ID          string
    Description string
    Run         func() error
    Soft        bool // warning vs failure
}

type Report struct {
    Passed   []CheckResult
    Failed   []CheckResult
    Warnings []CheckResult
}
```

**Checks propuestos**:
- Files from lockfile exist (ya existe en doctor)
- Cortex MCP binary available (`cortex --version`)
- Node.js + npx available (para MCP servers)
- System prompt markers intact (no corrupcion manual)
- Skills directory has all 19 skills
- Convention file present in shared dir
- MCP configs parseable (JSON/TOML valido)
- State and lock consistent (mismos agents/components)

**Impacto**: Diagnostico completo post-install, deteccion de problemas antes de que el usuario los reporte.

---

## P1 — Importantes (Sprint 2)

### 4. System Detection Extendido

**Problema**: Deteccion actual solo cubre OS/arch/pkg-manager. No verifica dependencias runtime criticas (Node.js, npx, cortex binary).

**Cambios en** `internal/system/detect.go`:

```go
type SystemInfo struct {
    OS              string
    Arch            string
    Profile         PlatformProfile
    // NUEVOS:
    NodeVersion     string   // "v20.11.0" o ""
    NpxAvailable    bool
    CortexVersion   string   // "0.3.1" o ""
    GoVersion       string   // "go1.26.1" o ""
    Shell           string   // "bash", "zsh", "powershell"
    GitVersion      string
    HomeDir         string
    InstalledAgents []string // pre-scan de dirs existentes
}
```

**Impacto**: `cortex-ia detect` muestra diagnostico completo. Pipeline puede fallar temprano si faltan deps.

---

### 5. Topological Sort + Parallel Groups

**Problema**: `ResolveDeps` usa DFS que produce orden valido pero no expone grupos paralelos. No puede ejecutar componentes independientes en paralelo.

**Cambios**:
```
internal/catalog/
├── components.go   → mantener AllComponents()
├── order.go        → NUEVO: TopoSort con level assignment
└── graph.go        → NUEVO: dependency graph con cycle detection
```

**Diseño**:
```go
// order.go
type OrderedGroups [][]ComponentID

func TopoSort(selected []ComponentID) (OrderedGroups, error) {
    // Kahn's algorithm
    // Returns: [[cortex, forgespec, mailbox, context7], [conventions], [sdd]]
    // Groups can run in parallel within each level
}
```

**Impacto**: Preparacion para ejecucion paralela. Deteccion de ciclos. Mejor visualizacion en TUI.

---

### 6. Permissions & Security Guardrails

**Problema**: No hay proteccion contra comandos destructivos ni manejo de secrets. Agentes pueden ejecutar `rm -rf /` o leer `.env`.

**Cambios**:
```
internal/components/permissions/
└── inject.go   → NUEVO: inyeccion de deny lists per-agent
```

**Diseño per-agent**:
```go
// Claude Code: settings.json
{"permissions": {"deny": [".env", ".env.*", "*.pem", "*.key"]}}

// OpenCode: opencode.json
{"permissions": {"bash": {"deny": ["rm -rf /", "sudo rm -rf"]}}}

// Codex: instrucciones en system prompt
"NEVER read or modify .env files or files containing secrets."
```

**Impacto**: Safety by default. Previene filtracion de secrets y comandos destructivos.

---

### 7. Self-Update Mechanism

**Problema**: No hay forma de actualizar cortex-ia sin reinstalar manualmente.

**Cambios**:
```
internal/update/
├── check.go    → NUEVO: check GitHub releases API
├── apply.go    → NUEVO: download + replace binary
└── guard.go    → NUEVO: env vars para skip, loop detection
```

**Diseño**:
```go
func CheckUpdate(currentVersion string) (*Release, error) {
    // GET https://api.github.com/repos/lleontor705/cortex-ia/releases/latest
    // Compare semver
    // Return nil if up to date
}

func ApplyUpdate(release *Release, profile PlatformProfile) error {
    // Download binary for OS/arch
    // Verify checksum
    // Replace current binary (Unix: syscall.Exec, Windows: message)
}
```

**CLI**: `cortex-ia update` (check) + `cortex-ia upgrade` (apply).

**Impacto**: Mantenimiento sin friccion. Users siempre en ultima version.

---

## P2 — Deseables (Sprint 3)

### 8. Sync Command

**Problema**: Para actualizar skills/prompts hay que reinstalar todo. No hay refresh parcial.

**Nuevo comando**: `cortex-ia sync [--component sdd] [--dry-run]`

**Comportamiento**:
1. Lee state.json para saber que esta instalado
2. Re-inyecta solo los componentes seleccionados (o todos)
3. NO crea backup completo (solo snapshot de archivos afectados)
4. Reporta diff: files changed/unchanged

**Impacto**: Actualizar skills despues de `cortex-ia upgrade` sin tocar MCP configs.

---

### 9. Persona System

**Problema**: Todos los agentes reciben el mismo tono. No hay diferenciacion UX.

**Diseño**:
```
internal/components/persona/
└── inject.go

internal/assets/
├── generic/persona-professional.md    → directo, conciso
├── generic/persona-mentor.md          → pedagogico, explica decisiones
└── generic/persona-minimal.md         → sin personalidad adicional
```

**Inyeccion**: Marker `<!-- cortex-ia:persona -->` en system prompt.

**Impacto**: UX personalizable. Usuarios avanzados prefieren minimal; nuevos prefieren mentor.

---

### 10. CLI Expansion

**Nuevos comandos**:

| Comando | Descripcion |
|---------|-------------|
| `cortex-ia config` | Muestra configuracion actual (agents, components, version) |
| `cortex-ia list agents` | Lista agentes detectados con estado |
| `cortex-ia list components` | Lista componentes instalados |
| `cortex-ia list backups` | Lista backups disponibles |
| `cortex-ia backup create` | Crea backup manual |
| `cortex-ia backup delete <id>` | Elimina backup |
| `cortex-ia uninstall [--agent X]` | Remueve inyecciones de un agente |

---

## P3 — Nice-to-Have (Sprint 4+)

### 11. TUI Screens Adicionales

| Screen | Descripcion |
|--------|-------------|
| Component picker | Seleccion individual de componentes (no solo presets) |
| Skill picker | Seleccion de skills especificos |
| Progress bar | Barra de progreso real durante install |
| Backup browser | Listar, restaurar, eliminar backups |
| Dependency tree | Visualizacion de arbol de deps resuelto |
| Detection screen | Muestra sistema detectado antes de instalar |
| Model config | Selector de modelo por fase SDD |

---

### 12. E2E Tests Docker

```
e2e/
├── Dockerfile.ubuntu    → Ubuntu 22.04 + Go + Node.js
├── Dockerfile.arch      → Arch Linux + Go + Node.js
├── Dockerfile.fedora    → Fedora + Go + Node.js
├── e2e_test.sh          → Test suite
└── lib.sh               → Helpers
```

**Tests propuestos**:
- Full install → verify → repair cycle
- Install → delete file → doctor detects → repair fixes
- Install → rollback → verify clean
- Dry-run produces no changes
- Idempotent: install twice = same result
- Multi-agent: install for claude + codex simultaneously

---

## Roadmap Visual

```
Sprint 1 (P0)                    Sprint 2 (P1)
┌─────────────────────┐          ┌─────────────────────┐
│ Pipeline 2-stage     │          │ System detection     │
│ Model assignments    │          │ Topo sort + groups   │
│ Health checks        │          │ Permissions          │
│                      │          │ Self-update          │
└─────────────────────┘          └─────────────────────┘
         ↓                                ↓
Sprint 3 (P2)                    Sprint 4+ (P3)
┌─────────────────────┐          ┌─────────────────────┐
│ Sync command         │          │ TUI screens          │
│ Persona system       │          │ E2E Docker tests     │
│ CLI expansion        │          │ Community skills     │
└─────────────────────┘          └─────────────────────┘
```

---

## Ventaja Competitiva Post-Mejoras

Despues de implementar P0+P1, cortex-ia tendria:

| Dimension | Estado actual | Post-mejoras |
|-----------|:------------:|:------------:|
| MCP ecosystem | 5 servers, ~52 tools | 5 servers, ~52 tools (ya es lider) |
| Pipeline robustez | Single-pass | 2-stage + rollback + health checks |
| Cost optimization | N/A | Per-phase model routing |
| Platform support | Basico | Completo (Node.js, npx, cortex, shell) |
| Security | Ninguno | Deny lists + guardrails |
| Dependency order | DFS | Topo sort + parallel groups |
| Maintainability | Manual | Self-update + sync |

**Resultado**: cortex-ia mantendria su ventaja en ecosistema MCP (ForgeSpec, Mailbox, CLI Orchestrator — que gentle-ai no tiene) mientras cierra los gaps de infraestructura, platform support, y UX.
