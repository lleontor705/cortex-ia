import { type Plugin } from "@opencode-ai/plugin";
import * as fs from "node:fs";
import * as path from "node:path";

interface LocalSkill {
  name: string;
  path: string;
  description?: string;
}

function discoverRepoSkills(directory: string): LocalSkill[] {
  const discovered: LocalSkill[] = [];
  const searchCandidates = [
    path.join(directory, ".cortex-ia", "skills"),
    path.join(directory, "skills"),
  ];

  for (const candidateDir of searchCandidates) {
    if (!fs.existsSync(candidateDir)) continue;
    try {
      const entries = fs.readdirSync(candidateDir, { withFileTypes: true });
      for (const entry of entries) {
        if (!entry.isDirectory()) continue;
        const skillFile = path.join(candidateDir, entry.name, "SKILL.md");
        if (fs.existsSync(skillFile)) {
          let description = "";
          try {
            const content = fs.readFileSync(skillFile, "utf-8");
            const descMatch = content.match(/^description:\s*["']?([^"'\r\n]+)["']?/m);
            if (descMatch) description = descMatch[1].trim();
          } catch {}
          discovered.push({
            name: entry.name,
            path: skillFile,
            description,
          });
        }
      }
    } catch {}
  }

  return discovered;
}

/**
 * CortexSkillDiscoveryPlugin automatically detects repository-local custom skills
 * located in ./.cortex-ia/skills/ or ./skills/ at startup, enabling dynamic skill loading
 * without rebuilding or reinstalling global Cortex-IA assets.
 */
export const CortexSkillDiscoveryPlugin: Plugin = async (ctx) => {
  const repoSkills = discoverRepoSkills(ctx.directory);

  if (repoSkills.length > 0) {
    console.info(`[cortex-skill-discovery] Discovered ${repoSkills.length} repository skill(s): ${repoSkills.map(s => s.name).join(", ")}`);
  }

  return {
    "experimental.chat.system.transform": async (_input, output) => {
      if (repoSkills.length === 0) return;

      const skillSummary = repoSkills
        .map((s) => `- \`${s.name}\`: ${s.description || "Repository skill"} (Path: \`${s.path}\`)`)
        .join("\n");

      const note = `\n\n### Repository-Local Custom Skills Available:\n${skillSummary}\n` +
        `The orchestrator and subagents may load these project-specific skills directly from their workspace paths.\n`;

      if (output.system.length > 0) {
        output.system[output.system.length - 1] += note;
      } else {
        output.system.push(note);
      }
    },
  };
};

export default CortexSkillDiscoveryPlugin;
