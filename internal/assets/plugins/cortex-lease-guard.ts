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
  const direct = [args?.TargetFile, args?.targetFile, args?.filePath, args?.file_path, args?.path, args?.file].filter(value => value !== undefined);
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

/**
 * Fail-closed admission for native file tools. Typed planning/discovery tools
 * have separate policy. This hook does not sandbox shell writes or make the
 * lease check and subsequent filesystem mutation atomic.
 */
export const CortexLeaseGuardPlugin: Plugin = async (ctx) => ({
  "tool.execute.before": async (input, output) => {
    const toolName = input?.tool?.toLowerCase() || "";
    if (!["edit", "write_to_file", "write", "apply_patch"].includes(toolName)) return;

    if (typeof input.sessionID !== "string" || !/^[A-Za-z0-9_-]{1,256}$/.test(input.sessionID)) throw new Error("LEASE_CHECK_FAILED: host session identity is required");
    const targets = [...new Set(targetFiles(toolName, (output?.args || {}) as Record<string, any>).map(target => relativeTarget(ctx.directory, target)))].sort();
    const cortex = firstCortexIA();
    if (!targets.length) return;
    const cortex = firstCortexIA();

    try {
      let raw: string | undefined;
      let lastExecErr: any;
      const pathArgs = targets.length === 1
        ? ["--path", targets[0]]
        : targets.flatMap(t => ["--path", t]);
      const cliArgs = ["work", "verify-lease", "--project", path.resolve(ctx.directory), "--session-id", input.sessionID, ...pathArgs];

      for (let attempt = 1; attempt <= 2; attempt++) {
        try {
          raw = execFileSync(cortex, cliArgs, {
            cwd: ctx.directory,
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
      if (result.owner !== `opencode-session:${input.sessionID}`) {
        throw new Error(`claim owner mismatch for targets: expected 'opencode-session:${input.sessionID}', got '${result.owner}'`);
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
      throw new Error(`LEASE_REQUIRED${suffix} (target: '${targetsStr}')`);
    }
  },
});

export default CortexLeaseGuardPlugin;
