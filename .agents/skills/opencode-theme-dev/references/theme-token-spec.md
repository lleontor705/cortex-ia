# OpenCode v2 Theme Token Specification Reference

This document provides the full token schema and reference values for authoring compliant themes for OpenCode v2 (`opencode2`).

## Canonical Theme Template

```json
{
  "$schema": "https://opencode.ai/theme.json",
  "version": 2,
  "base": {
    "categorical": ["accent", "purple", "cyan", "blue", "green", "yellow", "orange"],
    "text": {
      "base": "$hue.neutral.100",
      "muted": "$hue.neutral.400",
      "action": {
        "primary": {
          "base": "$hue.accent.300",
          "$hovered": "$hue.accent.200",
          "$disabled": "$hue.neutral.600"
        },
        "secondary": {
          "base": "$hue.neutral.200",
          "$hovered": "$hue.neutral.100",
          "$disabled": "$hue.neutral.600"
        },
        "destructive": {
          "base": "$hue.red.300",
          "$hovered": "$hue.red.200",
          "$disabled": "$hue.neutral.600"
        }
      },
      "formfield": {
        "base": "$hue.neutral.100",
        "$focused": "$hue.accent.200",
        "$disabled": "$hue.neutral.500"
      },
      "feedback": {
        "error": { "base": "$hue.red.300" },
        "warning": { "base": "$hue.yellow.300" },
        "success": { "base": "$hue.green.300" },
        "info": { "base": "$hue.cyan.300" }
      }
    },
    "background": {
      "base": "#0a0e17",
      "raised": {
        "base": "#111927",
        "high": "#1e293b",
        "max": "#334155"
      },
      "action": {
        "primary": {
          "base": "$hue.accent.600",
          "$hovered": "$hue.accent.500",
          "$pressed": "$hue.accent.700"
        },
        "secondary": {
          "base": "$background.raised.base",
          "$hovered": "$background.raised.high",
          "$pressed": "$background.raised.max"
        },
        "destructive": {
          "base": "$hue.red.700",
          "$hovered": "$hue.red.600",
          "$pressed": "$hue.red.800"
        }
      },
      "formfield": {
        "base": "$background.raised.base",
        "$focused": "$background.raised.high",
        "$disabled": "$background.base"
      },
      "feedback": {
        "error": { "base": "$hue.red.900" },
        "warning": { "base": "$hue.yellow.900" },
        "success": { "base": "$hue.green.900" },
        "info": { "base": "$hue.cyan.900" }
      }
    },
    "border": {
      "base": "$hue.neutral.700",
      "active": "$hue.accent.400"
    },
    "scrollbar": {
      "base": "$hue.neutral.600"
    },
    "diff": {
      "text": {
        "added": "$hue.green.300",
        "removed": "$hue.red.300",
        "context": "$text.muted",
        "hunkHeader": "$hue.neutral.400"
      },
      "background": {
        "added": "$hue.green.900",
        "removed": "$hue.red.900",
        "context": "$background.raised.base"
      },
      "highlight": {
        "added": "$hue.green.400",
        "removed": "$hue.red.400"
      },
      "lineNumber": {
        "text": "$hue.neutral.400",
        "background": {
          "added": "$hue.green.900",
          "removed": "$hue.red.900"
        }
      }
    },
    "syntax": {
      "comment": "$hue.neutral.500",
      "keyword": "$hue.purple.300",
      "function": "$hue.cyan.300",
      "variable": "$hue.neutral.200",
      "string": "$hue.green.300",
      "number": "$hue.yellow.300",
      "type": "$hue.blue.300",
      "operator": "$hue.cyan.400",
      "punctuation": "$hue.neutral.400"
    },
    "markdown": {
      "text": "$hue.neutral.100",
      "heading": "$hue.cyan.300",
      "link": "$hue.interactive.300",
      "linkText": "$hue.cyan.300",
      "code": "$hue.accent.300",
      "codeBlock": "$hue.neutral.900",
      "blockQuote": "$hue.neutral.400",
      "emphasis": "$hue.yellow.300",
      "strong": "$hue.accent.300",
      "horizontalRule": "$hue.neutral.700",
      "listItem": "$hue.neutral.100",
      "listEnumeration": "$hue.cyan.400",
      "image": "$hue.blue.400",
      "imageText": "$hue.neutral.400"
    },
    "@dialog": {
      "background": {
        "base": "$background.raised.base"
      }
    }
  },
  "dark": {
    "hue": {
      "gray": {
        "100": "#f8fafc", "200": "#e2e8f0", "300": "#cbd5e1",
        "400": "#94a3b8", "500": "#64748b", "600": "#475569",
        "700": "#334155", "800": "#1e293b", "900": "#0f172a"
      },
      "purple": {
        "100": "#f3e8ff", "200": "#e9d5ff", "300": "#c084fc",
        "400": "#a855f7", "500": "#8b5cf6", "600": "#7c3aed",
        "700": "#6d28d9", "800": "#4c1d95", "900": "#2e1065"
      },
      "cyan": {
        "100": "#cffafe", "200": "#a5f3fc", "300": "#67e8f9",
        "400": "#22d3ee", "500": "#06b6d4", "600": "#0891b2",
        "700": "#0e7490", "800": "#155e75", "900": "#164e63"
      },
      "blue": {
        "100": "#e0f2fe", "200": "#bae6fd", "300": "#7dd3fc",
        "400": "#38bdf8", "500": "#6366f1", "600": "#4f46e5",
        "700": "#4338ca", "800": "#312e81", "900": "#1e1b4b"
      },
      "green": {
        "100": "#d1fae5", "200": "#a7f3d0", "300": "#6ee7b7",
        "400": "#34d399", "500": "#10b981", "600": "#059669",
        "700": "#047857", "800": "#065f46", "900": "#064e3b"
      },
      "yellow": {
        "100": "#fef3c7", "200": "#fde68a", "300": "#fcd34d",
        "400": "#fbbf24", "500": "#f59e0b", "600": "#d97706",
        "700": "#b45309", "800": "#92400e", "900": "#78350f"
      },
      "orange": {
        "100": "#ffedd5", "200": "#fed7aa", "300": "#fdba74",
        "400": "#fb923c", "500": "#f97316", "600": "#ea580c",
        "700": "#c2410c", "800": "#9a3412", "900": "#7c2d12"
      },
      "red": {
        "100": "#ffe4e6", "200": "#fecdd3", "300": "#fda4af",
        "400": "#fb7185", "500": "#f43f5e", "600": "#e11d48",
        "700": "#be123c", "800": "#9f1239", "900": "#881337"
      },
      "accent": "$hue.purple",
      "interactive": "$hue.cyan",
      "neutral": "$hue.gray"
    }
  }
}
```

## Strict Schema Rules (ArkType / OpenCode v2)

1. **Syntax & Markdown Values (`j` schema)**:
   - Values under `base.syntax.*` and `base.markdown.*` **MUST** be either a raw hex string (`#rrggbb`) or a direct hue reference (`$hue.<hue>.<step>`).
   - **DO NOT** use semantic references like `"$text.base"`, `"$background.raised.base"`, `"$border.base"`, or `"$text.muted"` in `syntax` or `markdown`. The parser validates them with rule `j = R([U, Jh(["$hue.", z, ".", p])])` before semantic token resolution occurs; any non-hue alias will fail with:
     `Invalid theme: <name> "$text.base" is an invalid value`.

2. **Action & Formfield Fallbacks**:
   - Interactive tokens under `action` and `formfield` will auto-fallback to `.base` if state variants (`$hovered`, `$pressed`, `$focused`, `$disabled`) are omitted, but specifying them explicitly guarantees high contrast across all terminal types.

