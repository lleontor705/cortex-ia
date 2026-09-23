// OpenCode Plugin helper ensuring default export is a valid plugin definition object for v1 and v2
export const Plugin = {
  define: <T extends { id: string; setup?: (ctx: any) => Promise<any> | any; server?: (ctx: any) => Promise<any> | any }>(def: T): T => {
    if (!def.server && def.setup) {
      def.server = def.setup;
    }
    return def;
  },
};

import { Buffer } from "node:buffer";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";

export const CortexSnapshotPlugin = Plugin.define({
  id: "cortex-snapshot",
  async setup(ctx: any) {
    const toolDef = {
      name: "cortex_ia_snapshot_read",
      description: "Read one exact local Cortex observation through Cortex-IA's bounded streaming project export. Verifies identity and optional SHA-256; returns content and digest. The upstream still exports the project (64 MiB total, 2 MiB per record, 1 MiB selected content, 30 seconds); no project array is retained. Never substitutes for a remote MCP store.",
      input: {
        type: "object",
        properties: {
          observation_id: { type: "integer", description: "Positive safe integer observation ID" },
          project: { type: "string", description: "Project identifier (1-512 UTF-8 bytes)" },
          expected_sha256: { type: "string", description: "Optional lowercase SHA-256 digest" },
        },
        required: ["observation_id", "project"],
        additionalProperties: false,
      },
      async execute(args: any) {
        if (!Number.isSafeInteger(args.observation_id) || args.observation_id < 1) throw new Error("observation_id must be a positive safe integer");
        if (!args.project?.trim() || Buffer.byteLength(args.project, "utf8") > 512) throw new Error("project must contain 1-512 UTF-8 bytes");
        if (args.expected_sha256 !== undefined && !/^[a-f0-9]{64}$/.test(args.expected_sha256)) throw new Error("expected_sha256 must be a lowercase SHA-256 digest");
        const command = ["snapshot", "read", "--project", args.project, "--id", String(args.observation_id)];
        if (args.expected_sha256 !== undefined) command.push("--expected-sha256", args.expected_sha256);
        let raw: string;
        try {
          raw = execFileSync("cortex-ia", command, { encoding: "utf8", maxBuffer: 8 * 1024 * 1024, timeout: 35000, windowsHide: true });
        } catch {
          throw new Error("Local Cortex snapshot failed, timed out or exceeded bounds; no verified snapshot available");
        }
        let snapshot: any;
        try { snapshot = JSON.parse(raw); } catch { throw new Error("Invalid structured Cortex snapshot"); }
        if (snapshot?.transport !== "local_cortex_cli" || snapshot.project !== args.project || snapshot.observation_id !== args.observation_id || typeof snapshot.content !== "string") throw new Error("Snapshot identity mismatch");
        const bytes = Buffer.from(snapshot.content, "utf8");
        if (bytes.length > 1048576 || bytes.toString("utf8") !== snapshot.content || snapshot.byte_length !== bytes.length) throw new Error("Invalid snapshot content or byte length");
        const digest = createHash("sha256").update(bytes).digest("hex");
        if (snapshot.sha256 !== digest || (args.expected_sha256 !== undefined && args.expected_sha256 !== digest)) throw new Error("Snapshot digest mismatch");
        return { content: JSON.stringify(snapshot) };
      }
    };

    if (ctx?.tool?.transform) {
      await ctx.tool.transform((editor: any) => {
        editor.add(toolDef);
      });
    }

    const cleanup = async () => {};
    (cleanup as any).dispose = cleanup;
    (cleanup as any).tool = {
      cortex_ia_snapshot_read: toolDef,
    };
    return cleanup;
  }
});

export default CortexSnapshotPlugin;
