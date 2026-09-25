[English](installation.md) | **Español**

# Instalación

## Requisitos previos

- **OpenCode** — objetivo soportado para la orquestación de agentes de IA.
- **Herdr** (opcional) — integración solo para diagnóstico; su único consumidor en producción es la visualización de estado de la consola web.
- **Node.js 18+** — para el runtime de los plugins de OpenCode.
- **`cortex` en el PATH** (opcional) — para el servidor MCP del Grafo de Conocimiento de Cortex.

---

## Instalación rápida

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.ps1 | iex
```

### Linux y macOS (Bash)

```bash
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
```

Ambos instaladores hacen automáticamente lo siguiente:
1. Descargar e instalar el binario `cortex-ia` en tu directorio de binarios de usuario.
2. Añadir el directorio del binario a tu `PATH` persistente.
3. Establecer `OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS="true"` en todos los perfiles de shell.
4. Ejecutar `cortex-ia sync` para desplegar todos los MCP, plugins de OpenCode y prompts de agentes.

---

## Métodos alternativos

### Go Install

```bash
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
cortex-ia sync
```

### Homebrew (macOS / Linux)

```bash
brew install lleontor705/tap/cortex-ia
cortex-ia sync
```

### Desde el código fuente

```bash
git clone https://github.com/lleontor705/cortex-ia.git
cd cortex-ia
go build -o bin/cortex-ia.exe ./cmd/cortex-ia
.\bin\cortex-ia.exe sync
```

---

## Verificar la instalación

```bash
cortex-ia version
cortex-ia doctor
```

## Primera ejecución

```bash
cortex-ia            # Interactive Bubble Tea TUI
cortex-ia web        # Web Console (http://127.0.0.1:7331)
cortex-ia sync       # Reconcile all plugins, MCPs, and agents
```

---

## Actualización automática

`cortex-ia` puede comprobar, descargar, verificar y reemplazarse a sí mismo con una versión oficial firmada. Una versión solo avanza: las repeticiones y las degradaciones de una versión ya aplicada se rechazan antes de descargar el archivo.

### Requisito: una compilación con la clave de confianza empaquetada

Aplicar una actualización requiere una compilación que lleve la clave de confianza de la versión. Las versiones oficiales se compilan con ella; las compilaciones locales, de desarrollo y de `go install` no, y fallan de forma cerrada con exactamente:

```text
Authenticated update unavailable: no trusted release key is packaged
```

Ese mensaje es un contrato, no un defecto: no existe una ruta alternativa sin firma, por lo que una compilación sin clave no puede instalar nada. Las compilaciones locales siguen funcionando — solo la superficie de `update` permanece deshabilitada. La ceremonia de la clave (generación offline del par, empaquetado del bundle, rotación con un máximo de dos claves, procedimiento ante compromiso) está documentada en [Release Signing Keys](release-keys.md). Es un documento de plan y nunca lo ejecutan la compilación ni la CI.

### Assets de versión firmados

Cada versión publica archivos por plataforma más dos assets de verificación:

- `release-manifest.json` — lista con esquema 1 de los artefactos publicados con su tamaño exacto y su digest SHA-256. Los archivos siguen el patrón `cortex-ia_{version}_{os}_{arch}.zip` en Windows y `.tar.gz` en el resto.
- `release-manifest.sig` — sobre de firma Ed25519 sobre el manifiesto, que nombra el ID de la clave que lo firmó.

El actualizador verifica la firma y la ventana de validez de la clave de confianza antes de descargar un archivo, y luego vuelve a comprobar el digest y el tamaño del archivo contra el manifiesto firmado. Las descargas están restringidas a una lista de hosts HTTPS permitidos y limitadas a 128 MiB. El reemplazo se prepara por etapas, se verifica el digest por segunda vez y se revierte si falla cualquier paso.

### Comprobar y aplicar

```bash
cortex-ia update            # check, then download and apply if newer
cortex-ia update --check    # report availability only; never downloads or applies
cortex-ia update --help
```

---

## Ver también

- [`configuration.md`](configuration.md) — Referencia de la CLI y variables de entorno
- [`security.md`](security.md) — Firma de versiones y verificación del bundle de confianza
- [`rollback.md`](rollback.md) — Mecánica de copia de seguridad y restauración

Los resultados de la comprobación se cachean en `~/.cortex-ia/update-state.json` (esquema 1, escrito atómicamente con modo `0600`): hora de la última comprobación, la versión disponible, el suelo aplicado, la ruta del binario gestionado y los candidatos de instalación detectados. `CORTEX_IA_HOME` sobrescribe la raíz de estado. Un archivo de estado ausente o corrupto se trata como un estado vacío de primera ejecución, nunca como un bloqueo.

### Comprobación programada (solo comprobación)

```bash
cortex-ia update schedule enable    # register the daily check-only task
cortex-ia update schedule disable   # remove the managed task
cortex-ia update schedule status    # report registration and cadence
```

La tarea registrada es de nivel de usuario (sin elevación) y ejecuta exactamente `update --check --scheduled` una vez al día a las 12:00. Es estrictamente de solo comprobación: refresca el estado cacheado y sale, y nunca descarga ni reemplaza un binario, por lo que el planificador no puede abrir una ruta de aplicación desatendida. `--scheduled` solo es válido junto con `--check`.

En Windows la tarea se registra mediante `schtasks` como `CortexIA Update Check` (`/SC DAILY /ST 12:00`); en sistemas POSIX se usa una línea de `crontab` marcada. `enable` se niega a sobrescribir un nombre de tarea que no reconoce como propio, y `disable` informa de éxito cuando la tarea ya estaba ausente.

### Prompt de arranque de la TUI

Antes de que la TUI interactiva se haga cargo de la terminal, lee el estado cacheado y, cuando hay una versión más reciente pendiente, pregunta:

```text
Actualización disponible: vX.Y.Z (actual: vA.B.C).
¿Actualizar a vX.Y.Z? [y/N]
```

- `n`, EOF o ninguna versión cacheada continúa hacia la TUI y conserva el estado.
- `y` primero evalúa la compuerta de autoridad de trabajo. Si existen claims, leases o tareas `in_progress` no caducadas, la actualización se aplaza con un mensaje y la TUI arranca igualmente; la versión permanece cacheada para el siguiente arranque en reposo.
- Solo tras una compuerta abierta descarga, verifica y reemplaza el binario, y luego imprime el aviso de reinicio.

El prompt nunca bloquea el arranque: un prompt rechazado, una compuerta cerrada o cualquier fallo tipado de aplicación deja el binario en ejecución intacto. No existe una ruta de aplicación silenciosa ni desatendida — una versión solo se aplica tras un `[y/N]` interactivo en el mismo proceso. Sin estado cacheado, la TUI permanece en silencio salvo que se establezca `CORTEX_IA_UPDATE_INLINE_CHECK=1`, que habilita una comprobación inline opcional con un tiempo de espera corto que se degrada a silencio cuando la red no está disponible.

### Instalaciones duales

El actualizador solo reemplaza el binario con el que se está ejecutando. Cuando existe más de un binario `cortex-ia` entre las ubicaciones de instalación conocidas, las superficies de comprobación y programación advierten:

```text
Warning: multiple cortex-ia installations detected: <path>, <path>
```

Las ubicaciones conocidas son el destino del instalador de Windows `%LOCALAPPDATA%\Programs\cortex-ia\bin\cortex-ia.exe` y el directorio bin de Go (`GOPATH\bin\cortex-ia.exe`, o `%USERPROFILE%\go\bin\cortex-ia.exe` cuando `GOPATH` no está definido). Una copia de `go install` y una copia del instalador pueden por tanto divergir en silencio. Mantén exactamente un `cortex-ia` en tu `PATH` y elimina el otro.
