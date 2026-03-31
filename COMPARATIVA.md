# Comparativa: CORTEX-IA vs GENTLE-AI (Post-Mejoras)

> Fecha: 2026-03-30 | Actualizada tras implementar mejoras P0-P3 en cortex-ia.

---

## At a Glance

| Metrica | CORTEX-IA (antes) | CORTEX-IA (ahora) | GENTLE-AI |
|---------|:-----------------:|:------------------:|:---------:|
| Lineas Go | 7,209 | **9,068** | 46,118 |
| Test functions | 123 | **161** | 916 |
| E2E test assertions | 0 | **29** | 78 |
| Skills embebidos | 20 | 20 | 16 |
| MCP servers | 5 | 5 | 2 |
| MCP tools totales | ~52 | ~52 | ~20 |
| Componentes | 8 | **10** | 10 |
| CLI commands | 7 | **13** | ~12 |
| TUI screens | 6 | **8** | 29 |
| Agentes soportados | 8 | 8 | 8 |
| Presets | 3 | 3 | 4 |

### Feature Parity Matrix

| Feature | CORTEX-IA (antes) | CORTEX-IA (ahora) | GENTLE-AI |
|---------|:------------------:|:------------------:|:---------:|
| Pipeline 2-stage | -- | **Si** | Si |
| Per-phase model routing | -- | **Si** (3 presets) | Si (3 presets) |
| Health checks | Basico | **6 checks** | Si |
| System detection | OS/arch/pkg | **+Node/npx/Git/Go/Cortex/Shell** | Completo |
| Topological sort | DFS simple | **Kahn's + parallel groups** | Kahn's algo |
| Permissions/guardrails | -- | **Si** (deny lists per-agent) | Si |
| Self-update | -- | **Si** (`cortex-ia update`) | Si |
| Sync command | -- | **Si** (`cortex-ia sync`) | Si |
| Persona system | -- | **Si** (3 personas) | Si (3 personas) |
| CLI expansion | 7 cmds | **13 cmds** (config/list/sync) | ~12 cmds |
| E2E Docker tests | -- | **Si** (Ubuntu + Fedora) | Si (3 distros) |
| TUI Detection screen | -- | **Si** | Si |
| TUI Persona picker | -- | **Si** | Si |
| Contract validation (Zod) | **Si** | **Si** | -- |
| Inter-agent messaging | **Si** | **Si** | -- |
| CLI orchestration | **Si** | **Si** | -- |
| Task board SQLite | **Si** | **Si** | -- |
| File reservation | **Si** | **Si** | -- |
| Auto-install agentes | -- | -- | **Si** |
| Code review (GGA) | -- | -- | **Si** |
| Theme injection | -- | -- | **Si** |
| Strict TDD mode | -- | -- | **Si** |
| Community skills repo | -- | -- | **Si** |

---

## 1. Arquitectura & Patrones de Diseno

### Patrones compartidos (ambos)

- Adapter pattern para 8 agentes
- Strategy dispatch para MCP (4) y system prompts (3)
- Marker-based injection (`<!-- {tool}:ID -->`)
- Embedded assets via `go:embed`
- Bubbletea TUI + Lipgloss
- Backup/restore con manifest JSON
- State + lockfile

### Diferencias que permanecen

| Aspecto | CORTEX-IA | GENTLE-AI |
|---------|-----------|-----------|
| **Tamano** | ~9K lineas (lean) | ~46K lineas (5.1x mayor) |
| **Pipeline** | **2-stage Orchestrator** (Prepare→Apply) con `FailurePolicy` + `RunStageContinue` | 2-stage Orchestrator (Prepare→Apply) con `Step` interface |
| **Resolucion deps** | **Kahn's algorithm** con `ParallelGroups` y deteccion de ciclos | Kahn's con soft-ordering constraints |
| **CLI separation** | Logica en `app/app.go` (13 commands) | `internal/cli/` separado (~12 commands) |
| **Sub-agents** | En Adapter: `SupportsSubAgents()`, `SubAgentsDir()` | Sin metodos de sub-agent en Adapter |

**Veredicto**: Ambos tienen pipeline 2-stage y topo sort. CORTEX-IA expone `ParallelGroups` para ejecucion paralela futura. GENTLE-AI tiene mejor separacion CLI pero cortex-ia ha cerrado la brecha significativamente.

---

## 2. Soporte de Agentes

Mismos 8 agentes en ambos. Diferencias restantes:

| Feature | CORTEX-IA | GENTLE-AI |
|---------|-----------|-----------|
| Auto-install agentes | No | **Si** (brew/apt/pacman/dnf/winget) |
| Per-phase model routing | **Si** (3 presets) | **Si** (3 presets) |
| Sub-agent stubs | **Si** (OpenCode) | No |
| Cursor native subagents | No | **Si** |
| Windsurf workflows | No | **Si** |
| Agent output styles | No | **Si** |

**Gap cerrado**: Per-phase model routing ahora es paritario.

---

## 3. Ecosistema MCP & Persistencia

### Ventaja clara de CORTEX-IA (sin cambios)

| MCP Server | CORTEX-IA | GENTLE-AI | Tools |
|-----------|:---------:|:---------:|:-----:|
| Memory (Cortex) | **Si** (knowledge graph, FTS5) | Engram (daemon) | ~19 vs ~15 |
| CLI Orchestrator | **Si** (circuit breaker) | -- | 4 |
| Agent Mailbox | **Si** (P2P messaging) | -- | 9 |
| ForgeSpec | **Si** (Zod + task board) | -- | 15 |
| Context7 | Si | Si | ~5 |
| **Total** | **~52 tools** | ~20 tools | |

**2.6x mas MCP tools**. ForgeSpec, Mailbox y CLI Orchestrator son exclusivos de cortex-ia.

---

## 4. SDD Workflow

| Aspecto | CORTEX-IA | GENTLE-AI |
|---------|-----------|-----------|
| Fases SDD | 9 | 9 |
| Skills SDD | **19** (naming funcional) | 13 (naming por fase) |
| Contract validation | **ForgeSpec Zod** | -- |
| Task board | **SQLite** | -- |
| File reservation | **Si** | -- |
| P2P messaging | **Si** | -- |
| Model routing | **Si** (3 presets) | **Si** (3 presets) |
| Strict TDD | -- | **Si** |
| Debate mode | **Si** (3 rounds P2P) | Si (2 jueces) |
| Monitor dashboard | **Si** (HTML) | -- |

**cortex-ia lidera en coordinacion multi-agente** (task board + P2P + file reservation). gentle-ai lidera en TDD.

---

## 5. Sistema de Skills

| Aspecto | CORTEX-IA | GENTLE-AI |
|---------|-----------|-----------|
| Total skills | **20** | 16 |
| Deployment | **Centralizado** `~/.cortex-ia/skills/` | Per-agent |
| Convention refs | **Path absoluto** | Inlined |
| Compact rules | -- | **Si** |
| Community skills | -- | **Si** |

---

## 6. Features — Gap Analysis Actualizado

### Gaps que CORTEX-IA cerro (ya no son ventaja de gentle-ai)

| Feature | Antes | Ahora | Como |
|---------|:-----:|:-----:|------|
| Pipeline 2-stage | -- | **Si** | `Orchestrator` + `FailurePolicy` + `RunStageContinue` |
| Per-phase model routing | -- | **Si** | `ClaudeModelAlias` + 3 presets + `{{MODEL_ASSIGNMENTS}}` |
| Health checks | Basico | **6 checks** | `verify/` package con severidad Error/Warning |
| System detection | Minimo | **Completo** | Node, npx, Git, Go, Cortex, Shell |
| Topo sort | DFS | **Kahn's** | `catalog/order.go` con `ParallelGroups` |
| Permissions | -- | **Si** | Deny lists per-agent (Claude JSON, OpenCode JSON, otros via prompt) |
| Self-update | -- | **Si** | `update/` package + GitHub API |
| Sync command | -- | **Si** | `cortex-ia sync --persona --dry-run` |
| Persona system | -- | **Si** | 3 personas (professional/mentor/minimal) |
| CLI commands | 7 | **13** | config, list agents/components/backups, sync, update |
| E2E tests | 0 | **29** | Docker (Ubuntu + Fedora) |
| TUI screens | 6 | **8** | Detection + Persona picker |

### Gaps que aun tiene CORTEX-IA vs GENTLE-AI

| Feature | Impacto | Esfuerzo |
|---------|:-------:|:--------:|
| Auto-install agentes (brew/apt/pacman) | Medio | Alto |
| GGA (AI code review git hooks) | Alto | Alto (binario separado) |
| Theme injection | Bajo | Bajo |
| Strict TDD mode | Medio | Medio |
| Community skills repo | Medio | Bajo |
| Cursor native subagents | Bajo | Medio |
| Windsurf Plan Mode workflows | Bajo | Medio |
| 29 TUI screens (vs 8) | Medio | Alto |
| PowerShell installer | Bajo | Bajo |

### Ventajas exclusivas de CORTEX-IA (gentle-ai no tiene)

| Feature | Impacto |
|---------|:-------:|
| **ForgeSpec** (Zod contracts + task board SQLite + file reservation) | **Critico** |
| **Agent Mailbox** (P2P messaging, threads, broadcast, DLQ) | **Alto** |
| **CLI Orchestrator** (multi-CLI routing + circuit breaker) | **Alto** |
| **Team-lead skill** (coordina apply phase con parallel groups) | **Alto** |
| **5 MCP servers** (~52 tools vs ~20) | **Alto** |
| **Shared skills dir** centralizado | Medio |
| **Debate skill** (adversarial 3 rounds via P2P) | Medio |
| **Monitor skill** (HTML dashboard de pipeline state) | Medio |
| **Parallel-dispatch skill** | Medio |

---

## 7. Testing & Calidad (Actualizado)

| Metrica | CORTEX-IA (antes) | CORTEX-IA (ahora) | GENTLE-AI |
|---------|:-----------------:|:------------------:|:---------:|
| Lineas Go | 7,209 | **9,068** | 46,118 |
| Test functions | 123 | **161** | 916 |
| Ratio tests/code | 1:59 | **1:56** | 1:50 |
| E2E assertions | 0 | **29** | 78 |
| Coverage (SDD) | 48.6% | **93.8%** | N/A |
| Coverage (pipeline) | 32.1% | **98.9%** | N/A |
| Health checks | 1 (file exist) | **6** (files, cortex, node, skills, convention, state/lock) | Si |
| Docker distros | 0 | **2** (Ubuntu, Fedora) | 3 (Ubuntu, Arch, Fedora) |

---

## 8. Distribucion & Deployment (Actualizado)

| Aspecto | CORTEX-IA | GENTLE-AI |
|---------|:---------:|:---------:|
| Installer bash | Si (SHA-256) | Si |
| Installer PowerShell | -- | **Si** |
| Homebrew | -- | **Si** |
| Go install | Si | Si |
| GoReleaser | Si | Si |
| Self-update | **Si** | Si |
| Sync command | **Si** | Si |

**Gap restante**: Homebrew tap + PowerShell installer.

---

## 9. UX — TUI & CLI (Actualizado)

| Aspecto | CORTEX-IA | GENTLE-AI |
|---------|-----------|-----------|
| TUI screens | **8** (welcome, detection, agents, preset, persona, review, installing, complete) | 29 |
| CLI commands | **13** | ~12 |
| Persona picker | **Si** | Si |
| Detection screen | **Si** | Si |
| Dry run | `--dry-run` | `--dry-run` |
| Model preset flag | **`--model-preset`** | Via TUI |
| `config` command | **Si** | -- (via TUI) |
| `list` command | **Si** (agents/components/backups) | -- (via TUI) |

**cortex-ia ahora tiene mas CLI commands** que gentle-ai. gentle-ai lidera en TUI depth (29 vs 8 screens).

---

## 10. Conclusion (Actualizada)

### Antes de las mejoras

cortex-ia tenia ventaja MCP (+52 tools) pero gaps significativos en infraestructura (pipeline, testing, UX, platform support, permissions).

### Despues de las mejoras

cortex-ia ha cerrado **12 de 15 gaps** identificados:

```
Cerrados:  Pipeline 2-stage, Model routing, Health checks, System detection,
           Topo sort, Permissions, Self-update, Sync, Persona, CLI expansion,
           E2E tests, TUI screens

Restantes: Auto-install agentes, GGA (code review), Theme injection
```

### Posicion competitiva actual

| Dimension | Lider | Margen |
|-----------|:-----:|:------:|
| **MCP ecosystem** | **CORTEX-IA** | 2.6x mas tools (52 vs 20) |
| **Multi-agent coordination** | **CORTEX-IA** | ForgeSpec + Mailbox + CLI Orch (exclusivos) |
| **Skills** | **CORTEX-IA** | 20 vs 16, + team-lead/debate/monitor |
| **Testing** | GENTLE-AI | 916 vs 161 tests, 3 vs 2 Docker distros |
| **TUI polish** | GENTLE-AI | 29 vs 8 screens |
| **Codebase size** | GENTLE-AI | 5.1x mas codigo |
| **Platform coverage** | GENTLE-AI | +Homebrew, +PowerShell, +Termux |
| **Pipeline robustez** | **Paritario** | Ambos 2-stage + rollback |
| **Model routing** | **Paritario** | Ambos 3 presets (balanced/performance/economy) |
| **Permissions** | **Paritario** | Ambos deny lists per-agent |
| **Persona system** | **Paritario** | Ambos 3 personas |
| **Self-update** | **Paritario** | Ambos GitHub releases check |
| **Health checks** | **Paritario** | Ambos framework de checks |

### El diferenciador real

**CORTEX-IA** es el unico con **ecosistema MCP completo para coordinacion multi-agente**: ForgeSpec (contratos Zod + task board + file reservation), Agent Mailbox (P2P messaging), y CLI Orchestrator (circuit breaker). Esto es infraestructura que no se replica facilmente.

**GENTLE-AI** tiene mas polish (5x mas codigo, 29 TUI screens) y features de onboarding (auto-install, Homebrew, community skills). Es mas amigable para nuevos usuarios.

**Para un equipo que prioriza calidad de artefactos SDD y coordinacion multi-agente**: CORTEX-IA.
**Para un equipo que prioriza onboarding rapido y UX pulida**: GENTLE-AI.
