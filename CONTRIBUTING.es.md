[English](CONTRIBUTING.md) | **Español**

# Contribuir a cortex-ia

Gracias por tu interés en contribuir a **cortex-ia** — un configurador CLI/TUI de Go para **OpenCode**. Instala el conjunto de activos de flujo de trabajo embebido en `~/.config/opencode/`, gestiona las entradas MCP de Cortex y Context7, y proporciona su propio control de tareas y concesiones en SQLite mediante `cortex-ia work`.

Antes de empezar, lee esta guía completa. Tenemos un flujo de trabajo estructurado para mantener el proyecto organizado y mantenible.

---

## Tabla de contenidos

- [Flujo de trabajo con issue primero](#flujo-de-trabajo-con-issue-primero)
- [Sistema de etiquetas](#sistema-de-etiquetas)
- [Arquitectura del proyecto](#arquitectura-del-proyecto)
- [Configuración de desarrollo](#configuración-de-desarrollo)
- [Pruebas](#pruebas)
- [Convención de commits](#convención-de-commits)
- [Nomenclatura de ramas](#nomenclatura-de-ramas)
- [Reglas de pull request](#reglas-de-pull-request)
- [Código de conducta](#código-de-conducta)

---

## Flujo de trabajo con issue primero

**Sin issue no hay PR. Sin excepciones.**

Este proyecto sigue un estricto flujo de trabajo con issue primero:

1. **Abre un issue** usando la plantilla adecuada ([Informe de error](https://github.com/lleontor705/cortex-ia/issues/new?template=bug_report.yml) o [Solicitud de función](https://github.com/lleontor705/cortex-ia/issues/new?template=feature_request.yml))
2. **Espera la aprobación** — un mantenedor añadirá la etiqueta `status:approved` cuando el issue esté listo para trabajarse
3. **Comenta en el issue** para que los demás sepan que estás trabajando en él
4. **Abre un PR** referenciando el issue aprobado

Los PR que no estén vinculados a un issue aprobado serán **rechazados automáticamente** por CI.

---

## Sistema de etiquetas

### Etiquetas de tipo (aplicadas a los PR)

| Etiqueta | Descripción |
|-------|-------------|
| `type:bug` | Corrección de errores |
| `type:feature` | Nueva función o mejora |
| `type:refactor` | Refactorización de código, sin cambios funcionales |
| `type:docs` | Solo documentación |
| `type:test` | Adiciones de cobertura de pruebas |
| `type:chore` | Cambios de build, CI o herramientas |
| `type:breaking` | Cambio incompatible |

### Etiquetas de estado (aplicadas a los issues)

| Etiqueta | Descripción |
|-------|-------------|
| `status:needs-review` | Recién abierto, a la espera de revisión del mantenedor |
| `status:approved` | Aprobado para implementación — el trabajo puede comenzar |
| `status:in-progress` | En proceso de trabajo |
| `status:blocked` | Bloqueado por otro issue o dependencia externa |
| `status:wont-fix` | Fuera del alcance o no se abordará |

### Etiquetas de prioridad

| Etiqueta | Descripción |
|-------|-------------|
| `priority:critical` | Issues bloqueantes, vulnerabilidades de seguridad |
| `priority:high` | Importante, afecta a muchos usuarios |
| `priority:medium` | Prioridad normal |
| `priority:low` | Deseable pero no imprescindible |

---

## Arquitectura del proyecto

cortex-ia instala un único conjunto de activos embebido en la raíz de configuración de OpenCode, de forma transaccional: primero planifica, captura una copia de seguridad verificada, aplica, y restaura desde la copia si la fase de aplicación falla. Familiarízate con estos conceptos antes de abrir un PR no trivial.

### Agentes soportados

`opencode` — el producto configura OpenCode únicamente y nunca escribe en ningún otro lugar. El diseño nativo de OpenCode y el mapeo puro de activos residen en `internal/agents/opencode/`.

### Paquetes clave

- **`internal/install`** — fachada de servicio: `install`, `sync`, `doctor`, `rollback`, `uninstall` y todas las operaciones MCP.
- **`internal/pipeline`** — planifica y aplica la copia de activos de forma transaccional.
- **`internal/backup`** — instantáneas, manifiestos, verificación, deduplicación y poda por retención.
- **`internal/mcpmanager`** — catálogo gestionado de MCP (`cortex`, `context7`), eliminación del legado ForgeSpec, validación de entradas deseadas y errores de conflicto.
- **`internal/delegation`** — almacén SQLite WAL para tareas/reclamos/concesiones de control de trabajo y trabajos de delegación externos.
- **`internal/components/filemerge`** — decodificación/fusión de JSONC y escrituras atómicas.
- **`internal/state` / `internal/installmeta`** — metadatos de instalación, bloqueo y digests de MCP en `~/.cortex-ia/`.
- **`internal/tui`** — interfaz de terminal Bubble Tea (se lanza sin argumentos).

### Activos embebidos

Las skills, agentes, comandos y el plugin bajo `internal/assets/` se embeben mediante `go:embed` y se copian byte a byte en el momento de la instalación (14 agentes, 9 comandos, 21 skills, 1 plugin). Cambiarlos requiere recompilar el binario.

### Comprobación de salud (`cortex-ia doctor`)

Un informe estrictamente de solo lectura: presencia y acuerdo de estado/bloqueo, comprobaciones de digests por artefacto, propiedad de MCP por preset y un veredicto general. Doctor nunca muta nada.

---

## Configuración de desarrollo

### Requisitos previos

- Go 1.26.1+ (`go.mod` es la fuente autoritativa)
- Git

### Clonar y compilar

```bash
git clone https://github.com/lleontor705/cortex-ia.git
cd cortex-ia
go build -o cortex-ia ./cmd/cortex-ia
```

### Ejecutar localmente

```bash
./cortex-ia            # interactive TUI
./cortex-ia --help     # CLI reference
./cortex-ia doctor     # read-only health report
./cortex-ia install --dry-run
```

---

## Pruebas

### Compuertas locales completas

Ejecuta el conjunto completo de compuertas en el orden de los hooks antes de abrir un PR:

```bash
gofmt -s -w .
go vet ./...
golangci-lint run ./...
go test -count=1 ./...
```

### Alcance de las pruebas

Las pruebas persistentes cubren exactamente la TUI (`internal/tui/...`) y el comportamiento de instalación del pipeline (`internal/pipeline/install_test.go`). No añadas suites de pruebas en otros lugares sin el acuerdo de un mantenedor; los oráculos transaccionales más profundos se ejecutan como smokes efímeros y se eliminan tras su ejecución.

Enfoca un paquete o una única prueba:

```bash
go test ./internal/tui/...
go test ./internal/pipeline -run '^TestInstall_DryRun$' -count=1
```

Las pruebas usan directorios home temporales. Nunca apuntes las pruebas a tu configuración real de OpenCode.

---

## Convención de commits

Este proyecto usa [Conventional Commits](https://www.conventionalcommits.org/).

Los mensajes de commit **deben** coincidir con este patrón:

```
^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([a-z0-9\._-]+\))?!?: .+
```

### Formato

```
<type>(<optional-scope>)!: <description>

[optional body]

[optional footer]
```

### Tipos permitidos

| Tipo | Propósito |
|------|---------|
| `feat` | Nueva función |
| `fix` | Corrección de errores |
| `docs` | Solo documentación |
| `refactor` | Cambio de código (sin cambio de comportamiento) |
| `chore` | Mantenimiento, dependencias, herramientas |
| `style` | Formato, linting (sin cambio de lógica) |
| `perf` | Mejora de rendimiento |
| `test` | Añadir o actualizar pruebas |
| `build` | Sistema de build o dependencias externas |
| `ci` | Configuración de CI |
| `revert` | Revierte un commit anterior |

### Ejemplos

```
feat(tui): add MCP preset picker to install review
fix(pipeline): rollback on apply failure preserves prior state
docs: document managed MCP presets
chore(deps): bump bubbletea to v1.4
refactor(mcpmanager): extract desired-entry validation
style: gofmt internal/agents/opencode
perf(doctor): reuse tolerant state load in one report
test(backup): cover checksum dedup path
build: pin goreleaser build image
ci: add check-branch-name to pr-check workflow
revert: undo custom MCP header support
```

### Cambios incompatibles

Añade `!` después del tipo/alcance e incluye un pie `BREAKING CHANGE:`:

```
feat(cli)!: require --yes for uninstall

BREAKING CHANGE: `cortex-ia uninstall` now requires an explicit `--yes`
confirmation. Update your scripts and aliases accordingly.
```

Los cambios incompatibles se asignan a la etiqueta `type:breaking`.

---

## Nomenclatura de ramas

Los nombres de las ramas **deben** coincidir con este patrón:

```
^(feat|fix|chore|docs|style|refactor|perf|test|build|ci|revert)\/[a-z0-9._-]+$
```

**Reglas:**
- Todo en minúsculas
- Usa guiones, puntos o guiones bajos como separadores (sin espacios, sin mayúsculas)
- La descripción debe ser corta y descriptiva

**Ejemplos:** `feat/mcp-preset-import`, `fix/backup-dedup-windows`, `docs/cortex-memory-tools`, `ci/pin-go-version`

---

## Reglas de pull request

### Antes de abrir un PR

- [ ] Hay un issue aprobado vinculado (`Closes #<N>`)
- [ ] Todas las compuertas pasan (`gofmt -s -w .`, `go vet ./...`, `golangci-lint run ./...`, `go test -count=1 ./...`)
- [ ] Los commits siguen el formato de Conventional Commits
- [ ] El código ha sido auto-revisado

### Título del PR

Usa el mismo formato de Conventional Commits que en los mensajes de commit:

```
feat(tui): add MCP preset picker to install review
fix(pipeline): handle empty selection gracefully
```

### Comprobaciones automatizadas de PR

Todos los PR pasan por comprobaciones automatizadas:

| Comprobación | Qué verifica |
|-------|-----------------|
| **Check Issue Reference** | El cuerpo del PR contiene `Closes/Fixes/Resolves #N` |
| **Check Issue Has status:approved** | El issue vinculado ha sido aprobado por un mantenedor |
| **Check PR Has type:* Label** | Se aplica exactamente una etiqueta `type:*` |
| **Check Branch Name Convention** | La rama coincide con `<type>/<lowercase-name>` |

Además, el flujo de trabajo de **CI** ejecuta escaneos de calidad y seguridad de Go sobre la rama del PR.

**Todas las comprobaciones deben pasar** antes de que un PR pueda fusionarse.

### Vincular tu issue

En el cuerpo del PR, incluye uno de:

```
Closes #42
Fixes #42
Resolves #42
```

---

## Código de conducta

Sé respetuoso. Estamos construyendo algo juntos.

- Critica el código, no a las personas
- Sé constructivo en las revisiones
- Da la bienvenida a quienes llegan nuevos

Las infracciones pueden dar lugar a la expulsión del proyecto.

---

## ¿Preguntas?

Usa [GitHub Discussions](https://github.com/lleontor705/cortex-ia/discussions) — no los issues — para preguntas, ideas y conversación general.
