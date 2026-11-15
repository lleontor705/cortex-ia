[English](cli.md) | **Español**

# Referencia de la CLI

Todos los grupos de comandos de `cortex-ia`, con su sintaxis y propósito. Ejecutar `cortex-ia` sin argumentos abre la interfaz de terminal; cualquier argumento pasa por el despachador de comandos. Cada grupo imprime su uso con `--help`.

Los verbos de autoridad de trabajo (`claim`, `lease`, `transition`, `approve` y similares) están pensados para los controladores de rol a través de las herramientas tipadas `cortex_ia_*`. La CLI los rechaza cuando se invocan desde el shell de un agente.

Los comandos que emiten recibos legibles por máquina imprimen JSON en stdout; los comandos de diagnóstico humano (`doctor`, `rollback`, `recover`, `report status`, `help`, `update`) imprimen texto plano. Las consultas de estado aceptan los alias `show`/`get` donde se indica, y cada grupo de subcomandos imprime su uso con `--help`.

## Tableros de tareas (`cortex-ia board`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Crear** | `cortex-ia board create <id> "<title>" "[desc]"` | Inicializa un límite de tablero de tareas duradero |
| **Listar** | `cortex-ia board list` | Lista todos los tableros con contadores de completadas/total |
| **Estado** | `cortex-ia board status <id>` *(o `show`, `get`)* | Consulta los metadatos del tablero y la instantánea completa del DAG de tareas |
| **Archivar** | `cortex-ia board archive <id>` | Marca un tablero completado como archivado |
| **Desarchivar** | `cortex-ia board unarchive <id>` | Restaura un tablero archivado a activo |
| **Eliminar** | `cortex-ia board delete <id>` | Elimina permanentemente un tablero archivado y sus tareas |
| **Servir** | `cortex-ia board serve [--addr 127.0.0.1:7331]` | Ejecuta el panel web loopback embebido |

## Elementos de trabajo y concesiones (`cortex-ia work`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Crear** | `cortex-ia work create <id> "<title>" [--board <board>] [--depends <id>]... [--objective <text>] [--acceptance <text>] [--verify <cmd>] [--file <path>]...` | Añade una tarea al DAG (`backlog`/`ready`) con su definición |
| **Revisar** | `cortex-ia work revise --plan <file\|@stdin>` | Revisa de forma segura la definición de una tarea no reclamada |
| **Refrescar revisión** | `cortex-ia work review-refresh <id> --revision <n>` | Vuelve a vincular la revisión a una revisión observada |
| **Archivar** | `cortex-ia work archive --board <id> --change <id> --workflow <sdd-lite\|sdd-full> --spec-plane <openspec\|cortex\|hybrid>` | Cierra trabajo SDD aprobado de forma independiente |
| **Listar** | `cortex-ia work list [--board <board-id>]` | Lista los elementos de trabajo, opcionalmente acotados a un tablero |
| **Estado** | `cortex-ia work status <id>` *(o `show`, `get`)* | Consulta el estado de la tarea, la revisión, el reclamo y las concesiones activas |
| **Aprobaciones** | `cortex-ia work approvals <id>` | Lista los registros históricos de aprobación |
| **Huella** | `cortex-ia work fingerprint <id>` | Calcula las huellas actuales y las compara con la aprobación |
| **Reclamar** | `cortex-ia work claim <id> --owner <owner> [--path <file> ...] [--ttl 15m]` | Adquiere atómicamente una tarea (y concesiones opcionales); devuelve `claim_token` |
| **Renovar controlador** | `cortex-ia work controller-renew <id> --owner <owner> --authority @stdin` | Renueva un reclamo activo y su conjunto completo de concesiones |
| **Renovar** | `cortex-ia work renew <id> --claim-token <tok> [--ttl 15m]` | Extiende el TTL del reclamo activo antes de su vencimiento |
| **Conceder** | `cortex-ia work lease <id> --claim-token <tok> --path <file> [--ttl 15m]` | Reserva una concesión de archivo exclusiva; devuelve `lease_token` |
| **Reservar** | `cortex-ia work reserve <id> --claim-token <tok> --path <file> [--path <file> ...] [--ttl 15m]` *(o `file-reserve`)* | Reserva uno o más archivos de forma atómica |
| **Renovar concesión** | `cortex-ia work lease-renew --path <file> --lease-token <tok> [--ttl 15m]` | Extiende el TTL de una concesión de archivo mientras se edita |
| **Liberar** | `cortex-ia work release --path <file> --lease-token <tok>` | Libera una concesión de archivo |
| **Liberar todo** | `cortex-ia work release-all <id> --claim-token <tok>` | Libera todas las concesiones de archivo en poder de una tarea |
| **Transicionar** | `cortex-ia work transition <id> --claim-token <tok> [--revision <n>] --to <in_review\|in_progress\|blocked>` | Cambia el estado de la tarea con un recibo de envío opcional |
| **Aprobar** | `cortex-ia work approve <id> --reviewer <id> --verdict <PASS\|FAIL\|BLOCKED\|INCONCLUSIVE> [--evidence <ref>]` | Registra un veredicto de revisión; `PASS` desbloquea las tareas posteriores |
| **Reintentar** | `cortex-ia work retry <id> [--revision <n>]` | Elimina los bloqueos residuales y devuelve una tarea `blocked` a `ready` |
| **Descomponer** | `cortex-ia work decompose <id> --revision <n> --plan <file\|@stdin> [--contract-file <file>]` | Reemplaza una tarea bloqueada por tareas atómicas |
| **Recuperar** | `cortex-ia work recover` | Barre los reclamos y concesiones vencidos |
| **Reconciliar** | `cortex-ia work reconcile <id> --reason <text> --session <id> --revision <n>` | Fuerza la liberación de un reclamo vivo pero huérfano (solo orquestador, falla de forma cerrada) |
| **Verificar concesión** | `cortex-ia work verify-lease --path <file> [--task <id>] [--owner <owner>]` *(o `check-lease`)* | Verifica una concesión de archivo activa |

## Espacio de trabajo OpenSpec SDD (`cortex-ia openspec`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Validar** | `cortex-ia openspec validate <change> --workflow <sdd-lite\|sdd-full> --phase <phase> [--json]` | Valida estructuralmente los artefactos de planificación. `--workflow` y `--phase` son **obligatorios** |
| **Listar** | `cortex-ia openspec list` | Lista las propuestas de cambio activas en `openspec/changes/` |
| **Estado** | `cortex-ia openspec status [change-name]` | Inspecciona el progreso de las tareas y el estado de los cambios |
| **Archivar** | `cortex-ia openspec archive <change-name> --board <id> --workflow <sdd-lite\|sdd-full> --spec-plane <openspec\|cortex\|hybrid>` | Cierra trabajo SDD aprobado de forma independiente |
| **Nuevo** | `cortex-ia openspec new <change-name> [domain]` | Genera la estructura de un nuevo directorio de cambio OpenSpec |

## Instantáneas de Cortex (`cortex-ia snapshot`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Leer** | `cortex-ia snapshot read --project <project> --id <id> [--expected-sha256 <digest>]` | Lee y verifica una observación local acotada de Cortex |

## Worktrees de Git (`cortex-ia worktree`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Listar** | `cortex-ia worktree list [--repo <repo-path>]` | Lista los worktrees de Git autoritativos |
| **Validar** | `cortex-ia worktree validate <worktree-path> [--repo <repo-path>] [--head <commit>]` | Valida un contrato de worktree contra git porcelain |

`current_workspace` es la única estrategia de ejecución soportada. `worktree create`, `clean`, `drop`, `delete`, `remove` y `prune` están retirados y fallan de forma cerrada; los worktrees existentes se conservan.

## Doble libro mayor (`cortex-ia ledger`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Añadir hecho** | `cortex-ia ledger fact add <text> [--board <id>] [--source <src>] [--sync-cortex]` | Registra un hecho verificado (opcionalmente sincronizado con la memoria de Cortex) |
| **Listar hechos** | `cortex-ia ledger fact list [--board <board-id>] [--json]` | Lista los hechos verificados en orden cronológico |
| **Progreso** | `cortex-ia ledger progress record --summary <text> [--drift] [--action <act>]` | Registra una evaluación de progreso del orquestador |
| **Estado** | `cortex-ia ledger status [--board <board-id>] [--json]` | Muestra el informe completo del doble libro mayor (hechos + progreso) |

## Instantánea de la UI (`cortex-ia ui`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Instantánea** | `cortex-ia ui snapshot [--project <path>] [--session-id <id>] [--root-session-id <id>]` | Imprime una instantánea acotada de solo lectura de la TUI |

## Documentos y diagramas (`cortex-ia doc` / `cortex-ia diagram`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Convertir documento** | `cortex-ia doc convert <file> [-o <out.md>] [--standalone] [--format <fmt>] [--max-lines <n>] [--ocr <hosted\|reject>] [--json]` | Convierte documentos de oficina/PDF a Markdown |
| **Inspeccionar documento** | `cortex-ia doc inspect <file> [--json]` | Inspecciona los metadatos del documento |
| **Validar diagrama** | `cortex-ia diagram validate <type> <spec.json> [--quality <standard\|showcase>] [--json]` | Valida la topología del diagrama |
| **Renderizar diagrama** | `cortex-ia diagram render <type> <spec.json> [output.html] [--quality <standard\|showcase>] [--json]` | Renderiza un archivo HTML de diagrama interactivo |
| **Comparar diagrama** | `cortex-ia diagram compare <base.json> <head.json> [output.html] [--json]` | Compara dos instantáneas de arquitectura |
| **Alcance del diagrama** | `cortex-ia diagram reach <type> <spec.json> --from <node-id> [--direction <upstream\|downstream\|both>] [--json]` | Rastrea la alcanzabilidad del grafo desde un nodo |

## Gestión de MCP (`cortex-ia mcp`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Añadir (preset)** | `cortex-ia mcp add <name> --preset [--dry-run]` | Registra un preset de MCP del catálogo gestionado |
| **Añadir (local)** | `cortex-ia mcp add <name> --local [--env KEY=VALUE]... -- <command> [args...]` | Registra un servidor MCP local personalizado gestionado |
| **Añadir (remoto)** | `cortex-ia mcp add <name> --remote <url> [--header KEY=VALUE]... [--dry-run]` | Registra un servidor MCP remoto personalizado gestionado |
| **Listar** | `cortex-ia mcp list [--json]` | Lista las entradas de MCP gestionadas y su propiedad |
| **Adoptar** | `cortex-ia mcp adopt <name> [--dry-run]` | Acredita una entrada MCP existente del usuario que ya equivale a un preset gestionado |
| **Eliminar** | `cortex-ia mcp remove <name> [--dry-run]` | Da de baja una entrada de MCP gestionada |

`--preset`, `--local` y `--remote` son mutuamente excluyentes: se requiere exactamente uno por cada `add`.

## Informes (`cortex-ia report`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Informar error** | `cortex-ia report error --code <code> --message <msg> [--details <text\|@stdin>]` *(o `send`)* | Genera y envía un informe de error firmado |
| **Configurar informe** | `cortex-ia report config [--endpoint <url>] [--secret <key>] [--enable\|--disable]` | Configura el endpoint de informes |
| **Vaciar informes** | `cortex-ia report flush` | Reintenta informes en cola acotados |
| **Estado de informes** | `cortex-ia report status` | Muestra la configuración actual de informes |

`cortex-ia hook pre-tool --host <claude|agy>` es el guardián pre-tool del host, invocado por los hooks de Claude Code y agy y no por usuarios. Los antiguos subcomandos `delegate` y `herdr` están retirados y fallan de forma cerrada.

## Gestión de modelos (`cortex-ia model`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Listar** | `cortex-ia model list` | Lista todas las asignaciones de modelo configuradas |
| **Obtener** | `cortex-ia model get <agent>` | Muestra el modelo asignado a un agente |
| **Asignar** | `cortex-ia model set <agent> <provider/model[#variant]> [--effort <level>]` | Asigna un modelo a un agente |
| **Quitar** | `cortex-ia model unset <agent>` | Elimina la asignación de modelo de un agente |
| **Doctor** | `cortex-ia model doctor` | Verifica la salud de la configuración de modelos |
| **Catálogo** | `cortex-ia model catalog` | Lista de solo lectura de los proveedores, modelos y variantes disponibles |

## Estadísticas de uso (`cortex-ia stats`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Estadísticas** | `cortex-ia stats [--json]` | Imprime estadísticas de uso acotadas de solo lectura (`--json` para salida legible por máquina) |

## Mantenimiento y ciclo de vida (`install` / `sync` / `doctor` / `rollback` / `recover` / `uninstall` / `update`)

| Comando | Sintaxis | Propósito |
|---|---|---|
| **Instalar** | `cortex-ia install [--target <list>] [--dry-run] [--overwrite] [--theme]` | Instala activos y plugins para `opencode` (predeterminado), `claude`, `agy` o `all`; `--theme` aplica el tema cortex incluido (solo objetivo opencode) |
| **Sincronizar** | `cortex-ia sync [--target <list>] [--dry-run] [--overwrite]` | Reconcilia el home instalado con el conjunto de activos actual |
| **Doctor** | `cortex-ia doctor` | Informe de salud de la instalación de solo lectura |
| **Revertir** | `cortex-ia rollback [backup-id]` / `cortex-ia rollback list` | Restaura una copia de seguridad o lista las copias disponibles |
| **Recuperar** | `cortex-ia recover [list]` / `cortex-ia recover <journal-id>` | Lista o restaura los diarios de recuperación pendientes |
| **Desinstalar** | `cortex-ia uninstall [--target <list>] [--dry-run]` | Elimina la instalación acreditada |
| **Actualizar** | `cortex-ia update [--check] [--scheduled] [--allow-checksum-updates]` *(o `upgrade`)* | Busca / instala la última versión |
| **Programar actualización** | `cortex-ia update schedule <enable\|disable\|status>` | Gestiona la tarea diaria de comprobación de actualizaciones sin interfaz |

`--theme`, `--scheduled` y `--allow-checksum-updates` se documentan con sus tablas completas de flags en [Configuration](../getting-started/configuration.md).
