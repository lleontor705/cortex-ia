import { type Plugin } from "@opencode-ai/plugin";
import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";

interface AuthorityEntry {
  task_id: string;
  session_id: string;
}

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

function authoritiesForSession(sessionID: string): AuthorityEntry[] {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const statePath = path.resolve(home, ".config", "opencode", `cortex-authority-state-${process.pid}.json`);
  const state = JSON.parse(fs.readFileSync(statePath, "utf-8"));
  return (state?.authorities || []).filter((entry: AuthorityEntry) => entry?.session_id === sessionID);
}

function targetFiles(toolName: string, args: Record<string, any>): string[] {
  const direct = args?.TargetFile || args?.targetFile || args?.filePath || args?.file_path || args?.path || args?.file;
  if (typeof direct === "string" && direct) return [direct];
  if (toolName === "apply_patch" && typeof args?.patch === "string") {
    return [...args.patch.matchAll(/^\*\*\* (?:Add|Update|Delete) File: (.+)$/gm)].map((match) => match[1].trim());
  }
  return [];
}

function normalizedAbsolute(root: string, value: string): string {
  const absolute = path.isAbsolute(value) ? path.resolve(value) : path.resolve(root, value);
  return process.platform === "win32" || process.platform === "darwin" ? absolute.toLowerCase() : absolute;
}

/**
 * Enforces session-owned Cortex-IA leases for file mutation tools when strict
 * lease mode is enabled. Authority tokens stay inside herdr-bridge; this guard
 * reads only a sanitized session-to-task handle and durable lease metadata.
 */
export const CortexLeaseGuardPlugin: Plugin = async (ctx) => ({
  "tool.execute.before": async (input, output) => {
    const toolName = input?.tool?.toLowerCase() || "";
    if (!["edit", "write_to_file", "write", "apply_patch"].includes(toolName)) return;

    const strict = process.env.CORTEX_ENFORCE_LEASES === "true" || process.env.CORTEX_LEASES_STRICT === "1";
    let matches: AuthorityEntry[] = [];
    try {
      matches = authoritiesForSession(input.sessionID);
    } catch (error: any) {
      if (!strict) return;
      throw new Error(`LEASE_CHECK_FAILED: ${error?.message || "unable to read authority state"}`);
    }
    if (!strict && matches.length === 0) return;
    if (matches.length !== 1) {
      throw new Error(`LEASE_AUTHORITY_REQUIRED: session has ${matches.length} live task authorities; expected exactly one`);
    }

    const targets = targetFiles(toolName, (output?.args || {}) as Record<string, any>);
    if (targets.length === 0) {
      throw new Error(`LEASE_CHECK_FAILED: ${toolName} did not expose a verifiable target path`);
    }

    try {
      const authority = matches[0];
      const raw = execFileSync(firstCortexIA(), ["work", "status", authority.task_id], {
        encoding: "utf-8",
        stdio: ["ignore", "pipe", "pipe"],
        timeout: 2000,
        windowsHide: true,
      });
      const item = JSON.parse(raw);
      const now = Date.now();
      const leased = new Set(
        (item?.leases || [])
          .filter((lease: any) => lease?.task_id === authority.task_id && Date.parse(lease?.expires_at || "") > now)
          .map((lease: any) => normalizedAbsolute(ctx.directory, lease.path)),
      );
      for (const target of targets) {
        const normalized = normalizedAbsolute(ctx.directory, target);
        if (!leased.has(normalized)) {
          throw new Error(`LEASE_REQUIRED: '${target}' has no active lease owned by task '${authority.task_id}' in this session`);
        }
      }
    } catch (error: any) {
      if (String(error?.message || "").startsWith("LEASE_")) throw error;
      throw new Error(`LEASE_CHECK_FAILED: ${error?.message || "unable to verify durable lease authority"}`);
    }
  },
});

export default CortexLeaseGuardPlugin;
