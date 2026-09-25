[English](non-interactive.md) | **Español**

# Modo no interactivo (solo CLI)

cortex-ia está pensado primero para la TUI, pero cada operación también tiene su conjunto de flags de CLI, de modo que puedes gobernar todo el pipeline desde scripts, CI y Dockerfiles sin la interfaz de Bubbletea.

## Recetas

### Instalar con un destino explícito

```bash
cortex-ia install --target opencode,claude
```

`--target` acepta una lista separada por comas de `opencode`, `claude` o `all`. Por defecto es `opencode`.

### Ejecución en seco (previsualizar sin tocar el disco)

```bash
cortex-ia install --dry-run --target opencode
```

Devuelve el código de salida 0 e imprime el plan. No se escribe ningún archivo ni se crea ninguna copia de seguridad.

### Sync (volver a ejecutar los inyectores sin cambiar la selección)

```bash
cortex-ia sync
```

Útil en CI: haz pull de main → `cortex-ia sync` para recoger nuevos skills SDD o actualizaciones de convenciones.

### Comprobación de estado

```bash
cortex-ia doctor
```

`doctor` es una evaluación de solo lectura del home instalado (presencia de artefactos y drift, CLIs de programación detectadas, entradas MCP gestionadas) y sale con un código distinto de cero cuando el veredicto es degradado o bloqueado. Vuelve a ejecutar `cortex-ia sync` para reconciliar el home instalado con el conjunto de assets actual.

### Rollback

```bash
cortex-ia rollback list             # list available backups
cortex-ia rollback                  # most recent backup
cortex-ia rollback 20260425-093015  # specific snapshot
```

Un rollback real requiere una terminal interactiva y confirmación explícita; una entrada redirigida o cerrada falla de forma cerrada sin escribir.

### Inspección de solo lectura para scripting

```bash
cortex-ia rollback list    # backups with id and label (plain text)
cortex-ia recover list     # pending recovery journals (plain text)
```

Los comandos que emiten recibos legibles por máquina imprimen JSON; usa `--json` donde un subcomando lo documente (por ejemplo `cortex-ia mcp list --json`).

### Comprobación de actualizaciones

```bash
cortex-ia update --check    # checks GitHub Releases; prints current vs latest
cortex-ia update            # downloads and applies the latest release
```

## Códigos de salida

| Código | Significado |
|---|---|
| `0` | Éxito |
| `1` | Fallo genérico |
| `2` | Fallo en la comprobación de doctor (también se dispara desde `--dry-run` si el plan fuese a fallar) |
| `3` | Error de validación (flag incorrecto, requisito previo ausente) |

## Entorno

cortex-ia no lee ninguna variable de entorno obligatoria. Opcionales:

- `CORTEX_IA_HOME` — sobrescribe la raíz de estado `~/.cortex-ia/` con una ruta absoluta (rara vez necesaria; las pruebas la usan)
- `CORTEX_IA_DEBUG` — habilita el trazado de depuración cuando se establece en `1`, `true` o `yes` (equivalente al flag `--debug` por invocación)

Las antiguas variables `CORTEX_IA_AGY_AUTH` / `GEMINI_API_KEY` autenticaban la hoja externa AGY retirada y están muertas: la ejecución es exclusivamente nativa.

## Ejemplos de CI

One-liner para GitHub Actions:

```yaml
- name: Install cortex-ia ecosystem
  run: |
    go install github.com/lleontor705/cortex-ia/cmd/cortex-ia@latest
    cortex-ia install --target opencode
    cortex-ia doctor
```

Docker (ver `e2e/Dockerfile.ubuntu`):

```dockerfile
RUN cortex-ia install --target opencode && cortex-ia doctor
```

---

## Ver también

- [`configuration.md`](configuration.md) — Referencia de la CLI y variables de entorno
- [`platforms.md`](platforms.md) — Plataformas soportadas y restricciones de rutas
- [`docker-e2e-testing.md`](docker-e2e-testing.md) — Suite de pruebas E2E con Docker
