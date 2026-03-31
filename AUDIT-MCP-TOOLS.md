# Audit: MCP Tool Usage en Skills y Workflows

> 68 tools disponibles en 4 MCPs. Auditados 21 archivos (10 skills + 9 supplementary + 2 orchestrators).

---

## Resumen Ejecutivo

| Categoria | Skills | Gaps Criticos | Gaps Medios | Gaps Bajos |
|-----------|:------:|:------------:|:-----------:|:----------:|
| SDD Pipeline (10 skills) | 10 | 4 | 6 | 2 |
| Supplementary (9 skills) | 9 | 2 | 3 | 4 |
| Orchestrators (2 prompts) | 2 | 1 | 2 | 0 |
| **Total** | **21** | **7** | **11** | **6** |

---

## Gap 1 — CRITICO: sdd_validate + sdd_save faltante en 4 skills

**Afectados**: draft-proposal, write-specs, architect, decompose

Estos 4 skills persisten artefactos a Cortex via `mem_save` pero NO validan el contrato contra el schema de ForgeSpec ni lo guardan en el historial de contratos.

**Impacto**: No hay audit trail de transiciones de fase. El orchestrator no puede verificar que el contrato cumple el schema. Sin `sdd_save`, `sdd_history` no muestra estas fases.

**Fix**: Agregar al final de cada skill, antes de retornar:
```
sdd_validate(phase: "{phase}", contract: {json})
sdd_save(contract: {validated_json}, project: "{project}")
```

---

## Gap 2 — CRITICO: mem_revision_history no usado en ningun skill

**Afectados**: Todos los 10 skills del pipeline

Ningun skill verifica si los artefactos upstream cambiaron desde que fueron leidos. Si el spec cambia mientras implement esta ejecutando, implement trabaja con datos obsoletos.

**Impacto**: Re-trabajo silencioso. Implement puede producir codigo que no matchea el spec actual.

**Fix**: Agregar despues de recuperar cada artefacto upstream:
```
mem_revision_history(observation_id: {id}) → check if revisions > 1
If changed since last read: flag WARNING in output
```

---

## Gap 3 — CRITICO: execute-plan usa TodoWrite en vez de tb_*

**Afectados**: execute-plan skill

El skill usa `TodoWrite` (session-only) para tracking de progreso. Si la sesion compacta, se pierde todo el progreso.

**Impacto**: Planes multi-dia no tienen recovery. Re-trabajo completo despues de compaction.

**Fix**: Reemplazar TodoWrite con ForgeSpec task board:
```
tb_create_board → tb_add_task per item → tb_claim → tb_update → tb_status for recovery
```

---

## Gap 4 — CRITICO: parallel-dispatch sin file_reserve/mem_save explicitos

**Afectados**: parallel-dispatch skill

Los steps mencionan "assign file ownership" pero no llaman explicitamente a `file_reserve()`, `file_check()`, `file_release()`. Tampoco guarda el dispatch plan a Cortex.

**Impacto**: Conflictos de archivo entre agentes paralelos. Sin recovery si compacta mid-dispatch.

---

## Gap 5 — MEDIO: msg_send con parametro incorrecto en orchestrator

**Afectados**: sdd-orchestrator.md P2P section

El orchestrator muestra `msg_send(to_agent, recipient, ...)` pero el primer parametro es `sender`, no `to_agent`.

**Fix**: Corregir firma a `msg_send(sender, recipient, subject, body, priority?, thread_id?, dedup_key?)`

---

## Gap 6 — MEDIO: debate skill no usa sdd_save ni msg_broadcast correctamente

Round 3 usa `msg_send` individual en vez de `msg_broadcast`. El resultado del debate no se persiste en ForgeSpec via `sdd_save`.

---

## Gap 7 — MEDIO: monitor skill no usa mem_graph ni mem_timeline

El dashboard muestra estado actual pero no evolucion ni relaciones entre artefactos.

---

## Gaps por Skill (detalle)

### SDD Pipeline Skills

| Skill | Falta sdd_validate/save | Falta mem_revision_history | Falta cli_execute | Falta msg_request (P2P) | Otros |
|-------|:-:|:-:|:-:|:-:|-------|
| bootstrap | - | - | SI (version detection) | - | mem_session_start/end |
| investigate | - | SI | SI (grep codebase) | SI (ask architect) | - |
| draft-proposal | SI | SI | SI (verify paths) | SI (ask investigate) | - |
| write-specs | SI | SI | SI (detect framework) | SI (ask architect) | file_reserve (stubs) |
| architect | SI | SI | SI (verify structure) | - | file_check |
| decompose | SI | SI | - | - | tb_create_board, tb_add_task |
| team-lead | - | SI | - | - | msg_list_threads, msg_activity_feed |
| implement | - | SI | SI (local tests) | - | file_check, sdd_get (retries) |
| validate | - | SI | - | - | cli_list, cli_stats, mem_timeline |
| finalize | - | SI | - | - | temporal_create_snapshot, msg_broadcast, tb_status, mem_stats |

### Supplementary Skills

| Skill | Gap Principal |
|-------|--------------|
| debug | mem_relate (link bug→fix), mem_timeline |
| ideate | mem_relate (idea→context), mem_suggest_topic_key |
| execute-plan | **tb_* en vez de TodoWrite**, mem_save progress |
| debate | sdd_save, msg_broadcast (round 3), mem_relate |
| monitor | mem_graph, mem_timeline, mem_revision_history |
| parallel-dispatch | **file_reserve/release explicito**, mem_save plan, tb_create_board |
| open-pr | mem_relate (PR→issue→change) |
| file-issue | mem_relate (issue→project) |
| scan-registry | mem_suggest_topic_key |
