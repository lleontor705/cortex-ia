[English](index.md) | **Español**

<p align="center">
  <img src="assets/hero-banner.svg" alt="Banner de Cortex-IA" width="100%" />
</p>

# Cortex-IA

El **plano de control multiagente determinista y motor de orquestación** para el desarrollo de
software autónomo con **OpenCode** — un único binario portable de Go que hace seguro el trabajo
de agentes en paralelo.

[![Release](https://img.shields.io/github/v/release/lleontor705/cortex-ia?color=38BDF8&label=release)](https://github.com/lleontor705/cortex-ia/releases/latest)
[![Licencia](https://img.shields.io/github/license/lleontor705/cortex-ia?color=A855F7&label=license)](https://github.com/lleontor705/cortex-ia/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/lleontor705/cortex-ia)](https://goreportcard.com/report/github.com/lleontor705/cortex-ia)
[![Plataformas](https://img.shields.io/badge/platforms-Windows%20%7C%20Linux%20%7C%20macOS-blue)](https://github.com/lleontor705/cortex-ia)

## Primeros pasos

| 🚀 Inicio rápido | ⚙️ Instalación | 💻 CLI no interactiva | 🏛️ Arquitectura |
|---|---|---|---|
| Tu primer flujo de agentes coordinados en tres pasos. | Binarios precompilados, `go install` y el script de instalación. | Scripts, CI y recetas de Docker con recibos JSON. | Capas del motor, autoridad de tareas en SQLite e invariantes de seguridad. |
| [Empezar →](quickstart.md) | [Instalar →](installation.md) | [Automatizar →](non-interactive.md) | [Explorar →](architecture.md) |

## ¿Qué es Cortex-IA?

**Cortex-IA** es el plano de control operativo para el desarrollo multiagente sobre OpenCode.
Resuelve los fallos típicos del trabajo con agentes en paralelo — condiciones de carrera, ediciones
de archivos en conflicto, tareas que parecen listas sin estarlo y coordinación sin estructura —
manteniendo un DAG de tareas determinista en SQLite ACID y exigiendo concesiones de archivo
exclusivas antes de que cualquier agente escriba código.

Las tareas avanzan por una máquina de estados estricta
(`backlog → ready → in_progress → in_review → done`) con bloqueo optimista CAS, y ninguna tarea se
considera completa hasta que un revisor independiente registra un `PASS`. El servidor MCP de Cortex
lo complementa con grafos de conocimiento AST, análisis de radio de impacto y memoria duradera entre
sesiones — evidencia de carácter informativo, nunca un sustituto de la autoridad de trabajo.

## Participa

- Código fuente y versiones: [github.com/lleontor705/cortex-ia](https://github.com/lleontor705/cortex-ia)
- Guía de contribución: [CONTRIBUTING.md](https://github.com/lleontor705/cortex-ia/blob/main/CONTRIBUTING.md)
