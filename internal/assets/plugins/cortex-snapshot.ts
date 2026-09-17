import { type Plugin, tool } from "@opencode-ai/plugin";
import { Buffer } from "node:buffer";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";

export const CortexSnapshotPlugin: Plugin = async () => ({
  tool: {
    cortex_ia_snapshot_read: tool({
      description: "Read one exact local Cortex observation through Cortex-IA's bounded streaming project export. Verifies identity and optional SHA-256; returns content and digest. The upstream still exports the project (64 MiB total, 2 MiB per record, 1 MiB selected content, 30 seconds); no project array is retained. Never substitutes for a remote MCP store.",
      args: {
        observation_id: tool.schema.number(), project: tool.schema.string(),
        expected_sha256: tool.schema.string().optional()
      },
      async execute(args) {
        if (!Number.isSafeInteger(args.observation_id) || args.observation_id < 1) throw new Error("observation_id must be a positive safe integer");
        if (!args.project.trim() || Buffer.byteLength(args.project, "utf8") > 512) throw new Error("project must contain 1-512 UTF-8 bytes");
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
        return JSON.stringify(snapshot);
      }
    })
  }
});

export default CortexSnapshotPlugin;
