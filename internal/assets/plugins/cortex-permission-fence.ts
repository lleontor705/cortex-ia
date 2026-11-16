// OpenCode Plugin helper ensuring default export is a valid plugin definition object for v1 and v2
export const Plugin = {
  define: <T extends { id: string; setup?: (ctx: any) => Promise<any> | any; server?: (ctx: any) => Promise<any> | any }>(def: T): T => {
    if (!def.server && def.setup) {
      def.server = def.setup;
    }
    return def;
  },
};

import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";

// A stalled PATH scan would block the permission hook (and the host event
// loop) indefinitely; the lookup is bounded so a stall falls through to the
// default permission decision instead of hanging admission. Slow positive
// verifications are logged so lease latency is observable in host logs.
const EXECUTABLE_LOOKUP_TIMEOUT_MS = 3000;
const SLOW_ADMISSION_LOG_MS = 2000;
// The workload audit runs a git diff synchronously on the host event loop; a
// hung repository degrades to empty stats instead of blocking the session.
const GIT_DIFF_TIMEOUT_MS = 10000;

function resolveExecutable(cmd: string): string | null {
  const isWin = process.platform === "win32";
  const locator = isWin ? "where.exe" : "which";
  try {
    const out = execFileSync(locator, [cmd], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
      windowsHide: true,
      timeout: EXECUTABLE_LOOKUP_TIMEOUT_MS,
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

function firstCortexIA(directory?: string): string | null {
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
      return null;
    }
    return resolved;
  }
  return null;
}

const READ_ONLY_ROLES = new Set(["discovery", "investigate", "planner", "reviewer"]);
const MUTATING_TOOLS = new Set([
  "edit",
  "multiedit",
  "write_to_file",
  "write",
  "replace_file_content",
  "notebook_edit",
  "apply_patch",
  "patch",
  "delete_file",
]);

// Mirrors cortex-lease-guard.ts patch framing so both plugins agree on targets.
// Returns null for an unparseable patch, which the caller must treat as a denial.
function patchTargets(patchText: unknown): string[] | null {
  if (typeof patchText !== "string" || patchText.length > 1024 * 1024) return null;
  const lines = patchText.replace(/\r\n/g, "\n").trim().split("\n");
  if (lines.shift() !== "*** Begin Patch" || lines.pop() !== "*** End Patch") return null;
  const targets: string[] = [];
  let kind = "";
  for (const line of lines) {
    const header = /^\*\*\* (Add|Update|Delete) File: (.+)$/.exec(line);
    if (header) { kind = header[1]; targets.push(header[2]); continue; }
    const move = /^\*\*\* Move to: (.+)$/.exec(line);
    if (move && kind === "Update") { targets.push(move[1]); continue; }
    if (line.startsWith("*** ") && line !== "*** End of File") return null;
  }
  return targets.length && targets.length <= 128 ? targets : null;
}

// Only the workspace-root .gitignore is discovery's sanctioned hygiene target.
function isRootGitignore(directory: string, target: string): boolean {
  if (!target || target.includes("\0")) return false;
  const root = path.resolve(directory);
  return path.relative(root, path.resolve(root, target)).split(path.sep).join("/") === ".gitignore";
}

function isProtectedPath(filePath: string): boolean {
  // Segment matching keeps names like `.environment.ts` writable while still
  // covering SQLite sidecars (`delegation.db-wal`, `delegation.db-shm`).
  const segments = filePath.replace(/\\/g, "/").toLowerCase().split("/");
  return segments.some(segment =>
    segment === ".git" ||
    segment === ".env" ||
    segment.startsWith(".env.") ||
    /^delegation\.db([-.].*)?$/.test(segment)
  );
}

export interface WorkloadAuditResult {
  workload_status: "PASS" | "EXCEEDED_ADVISORY" | "WORKLOAD_SOURCE_BUDGET_EXCEEDED" | "WORKLOAD_TEST_BUDGET_EXCEEDED";
  policy: "strict" | "flexible" | "unbounded";
  logic_lines: number;
  test_lines: number;
  total_weighted: number;
  max_logic_cap: number;
  max_test_cap: number;
  files_changed: number;
  files: { path: string; added: number; deleted: number; category: string }[];
}

const AUDIT_POLICIES = ["strict", "flexible", "unbounded"] as const;

export function auditWorkload(
  directory: string,
  policy: "strict" | "flexible" | "unbounded" = "flexible",
  baseRef: string = "HEAD"
): WorkloadAuditResult {
  if (!(AUDIT_POLICIES as readonly string[]).includes(policy)) {
    throw new Error("workload audit: policy must be strict, flexible or unbounded");
  }
  // A model-supplied ref must never reach git as an option (`--output=<file>` writes).
  if (typeof baseRef !== "string" || baseRef.length === 0 || baseRef.startsWith("-") || baseRef.includes("\0")) {
    throw new Error("workload audit: baseRef must be a git revision, not an option");
  }
  const files: { path: string; added: number; deleted: number; category: string }[] = [];
  let logicAdded = 0;
  let logicDeleted = 0;
  let testAdded = 0;
  let testDeleted = 0;

  let raw: string;
  try {
    raw = execFileSync("git", ["diff", "--numstat", baseRef, "--"], {
      cwd: directory,
      encoding: "utf8",
      maxBuffer: 1024 * 1024,
      stdio: ["ignore", "pipe", "ignore"],
      windowsHide: true,
      timeout: GIT_DIFF_TIMEOUT_MS,
    });
  } catch {
    // A failed diff must not read as a clean PASS.
    throw new Error(`workload audit: git diff failed for baseRef '${baseRef}'`);
  }

  const lines = raw.trim().split(/\r?\n/);
  for (const line of lines) {
    if (!line.trim()) continue;
    const parts = line.split(/\t/);
    if (parts.length < 3) continue;
    const added = parseInt(parts[0], 10) || 0;
    const deleted = parseInt(parts[1], 10) || 0;
    const filePath = parts[2].trim();

    const isTest = /(_test\.|[._]test\.|[._]spec\.|[/\\]tests?[/\\]|[/\\]fixtures?[/\\])/i.test(filePath);
    const isSchemaOrDoc = /(\.md|\.json|\.ya?ml|\.sql)$/i.test(filePath);

    let category = "source";
    if (isTest) {
      category = "test";
      testAdded += added;
      testDeleted += deleted;
    } else if (isSchemaOrDoc) {
      category = "schema_doc";
    } else {
      category = "source";
      logicAdded += added;
      logicDeleted += deleted;
    }

    files.push({ path: filePath, added, deleted, category });
  }

  const weightedLogic = Math.round(logicAdded + logicDeleted * 0.2);
  const totalTest = testAdded + testDeleted;

  const maxLogicCap = policy === "strict" ? 350 : policy === "flexible" ? 700 : Infinity;
  const maxTestCap = policy === "strict" ? 600 : policy === "flexible" ? 1200 : Infinity;

  let status: WorkloadAuditResult["workload_status"] = "PASS";
  if (policy === "strict") {
    if (weightedLogic > maxLogicCap) {
      status = "WORKLOAD_SOURCE_BUDGET_EXCEEDED";
    } else if (totalTest > maxTestCap) {
      status = "WORKLOAD_TEST_BUDGET_EXCEEDED";
    }
  } else if (policy === "flexible") {
    if (weightedLogic > maxLogicCap || totalTest > maxTestCap) {
      status = "EXCEEDED_ADVISORY";
    }
  }

  return {
    workload_status: status,
    policy,
    logic_lines: weightedLogic,
    test_lines: totalTest,
    total_weighted: weightedLogic + totalTest,
    max_logic_cap: maxLogicCap,
    max_test_cap: maxTestCap,
    files_changed: files.length,
    files,
  };
}

function isPlannerPlanTarget(directory: string, target: string): boolean {
  if (!target || target.includes("\0")) return false;
  const root = path.resolve(directory);
  const rel = path.relative(root, path.resolve(root, target)).split(path.sep).join("/");
  return /^docs\/plans\/[a-z0-9][a-z0-9._-]{0,127}\.md$/.test(rel);
}

const verifiedPermLeaseCache = new Map<string, number>();

export const CortexPermissionFencePlugin = Plugin.define({
  id: "cortex-permission-fence",
  async setup(ctx) {
    const directory = (ctx as any).location?.directory || (ctx as any).directory || process.cwd();

    // 1. Engine-Level Permission Evaluation Hook (OpenCode v2)
    if ((ctx as any).permission?.hook) {
      await (ctx as any).permission.hook("evaluate", async (event: any) => {
        const toolName = String(event?.tool || "").toLowerCase();
        const targetPath = String(event?.path || event?.input?.TargetFile || event?.input?.targetFile || event?.input?.path || event?.input?.filePath || event?.input?.file_path || event?.input?.target_path || "");

        // A patch carries its own target list; an unparseable one is denied rather than
        // treated as targetless, since targetless writes skip every path check.
        const isPatchTool = toolName === "apply_patch" || toolName === "patch";
        const patchFiles = isPatchTool ? patchTargets(event?.input?.patchText) : undefined;
        if (isPatchTool && patchFiles === null) {
          return {
            effect: "deny",
            reason: "CORTEX_PERMISSION_FENCE: patch text could not be parsed; its target paths cannot be verified.",
          };
        }
        const targets = patchFiles ?? (targetPath ? [targetPath] : []);

        // Protected system paths check
        const protectedTarget = targets.find(isProtectedPath);
        if (protectedTarget !== undefined) {
          return {
            effect: "deny",
            reason: `CORTEX_PERMISSION_FENCE: Path '${protectedTarget}' is protected by Cortex-IA governance policy.`,
          };
        }

        // Role-based Read-Only Enforcement
        const role = String(event?.session?.role || (ctx as any)?.session?.role || process.env.CORTEX_ROLE || "").toLowerCase();
        if (READ_ONLY_ROLES.has(role) && MUTATING_TOOLS.has(toolName)) {
          const isDiscoveryGitignore = role === "discovery" && toolName === "edit" &&
            targets.length === 1 && isRootGitignore(directory, targets[0]);
          // planner's only sanctioned mutation is its plan file
          const isPlannerPlan = role === "planner" && targets.length > 0 &&
            targets.every(target => isPlannerPlanTarget(directory, target));
          if (!isDiscoveryGitignore && !isPlannerPlan) {
            return {
              effect: "deny",
              reason: `CORTEX_PERMISSION_FENCE: Role '${role}' is restricted to read-only operations. File mutations are forbidden.`,
            };
          }
        }

        // Zero-Prompt Auto-Approval for verified active file leases
        if (MUTATING_TOOLS.has(toolName) && targetPath) {
          const sessionID = event?.sessionID || (ctx as any)?.session?.id;
          const cacheKey = `${sessionID}:${targetPath}`;
          if (verifiedPermLeaseCache.get(cacheKey) && verifiedPermLeaseCache.get(cacheKey)! > Date.now()) {
            return { effect: "allow", reason: "Verified active lease (cached)" };
          }
          const cortex = firstCortexIA(directory);
          if (cortex && sessionID) {
            try {
              const cliArgs = ["work", "verify-lease", "--project", path.resolve(directory), "--session-id", sessionID, "--path", targetPath];
              const verifyStart = Date.now();
              const raw = execFileSync(cortex, cliArgs, {
                cwd: directory,
                encoding: "utf8",
                maxBuffer: 32 * 1024,
                stdio: ["ignore", "pipe", "ignore"],
                timeout: 5000,
                windowsHide: true,
              });
              const verifyMs = Date.now() - verifyStart;
              if (verifyMs >= SLOW_ADMISSION_LOG_MS) {
                console.warn(`[CORTEX_PERMISSION_FENCE] slow lease admission: verify-lease ${verifyMs}ms`);
              }
              const result = JSON.parse(raw);
              if (result?.valid === true && result?.owner === `opencode-session:${sessionID}`) {
                verifiedPermLeaseCache.set(cacheKey, Date.now() + 5_000);
                return { effect: "allow", reason: "Verified active lease" };
              }
            } catch {}
          }
        }

        return undefined;
      });
    }

    // 2. Dynamic Tool Registration: cortex_ia_workload_audit (git-only; no CLI wrapper)
    const auditInput = {
      type: "object",
      properties: {
        policy: {
          type: "string",
          enum: ["strict", "flexible", "unbounded"],
          description: "Budget tier to evaluate against (default flexible)",
        },
        baseRef: {
          type: "string",
          description: "Git revision to diff against (default HEAD)",
        },
      },
    };

    const auditTool = {
      name: "cortex_ia_workload_audit",
      description: "Audit uncommitted git diff against the workload budget. Read-only; run before work_transition to in_review. Returns workload_status (PASS, EXCEEDED_ADVISORY, WORKLOAD_SOURCE_BUDGET_EXCEEDED, WORKLOAD_TEST_BUDGET_EXCEEDED).",
      input: auditInput,
      parameters: auditInput,
      execute: async (args: any) => {
        const res = auditWorkload(directory, args?.policy || "flexible", args?.baseRef || "HEAD");
        return JSON.stringify(res, null, 2);
      },
    };

    if (typeof (ctx as any).tool?.transform === "function") {
      await (ctx as any).tool.transform((editor: any) => {
        if (editor && typeof editor.add === "function") {
          editor.add({
            name: auditTool.name,
            description: auditTool.description,
            input: auditTool.input,
            execute: async (input: any) => auditTool.execute(input),
          });
        }
      });
    } else if (typeof (ctx as any).tool?.register === "function") {
      (ctx as any).tool.register(auditTool);
    }

    const cleanup = async () => {};
    (cleanup as any).dispose = cleanup;
    return cleanup;
  },
});

export default CortexPermissionFencePlugin;
