# Informe de la iniciativa `orchestration-quality-hardening`

**Corte documental:** 2026-09-06, a partir del estado del board, el Dual Ledger y las observaciones Cortex citadas abajo.  Este archivo es un informe de usuario, no una fuente de autoridad: **no sustituye SQLite, un claim activo, una revisión independiente ni los leases**. Antes de cualquier acción se debe volver a consultar el estado autoritativo de la tarea y la autoridad viva del bridge.

## Alcance y ubicación

- El producto y los cambios de la iniciativa se trabajan en el worktree aislado [`C:/Users/usrLuisLeon/AppData/Local/Temp/opencode/cortex-ia-v040-oqh`](C:/Users/usrLuisLeon/AppData/Local/Temp/opencode/cortex-ia-v040-oqh), rama `fix/orchestration-quality-hardening`, base `f444475b99226321f58315cf314fd2e0567d5df5`.
- Este informe es el único artefacto escrito en la raíz [`D:/cortex-ia/SPEC.md`](D:/cortex-ia/SPEC.md). No publica, instala, sincroniza ni reconcilia estado vivo.
- La iniciativa conserva el objetivo de un único PR, sin `Install`/`Sync`, publicación, SQL directo ni mutaciones web no autorizadas.

## Inventario autoritativo observado (17 tareas)

Los estados y revisiones siguientes son el *snapshot* leído antes de cerrar este informe; los identificadores de dependencia son contractuales.

| Tarea | Estado / rev. | Dependencias | Resultado, alcance o siguiente condición |
|---|---:|---|---|
| `oqh-00-prepare-worktree` | **superseded / 4** | — | Sustituida por `oqh-00a` y `oqh-00b-create-worktree`; su contrato exigía Go 1.26.1 y una preparación que después se reconcilió. |
| `oqh-00a-resolve-base` | **done / 4** | — | Resolvió `f444475` y verificó Go 1.26.5 sin aprovisionar. |
| `oqh-00b-create-worktree` | **superseded / 6** | `oqh-00a-resolve-base` | Sustituida por `oqh-00b1`/`oqh-00b2` tras el contrato de ruta D: erróneo. |
| `oqh-00b1-verify-created-worktree` | **done / 4** | `oqh-00a-resolve-base` | Verificación de sólo lectura del worktree C: real y del canónico preservado. |
| `oqh-00b2-review-worktree-evidence` | **done / 5** | `oqh-00b1-verify-created-worktree` | Gate cero-escritura completado tras corregir su lifecycle (no fue aprobación directa de una tarea `ready`). |
| `oqh-01-secure-permission-optin` | **in_review / 7** | `oqh-00b2-review-worktree-evidence` | Diff de `config.go`/`runner.go` y checks/tabla de permisos realizados; falta revisión empírica independiente concluyente con oracle Windows corregido. |
| `oqh-02-atomic-review-lease-cleanup` | **done / 8** | `oqh-00b2-review-worktree-evidence` | Limpieza atómica de leases al pasar a revisión, aprobada independientemente. |
| `oqh-02a-align-lease-test-contract` | **backlog / 1** | `oqh-02-atomic-review-lease-cleanup` | Scope único: `internal/delegation/store_test.go`; quedó varada aunque su dependencia ya está `done`. No duplicar ni reclamar mientras no sea `ready`. |
| `oqh-03-align-bridge-review-cleanup` | **ready / 2** | `oqh-02-atomic-review-lease-cleanup` | Alinear cache del bridge y protocolo con la limpieza Store-atómica; comparte `herdr-bridge.ts`, por ello debe coordinarse con `oqh-08`. |
| `oqh-04-security-review-gate` | **backlog / 1** | `oqh-01-secure-permission-optin`, `oqh-02-atomic-review-lease-cleanup` | Gate histórico: requiere reconciliación de contrato/lifecycle; no está automáticamente supersedido. |
| `oqh-05-protocol-review-gate` | **backlog / 1** | `oqh-03-align-bridge-review-cleanup` | Gate histórico: requiere reconciliación de contrato/lifecycle; no está automáticamente supersedido. |
| `oqh-06-single-pr-closure-gate` | **backlog / 1** | `oqh-04-security-review-gate`, `oqh-05-protocol-review-gate` | Cierre histórico: depende de la reconciliación explícita de 04/05 y no acredita cierre por sí solo. |
| `oqh-07-ground-worktree-path-contracts` | **ready / 1** | — | Prevención fail-closed que debe consumir path+HEAD de `git worktree list --porcelain`. |
| `oqh-08-reconcile-stale-bridge-authority` | **ready / 1** | — | Corregir handles `workAuthority` obsoletos sin desalojar claims vivos/ambiguos. |
| `oqh-09-repair-create-readiness-reconcile` | **in_progress / 2** | — | Hotfix Tier-2 Store/CLI/bridge para readiness de creación y CAS de reconciliación. Se informó implementación, pero el controlador agotó contrato antes de transición/limpieza; **no está aprobada**. Hay que comprobar claim, leases y autoridad vivos, no asumir que expiraron. |
| `oqh-env-disable-reviewer-delegation` | **ready / 1** | — | Ajuste externo de configuración, bloqueado operacionalmente por workspace/allowlist D: frente a `C:/Users/usrLuisLeon/.config/opencode`; no verificar ni reescribir sin autoridad correcta. |
| `oqh-doc-spec-report` | **ready / 1 al capturar; intento reclamado rev. 2** | — | Esta documentación. Su transición posterior a `in_review` no modifica la naturaleza no autoritativa de este snapshot. |

### Entregas completadas y evidencia de producto

1. **Preparación empírica:** `oqh-00a`, `oqh-00b1` y `oqh-00b2` están `done`. La evidencia del Ledger confirma el worktree aislado C: limpio, en `f444475b99226321f58315cf314fd2e0567d5df5`, y el canónico C: en `7227313a471afe1d89ca96e524b5f8ff034ed1e5`.
2. **REQ-LEASE-001:** `oqh-02` está `done / 8`. [`internal/delegation/work.go`](C:/Users/usrLuisLeon/AppData/Local/Temp/opencode/cortex-ia-v040-oqh/internal/delegation/work.go) borra todos los leases dentro de la transición exitosa a `in_review`, conserva claim/owner de implementación y evita limpieza parcial en fallos. La smoke aislada cubrió múltiples leases y rollbacks; véase [Cortex #250](cortex://observations/250).
3. **REQ-ENV-001 aún sin aprobación final:** `oqh-01` está `in_review / 7`. El cambio opt-in está en [`internal/delegation/config.go`](C:/Users/usrLuisLeon/AppData/Local/Temp/opencode/cortex-ia-v040-oqh/internal/delegation/config.go) y [`internal/delegation/runner.go`](C:/Users/usrLuisLeon/AppData/Local/Temp/opencode/cortex-ia-v040-oqh/internal/delegation/runner.go); el harness previo y checks pasaron ([Cortex #252](cortex://observations/252)), pero el reviewer requiere el oracle corregido de fake `agy.exe` y PATH controlado ([#255](cortex://observations/255), [#258](cortex://observations/258)). No declarar REQ-ENV-001 cerrado.

## Problemas, causas, impacto y estado

| Problema | Causa y evidencia | Impacto | Resolución / estado |
|---|---|---|---|
| Contrato inicial corregido pero con deriva heredada | [#168](cortex://observations/168) corrigió reglas de ejecución, smoke y revisión, pero el historial incluyó una ruta canónica D: que resultó falsa. | Preparación y gates originales no eran ejecutables literalmente. | Mantener trazabilidad; usar la ruta observada y reconciliar los contratos que aún conservan supuestos obsoletos. |
| Ruta de worktree D: inferida erróneamente | [#205](cortex://observations/205) identifica que se confundió el workspace `D:/cortex-ia` con `%TEMP%`; Git porcelain devolvió la ruta C: real. | `oqh-00b` falló revisión y tuvo que ser sustituida. | `oqh-00b1/00b2` terminadas; `oqh-07` pendiente para prevenir path+HEAD inferidos. |
| Gate reviewer creado como `ready` | [#215](cortex://observations/215): reviewer sólo aprueba `in_review`, no puede reclamar/transicionar un gate `ready`. | Bloqueo de `oqh-00b2`; el modelo de gates 04/05/06 conserva esta tensión. | `oqh-00b2` se resolvió con fase no-review autorizada. `oqh-04/05/06` requieren reconciliación explícita, **no** se consideran supersedidas automáticamente. |
| Agotamiento de contrato de controladores | [#226](cortex://observations/226), [#230](cortex://observations/230), [#235](cortex://observations/235) y patrón [#237](cortex://observations/237). | Implementaciones/verificaciones podían quedar sin transición, con claim aún vivo; reintentar un writer duplicado sería inseguro. | Recuperación: esperar TTL, `work recover`, retry CAS y controlador nuevo que vuelva a reservar/verificar/transicionar. No se soluciona editando producto. La regla exige `max_steps=32` y reservar cinco pasos finales. |
| Handle de autoridad bridge obsoleto | [#246](cortex://observations/246): `recover`/`retry` sólo reconcilian SQLite, no el mapa global `workAuthority`. | Una tarea durable `ready` sin claim podía rechazar claim/reserva como si estuviese tomada. | Recargar/dispose era mitigación temporal; `oqh-08` pendiente para una reconciliación tipada fail-closed. |
| Oracle de permisos Windows y test de lease obsoleto | [#255](cortex://observations/255) exige overlay in-package; [#258](cortex://observations/258) confirma que hacía falta un `agy.exe` real en PATH y que la expectativa de `store_test.go` contradice el contrato ya aprobado de `oqh-02`. | Review de `oqh-01` inconclusa/FAIL de oracle; test focal falla de forma determinista tras semántica válida. | `oqh-02a` es la corrección estrecha del test, pero está varada; repetir la revisión de `oqh-01` sólo con oracle exe/overlay correcto. |
| Readiness de creación varada | [#264](cortex://observations/264): `createWorkInBoardWithDefinition` pone `backlog` si hay dependencias sin mirar si ya están `done`; `unlockDependents` sólo corre al aprobar en ese instante. | `oqh-02a` permanece `backlog / 1` pese a depender de `oqh-02 done / 8`; no se puede reclamar legalmente. | `oqh-09` debe calcular estado inicial y exponer CAS `backlog → ready`; hasta revisión PASS no reconciliar `oqh-02a` ni tocar SQLite directa. |
| Drift Go y criterios incompatibles | Ledger #3 documenta autorización de usuario para Go **1.26.5**, mientras los contratos heredados de `oqh-00`, `oqh-04`, `oqh-05` y `oqh-06` aún piden **1.26.1**. | Los checks heredados pueden fallar aunque la base autorizada sea válida. | Reconciliar los contratos/gates antes de ejecutarlos; no descargar/instalar Go ni cambiar `go.mod` sin nueva autorización. |
| Workspace de configuración externo | Ledger #1–2: `oqh-env-disable-reviewer-delegation` se materializó bajo D: aunque sus archivos viven en configuración C:. | El guard de leases no puede autorizar con seguridad esa escritura externa. | Mantener `ready`/sin cambio hasta disponer de workspace-root y leases correctos; el aviso manual del usuario no es verificación autoritativa. |
| Calidad del Progress Ledger | Hay ciclos duplicados (12 y 31) y una entrada `Cycle 31 ... undefined`. | El relato cronológico puede ser ambiguo; no debe derivarse autoridad de texto duplicado/malformado. | Usar el board y `work status` como fuente de estado; conservar estas entradas como hallazgo de higiene, sin ocultarlas. |
| Escritura de SPEC anterior rechazada | Hubo un intento anterior de escribir `SPEC.md` sin claim/autoridad de tarea. | No se creó informe entonces; demuestra que la documentación también está sujeta a autoridad. | Esta escritura se realiza sólo tras claim y lease normalizado `spec.md`; no implica preferencia permanente de plano de especificación. |

## Cronología resumida

1. Se auditó el plan, se aprobó el hardening A1/B1 y se corrigió el contrato de ejecución ([#168](cortex://observations/168)).
2. La preparación encontró el conflicto de Go y después el path D: inexistente; se verificó empíricamente el worktree C:, se sustituyó `oqh-00b` y se cerraron `00a/00b1/00b2`.
3. `oqh-01` y `oqh-02` avanzaron en paralelo con scopes distintos. Ambos sufrieron `AGENT_CONTRACT_EXCEEDED`; se recuperaron de forma segura con TTL/recover/CAS/controladores nuevos ([#226](cortex://observations/226), [#230](cortex://observations/230), [#237](cortex://observations/237)).
4. Se descubrió que la autoridad bridge residual podía bloquear el retry ([#246](cortex://observations/246)); se dejó `oqh-08` como reparación estructural.
5. `oqh-02` fue revisada y quedó `done / 8`. `oqh-01` llegó a `in_review / 7`, pero su oracle necesita corregirse; el test de leases reveló una expectativa obsoleta y originó `oqh-02a` ([#250](cortex://observations/250), [#258](cortex://observations/258)).
6. `oqh-02a` quedó varada por el defecto de readiness. El RCA creó `oqh-09`; esta última se reporta implementada pero sin transición/cleanup ni aprobación, por lo que sólo el estado vivo puede decidir su recuperación ([#264](cortex://observations/264)).

## Próximos pasos seguros

1. Consultar `work status` de `oqh-09`; si su claim/leases siguen vivos, no intervenir ni duplicar writer. Si expiró, aplicar exclusivamente la recuperación [#237](cortex://observations/237), verificar el diff y obtener revisión independiente antes de aprobar.
2. Tras PASS de `oqh-09`, usar su operación CAS pública para reconciliar `oqh-02a` con revisión exacta; no usar SQL directa ni crear una tarea duplicada. Implementar/revisar sólo entonces la alineación de [`internal/delegation/store_test.go`](C:/Users/usrLuisLeon/AppData/Local/Temp/opencode/cortex-ia-v040-oqh/internal/delegation/store_test.go).
3. Repetir el reviewer de `oqh-01` con el overlay in-package y `fake/agy.exe` PATH-prepend descritos por [#255](cortex://observations/255) y [#258](cortex://observations/258); aprobar únicamente con evidencia independiente concluyente.
4. Planificar `oqh-03` y `oqh-08` secuencialmente porque ambos reservan [`internal/assets/plugins/herdr-bridge.ts`](C:/Users/usrLuisLeon/AppData/Local/Temp/opencode/cortex-ia-v040-oqh/internal/assets/plugins/herdr-bridge.ts); ejecutar `oqh-07` con su smoke Git aislada.
5. Reconciliar de forma explícita los contratos de `oqh-04/05/06`, el drift 1.26.1/1.26.5 y el task externo de configuración antes de afirmar cierre de la iniciativa. No hay autorización de publicación, `Install` o `Sync`.

## Índice de evidencia Cortex

- [#168](cortex://observations/168) contrato de ejecución corregido y defectos de planificación/gates.
- [#205](cortex://observations/205) causa de la ruta D: inferida erróneamente.
- [#215](cortex://observations/215) lifecycle inválido de reviewer gate `ready`.
- [#226](cortex://observations/226), [#230](cortex://observations/230), [#235](cortex://observations/235), [#237](cortex://observations/237) agotamiento de contrato y recuperación segura.
- [#246](cortex://observations/246) latch de autoridad bridge obsoleto.
- [#250](cortex://observations/250) limpieza atómica aprobada de `oqh-02`.
- [#252](cortex://observations/252) evidencia de implementación de permisos opt-in.
- [#255](cortex://observations/255), [#258](cortex://observations/258) oracle Windows y política/test de leases obsoleto.
- [#264](cortex://observations/264) defecto de readiness de creación y reconciliación necesaria.

**Veredicto del informe:** hay avances comprobados, pero la auditoría/initiative completa **no está resuelta ni lista para publicación**. La fuente de verdad operativa continúa siendo la autoridad SQLite/bridge vigente, no este documento.
