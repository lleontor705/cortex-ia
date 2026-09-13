import { type Plugin } from "@opencode-ai/plugin";
import * as fs from "node:fs";
import * as path from "node:path";

export type SkillOrigin = "explicit" | "host-installed" | "repository" | "embedded-source";

export interface ResolvedSkill {
  name: string;
  path: string;
  origin: SkillOrigin;
  precedence: number;
  description?: string;
}

export interface SkillDiscoveryOptions {
  directory?: string;
  hostSkillsDirs?: string[];
  repoSkillsDirs?: string[];
  embeddedSkillsDirs?: string[];
}

export function canonicalPath(p: string): string {
  if (!p || typeof p !== "string" || !p.trim()) {
    throw new Error("CONTEXT_RESOLUTION_ERROR: invalid locator path");
  }
  return path.resolve(p).replaceAll(path.sep, "/");
}

function parseSkillDescription(skillFilePath: string): string {
  try {
    const content = fs.readFileSync(skillFilePath, "utf-8");
    const descMatch = content.match(/^description:\s*["']?([^"'\r\n]+)["']?/m);
    if (descMatch) return descMatch[1].trim();
  } catch {}
  return "";
}

function scanDirForSkills(dir: string, origin: SkillOrigin, precedence: number): Map<string, ResolvedSkill> {
  const map = new Map<string, ResolvedSkill>();
  if (!fs.existsSync(dir)) return map;

  try {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
      if (!entry.isDirectory()) continue;
      const skillFile = path.join(dir, entry.name, "SKILL.md");
      if (fs.existsSync(skillFile)) {
        const canonical = canonicalPath(skillFile);
        const name = entry.name;
        if (map.has(name) && map.get(name)!.path.toLowerCase() !== canonical.toLowerCase()) {
          throw new Error(
            `CONTEXT_RESOLUTION_ERROR: same-precedence collision for skill '${name}': candidates [${map.get(name)!.path}, ${canonical}]`
          );
        }
        map.set(name, {
          name,
          path: canonical,
          origin,
          precedence,
          description: parseSkillDescription(skillFile),
        });
      }
    }
  } catch (e: any) {
    if (e.message?.startsWith("CONTEXT_RESOLUTION_ERROR:")) throw e;
  }
  return map;
}

export function discoverInventory(
  dirs: string[],
  origin: SkillOrigin,
  precedence: number
): Map<string, ResolvedSkill> {
  const combined = new Map<string, ResolvedSkill>();
  for (const dir of dirs) {
    const skills = scanDirForSkills(dir, origin, precedence);
    for (const [name, skill] of skills) {
      if (combined.has(name) && combined.get(name)!.path.toLowerCase() !== skill.path.toLowerCase()) {
        throw new Error(
          `CONTEXT_RESOLUTION_ERROR: same-precedence collision for skill '${name}': candidates [${combined.get(name)!.path}, ${skill.path}]`
        );
      }
      combined.set(name, skill);
    }
  }
  return combined;
}

export function resolveLocator(locator: string, _options: SkillDiscoveryOptions = {}): ResolvedSkill {
  if (!locator || typeof locator !== "string" || !locator.trim()) {
    throw new Error("CONTEXT_RESOLUTION_ERROR: invalid locator path");
  }
  const canonical = canonicalPath(locator);
  if (!fs.existsSync(canonical)) {
    throw new Error(`CONTEXT_RESOLUTION_ERROR: explicit locator not found or unreadable: ${locator}`);
  }
  try {
    fs.accessSync(canonical, fs.constants.R_OK);
  } catch {
    throw new Error(`CONTEXT_RESOLUTION_ERROR: explicit locator not found or unreadable: ${locator}`);
  }

  const skillName = path.basename(path.dirname(canonical));
  return {
    name: skillName,
    path: canonical,
    origin: "explicit",
    precedence: 0,
    description: parseSkillDescription(canonical),
  };
}

export function resolveSkill(name: string, options: SkillDiscoveryOptions = {}): ResolvedSkill {
  if (!name || typeof name !== "string" || !name.trim()) {
    throw new Error("CONTEXT_RESOLUTION_ERROR: skill name is required");
  }

  const baseDir = options.directory || process.cwd();
  const home = process.env.USERPROFILE || process.env.HOME || "";

  const hostDirs = options.hostSkillsDirs || [
    path.join(home, ".cortex-ia", "skills"),
    path.join(home, ".gemini", "config", "plugins", "cortex-ia", "skills"),
  ];
  const repoDirs = options.repoSkillsDirs || [
    path.join(baseDir, ".cortex-ia", "skills"),
    path.join(baseDir, "skills"),
  ];
  const embeddedDirs = options.embeddedSkillsDirs || [
    path.join(baseDir, "internal", "assets", "skills"),
  ];

  // 1. Host-installed (precedence 1)
  const hostSkills = discoverInventory(hostDirs, "host-installed", 1);
  if (hostSkills.has(name)) return hostSkills.get(name)!;

  // 2. Repository (precedence 2)
  const repoSkills = discoverInventory(repoDirs, "repository", 2);
  if (repoSkills.has(name)) return repoSkills.get(name)!;

  // 3. Embedded-source (precedence 3)
  const embeddedSkills = discoverInventory(embeddedDirs, "embedded-source", 3);
  if (embeddedSkills.has(name)) return embeddedSkills.get(name)!;

  throw new Error(`CONTEXT_RESOLUTION_ERROR: skill '${name}' not found in any inventory`);
}

export function getAllDiscoveredSkills(options: SkillDiscoveryOptions = {}): ResolvedSkill[] {
  const baseDir = options.directory || process.cwd();
  const home = process.env.USERPROFILE || process.env.HOME || "";

  const hostDirs = options.hostSkillsDirs || [
    path.join(home, ".cortex-ia", "skills"),
    path.join(home, ".gemini", "config", "plugins", "cortex-ia", "skills"),
  ];
  const repoDirs = options.repoSkillsDirs || [
    path.join(baseDir, ".cortex-ia", "skills"),
    path.join(baseDir, "skills"),
  ];
  const embeddedDirs = options.embeddedSkillsDirs || [
    path.join(baseDir, "internal", "assets", "skills"),
  ];

  const hostSkills = discoverInventory(hostDirs, "host-installed", 1);
  const repoSkills = discoverInventory(repoDirs, "repository", 2);
  const embeddedSkills = discoverInventory(embeddedDirs, "embedded-source", 3);

  const merged = new Map<string, ResolvedSkill>();
  for (const [name, s] of embeddedSkills) merged.set(name, s);
  for (const [name, s] of repoSkills) merged.set(name, s);
  for (const [name, s] of hostSkills) merged.set(name, s);

  return [...merged.values()].sort((a, b) => a.name.localeCompare(b.name));
}

export const CortexSkillDiscoveryPlugin: Plugin = async (ctx) => {
  const allSkills = getAllDiscoveredSkills({ directory: ctx.directory });
  const repoOnly = allSkills.filter(s => s.origin === "repository");

  return {
    "experimental.chat.system.transform": async (_input, output) => {
      if (repoOnly.length === 0) return;

      const skillSummary = repoOnly
        .map((s) => `- \`${s.name}\` [${s.origin}, p${s.precedence}]: ${s.description || "Repository skill"} (Path: \`${s.path}\`)`)
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
