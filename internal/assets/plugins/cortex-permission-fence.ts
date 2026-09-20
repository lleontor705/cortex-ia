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
  "write_to_file",
  "write",
  "replace_file_content",
  "apply_patch",
  "delete_file",
]);

function isProtectedPath(filePath: string): boolean {
  const norm = filePath.replace(/\\/g, "/");
  return (
    norm.includes(".git/") ||
    norm.endsWith("/.git") ||
    norm.includes(".env") ||
    norm.includes("delegation.db") ||
    norm.includes(".cortex-ia/delegation.db")
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

export function auditWorkload(
  directory: string,
  policy: "strict" | "flexible" | "unbounded" = "flexible",
  baseRef: string = "HEAD"
): WorkloadAuditResult {
  const files: { path: string; added: number; deleted: number; category: string }[] = [];
  let logicAdded = 0;
  let logicDeleted = 0;
  let testAdded = 0;
  let testDeleted = 0;

  try {
    const raw = execFileSync("git", ["diff", "--numstat", baseRef], {
      cwd: directory,
      encoding: "utf8",
      maxBuffer: 1024 * 1024,
      stdio: ["ignore", "pipe", "ignore"],
      windowsHide: true,
    });

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
  } catch {}

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

const verifiedPermLeaseCache = new Map<string, number>();

export const CortexPermissionFencePlugin = Plugin.define({
  id: "cortex-permission-fence",
  async setup(ctx) {
    const directory = (ctx as any).location?.directory || (ctx as any).directory || process.cwd();

    // 1. Engine-Level Permission Evaluation Hook (OpenCode v2)
    if ((ctx as any).permission?.hook) {
      await (ctx as any).permission.hook("evaluate", async (event: any) => {
        const toolName = String(event?.tool || "").toLowerCase();
        const targetPath = String(event?.path || event?.input?.TargetFile || event?.input?.path || "");

        // Protected system paths check
        if (targetPath && isProtectedPath(targetPath)) {
          return {
            effect: "deny",
            reason: `CORTEX_PERMISSION_FENCE: Path '${targetPath}' is protected by Cortex-IA governance policy.`,
          };
        }

        // Role-based Read-Only Enforcement
        const role = String(event?.session?.role || (ctx as any)?.session?.role || process.env.CORTEX_ROLE || "").toLowerCase();
        if (READ_ONLY_ROLES.has(role) && MUTATING_TOOLS.has(toolName)) {
          return {
            effect: "deny",
            reason: `CORTEX_PERMISSION_FENCE: Role '${role}' is restricted to read-only operations. File mutations are forbidden.`,
          };
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
              const raw = execFileSync(cortex, cliArgs, {
                cwd: directory,
                encoding: "utf8",
                maxBuffer: 32 * 1024,
                stdio: ["ignore", "pipe", "ignore"],
                timeout: 5000,
                windowsHide: true,
              });
              const result = JSON.parse(raw);
              if (result?.valid === true && result?.owner === `opencode-session:${sessionID}`) {
                verifiedPermLeaseCache.set(cacheKey, Date.now() + 30_000);
                return { effect: "allow", reason: "Verified active lease" };
              }
            } catch {}
          }
        }

        return undefined;
      });
    }

    // 2. Dynamic Tool Registration: cortex_ia_workload_audit & cortex_ia_work_watch
    const auditInput = {
      type: "object",
      properties: {
        policy: {
          type: "string",
          enum: ["strict", "flexible", "unbounded"],
          description: "Workload policy threshold to evaluate against.",
        },
        baseRef: {
          type: "string",
          description: "Git reference to compare against (defaults to HEAD).",
        },
      },
    };

    const watchInput = {
      type: "object",
      properties: {
        board: {
          type: "string",
          description: "Board ID to query (defaults to 'default').",
        },
      },
    };

    const auditTool = {
      name: "cortex_ia_workload_audit",
      description: "Audit uncommitted git diff lines against the Cortex-IA workload budget (strict <=350 LOC, flexible <=700 LOC) before transitioning to review.",
      input: auditInput,
      parameters: auditInput,
      execute: async (args: any) => {
        const res = auditWorkload(directory, args?.policy || "flexible", args?.baseRef || "HEAD");
        return { content: JSON.stringify(res, null, 2) };
      },
    };

    const watchTool = {
      name: "cortex_ia_work_watch",
      description: "Check the live Cortex-IA task authority, active board state, claims, and remaining lease TTL.",
      input: watchInput,
      parameters: watchInput,
      execute: async (args: any) => {
        const cortex = firstCortexIA(directory);
        if (!cortex) {
          return { content: JSON.stringify({ error: "cortex-ia binary not found" }) };
        }
        try {
          const board = args?.board || "default";
          const raw = execFileSync(cortex, ["work", "status", "--project", path.resolve(directory), "--board", board], {
            cwd: directory,
            encoding: "utf8",
            maxBuffer: 64 * 1024,
            stdio: ["ignore", "pipe", "ignore"],
            timeout: 10000,
            windowsHide: true,
          });
          return { content: raw.trim() };
        } catch (err: any) {
          return { content: JSON.stringify({ error: err?.message || "failed to query cortex-ia work status" }) };
        }
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
          editor.add({
            name: watchTool.name,
            description: watchTool.description,
            input: watchTool.input,
            execute: async (input: any) => watchTool.execute(input),
          });
        }
      });
    } else if (typeof (ctx as any).tool?.register === "function") {
      (ctx as any).tool.register(auditTool);
      (ctx as any).tool.register(watchTool);
    }

    const cleanup = async () => {};
    (cleanup as any).dispose = cleanup;
    return cleanup;
  },
});

export default CortexPermissionFencePlugin;
