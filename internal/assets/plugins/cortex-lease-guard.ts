// Universal Plugin helper ensuring full compatibility with OpenCode v1 (1.18.x) and v2 (2.x)
export const Plugin = {
  define: <T extends { id: string; setup?: (ctx: any) => Promise<any> | any; v1?: (ctx: any) => Promise<any> | any }>(def: T): any => {
    const fn: any = async (ctx: any) => {
      if (typeof def.v1 === "function") {
        return await def.v1(ctx);
      }
      if (typeof def.setup === "function") {
        const res = await def.setup(ctx);
        if (res && typeof res === "object") return res;
      }
      return {};
    };
    fn.id = def.id;
    fn.setup = def.setup;
    return fn;
  },
};

import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";

function resolveExecutable(cmd: string): string | null {
  const isWin = process.platform === "win32";
  const locator = isWin ? "where.exe" : "which";
  try {
    const out = execFileSync(locator, [cmd], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
      windowsHide: true,
    }).trim();
    if (out) {
      const first = out.split(/\r?\n/)[0].trim();
      if (path.isAbsolute(first) && fs.existsSync(first)) {
        return first;
      }
    }
  } catch {}
  return null;
}

function firstCortexIA(directory?: string): string {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const local = process.env.LOCALAPPDATA || path.join(home, "AppData", "Local");
  const candidates = [
    path.join(home, "go", "bin", "cortex-ia.exe"),
    path.join(local, "Programs", "cortex-ia", "bin", "cortex-ia.exe"),
    path.join(home, ".local", "bin", "cortex-ia"),
    "/usr/local/bin/cortex-ia",
    "/usr/bin/cortex-ia",
  ];
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) return candidate;
  }
  const resolved = resolveExecutable("cortex-ia");
  if (resolved) {
    if (directory && path.resolve(resolved).startsWith(path.resolve(directory) + path.sep)) {
      throw new Error("SECURITY_ERROR: cortex-ia executable found inside workspace directory is untrusted");
    }
    return resolved;
  }
  throw new Error("cortex-ia executable not found");
}

function targetFiles(toolName: string, args: Record<string, any>): string[] {
  if (toolName === "cortex_ia_doc_convert" || toolName === "cortex_ia_diagram_render") {
    if (typeof args?.output_path !== "string" || !args.output_path) throw new Error("LEASE_CHECK_FAILED: an output path is required");
    return [args.output_path];
  }
  if (toolName === "apply_patch") {
    if (typeof args?.patchText !== "string" || args.patchText.length > 1024 * 1024) throw new Error("LEASE_CHECK_FAILED: bounded patchText is required");
    const lines = args.patchText.replace(/\r\n/g, "\n").trim().split("\n");
    if (lines.shift() !== "*** Begin Patch" || lines.pop() !== "*** End Patch") throw new Error("LEASE_CHECK_FAILED: unrecognized patch framing");
    const targets: string[] = [];
    let kind = "";
    for (const line of lines) {
      const header = /^\*\*\* (Add|Update|Delete) File: (.+)$/.exec(line);
      if (header) { kind = header[1]; targets.push(header[2]); continue; }
      const move = /^\*\*\* Move to: (.+)$/.exec(line);
      if (move && kind === "Update") { targets.push(move[1]); continue; }
      if (line.startsWith("*** ") && line !== "*** End of File") throw new Error("LEASE_CHECK_FAILED: unrecognized patch operation");
    }
    if (!targets.length || targets.length > 128) throw new Error("LEASE_CHECK_FAILED: patch must expose 1-128 target paths");
    return targets;
  }
  const direct = [
    args?.TargetFile,
    args?.targetFile,
    args?.filePath,
    args?.file_path,
    args?.path,
    args?.file,
    args?.destination,
    args?.dest,
    args?.target,
    args?.target_path,
  ].filter(value => value !== undefined);
  if (!direct.length || direct.some(value => typeof value !== "string" || !value || value !== direct[0])) throw new Error("LEASE_CHECK_FAILED: a single verifiable target path is required");
  return [direct[0]];
}

function relativeTarget(directory: string, target: string): string {
  if (target.includes("\0")) throw new Error("LEASE_CHECK_FAILED: invalid target path");
  const root = path.resolve(directory);
  const relative = path.relative(root, path.resolve(root, target));
  if (!relative || path.isAbsolute(relative) || relative === ".." || relative.startsWith(`..${path.sep}`) || relative.includes(":")) throw new Error("LEASE_CHECK_FAILED: target is outside the workspace");
  // Do not authorize a logical lease path that writes through a link to another
  // physical target. Missing components are allowed for newly created files.
  let current = fs.realpathSync(root);
  for (const component of relative.split(path.sep)) {
    current = path.join(current, component);
    try {
      if (fs.lstatSync(current).isSymbolicLink()) throw new Error("LEASE_CHECK_FAILED: symlink targets are not verifiable");
    } catch (error: any) {
      if (error?.code !== "ENOENT") throw error;
    }
  }
  return relative.replaceAll(path.sep, "/");
}

async function verifyLeasesForTool(
  toolName: string,
  rawArgs: any,
  sessionID: string,
  directory: string
) {
  if (toolName === "cortex_ia_doc_convert" && !(rawArgs as any)?.output_path) return;
  if (!["edit", "write_to_file", "write", "apply_patch", "cortex_ia_doc_convert", "cortex_ia_diagram_render"].includes(toolName)) return;

  if (typeof sessionID !== "string" || !/^[A-Za-z0-9_-]{1,256}$/.test(sessionID)) throw new Error("LEASE_CHECK_FAILED: host session identity is required");
  const rawTargets = targetFiles(toolName, (rawArgs || {}) as Record<string, any>);
  // Every target must be contained and leased, including logs, scratch files,
  // and control directories. Path names never confer mutation authority.
  const targets = [...new Set(rawTargets.map(target => relativeTarget(directory, target)))].sort();
  const cortex = firstCortexIA(directory);

  try {
    let raw: string | undefined;
    let lastExecErr: any;
    const pathArgs = targets.length === 1
      ? ["--path", targets[0]]
      : targets.flatMap(t => ["--path", t]);
    const cliArgs = ["work", "verify-lease", "--project", path.resolve(directory), "--session-id", sessionID, ...pathArgs];

    for (let attempt = 1; attempt <= 2; attempt++) {
      try {
        raw = execFileSync(cortex, cliArgs, {
          cwd: directory,
          encoding: "utf8",
          maxBuffer: 64 * 1024,
          stdio: ["ignore", "pipe", "pipe"],
          timeout: 15000,
          windowsHide: true,
        });
        break;
      } catch (execErr: any) {
        lastExecErr = execErr;
        if (execErr?.code === "ETIMEDOUT" && attempt === 1) {
          continue;
        }
        throw execErr;
      }
    }
    if (!raw && lastExecErr) throw lastExecErr;
    const result = JSON.parse(raw!);
    if (result.valid !== true) {
      throw new Error(result.reason || "lease verification rejected by cortex-ia");
    }
    if (result.owner !== `opencode-session:${sessionID}`) {
      throw new Error(`claim owner mismatch for targets: expected 'opencode-session:${sessionID}', got '${result.owner}'`);
    }
    if (typeof result.task_id !== "string" || !result.task_id) {
      throw new Error("missing task identity in lease verification response");
    }
    const expiresAt = Date.parse(result.expires_at);
    if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
      throw new Error("lease for targets has expired");
    }
  } catch (err: any) {
    let reason = "";
    if (err?.stdout) {
      try {
        const parsed = JSON.parse(typeof err.stdout === "string" ? err.stdout : err.stdout.toString("utf8"));
        if (typeof parsed?.reason === "string" && parsed.reason) {
          reason = parsed.reason;
        }
      } catch {}
    }
    if (!reason && err?.stderr) {
      const stderrStr = (typeof err.stderr === "string" ? err.stderr : err.stderr.toString("utf8")).trim();
      if (stderrStr) {
        reason = stderrStr.replace(/^lease verification failed:\s*/i, "");
      }
    }
    if (!reason && typeof err?.message === "string" && err.message && !err.message.includes("Command failed")) {
      reason = err.message;
    }
    const targetsStr = targets.join(", ");
    const suffix = reason ? `: ${reason}` : ": all native mutation targets require a live session-owned claim and lease in this workspace";
    const recoveryGuidance = `\n[RECOVERY GUIDANCE] Run cortex_ia_work_claim({ task_id, paths: ['${targets.join("', '")}'] }) to acquire claim and lease before editing, or cortex_ia_work_lease_renew if expired. If authority was lost, run cortex_ia_work_recover.`;
    throw new Error(`LEASE_REQUIRED${suffix} (target: '${targetsStr}')${recoveryGuidance}`);
  }
}

/**
 * Fail-closed admission for native file tools. Typed planning/discovery tools
 * have separate policy. This hook does not sandbox shell writes or make the
 * lease check and subsequent filesystem mutation atomic.
 */
export const CortexLeaseGuardPlugin = Plugin.define({
  id: "cortex-lease-guard",
  async setup(ctx: any) {
    await ctx.tool.hook("execute.before", async (event: any) => {
      const toolName = (event?.tool || "").toLowerCase();
      const sessionID = event?.sessionID || event?.sessionId || (ctx as any)?.session?.id || "";
      const directory = (ctx as any).location?.directory || (ctx as any).directory || process.cwd();
      await verifyLeasesForTool(toolName, event?.input, sessionID, directory);
    });

    // OpenCode v2: Shell Fencing and Environment Injection
    if ((ctx as any).shell?.hook) {
      await (ctx as any).shell.hook("create.before", (event: any) => {
        const directory = (ctx as any).location?.directory || (ctx as any).directory || process.cwd();
        const sessionID = event?.sessionID || event?.sessionId || (ctx as any)?.session?.id || "";
        if (sessionID && event?.env) {
          event.env.CORTEX_SESSION_ID = sessionID;
          event.env.CORTEX_WORKSPACE_ROOT = directory;
        }
        if (event?.timeout && typeof event.timeout === "number") {
          event.timeout = Math.min(event.timeout, 300_000);
        }
        const cmd = typeof event?.command === "string" ? event.command.trim() : "";
        if (/\b(rm\s+-rf\s+[\/\*]|git\s+clean\s+-fdx|git\s+push\s+--force)(?:\s|$|;)/i.test(cmd)) {
          throw new Error("CORTEX_SECURITY_SHIELD: Destructive shell command intercepted by CortexLeaseGuard");
        }
      });
    }

    const cleanup = async () => {};
    (cleanup as any).dispose = cleanup;
    return cleanup;
  },
  async v1(ctx: any) {
    const directory = ctx?.directory || process.cwd();
    return {
      "tool.execute.before": async (input: any, output: any) => {
        const toolName = (input?.tool || "").toLowerCase();
        const sessionID = input?.sessionID || "";
        await verifyLeasesForTool(toolName, output?.args, sessionID, directory);
      },
    };
  },
});

export default CortexLeaseGuardPlugin;

