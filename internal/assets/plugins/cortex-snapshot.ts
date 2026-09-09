import { type Plugin, tool } from "@opencode-ai/plugin";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";

// Local Cortex exports structured content; never parse display text or search previews.
export const CortexSnapshotPlugin: Plugin = async () => ({
  tool: {
    cortex_ia_snapshot_read: tool({
      description: "Retrieve one full observation from the local Cortex CLI and hash its exact UTF-8 content in the same operation. Local CLI store only: never substitute this for a remote MCP store. Returns content, project, observation_id, transport, sha256 and byte_length. Export bounded to 8 MiB, content to 1 MiB; no files written.",
      args: {
        observation_id: tool.schema.number(),
        project: tool.schema.string(),
        expected_sha256: tool.schema.string().optional()
      },
      async execute(args) {
        if (!Number.isSafeInteger(args.observation_id) || args.observation_id < 1) throw new Error("observation_id must be a positive safe integer");
        if (!args.project.trim() || args.project.length > 512) throw new Error("project must contain 1-512 characters");
        if (args.expected_sha256 !== undefined && !/^[a-f0-9]{64}$/.test(args.expected_sha256)) throw new Error("expected_sha256 must be a lowercase SHA-256 digest");
        let raw: string | undefined;
        let lastErr: unknown;
        for (let attempt = 1; attempt <= 3; attempt++) {
          try {
            raw = execFileSync("cortex", ["export", "--project", args.project], {
              encoding: "utf8", maxBuffer: 8 * 1024 * 1024, timeout: 30000, windowsHide: true
            });
            break;
          } catch (err) {
            lastErr = err;
            if (attempt < 3) {
              await new Promise((resolve) => setTimeout(resolve, attempt * 200));
            }
          }
        }
        if (raw === undefined) {
          const detail = (lastErr as any)?.message ? `: ${(lastErr as any).message}` : "";
          throw new Error(`Local Cortex export failed${detail}, timed out, or exceeded 8 MiB; snapshot remains unverified`);
        }
        let rows: unknown;
        try { rows = JSON.parse(raw); } catch { throw new Error("Local Cortex export is not valid structured JSON"); }
        if (!Array.isArray(rows)) throw new Error("Local Cortex export must be an observation array");
        const matches = rows.filter(row => row?.id === args.observation_id);
        if (matches.length !== 1) throw new Error("Observation missing or ambiguous in bounded local export; snapshot remains unverified");
        const observation = matches[0];
        if (observation.project !== args.project || typeof observation.content !== "string") throw new Error("Snapshot project or content does not match the requested identity");
        const bytes = Buffer.from(observation.content, "utf8");
        if (bytes.length > 1048576 || bytes.toString("utf8") !== observation.content) throw new Error("Snapshot content exceeds 1 MiB or contains malformed Unicode");
        const sha256 = createHash("sha256").update(bytes).digest("hex");
        if (args.expected_sha256 !== undefined && args.expected_sha256 !== sha256) throw new Error("Snapshot digest mismatch; contract validation failed");
        return JSON.stringify({ transport: "local_cortex_cli", project: args.project,
          observation_id: args.observation_id, content: observation.content,
          sha256, byte_length: bytes.length });
      }
    })
  }
});

export default CortexSnapshotPlugin;
