/**
 * Example OpenCode v2 Plugin: Tool Guard & Security Fence
 *
 * Demonstrates:
 * 1. Dual v1/v2 compatibility wrapper (`Plugin.define`).
 * 2. Hooking tool calls via `ctx.tool.hook`.
 * 3. Enforcing permission checks and rejecting prohibited actions.
 */

export const Plugin = {
  define: <T extends { id: string; setup?: (ctx: any) => Promise<any> | any; server?: (ctx: any) => Promise<any> | any }>(def: T): any => {
    const fn = (ctx: any) => {
      const init = def.setup ?? def.server;
      return init ? init(ctx) : undefined;
    };
    Object.assign(fn, def);
    if (!def.server && def.setup) {
      def.server = def.setup;
    }
    return fn;
  },
};

export const ToolGuardPlugin = Plugin.define({
  id: "tool-guard-example",
  setup: async (ctx: any) => {
    if (!ctx?.tool?.hook) {
      return;
    }

    ctx.tool.hook(async (event: { tool: string; input: Record<string, unknown> }, next: () => Promise<unknown>) => {
      // Guard dangerous shell commands
      if (event.tool === "shell") {
        const cmd = String(event.input?.command || "");
        if (cmd.includes("rm -rf /") || cmd.includes(":(){ :|:& };:")) {
          throw new Error(`[SECURITY ALERT] Dangerous command blocked by ToolGuardPlugin: ${cmd}`);
        }
      }

      // Guard sensitive file edits
      if (event.tool === "edit" || event.tool === "write") {
        const targetPath = String(event.input?.path || "");
        if (targetPath.endsWith(".env") || targetPath.includes("id_rsa")) {
          throw new Error(`[SECURITY ALERT] Direct edits to secret file blocked: ${targetPath}`);
        }
      }

      // Proceed with execution
      return await next();
    });
  },
});

export default ToolGuardPlugin;
