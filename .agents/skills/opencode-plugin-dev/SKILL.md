---
name: opencode-plugin-dev
description: Design, implement, test, and harden plugins for OpenCode v2 (opencode2) using @opencode/plugin, dual v1/v2 definitions, hooks, transforms, and events.
license: MIT
metadata:
  author: lleontor705
  version: "2.0.0"
---

# OpenCode v2 Plugin Development Skill

This skill provides the architectural foundation and best practices for developing native plugins in **OpenCode v2 (`opencode2`)**.

---

## 1. Plugin Architecture in OpenCode v2

OpenCode v2 introduces a clean separation between UI components and background server hooks:

```
OpenCode Plugin Model
├── Plugin Definition: Plugin.define({ id, setup / server })
├── Context (ctx):
│   ├── ctx.tool.hook(fn)           <-- Intercept & wrap tool executions
│   ├── ctx.tool.transform(fn)      <-- Intercept/augment tool schema and returns
│   ├── ctx.event(name, handler)    <-- Subscribe to operational events
│   ├── ctx.agent(fn)               <-- Intercept agent selection & prompt injection
│   ├── ctx.model(fn)               <-- Model resolution overrides
│   ├── ctx.mcp(fn)                 <-- Dynamic MCP server registration
│   └── ctx.command(fn)             <-- Slash command handlers
└── Lifetime:
    ├── Server lifecycle (background daemon)
    └── UI / TUI lifecycle (@opencode/plugin/tui)
```

---

## 2. The Universal Dual-Mode Pattern (v1 & v2 Compatible)

To ensure plugins function smoothly across OpenCode runtime shims (both function-style v1 and object-style v2):

```typescript
export const Plugin = {
  define: <T extends { id: string; setup?: (ctx: any) => Promise<any> | any; server?: (ctx: any) => Promise<any> | any }>(def: T): any => {
    // If runtime expects a function (v1 style):
    const fn = (ctx: any) => {
      const init = def.setup ?? def.server;
      return init ? init(ctx) : undefined;
    };
    // Attach properties for v2 object-style runners:
    Object.assign(fn, def);
    if (!def.server && def.setup) {
      def.server = def.setup;
    }
    return fn;
  },
};
```

---

## 3. Hook Capabilities and Invariants

### 3.1 Tool Execution Guards (`ctx.tool.hook`)
Use `ctx.tool.hook` to validate arguments, enforce permission fences, or attach metadata before/after tool execution:

```typescript
ctx.tool.hook(async (event: { tool: string; input: Record<string, unknown> }, next: () => Promise<unknown>) => {
  // 1. Pre-execution validation
  if (event.tool === "shell" && isDangerous(event.input.command)) {
    throw new Error("Command rejected by security fence");
  }

  // 2. Call next handler in chain
  const result = await next();

  // 3. Post-execution telemetry / audit
  return result;
});
```

### 3.2 Tool Transformation (`ctx.tool.transform`)
Use `ctx.tool.transform` to intercept tool outputs or inject custom dynamic tools directly into agent context:

```typescript
if (typeof ctx?.tool?.transform === "function") {
  ctx.tool.transform(async (tools: Record<string, unknown>) => {
    return {
      ...tools,
      custom_read_tool: myCustomToolDefinition,
    };
  });
}
```

### 3.3 Event Subscriptions (`ctx.event`)
Subscribe to daemon, session, and agent lifecycle events without blocking the main event loop:

```typescript
if (typeof ctx?.event === "function") {
  ctx.event("session.created", async (event: any) => {
    // Record session start
  });
  ctx.event("provider.updated", async (event: any) => {
    // Handle provider config change
  });
}
```

---

## 4. Failure Modes & Anti-Patterns

1. **Unbounded Synchronous Execution**:
   - *Anti-pattern*: Performing synchronous filesystem writes or blocking network requests inside an event hook.
   - *Fix*: Always use async operations with bounded timeouts (e.g. `AbortController` with 3-5s deadline).

2. **Hardcoded Home Directory Paths**:
   - *Anti-pattern*: `path.join("C:/Users/name/.config", ...)` or assuming `/home/user`.
   - *Fix*: Use `os.homedir()` or `process.env.XDG_CONFIG_HOME` with platform-agnostic fallback.

3. **Silent Error Swallowing**:
   - *Anti-pattern*: `try { ... } catch (e) {}` with no logging or recovery.
   - *Fix*: Classify failures into structured error types (`unauthorized`, `forbidden`, `validation`, `unavailable`) and log bounded diagnostic output without leaking tokens or private payloads.

---

## 5. Testing & Verification

Every plugin should be tested using mock contexts before deployment:
```bash
node scripts/harness-v2.mjs
```
The test harness validates that plugins load cleanly, export appropriate modules, and adhere to isolation contracts.
