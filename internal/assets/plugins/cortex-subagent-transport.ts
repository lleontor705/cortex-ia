import { type Plugin } from "@opencode-ai/plugin";

/**
 * Isolated system instruction injected into child subagents to replace
 * inherited parent conversation history and prevent context bloat / role confusion.
 */
const TRANSPORT_ISOLATION_SYSTEM = [
  "You are an ephemeral OpenCode subagent operating under Cortex-IA work authority and Bounded Agent Contract.",
  "- Authority Invariant: You have no session lifecycle authority. NEVER call cortex_session_start, cortex_session_end, or cortex-ia board create.",
  "- File Leases: Before modifying any file with edit or write_to_file, you MUST hold an active file lease (cortex_ia_file_reserve). Stop immediately if authority expires.",
  "- Bounded Scope: Follow strictly the assigned <minion-contract> envelope and deliver a structured JSON receipt with phase_status, verification_verdict, and summary.",
  "- Contract SLA: Complete your objective within your allocated step and token budget. Do not loop repetitively without emitting verifiable progress.",
  "- Context Hygiene: Focus exclusively on your immediate objective; do not assume orchestrator routing duties."
].join("\n");

/**
 * CortexSubagentTransportPlugin provides runtime transport isolation and contract SLA bounding
 * for OpenCode subagents. It ensures child sessions spawned via the task tool receive a sanitized
 * system prompt, eliminating conversational residue from the parent orchestrator and enforcing strict
 * step limits to prevent runaway tool loops.
 */
export const CortexSubagentTransportPlugin: Plugin = async (ctx) => {
  const childSessions = new Set<string>();
  const sessionStepCounts = new Map<string, number>();
  const sessionStepLimits = new Map<string, number>();
  const DEFAULT_MAX_STEPS = 35;

  return {
    dispose: async () => {
      childSessions.clear();
      sessionStepCounts.clear();
      sessionStepLimits.clear();
    },

    event: async ({ event }) => {
      if (!event) return;
      const type = event.type || event.event || "";
      const info = (event.properties as any)?.info;

      // Register child subagent sessions spawned with a parentID
      if (type === "session.created" && info?.parentID && info?.id) {
        childSessions.add(info.id);
        sessionStepCounts.set(info.id, 0);
        sessionStepLimits.set(info.id, DEFAULT_MAX_STEPS);
      } else if (type === "session.deleted") {
        const id = info?.id;
        if (id) {
          childSessions.delete(id);
          sessionStepCounts.delete(id);
          sessionStepLimits.delete(id);
        }
      }
    },

    "experimental.chat.system.transform": async (input, output) => {
      if (!input?.sessionID || !output?.system) return;
      
      // If this session is an identified child subagent, replace inherited system instructions
      // with the sanitized Cortex-IA subagent isolation prompt.
      if (childSessions.has(input.sessionID)) {
        output.system.splice(0, output.system.length, TRANSPORT_ISOLATION_SYSTEM);
      }
    },

    "tool.execute.before": async (input, output) => {
      const toolName = (input?.tool || "").toLowerCase();
      const sessionID = input?.sessionID || "";

      // 1. Task dispatch gate: enforce non-empty prompt and detect contract limits
      if (toolName === "task") {
        const args = output?.args as Record<string, any> | undefined;
        const prompt = args?.prompt || args?.description || "";
        if (typeof prompt === "string" && prompt.trim().length === 0) {
          throw new Error("SUBAGENT_TRANSPORT_ERROR: task dispatch requires a non-empty prompt or envelope");
        }
        return;
      }

      // 2. Subagent step budget enforcement
      if (sessionID && childSessions.has(sessionID)) {
        const count = (sessionStepCounts.get(sessionID) || 0) + 1;
        sessionStepCounts.set(sessionID, count);
        const limit = sessionStepLimits.get(sessionID) || DEFAULT_MAX_STEPS;

        if (count > limit) {
          throw new Error(`AGENT_CONTRACT_EXCEEDED: subagent reached step budget limit (${limit}) without delivering completion receipt`);
        }
      }
    }
  };
};

export default CortexSubagentTransportPlugin;
