[English](quickstart.md) | **Español**

# Guía de inicio rápido

Pon en marcha **Cortex-IA** en tres pasos sencillos.

---

## 1. Instalar el binario

```bash
# Via Go (recommended)
go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest

# Or via install script (Linux / macOS)
curl -sSL https://raw.githubusercontent.com/lleontor705/cortex-ia/main/scripts/install.sh | bash
```

---

## 2. Inicializar tu entorno

### Opción A: Asistente interactivo (TUI)
Inicia la interfaz de terminal enriquecida:
```bash
cortex-ia
```

### Opción B: Instalación automatizada
```bash
cortex-ia install
```

Esto instala de forma transaccional el conjunto de assets embebidos bajo `~/.config/opencode/`:
- El prompt de sistema `AGENTS.md` y los 6 roles nativos (`orchestrator`, `discovery`, `investigate`, `planner`, `implement`, `reviewer`).
- Comandos de barra SDD, skills y plugins de OpenCode (el puente de delegación Herdr retirado ya no se instala).
- Registra el servidor MCP del grafo de conocimiento **Cortex**.

---

## 3. Verificar el estado y lanzar el panel web

```bash
# 1. Check system diagnostics & path wiring
cortex-ia doctor

# 2. Launch the real-time operations web console
cortex-ia web --open
```

---

## 4. Explorar los flujos de trabajo principales

### A. Gestionar tableros de tareas y slices
```bash
# Create an initiative board
cortex-ia board create my-feature "Authentication & JWT"

# Add tasks with dependency chaining
cortex-ia work create auth-1.1 "Scaffold JWT token handler" --board my-feature
cortex-ia work create auth-1.2 "Add refresh token rotation" --board my-feature --depends auth-1.1
```

### B. Validar propuestas de OpenSpec
```bash
openspec validate
```

### C. Observar el estado del trabajo de forma nativa
Cada controlador de rol se ejecuta de forma nativa dentro de OpenCode bajo la autoridad de trabajo de Cortex-IA; la delegación externa está retirada. Observa los tableros, claims, leases y transiciones de revisión en la consola de operaciones local:

```bash
cortex-ia web --open
```

---

## Ver también

- [`installation.md`](installation.md) — Métodos de instalación alternativos
- [`configuration.md`](configuration.md) — Referencia completa de la CLI y variables de entorno
- [`architecture.md`](architecture.md) — Diseño interno del motor
- [`agents.md`](agents.md) — Topología de roles y contratos de coordinación
