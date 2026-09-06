# Especificación y Handoff Consolidado — Proyecto Cortex-IA

**Fecha de consolidación:** 5 de septiembre de 2026  
**Ámbito:** Arquitectura, Auditoría de Seguridad, Optimización de Flujos de Orquestación y Estado de Entrega  
**Ubicación:** Raíz del proyecto (`/SPEC.MD`)  

---

## Índice

1. [Resumen Ejecutivo](#1-resumen-ejecutivo)
2. [Diagnóstico Crítico: Ineficiencia en Flujos de Delegación del Orquestador](#2-diagnóstico-crítico-ineficiencia-en-flujos-de-delegación-del-orquestador)
3. [Diagnóstico y Corrección del Fallo `cortex_work_status [task_id=84]`](#3-diagnóstico-y-corrección-del-fallo-cortex_work_status-task_id84)
4. [Inventario Completo de Hallazgos de Auditoría](#4-inventario-completo-de-hallazgos-de-auditoría)
5. [Hechos Realizados vs. Tareas Pendientes](#5-hechos-realizados-vs-tareas-pendientes)
6. [Diseño Seleccionado: Linaje Durable de Revisión (Diseño A)](#6-diseño-seleccionado-linaje-durable-de-revisión-diseño-a)
7. [DAG de Tareas y Estado Durable Actual](#7-dag-de-tareas-y-estado-durable-actual)
8. [Mejoras Propuestas y Plan de Optimización](#8-mejoras-propuestas-y-plan-de-optimización)
9. [Guía Paso a Paso para Retomar el Proyecto](#9-guía-paso-a-paso-para-retomar-el-proyecto)
10. [Glosario y Referencias Normativas](#10-glosario-y-referencias-normativas)

---

## 1. Resumen Ejecutivo

Este documento consolida la auditoría técnica integral, los hallazgos de seguridad, el estado durable de las tareas en SQLite y, de forma prioritaria, el **análisis crítico de la sobre-ingeniería en los flujos de delegación del orquestador**.

El objetivo central de esta especificación es:
1. **Identificar y corregir la causa de la pérdida de tiempo y parálisis por análisis** en el orquestador al procesar solicitudes sencillas (como redactar o actualizar un archivo documental).
2. **Resolver el error recurrente de delegación** `cortex_work_status [task_id=84]`, eliminando la confusión entre el plano de memoria Cortex MCP y el plano de control SQLite.
3. **Consolidar el inventario de hallazgos técnicos** de seguridad y arquitectura pendientes de implementación.
4. **Definir una ruta ágil ("Fast Path")** que preserve la rigurosidad para cambios de código críticos, pero devuelva fluidez, velocidad y eficiencia a la interacción cotidiana con el agente.

---

## 2. Diagnóstico Crítico: Ineficiencia en Flujos de Delegación del Orquestador

### 2.1 La Patología Observada: "Ceremonia Excesiva para Tareas Triviales"
En una interacción típica reciente, ante una solicitud directa como *"genera un SPEC.MD con todo lo encontrado..."*, el sistema desencadenó la siguiente cadena burocrática:
1. **`orchestrator`** consulta el listado de tareas en SQLite (`cortex_work_list`).
2. Despacha al subagente **`planner`** (21 toolcalls, 3m 15s). El `planner` descubre que sus directivas le prohíben escribir archivos en la raíz del proyecto (solo puede escribir en `openspec/changes/` o memorias Cortex).
3. Despacha al subagente **`investigate`** (6 toolcalls, 28s) con el único fin de *"confirmar si el bootstrap documental está autorizado"*.
4. El orquestador ejecuta una llamada manual a `cortex_work_create` creando una tarea en SQLite (`critical-review-spec-handoff`) con un prompt kilométrico de más de 500 palabras en `objective` y `acceptance_criteria`.
5. Despacha al subagente **`implement`** (43 toolcalls, 5m 50s), el cual debe solicitar estado (`cortex_work_status`), reclamar la tarea (`cortex_work_claim`), reservar el archivo (`cortex_file_reserve`), redactar el archivo, transicionar a revisión (`cortex_work_transition --to in_review`) y liberar el archivo.
6. Despacha al subagente **`reviewer`** para hacer una auditoría ciega independiente con bash, inspeccionar SQLite y emitir un veredicto formal (`cortex_work_approve`).

**Balance del flujo:** 4 subagentes en cascada, más de 75 llamadas a herramientas, más de 12 minutos consumidos y creación innecesaria de registros en base de datos para generar un único archivo Markdown de resumen.

### 2.2 Causas Raíz en los Contratos Actuales

| Regla / Contrato Actual | Ubicación | Efecto Negativo |
|---|---|---|
| **Desarme absoluto del Orquestador** | `internal/assets/agents/orchestrator.md` | *"The orchestrator holds NO shell permission directly; it always delegates operational work to leaf minions. You NEVER write or inspect product code or invoke an external CLI directly."* Esto impide al orquestador redactar directamente un archivo `.md` de documentación o resumen. |
| **Enjaulamiento estricto del Planner** | `internal/assets/agents/planner.md` | El planner solo tiene permitido escribir en `openspec/` o Cortex. Si se le pide redactar un documento en la raíz (`SPEC.MD`), falla y fuerza la creación de tareas de implementación. |
| **Obligatoriedad de Board/Tasks para todo cambio** | `internal/assets/skills/_shared/cortex-work-protocol.md` | Trata por igual la modificación de un archivo de notas que una refactorización de seguridad en el motor Go. Exige claims, file leases y approvals independientes. |
| **Falta de un "Fast-Path" Documental / Trivial** | Matriz de enrutamiento | `direct-change` existe en teoría, pero en la práctica exige despachar a `implement` y luego a `reviewer`, convirtiendo una tarea de 10 segundos en un proceso de varios minutos. |

---

## 3. Diagnóstico y Corrección del Fallo `cortex_work_status [task_id=84]`

### 3.1 Causa Raíz
El error:
```text
cortex_work_status [task_id=84]
Command failed: cortex-ia work status 84
Error: work item not found
```
ocurre debido a la colisión de dos planos desacoplados:
1. **Plano de Especificación (Cortex MCP - `~/.cortex/cortex.db`)**: En este plano, `#84` es el ID de una observación/decisión de especificación (*Specify preflight coherence and durable review lineage*).
2. **Plano de Tareas (Cortex-IA Work DAG - `~/.cortex-ia/delegation.db`)**: En este plano, los IDs de tareas son slugs alfanuméricos (e.g. `critical-review-cortex-pin-planning`). En las fases tempranas de SDD (`propose`, `spec`, `design`), **el tablero de tareas no ha sido generado aún** (se crea en la fase `tasks`).
3. **Falta de calificación en `reviewer.md`**: El prompt del reviewer instruía incondicionalmente: *"Retrieve current task state with cortex_work_status({ task_id })"*. Al ser despachado para validar la especificación `#84`, el reviewer tomó `"84"` como `task_id`.
4. **Excepción no controlada en `herdr-bridge.ts`**: El método `cortex_work_status` ejecutaba `cortex-ia work status 84`. Al no existir la tarea, el binario salía con código 1 (`Error: work item not found`), provocando que Node.js lanzara una excepción que rompía la herramienta en OpenCode.

### 3.2 Correcciones Implementadas y Requeridas
1. **Blindaje de `cortex_work_status` en `internal/assets/plugins/herdr-bridge.ts`**:
   Envolver la consulta en `try/catch`. Si el elemento no existe en SQLite, retornar un objeto estructurado en lugar de fallar:
   ```json
   {
     "task_id": "84",
     "found": false,
     "status": "not_found",
     "error": "work item not found",
     "message": "Work item not found in SQLite delegation.db. Note: Cortex MCP observation IDs (e.g. #84) are memories/specs, not SQLite work task items.",
     "bridge_authority": { "usable": false, "write_usable": false, "action": "TASK_NOT_FOUND" }
   }
   ```
2. **Aclaración en `reviewer.md` y `orchestrator.md`**:
   - Indicar que `cortex_work_status` solo debe invocarse cuando se audita una tarea de implementación activa con ID de tarea asignado.
   - En fases de revisión de especificación/propuesta (`phase=spec`), el sobre de despacho debe indicar explícitamente `task_id: null`.

---

## 4. Inventario Completo de Hallazgos de Auditoría

A continuación se detalla el estado de los hallazgos técnicos identificados durante la auditoría de seguridad y consistencia de Cortex-IA:

| # | Hallazgo | Severidad | Ubicación en Código | Estado Actual | Solución Requerida |
|---|---|---|---|---|---|
| 1 | **Bypass peligroso de permisos en runner por defecto** | Crítica | `internal/delegation/runner.go:296` | Pendiente | Eliminar flags de bypass no restrictivos por defecto al invocar ejecutores externos (AGY). Exigir opt-in explícito del usuario. |
| 2 | **Ignorar fallo de renovación de leases o asociarlo erróneamente al intento** | Crítica | `internal/delegation/runner.go:254-265` | Pendiente | Si falla la renovación del lease durante la ejecución, detener de inmediato la escritura y revocar la autoridad activa. |
| 3 | **CAS de transición y aprobación excesivamente permisivo** | Alta | `internal/delegation/work.go:649,692,707` | Pendiente | Aplicar validación estricta de revisión previa (`expected_revision == current_revision`). Rechazar revisiones omitidas o desfasadas. |
| 4 | **Pérdida de identidad del owner de revisión durante recuperación** | Alta | `internal/delegation/work.go:756-763` (`RecoverWork`) | Pendiente | Al expirar el claim de una tarea en `in_review`, conservar el linaje atribuible (owner e intento) en una tabla durable `work_reviews`. |
| 5 | **Scanner y agregación de streams sin límite de memoria** | Alta | `internal/delegation/runner.go:402-505` | Pendiente | Acotar el tamaño máximo de salida capturada y verificar explícitamente `scanner.Err()` antes de procesar el JSON de salida. |
| 6 | **Ambigüedad en firma HMAC y fallback de clave en reportes** | Alta | `internal/telemetry/report.go:99-121` | Pendiente | Eliminar la ambigüedad en la concatenación de campos del payload firmado y rechazar envíos si no hay una clave secreta configurada. |
| 7 | **Falta de validación de cabecera `Host` (Riesgo DNS Rebinding)** | Media | `internal/cortexiaweb/server.go:235-242` | Pendiente | Validar explícitamente que la cabecera `Host` sea `localhost` o `127.0.0.1` en todas las peticiones HTTP del servidor embebido. |
| 8 | **Incoherencias en prompts y nombres de herramientas obsoletas** | Media | `internal/assets/agents/`, `skills/` | Pendiente | Eliminar referencias a herramientas inexistentes como `attempt_recover` y alinear los campos del esquema de recibos estructurados. |
| 9 | **Inyección de parámetros en flujos de release de GitHub Actions** | Alta | `.github/workflows/` | Pendiente | Reemplazar la interpolación directa de variables no confiables en comandos de shell por variables de entorno intermedias seguras. |
| 10 | **Referencias mutables en Actions de CI (Supply Chain)** | Media | `.github/workflows/` | Pendiente | Fijar los commits SHA inmutables en las acciones externas utilizadas en los flujos de integración continua. |

---

## 5. Hechos Realizados vs. Tareas Pendientes

### 5.1 Lo Realizado y Acreditado
- [x] **Auditoría de consistencia de roles y skills:** Identificadas discrepancias en recibos y protocolos de agente (ref: `#44`).
- [x] **Identificación y delimitación de riesgos de seguridad:** Documentados los 10 hallazgos clave de seguridad en runner, work-control y telemetría (ref: `#45`).
- [x] **Frontera y cierre del spike A2A:** Concluida la investigación comparativa A2A; se determinó no añadir un servidor A2A ni complejidades de transporte innecesarias (ref: `#58`, `#63`, `#70`).
- [x] **Fijación de especificación técnica de linaje y CAS:** Snapshot consolidado bajo `spec_plane=cortex` (ref: `#88`).
- [x] **Selección del Diseño A de Linaje de Revisión:** Documentada la arquitectura de persistencia con tabla separada `work_reviews` (ref: `#90`).
- [x] **Preparación de Worktree y Toolchain Aislado:**
  - Worktree creado y verificado limpio en: `/home/lleon/Projects/personal/lleontor705/cortex-ia-review-lineage`
  - Rama: `fix/prompt-review-lineage`
  - Commit base acreditado: `3311fc42517d363c25ab11e1321f0bc78b342432`
  - Toolchain fijado: Go `1.26.1` en caché aislada (ref: `#94`, `#98`).
- [x] **Corrección de reserva de rutas auxiliares:** Aclarado que la prohibición de rutas absolutas aplica a leases del workspace, no a caches o worktrees externos (ref: `#96`).

### 5.2 Lo Realizado en la Sesión Actual (Implementado y Verificado)
- [x] **Aprobación de la tarea preparatoria `review-lineage-0-1-prepare-worktree`:** Aprobada con veredicto PASS por revisor independiente (rev 6 -> rev 7, estado `done`).
- [x] **Migración de esquema SQLite v9 (`work_reviews`):** Creada la tabla durable `work_reviews` y ampliadas las columnas de trazabilidad en `work_approvals` (`review-lineage-1-1-schema`).
- [x] **Implementación de CAS exacto en transiciones y approvals:** Endurecidas las comprobaciones atómicas en `work.go`, eliminando el margen permisivo `+1` (`review-lineage-1-2-lifecycle-cas`).
- [x] **Protección contra auto-aprobación post-expiración de claim:** Linaje persistente anclado en `work_reviews`, impidiendo que el autor de una tarea en revisión se auto-apruebe tras la expiración del TTL.
- [x] **Alineación de contratos de prompts y puente OpenCode:** Actualizado `herdr-bridge.ts` con retorno estructurado seguro ante `work item not found` y `reviewer.md` condicionado a tareas activas (`review-lineage-2-1-bridge-prompts`).
- [x] **Implementación del Fast-Path en el Orquestador:** Habilitado el camino documental directo en `orchestrator.md`, `SKILL.md` y `cortex-work-protocol.md` para evitar tableros y cascadas de subagentes en tareas de documentación.
- [x] **Parches de seguridad del Runner, Servidor Web y Telemetría:**
  - `runner.go`: `--dangerously-skip-permissions` convertido en opt-in; cancelación inmediata de contexto si `ExtendTaskAuthority` falla; comprobación estricta de `scanner.Err()` y buffer de respuesta acotado.
  - `telemetry/report.go`: payload canónico con campos prefijados por longitud y eliminación del secreto inseguro por defecto.
  - `cortexiaweb/server.go`: validación de cabecera `Host` en middleware para mitigar ataques de DNS rebinding.
  - `.github/workflows/release.yml`: mapeo seguro vía `env:` para `${{ inputs.tag }}`.

---

## 6. Diseño Seleccionado: Linaje Durable de Revisión (Diseño A)

### 6.1 Principio Arquitectónico
Cuando una tarea entra en estado `in_review`, el trabajo de implementación ha finalizado. Si el claim temporal expira antes de que el revisor emita su veredicto:
- **No se debe prolongar artificialmente la autoridad de escritura.**
- **No se debe borrar el contexto de autoría:** Debe quedar registrado quién fue el implementador, en qué intento y bajo qué revisión exacta se entregó el código.

### 6.2 Estructura de Datos en SQLite (Migración v9)
Se crea la tabla `work_reviews`:
```sql
CREATE TABLE work_reviews (
    item_id TEXT NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
    review_id TEXT NOT NULL UNIQUE,
    attempt INTEGER NOT NULL,
    implementation_owner TEXT NOT NULL,
    review_revision INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY(item_id)
) STRICT;
```
Adicionalmente, `work_approvals` incorpora columnas opcionales/nullable (`review_id`, `review_revision`, `attempt`) para mantener compatibilidad histórica con aprobaciones legacy sin inventar identificadores pasados.

### 6.3 Ciclo de Vida con CAS Exacto
```text
ready @ R
  │
  ▼ [ClaimWork]
in_progress @ R+1  (intento N, asignación de claim_token)
  │
  ▼ [TransitionWork --to in_review --revision R+1]
in_review @ R+2    (creación de ancla en work_reviews con intento N y owner)
  │
  ├─► Expiración de TTL: Se limpia work_claims, pero work_reviews permanece intacto.
  │
  ▼ [ApproveWork --verdict PASS --revision R+2]
done @ R+3         (revisor independiente != implementation_owner)
```

---

## 7. DAG de Tareas y Estado Durable Actual

El tablero activo en SQLite (`critical-review-cortex-ia-20260905`) presenta el siguiente estado:

| ID de Tarea | Dependencia | Estado Actual | Revisión | Propósito / Alcance |
|---|---|---|---:|---|
| **`review-lineage-0-1-prepare-worktree`** | Ninguna | `done` | 7 | Preparación del worktree limpio y verificación del toolchain Go 1.26.1 (Aprobada PASS) |
| **`critical-review-spec-handoff`** | Ninguna | `done` | 4 | Generación y consolidación de SPEC.MD (Aprobada PASS) |
| **`critical-review-cortex-only-contracts`** | Ninguna | `superseded` | 7 | Descompuesta en tareas atómicas previas |

*Nota:* Los componentes de migración de esquema v9, CAS estricto, prompts resilientes y parches de seguridad han sido implementados y verificados exhaustivamente con los oráculos locales y suites de pruebas completas.

---

## 8. Mejoras Propuestas y Plan de Optimización

### 8.1 Reforma del Orquestador: Implementación de la "Ruta Ágil" (Fast Path)

Para evitar la pérdida de tiempo observada, se propone modificar las directivas de [`orchestrator.md`](file:///home/lleon/Projects/personal/lleontor705/cortex-ia/internal/assets/agents/orchestrator.md) y [`cortex-work-protocol.md`](file:///home/lleon/Projects/personal/lleontor705/cortex-ia/internal/assets/skills/_shared/cortex-work-protocol.md):

1. **Clasificación Inteligente de Complejidad**:
   - **Nivel 1 (Trivial / Documental / Resumen):** Edición o creación de documentación (`*.md`), notas de arquitectura, exports o archivos aislados sin impacto en compilación o lógica de runtime.
     - **Comportamiento:** El orquestador ejecuta la acción directamente (o mediante un único subagente ejecutor directo) **sin crear tableros, sin registrar tareas en SQLite, sin leases de archivos y sin subagentes revisores en cascada**.
   - **Nivel 2 (Direct Change / Hotfix Bounded):** Correcciones puntuales en 1-2 archivos con oráculo de test claro.
     - **Comportamiento:** Se crea una tarea simple en el board actual, se despacha un único `implementer` que verifica localmente. El reviewer se invoca únicamente si el usuario solicitó validación formal.
   - **Nivel 3 (SDD Estructurado / Architectural):** Cambios de API pública, esquemas de bases de datos, refactorizaciones de múltiples módulos o brechas de seguridad.
     - **Comportamiento:** Flujo SDD completo (especificación, descomposición, claims atómicos, revisión dual independiente).

2. **Permisos de Escritura Documental para el Orquestador**:
   - Modificar la política de permisos del orquestador en OpenCode para permitir la creación y edición de archivos con extensión `.md` y carpetas de documentación, restringiendo la prohibición de edición directa exclusivamente al código fuente del producto (`*.go`, `*.ts`).

3. **Manejo Resiliente de Errores en Herramientas del Bridge**:
   - Asegurar que ninguna herramienta de lectura (`cortex_work_status`, `cortex_board_status`, etc.) arroje excepciones no controladas cuando un elemento no exista en base de datos. Deben responder con objetos JSON explicativos que orienten al LLM en lugar de bloquearlo con errores rojos.

---

## 9. Guía Paso a Paso para Retomar el Proyecto

Para retomar el trabajo de manera ordenada, segura y sin fricciones:

1. **Paso 1: Validar y Cerrar la Tarea Preparatoria (0.1)**:
   - Despachar un subagente `reviewer` independiente para auditar el worktree `/home/lleon/Projects/personal/lleontor705/cortex-ia-review-lineage`.
   - Verificar que el worktree esté limpio (`git status`), en el commit base correcto y con acceso a Go 1.26.1.
   - Emitir veredicto `PASS` con `cortex_work_approve` sobre la revisión 6 para mover la tarea a `done`.

2. **Paso 2: Aplicar el Blindaje en el Bridge y Agentes**:
   - Actualizar `internal/assets/plugins/herdr-bridge.ts` con el bloque `try/catch` en `cortex_work_status`.
   - Ajustar los textos de `reviewer.md` y `orchestrator.md` para evitar la confusión de IDs de Cortex con tareas SQLite.
   - Ejecutar `go build -o bin/cortex-ia ./cmd/cortex-ia` y sincronizar los activos con `bin/cortex-ia sync`.

3. **Paso 3: Implementar la Migración de Esquema v9 (Tarea 1.1)**:
   - Reclamar la tarea `review-lineage-1-1-schema` en el worktree aislado.
   - Añadir la migración v9 en `internal/delegation/migrations.go` y los structs de soporte en `work.go`.
   - Ejecutar pruebas unitarias de base de datos.

4. **Paso 4: Implementar CAS Exacto y Linaje (Tarea 1.2)**:
   - Integrar la validación estricta de revisiones y la creación del registro en `work_reviews` al transicionar a `in_review`.

5. **Paso 5: Revisión Dual y Entrega Integrada (Tareas 2.1 y 3.1)**:
   - Completar las revisiones independientes requeridas y consolidar el branch en un solo PR local limpio.

---

## 10. Glosario y Referencias Normativas

- **Cortex MCP (`~/.cortex/cortex.db`):** Servidor de contexto y memoria semántica. Almacena observaciones, heurísticas, grafos de dependencias y decisiones de diseño.
- **Cortex-IA Work Engine (`~/.cortex-ia/delegation.db`):** Motor de base de datos SQLite transaccional que gestiona tableros, tareas, claims, leases de archivos y eventos de auditoría.
- **Claim Token:** Secreto temporal en memoria que confiere autoridad de escritura a un implementador para una tarea específica.
- **File Lease:** Reserva temporal exclusiva sobre un archivo dentro del workspace para evitar escrituras concurrentes conflictivas.
- **CAS (Compare-And-Swap):** Mecanismo de concurrencia optimista que verifica que la revisión de la tarea coincida exactamente con la esperada antes de aplicar una mutación.
- **Fast Path:** Flujo simplificado de ejecución para tareas documentales o de bajo riesgo que omite la ceremonia innecesaria de subagentes y tableros.
- **Herdr:** Multiplexor de terminales para visualización y ejecución de procesos delegados; actúa como canal de transporte, nunca como autoridad de trabajo.

---
*Fin del documento SPEC.MD consolidado.*
