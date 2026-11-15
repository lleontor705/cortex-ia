# OpenCode v2 Plugin API Reference

This reference documents the context object (`ctx`) provided to plugins in OpenCode v2 (`opencode2`).

## Context (`ctx`) Interface

```typescript
interface PluginContext {
  // Directory & session context
  directory: string;
  workspaceID?: string;

  // Tool interception
  tool: {
    hook: (fn: ToolHookFn) => void;
    transform?: (fn: ToolTransformFn) => void;
  };

  // Event bus
  event: (eventName: string, handler: (event: any) => Promise<void> | void) => void;

  // Extension points
  agent?: (fn: (agents: Record<string, any>) => Record<string, any>) => void;
  model?: (fn: (models: Record<string, any>) => Record<string, any>) => void;
  provider?: (fn: (providers: Record<string, any>) => Record<string, any>) => void;
  mcp?: (fn: (mcpConfig: Record<string, any>) => Record<string, any>) => void;
  command?: (fn: (commands: Record<string, any>) => Record<string, any>) => void;
}

type ToolHookFn = (
  event: {
    tool: string;
    input: Record<string, unknown>;
    sessionID?: string;
  },
  next: () => Promise<unknown>
) => Promise<unknown>;

type ToolTransformFn = (
  tools: Record<string, unknown>
) => Promise<Record<string, unknown>> | Record<string, unknown>;
```

## Standard Event Names in OpenCode v2

| Event Name | Trigger Condition | Payload Data |
| :--- | :--- | :--- |
| `session.created` | New chat session started | `{ sessionID, directory }` |
| `session.compacted` | Conversation history compacted | `{ sessionID, tokensKept }` |
| `session.deleted` | Session deleted | `{ sessionID }` |
| `provider.updated` | AI provider configuration reloaded | `{ directory }` |
| `model.updated` | Model selection or limits updated | `{ directory }` |
| `tool.executed` | Background tool execution completed | `{ tool, durationMs, error? }` |
| `permission.asked` | User permission prompt triggered | `{ action, resource }` |

## Plugin Location & Precedence

1. **Global plugins**: `~/.config/opencode/plugins/<name>.ts`
2. **Project plugins**: `<project-root>/.opencode/plugins/<name>.ts`
3. **Declared in `cli.json` or `opencode.jsonc`**:
   ```json
   "plugins": [
     "./tui-plugins/cortex-ia",
     "./plugins/my-custom-plugin.ts"
   ]
   ```
