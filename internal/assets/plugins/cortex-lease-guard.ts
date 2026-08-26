import { type Plugin } from "@opencode-ai/plugin";
import { execFileSync } from "node:child_process";
import * as path from "node:path";

/**
 * cortex-lease-guard: Prevents destructive write collisions among concurrent background subagents.
 * Intercepts write/edit tool execution and ensures the caller holds a valid lease in cortex-ia.
 */
export const CortexLeaseGuardPlugin: Plugin = async (ctx) => {
  return {
    "tool.execute.before": async (input, output) => {
      const toolName = input?.tool?.toLowerCase() || "";
      
      // Only guard file mutation tools
      if (toolName !== "edit" && toolName !== "write_to_file" && toolName !== "write") {
        return;
      }

      const args = output?.args as Record<string, any> | undefined;
      const targetFile = args?.TargetFile || args?.targetFile || args?.path || args?.file || "";
      if (!targetFile || typeof targetFile !== "string") {
        return;
      }

      // Check if lease enforcement is enabled in environment or delegation config
      const enforceLeases = process.env.CORTEX_ENFORCE_LEASES === "true" || process.env.CORTEX_LEASES_STRICT === "1";
      if (!enforceLeases) {
        return;
      }

      const normTarget = path.normalize(targetFile).toLowerCase();

      try {
        // Query cortex-ia work list for active leases
        const out = execFileSync("cortex-ia", ["work", "list", "--json"], {
          encoding: "utf-8",
          stdio: ["ignore", "pipe", "ignore"],
          timeout: 2000
        });
        
        const data = JSON.parse(out);
        const activeLeases = data?.leases || [];
        
        // If the file is reserved by another task or not leased
        for (const lease of activeLeases) {
          const leasedPath = path.normalize(lease.path || "").toLowerCase();
          if (normTarget.includes(leasedPath) && lease.is_active && !lease.is_caller_owner) {
            throw new Error(`LEASE_COLLISION: File '${targetFile}' is currently locked under exclusive lease by task '${lease.task_id}' (Owner: ${lease.owner}). Modification denied to prevent concurrency conflicts.`);
          }
        }
      } catch (err: any) {
        if (err.message && err.message.includes("LEASE_COLLISION")) {
          throw err;
        }
        // If cortex-ia query failed or work list had no active locks, allow fallback
      }
    }
  };
};

export default CortexLeaseGuardPlugin;
