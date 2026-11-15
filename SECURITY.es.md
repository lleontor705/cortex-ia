[English](SECURITY.md) | **Español**

# Política de seguridad

Cortex-IA es un puente local y plano de control para el ecosistema OpenCode. Nos
tomamos en serio los informes de seguridad y te pedimos que divulgues los problemas de forma responsable.

## Versiones soportadas

Cortex-IA sigue un modelo de lanzamiento continuo. Solo la **última versión menor**
recibe correcciones de seguridad; las versiones anteriores se soportan únicamente mediante actualizaciones.

| Versión              | Soportada          |
| -------------------- | ------------------ |
| Última versión menor | :white_check_mark: |
| Versiones anteriores | :x:                |

El historial de versiones se registra en [CHANGELOG.md](CHANGELOG.md). Confirma tu
hallazgo contra la última versión antes de reportarlo.

## Reportar una vulnerabilidad

**No abras un issue público ni un pull request para vulnerabilidades de seguridad.**

Reporta de forma privada a través de GitHub Security Advisories:

- <https://github.com/lleontor705/cortex-ia/security/advisories/new>

Incluye, cuando sea posible:

- La versión, commit o rama afectada.
- Una descripción clara del problema y su impacto de seguridad.
- Pasos de reproducción o una prueba de concepto mínima.
- Una remediación sugerida, si la tienes.

## Política de divulgación

Seguimos una divulgación coordinada:

1. Confirmamos la recepción de nuevos informes en un plazo de **72 horas**.
2. Investigamos, confirmamos la gravedad y preparamos una corrección.
3. Publicamos la corrección y un GitHub Security Advisory que describe el
   problema y las versiones afectadas.
4. Acreditamos al reportero en el aviso salvo que solicite anonimato.

Concédenos una ventana razonable para publicar una corrección antes de cualquier
divulgación pública.

## Alcance

Esta política cubre el binario CLI/TUI `cortex-ia` y el código fuente de este
repositorio. Las vulnerabilidades en dependencias de terceros deben reportarse
a sus respectivos mantenedores.
