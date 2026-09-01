import { type Plugin } from "@opencode-ai/plugin";
import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";

function firstCortexIA(): string {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const local = process.env.LOCALAPPDATA || path.join(home, "AppData", "Local");
  const candidates = [
    path.join(home, "go", "bin", "cortex-ia.exe"),
    "cortex-ia",
    path.join(local, "Programs", "cortex-ia", "bin", "cortex-ia.exe"),
    path.join(home, ".local", "bin", "cortex-ia"),
    "/usr/local/bin/cortex-ia",
    "/usr/bin/cortex-ia",
  ];
  for (const candidate of candidates) {
    if (candidate !== "cortex-ia" && fs.existsSync(candidate)) return candidate;
    if (candidate === "cortex-ia") {
      try {
        execFileSync(candidate, ["version"], { stdio: "ignore", windowsHide: true });
        return candidate;
      } catch {}
    }
  }
  throw new Error("cortex-ia executable not found");
}

function targetFiles(toolName: string, args: Record<string, any>): string[] {
  const direct = args?.TargetFile || args?.targetFile || args?.filePath || args?.file_path || args?.path || args?.file;
  if (typeof direct === "string" && direct) return [direct];
  if (toolName === "apply_patch" && typeof args?.patch === "string") {
    return [...args.patch.matchAll(/^\*\*\* (?:Add|Update|Delete) File: (.+)$/gm)].map((match) => match[1].trim());
  }
  return [];
}

/**
 * Enforces session-owned Cortex-IA leases for file mutation tools when strict
 * lease mode is enabled. Directly queries SQLite authority via cortex-ia work verify-lease.
 */
export const CortexLeaseGuardPlugin: Plugin = async (ctx) => ({
  "tool.execute.before": async (input, output) => {
    const toolName = input?.tool?.toLowerCase() || "";
    if (!["edit", "write_to_file", "write", "apply_patch"].includes(toolName)) return;

    const strict = process.env.CORTEX_ENFORCE_LEASES === "true" || process.env.CORTEX_LEASES_STRICT === "1";
    const targets = targetFiles(toolName, (output?.args || {}) as Record<string, any>);
    if (targets.length === 0) {
      if (!strict) return;
      throw new Error(`LEASE_CHECK_FAILED: ${toolName} did not expose a verifiable target path`);
    }

    let cortex: string;
    try {
      cortex = firstCortexIA();
    } catch (error: any) {
      if (!strict) return;
      throw new Error(`LEASE_CHECK_FAILED: ${error?.message || "cortex-ia CLI not found"}`);
    }

    for (const target of targets) {
      const relPath = path.isAbsolute(target) ? path.relative(ctx.directory, target) : target;
      try {
        execFileSync(cortex, ["work", "verify-lease", "--path", relPath], {
          cwd: ctx.directory,
          stdio: ["ignore", "pipe", "pipe"],
          timeout: 3000,
          windowsHide: true,
        });
      } catch (error: any) {
        if (!strict) return;
        const msg = error?.stderr?.toString()?.trim() || error?.stdout?.toString()?.trim() || error?.message || "unleased file mutation";
        throw new Error(`LEASE_REQUIRED: '${target}' has no active lease in Cortex-IA database (${msg})`);
      }
    }
  },
});

export default CortexLeaseGuardPlugin;
