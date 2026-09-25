[English](README.md) | **Español**

<p align="center">
  <img src="docs/assets/hero-banner.svg" alt="Cortex-IA Hero Banner" width="100%" />
</p>

<p align="center">
  <a href="https://github.com/lleontor705/cortex-ia/releases/latest"><img src="https://img.shields.io/github/v/release/lleontor705/cortex-ia?color=38BDF8&label=release" alt="Release"></a>
  <a href="https://github.com/lleontor705/cortex-ia/blob/main/LICENSE"><img src="https://img.shields.io/github/license/lleontor705/cortex-ia?color=A855F7" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/lleontor705/cortex-ia"><img src="https://goreportcard.com/badge/github.com/lleontor705/cortex-ia" alt="Go Report Card"></a>
  <a href="https://github.com/lleontor705/cortex-ia/actions"><img src="https://img.shields.io/badge/tests-100%25%20passing-10B981" alt="Tests"></a>
  <a href="https://github.com/lleontor705/cortex-ia"><img src="https://img.shields.io/badge/platforms-Windows%20%7C%20Linux%20%7C%20macOS-blue" alt="Platforms"></a>
</p>

---

## ⚡ ¿Qué es Cortex-IA?

**Cortex-IA** es el **Plano de Control Multiagente y Motor de Orquestación** determinista, de nivel empresarial, diseñado para el desarrollo de software autónomo con **OpenCode**.

Construido como un único binario portable de Go, Cortex-IA resuelve los desafíos fundamentales de la codificación multiagente: **condiciones de carrera**, **ediciones de archivos en conflicto**, **disponibilidad de tareas alucinada**, **tareas en segundo plano sin supervisión** y **coordinación no estructurada**.

```text
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                   CORTEX-IA ECOSYSTEM                                       │
│                                                                                             │
│  ┌──────────────────────────┐   ┌─────────────────────────────┐   ┌─────────────────────────┐  │
│  │     OpenCode Agents      │   │   CORTEX-IA Control Plane   │   │  Cortex-IA Web Console  │  │
│  │ (Orchestrator, Discovery,│──▶│  (SQLite ACID DAG, Leases,  │──▶│  (Loopback SSE Kanban,  │  │
│  │  Investigate, Planner,   │   │   CAS Revisions, OpenSpec)  │   │   Audit Log & Intake)   │  │
│  │   Implement, Reviewer)   │   │                             │   │                         │  │
│  └──────────────────────────┘   └─────────────────────────────┘   └─────────────────────────┘  │
│                                             │                                                │
│                                             ▼                                                │
│                               ┌───────────────────────────┐                                  │
│                               │     CORTEX Server (MCP)   │                                  │
│                               │  (AST Graph & Blast Tree) │                                  │
│                               │  (Epistemic Evidence DB)  │                                  │
│                               └───────────────────────────┘                                  │
└─────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 🌟 Superpoderes clave

- 🔒 **Concurrencia sin carreras con concesiones de archivo exclusivas (`work lease`)**  
  Evita que los agentes sobrescriban el código de los demás. Los agentes deben reservar de forma atómica rutas de archivo exclusivas relativas al espacio de trabajo mediante concesiones con TTL antes de editar. Los implementadores nativos en paralelo comparten el espacio de trabajo de forma segura mediante reservas de archivo disjuntas (`cortex_ia_file_reserve`).
- 🎯 **DAG de tareas determinista con bloqueo optimista por CAS (`work claim` / `transition`)**  
  Las tareas transicionan a través de máquinas de estado estrictas (`backlog ➔ ready ➔ in_progress ➔ in_review ➔ done`). Las dependencias posteriores se desbloquean automáticamente solo cuando las dependencias previas reciben una aprobación de revisión independiente.
- 🛡️ **Barreras de revisión independiente obligatorias (`work approve`)**  
  Los implementadores no pueden autoaprobarse. Un agente revisor independiente debe verificar las suites de pruebas y la evidencia registrada antes de marcar cualquier tarea como completada.
- 🧹 **Higiene de chat sin JSON en crudo y autoridad por herramientas tipadas**  
  Elimina la inflación de tokens y el análisis de texto alucinado. Los recibos estructurados se pasan directamente mediante llamadas a herramientas tipadas (`cortex_ia_work_transition` y `cortex_ia_work_approve`) almacenadas atómicamente en SQLite, mientras que el chat muestra resúmenes Markdown limpios y legibles.
- 📐 **Integración nativa de OpenSpec SDD (`cortex-ia openspec`)**  
  Soporte integrado para propuestas de Desarrollo Guiado por Especificaciones, especificaciones delta RFC 2119 y descomposiciones de tareas acotadas a ≤350 LOC.
- 📊 **Panel de operaciones web en tiempo real (`cortex-ia web`)**  
  Interfaz web embebida en un único binario con transmisión SSE en tiempo real para la visualización del estado del tablero en vivo, la creación de tareas y el registro de auditoría.

---

## 🧭 CORTEX (MCP) vs CORTEX-IA (CLI)

<p align="center">
  <img src="docs/assets/cortex-vs-cortexia.svg" alt="Cortex vs Cortex-IA" width="100%" />
</p>

| Dimensión | 🧠 **CORTEX** (Servidor MCP) | ⚙️ **CORTEX-IA** (Plano de control y CLI) |
|---|---|---|
| **Naturaleza** | Servidor MCP estandarizado (32 herramientas: `cortex_*`) | Binario Go nativo independiente (`cortex-ia.exe`) |
| **Plano del sistema** | **Plano epistémico y de evidencia** | **Plano de control operativo** |
| **Almacenamiento** | Grafo de conocimiento y base de datos de símbolos AST | SQLite transaccional ACID (`~/.cortex-ia/delegation.db`) |
| **Enfoque principal** | • Símbolos de código AST y grafos de llamadas<br>• Análisis de impacto del radio de explosión<br>• Gotchas de errores duraderos y memorias ADR<br>• Contexto de proyecto entre sesiones | • Máquinas de estado del DAG de tareas y revisiones CAS<br>• Tokens de reclamo atómicos y concesiones de archivo exclusivas<br>• Controladores de rol nativos y recibos de herramientas tipadas<br>• Validador OpenSpec SDD y panel web |
| **Regla de autoridad** | **Solo informativo y consultivo.** Las observaciones almacenadas nunca autorizan escrituras de código ni marcan tareas como completadas. | **Única fuente de verdad.** La disponibilidad de tareas, las concesiones, las transiciones y las aprobaciones existen estrictamente en SQLite a través de `cortex-ia work`. |

---

## 🚀 Inicio rápido

### 1. Configuración interactiva (TUI)
Inicia la elegante interfaz de terminal BubbleTea para configurar tu entorno de OpenCode:
```bash
cortex-ia
```

### 2. Instalación rápida no interactiva
```bash
cortex-ia install         # Installs agents, skills, plugins & registers Cortex MCP
cortex-ia sync            # Converges installed home with embedded assets
cortex-ia doctor          # Verifies health, environment paths & tool dependencies
```

### 3. Lanza el panel web en tiempo real
```bash
cortex-ia web --open      # Launches the local dashboard at http://127.0.0.1:7331
```

---

## 📦 Instalación

### Binario precompilado (recomendado)
Descarga el último binario precompilado desde la página de [Releases](https://github.com/lleontor705/cortex-ia/releases) para Windows, macOS o Linux.

### Go Install
```bash
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
```

### Script de instalación (Linux / macOS)
```bash
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
```

### Compilar desde el código fuente
```bash
git clone https://github.com/lleontor705/cortex-ia.git
cd cortex-ia
go build -o bin/cortex-ia ./cmd/cortex-ia
```

---

## 💻 Superficie de comandos de la CLI

Los comandos que emiten recibos legibles por máquina imprimen JSON en stdout; los comandos de diagnóstico humano (`doctor`, `rollback`, `recover`, `report status`, `help`, `update`) imprimen texto plano. Las consultas de estado aceptan los alias `show`/`get` donde se indica, y cada grupo de subcomandos imprime su uso con `--help`.

### 1. Tableros de tareas (`cortex-ia board`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Crear** | `cortex-ia board create <id> "<title>" "[desc]"` | Inicializa un límite de tablero de tareas duradero |
| **Listar** | `cortex-ia board list` | Lista todos los tableros con contadores de completadas/total |
| **Estado** | `cortex-ia board status <id>` *(o `show`, `get`)* | Consulta los metadatos del tablero y la instantánea completa del DAG de tareas |
| **Archivar** | `cortex-ia board archive <id>` | Marca un tablero completado como archivado |
| **Desarchivar** | `cortex-ia board unarchive <id>` | Restaura un tablero archivado a activo |
| **Eliminar** | `cortex-ia board delete <id>` | Elimina permanentemente un tablero archivado y sus tareas |
| **Servir** | `cortex-ia board serve [--addr 127.0.0.1:7331]` | Ejecuta el panel web loopback embebido |

### 2. Elementos de trabajo y concesiones (`cortex-ia work`)
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
| **Verificar concesión** | `cortex-ia work verify-lease --path <file> [--task <id>] [--owner <owner>]` *(o `check-lease`)* | Verifica una concesión de archivo activa |

### 3. Espacio de trabajo OpenSpec SDD (`cortex-ia openspec`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Validar** | `cortex-ia openspec validate <change> --workflow <sdd-lite\|sdd-full> --phase <phase> [--json]` | Valida estructuralmente los artefactos de planificación. `--workflow` y `--phase` son **obligatorios** |
| **Listar** | `cortex-ia openspec list` | Lista las propuestas de cambio activas en `openspec/changes/` |
| **Estado** | `cortex-ia openspec status [change-name]` | Inspecciona el progreso de las tareas y el estado de los cambios |
| **Archivar** | `cortex-ia openspec archive <change-name> --board <id> --workflow <sdd-lite\|sdd-full> --spec-plane <openspec\|cortex\|hybrid>` | Cierra trabajo SDD aprobado de forma independiente |
| **Nuevo** | `cortex-ia openspec new <change-name> [domain]` | Genera la estructura de un nuevo directorio de cambio OpenSpec |

### 4. Instantáneas de Cortex (`cortex-ia snapshot`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Leer** | `cortex-ia snapshot read --project <project> --id <id> [--expected-sha256 <digest>]` | Lee y verifica una observación local acotada de Cortex |

### 5. Worktrees de Git (`cortex-ia worktree`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Listar** | `cortex-ia worktree list [--repo <repo-path>]` | Lista los worktrees de Git autoritativos |
| **Validar** | `cortex-ia worktree validate <worktree-path> [--repo <repo-path>] [--head <commit>]` | Valida un contrato de worktree contra git porcelain |

`current_workspace` es la única estrategia de ejecución soportada. `worktree create`, `clean`, `drop`, `delete`, `remove` y `prune` están retirados y fallan de forma cerrada; los worktrees existentes se conservan.

### 6. Doble libro mayor (`cortex-ia ledger`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Añadir hecho** | `cortex-ia ledger fact add <text> [--board <id>] [--source <src>] [--sync-cortex]` | Registra un hecho verificado (opcionalmente sincronizado con la memoria de Cortex) |
| **Listar hechos** | `cortex-ia ledger fact list [--board <board-id>] [--json]` | Lista los hechos verificados en orden cronológico |
| **Progreso** | `cortex-ia ledger progress record --summary <text> [--drift] [--action <act>]` | Registra una evaluación de progreso del orquestador |
| **Estado** | `cortex-ia ledger status [--board <board-id>] [--json]` | Muestra el informe completo del doble libro mayor (hechos + progreso) |

### 7. Instantánea de la UI (`cortex-ia ui`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Instantánea** | `cortex-ia ui snapshot [--project <path>] [--session-id <id>] [--root-session-id <id>]` | Imprime una instantánea acotada de solo lectura de la TUI |

### 8. Documentos y diagramas (`cortex-ia doc` / `cortex-ia diagram`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Convertir documento** | `cortex-ia doc convert <file> [-o <out.md>] [--standalone] [--format <fmt>] [--max-lines <n>] [--ocr <hosted\|reject>] [--json]` | Convierte documentos de oficina/PDF a Markdown |
| **Inspeccionar documento** | `cortex-ia doc inspect <file> [--json]` | Inspecciona los metadatos del documento |
| **Validar diagrama** | `cortex-ia diagram validate <type> <spec.json> [--quality <standard\|showcase>] [--json]` | Valida la topología del diagrama |
| **Renderizar diagrama** | `cortex-ia diagram render <type> <spec.json> [output.html] [--quality <standard\|showcase>] [--json]` | Renderiza un archivo HTML de diagrama interactivo |
| **Comparar diagrama** | `cortex-ia diagram compare <base.json> <head.json> [output.html] [--json]` | Compara dos instantáneas de arquitectura |
| **Alcance del diagrama** | `cortex-ia diagram reach <type> <spec.json> --from <node-id> [--direction <upstream\|downstream\|both>] [--json]` | Rastrea la alcanzabilidad del grafo desde un nodo |

### 9. Gestión de MCP (`cortex-ia mcp`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Añadir (preset)** | `cortex-ia mcp add <name> --preset [--dry-run]` | Registra un preset de MCP del catálogo gestionado |
| **Añadir (local)** | `cortex-ia mcp add <name> --local [--env KEY=VALUE]... -- <command> [args...]` | Registra un servidor MCP local personalizado gestionado |
| **Añadir (remoto)** | `cortex-ia mcp add <name> --remote <url> [--header KEY=VALUE]... [--dry-run]` | Registra un servidor MCP remoto personalizado gestionado |
| **Listar** | `cortex-ia mcp list [--json]` | Lista las entradas de MCP gestionadas y su propiedad |
| **Eliminar** | `cortex-ia mcp remove <name> [--dry-run]` | Da de baja una entrada de MCP gestionada |

`--preset`, `--local` y `--remote` son mutuamente excluyentes: se requiere exactamente uno por cada `add`.

### 10. Informes (`cortex-ia report`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Informar error** | `cortex-ia report error --code <code> --message <msg> [--details <text\|@stdin>]` *(o `send`)* | Genera y envía un informe de error firmado |
| **Configurar informe** | `cortex-ia report config [--endpoint <url>] [--secret <key>] [--enable\|--disable]` | Configura el endpoint de informes |
| **Vaciar informes** | `cortex-ia report flush` | Reintenta informes en cola acotados |
| **Estado de informes** | `cortex-ia report status` | Muestra la configuración actual de informes |

El antiguo subcomando `cortex-ia hook` está retirado y falla de forma cerrada con un error de superficie retirada.

### 11. Gestión de modelos (`cortex-ia model`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Listar** | `cortex-ia model list` | Lista todas las asignaciones de modelo configuradas |
| **Obtener** | `cortex-ia model get <agent>` | Muestra el modelo asignado a un agente |
| **Asignar** | `cortex-ia model set <agent> <provider/model[#variant]> [--effort <level>]` | Asigna un modelo a un agente |
| **Quitar** | `cortex-ia model unset <agent>` | Elimina la asignación de modelo de un agente |
| **Doctor** | `cortex-ia model doctor` | Verifica la salud de la configuración de modelos |
| **Catálogo** | `cortex-ia model catalog` | Lista de solo lectura de los proveedores, modelos y variantes disponibles |

### 12. Estadísticas de uso (`cortex-ia stats`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Instantánea** | `cortex-ia stats snapshot` | Imprime una instantánea acotada de solo lectura de estadísticas |

### 13. Mantenimiento y ciclo de vida (`install` / `sync` / `doctor` / `rollback` / `recover` / `uninstall` / `update`)
| Comando | Sintaxis | Propósito |
|---|---|---|
| **Instalar** | `cortex-ia install [--target <list>] [--dry-run] [--overwrite]` | Instala activos y plugins (objetivo predeterminado: `opencode`) |
| **Sincronizar** | `cortex-ia sync [--target <list>] [--dry-run] [--overwrite]` | Reconcilia el home instalado con el conjunto de activos actual |
| **Doctor** | `cortex-ia doctor` | Informe de salud de la instalación de solo lectura |
| **Revertir** | `cortex-ia rollback [backup-id]` / `cortex-ia rollback list` | Restaura una copia de seguridad o lista las copias disponibles |
| **Recuperar** | `cortex-ia recover [list]` / `cortex-ia recover <journal-id>` | Lista o restaura los diarios de recuperación pendientes |
| **Desinstalar** | `cortex-ia uninstall [--target <list>] [--dry-run]` | Elimina la instalación acreditada |
| **Actualizar** | `cortex-ia update [--check]` *(o `upgrade`)* | Busca / instala la última versión |

---

## 🤖 Topología de coordinación multiagente

<p align="center">
  <img src="docs/assets/multi-agent-orchestration.svg" alt="Multi-Agent Orchestration" width="100%" />
</p>

1. **`orchestrator` (Primario)**: Triaje, alineación inicial, ciclo de vida de la sesión de Cortex y despacho del DAG. Nunca reclama tareas ni mantiene concesiones de archivo.
2. **`discovery` (Subagente)**: Inspecciona skills, cadenas de herramientas, motores y arquitectura del proyecto en el perfil duradero `.cortex-ia/discovery.md`.
3. **`investigate` (Subagente)**: Diagnóstico de causa raíz, inspección del radio de explosión de AST, spikes y auditorías de diagnóstico de solo lectura.
4. **`planner` (Subagente)**: Escribe especificaciones delta de OpenSpec (RFC 2119), contratos Dado/Cuando/Entonces y descompone DAG de tareas (≤350 LOC).
5. **`implement` (Subagente)**: Reclama atómicamente una tarea, reserva concesiones de archivo exclusivas, ejecuta bucles de TDD rápidos y transiciona a revisión mediante herramientas tipadas.
6. **`reviewer` (Subagente)**: Verifica de forma independiente los diffs de git, ejecuta oráculos de prueba y otorga la aprobación `PASS` para desbloquear las dependencias posteriores.

---

## 🚦 Modelo de enrutamiento orgánico de 3 niveles

Cortex-IA adapta las solicitudes del usuario al flujo de trabajo más pequeño y seguro mediante un modelo de tres niveles:

| Nivel | Flujos de trabajo | Características | Modelo de ejecución |
|---|---|---|---|
| **Nivel 1: Ruta rápida** | `direct-answer`, `discovery`, `investigate`, `spike`, `hotfix`, `fast-tdd`, `ops-task` | Ejecución directa sin la sobrecarga del DAG de tareas. Especializado para preguntas y respuestas, onboarding, diagnóstico de causa raíz o TDD de unidades rápidas. | Despacho de un solo turno mediante `orchestrator ➔ subagent ➔ orchestrator`. |
| **Nivel 2: Tarea unitaria acotada** | `direct-change` | Cambios de dominio único y bajo riesgo con verificación rápida. Usa `board_id: "default"`. | Reclamar tarea ➔ concesión de archivo exclusiva ➔ editar y probar ➔ `cortex_ia_work_transition` ➔ barrera de revisión independiente. |
| **Nivel 3: SDD coordinado** | `sdd-lite`, `sdd-full` | Funciones de alta complejidad y múltiples archivos, o cambios arquitectónicos entre dominios. | Tablero de iniciativa estable ➔ especificaciones delta de OpenSpec ➔ descomposición de DAG (≤350 LOC) ➔ minions de implementación en paralelo ➔ revisión adversarial. |

---

## 🛡️ Garantías de seguridad transaccional

- **Determinismo de dry-run**: `--dry-run` calcula el plan de ejecución exacto sin realizar ninguna escritura en disco.
- **Bloqueo de archivos entre procesos**: Cada comando que muta el estado mantiene un bloqueo de archivo robusto entre procesos (`LockFileEx` en Windows, `flock` en Unix) que evita carreras concurrentes del instalador.
- **Copias de seguridad y reversiones verificadas**: Toma instantáneas de los archivos de configuración afectados en `~/.cortex-ia/backups/` y revierte automáticamente si una fase de aplicación encuentra un error.
- **Aislamiento estricto de rutas**: Las concesiones y las operaciones del espacio de trabajo rechazan el recorrido de directorios (`..`) y los intentos de escape por rutas absolutas.

---

## 📚 Referencia de documentación

### Primeros pasos
- 📖 [Guía de inicio rápido](docs/quickstart.md) — Configuración y onboarding guiados por primera vez
- ⚙️ [Instalación](docs/installation.md) — Métodos de instalación del binario, Homebrew y actualización automática
- 🔧 [Configuración](docs/configuration.md) — Referencia de la CLI, variables de entorno y diseño del estado
- 💻 [Modo no interactivo](docs/non-interactive.md) — Recetas de scripting, CI y Docker

### Arquitectura y diseño
- 🏛️ [Análisis profundo de la arquitectura](docs/architecture.md) — Capas internas del motor, modelos y concurrencia de SQLite
- 🧠 [Memoria y grafo de Cortex](docs/cortex-memory.md) — Grafo de símbolos AST, radio de explosión y observaciones duraderas
- 🤖 [Roles y contratos de los agentes](docs/agents.md) — Topología de coordinación de 6 roles y contratos de recibos tipados
- 🧩 [Componentes y MCP](docs/components.md) — Activos desplegados, presets de MCP y servidores personalizados
- 📑 [Guía del flujo de trabajo SDD](docs/sdd-workflow.md) — Ciclo de vida del Desarrollo Guiado por Especificaciones con OpenSpec

### Operaciones y seguridad
- 🔒 [Seguridad y recuperación](docs/security.md) — Garantías de seguridad, copias de seguridad, reversión y recuperación
- 🔄 [Copias de seguridad y reversión](docs/rollback.md) — Ciclo de vida de las copias, retención y reversión explícita
- 🖥️ [Plataformas soportadas](docs/platforms.md) — Soporte de SO, rutas y requisitos de terminal
- 🐳 [Pruebas E2E con Docker](docs/docker-e2e-testing.md) — Suite de pruebas basada en contenedores

### MCP e integración
- 🔌 [Gestor de MCP](docs/mcp.md) — Presets del catálogo, servidores personalizados y propiedad
- 🔑 [Claves de firma de releases](docs/release-keys.md) — Formato del paquete de confianza y ceremonia de claves

### Cualificación y CI
- 📋 [Entradas de CI y distribución](docs/qualification-inputs.md) — Procedencia del flujo de trabajo y línea base de la cadena de herramientas
- 🧪 [Cualificación del SDK y plugins](docs/sdk-qualification.md) — Bloqueo y aislamiento del SDK del arnés
- 🔗 [Cualificación de integración MCP](docs/mcp-qualification.md) — Evidencia de Context7 y Cortex MCP

### Guía para desarrolladores
- 🗺️ [Mapa del repositorio](docs/codebase/repository-map.md) — Diseño del código directorio por directorio
- 📇 [Mapa de referencia](docs/codebase/reference-map.md) — Índice de comandos de la CLI, paquetes Go y tipos
- 🧠 [Modelo mental](docs/codebase/mental-model.md) — Explicación del flujo de extremo a extremo
- 🔌 [Límites de MCP](docs/codebase/mcp-boundaries.md) — Separación entre autoridad epistémica y operativa
- 📊 [Panel y TUI](docs/codebase/dashboard.md) — Arquitectura de BubbleTea y estados de pantalla
- 🔗 [Integraciones y CI/CD](docs/codebase/integrations.md) — Pipeline de release y flujos de trabajo
- 📝 [Manual del mantenedor](docs/codebase/maintainer-playbook.md) — Runbook de release y mantenimiento de dependencias
- 🔄 [Sincronización, estado y copias de seguridad](docs/codebase/sync-and-cloud.md) — Persistencia y reconciliación del estado local
- 🧩 [Interfaces y contratos](docs/codebase/interfaces.md) — Referencia de las interfaces core de Go
- 📘 [Guía del proyecto y extensiones](docs/codebase/project-and-extension.md) — Añadir skills, agentes, comandos y plugins
- 🔀 [Coordinación SDD](docs/codebase/sdd-coordination.md) — Ciclo de vida del trabajo y reglas de autoridad

### Monitorización
- 📊 [Instantánea del Report Hub](docs/cortex-report-hub-snapshot.md) — Monitorización del servicio Railway

---

## 📄 Licencia

Licencia MIT · Creado con ❤️ por [Luis Leon](https://github.com/lleontor705) y colaboradores.
