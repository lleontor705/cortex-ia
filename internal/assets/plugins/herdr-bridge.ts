import { type Plugin, tool } from "@opencode-ai/plugin";
import { execFileSync, spawn } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";

interface DelegationConfigFile {
  version: string;
  delegation_enabled: boolean;
  use_herdr: boolean;
  herdr_settings: {
    auto_split: boolean;
    split_direction: string;
    timeout_seconds: number;
  };
  roles: Record<string, {
    delegate: boolean;
    cli: "agy" | "claude" | "native";
    command?: string;
    args?: string[];
  }>;
}

function logDelegation(workspaceDir: string, level: "INFO" | "WARN" | "ERROR", msg: string, data?: any) {
  const timestamp = new Date().toISOString();
  const line = `[${timestamp}] [${level}] ${msg}${data ? " | " + (typeof data === "string" ? data : JSON.stringify(data)) : ""}\n`;
  try {
    const localLog = path.resolve(workspaceDir, ".cortex-delegation.log");
    fs.appendFileSync(localLog, line, "utf-8");
  } catch {}
  try {
    const home = process.env.HOME || process.env.USERPROFILE || "";
    const globalLog = path.resolve(home, ".config", "opencode", "cortex-delegation.log");
    fs.appendFileSync(globalLog, line, "utf-8");
  } catch {}
}

function resolveBinary(cmd: string): string {
  const home = process.env.USERPROFILE || process.env.HOME || "";
  const localAppData = process.env.LOCALAPPDATA || path.join(home, "AppData", "Local");
  const appData = process.env.APPDATA || path.join(home, "AppData", "Roaming");

  if (cmd === "herdr") {
    const candidates = [
      "herdr",
      path.join(localAppData, "Programs", "Herdr", "bin", "herdr.exe"),
      path.join(localAppData, "Programs", "herdr", "bin", "herdr.exe"),
      path.join(home, ".cargo", "bin", "herdr.exe"),
      "/usr/local/bin/herdr",
      "/usr/bin/herdr"
    ];
    for (const c of candidates) {
      if (c === "herdr") {
        try {
          execFileSync("herdr", ["--version"], { stdio: "ignore" });
          return "herdr";
        } catch {}
      } else if (fs.existsSync(c)) {
        return c;
      }
    }
  }

  if (cmd === "agy") {
    const candidates = [
      "agy",
      path.join(localAppData, "agy", "bin", "agy.exe"),
      path.join(appData, "Antigravity", "bin", "agy-node.cmd"),
      path.join(localAppData, "Programs", "agy", "bin", "agy.exe"),
      path.join(home, ".agy", "bin", "agy"),
      "/usr/local/bin/agy",
      "/usr/bin/agy"
    ];
    for (const c of candidates) {
      if (c === "agy") {
        try {
          execFileSync("agy", ["--version"], { stdio: "ignore" });
          return "agy";
        } catch {}
      } else if (fs.existsSync(c)) {
        return c;
      }
    }
  }

  if (cmd === "claude") {
    const candidates = [
      "claude",
      path.join(appData, "npm", "claude.cmd"),
      path.join(localAppData, "Programs", "claude", "bin", "claude.exe"),
      path.join(home, ".claude", "bin", "claude"),
      "/usr/local/bin/claude",
      "/usr/bin/claude"
    ];
    for (const c of candidates) {
      if (c === "claude") {
        try {
          execFileSync("claude", ["--version"], { stdio: "ignore" });
          return "claude";
        } catch {}
      } else if (fs.existsSync(c)) {
        return c;
      }
    }
  }

  return cmd;
}

function loadConfig(): DelegationConfigFile | null {
  const home = process.env.HOME || process.env.USERPROFILE || "";
  const configPath = path.resolve(home, ".config", "opencode", "cortex-delegation.json");
  if (!fs.existsSync(configPath)) {
    return null;
  }
  try {
    return JSON.parse(fs.readFileSync(configPath, "utf-8"));
  } catch {
    return null;
  }
}

export const CortexDelegationBridge: Plugin = async (ctx) => {
  return {
    tool: {
      cortex_delegate_role: tool({
        description: "Routes a workflow phase (implement, investigate, reviewer, planner) to external CLI (agy or claude) or native OpenCode subagent based on cortex-delegation.json.",
        args: {
          role: tool.schema.enum(["implement", "investigate", "reviewer", "planner"]).describe("The workflow role to execute"),
          task_id: tool.schema.string().describe("Task identifier"),
          objective: tool.schema.string().describe("High level goal"),
          allowed_files: tool.schema.array(tool.schema.string()).optional().describe("Allowed files"),
          acceptance_checks: tool.schema.array(tool.schema.string()).optional().describe("Checks to verify"),
          context_data: tool.schema.string().optional().describe("Additional context"),
        },
        async execute(args, context) {
          logDelegation(context.directory, "INFO", `Tool cortex_delegate_role called for role '${args.role}'`, { task_id: args.task_id, objective: args.objective });

          const config = loadConfig();

          // 1. Si no hay configuración o la delegación está apagada -> Fallback nativo
          if (!config || !config.delegation_enabled) {
            logDelegation(context.directory, "INFO", `Delegation disabled in config -> Fallback to native`);
            return JSON.stringify({ delegated: false, reason: "DELEGATION_DISABLED", action: "USE_NATIVE_SUBAGENT" }, null, 2);
          }

          const roleCfg = config.roles?.[args.role];
          if (!roleCfg || !roleCfg.delegate || roleCfg.cli === "native") {
            logDelegation(context.directory, "INFO", `Role '${args.role}' configured as native -> Fallback to native`);
            return JSON.stringify({ delegated: false, reason: "ROLE_NATIVE", action: "USE_NATIVE_SUBAGENT" }, null, 2);
          }

          const targetCLI = roleCfg.cli; // "agy" | "claude"
          const rawCommand = roleCfg.command || (targetCLI === "agy" ? "agy" : "claude");
          const cliBin = resolveBinary(rawCommand);
          const defaultArgs = targetCLI === "agy"
            ? ["--dangerously-skip-permissions", "-p"]
            : (targetCLI === "claude" ? ["--dangerously-skip-permissions", "-p"] : ["-p"]);
          let customArgs = roleCfg.args && roleCfg.args.length > 0 ? [...roleCfg.args] : defaultArgs;
          if (targetCLI === "agy" && !customArgs.includes("--dangerously-skip-permissions")) {
            customArgs = ["--dangerously-skip-permissions", ...customArgs];
          }

          logDelegation(context.directory, "INFO", `Resolved CLI binary for ${targetCLI}: '${cliBin}' with args: ${JSON.stringify(customArgs)}`);

          // Build structured prompt payload
          const promptLines = [
            `# TASK DELEGATION: ${args.role.toUpperCase()} (Task ID: ${args.task_id})`,
            `## Objective:`,
            args.objective,
          ];
          if (args.allowed_files && args.allowed_files.length > 0) {
            promptLines.push(`\n## Allowed Files:\n${args.allowed_files.map(f => "- " + f).join("\n")}`);
          }
          if (args.acceptance_checks && args.acceptance_checks.length > 0) {
            promptLines.push(`\n## Acceptance Checks:\n${args.acceptance_checks.map(c => "- " + c).join("\n")}`);
          }
          if (args.context_data) {
            promptLines.push(`\n## Context:\n${args.context_data}`);
          }
          promptLines.push(`\n## Available MCP Tools:`);
          promptLines.push(`- **Cortex MCP**: Use \`cortex_save_observation\` / \`cortex_search_hybrid\` to record evidence and query memory.`);
          promptLines.push(`- **ForgeSpec MCP**: Use \`forgespec_task_transition\` to update task status for task \`${args.task_id}\`.`);
          promptLines.push(`\n## Expected Outcome:`);
          promptLines.push(`Execute the objective within the allowed constraints. Return a structured receipt with \`phase_status\`, \`task_status\`, and \`verification_verdict\`.`);
          const promptPayload = promptLines.join("\n");

          // Save prompt payload to a temporary task file for persistence
          const promptTmpFile = path.resolve(context.directory, `.task-${args.task_id}-${args.role}-prompt.md`);
          try {
            fs.writeFileSync(promptTmpFile, promptPayload, "utf-8");
            logDelegation(context.directory, "INFO", `Wrote prompt file: ${promptTmpFile}`);
          } catch (err: any) {
            logDelegation(context.directory, "WARN", `Could not write prompt file: ${err.message}`);
          }

          // Generate runner script to avoid escaping/quoting issues in terminals
          const isWin = process.platform === "win32";
          const runScriptFile = path.resolve(context.directory, `.task-${args.task_id}-${args.role}-run.${isWin ? "ps1" : "sh"}`);

          if (isWin) {
            const ps1Content = [
              `# Cortex Delegation Runner`,
              `$ErrorActionPreference = "Continue"`,
              `$prompt = Get-Content -Path "${promptTmpFile.replace(/"/g, '`"')}" -Raw`,
              `Write-Host "=================================================" -ForegroundColor Cyan`,
              `Write-Host "  CORTEX DELEGATION: ${args.role.toUpperCase()} -> ${targetCLI.toUpperCase()}" -ForegroundColor Cyan`,
              `Write-Host "  Task ID: ${args.task_id}" -ForegroundColor Cyan`,
              `Write-Host "=================================================" -ForegroundColor Cyan`,
              `& "${cliBin.replace(/"/g, '`"')}" ${customArgs.join(" ")} $prompt`,
              `Write-Host "` + "\n" + `=================================================" -ForegroundColor Green`,
              `Write-Host "  CORTEX DELEGATION FINISHED" -ForegroundColor Green`,
              `Write-Host "=================================================" -ForegroundColor Green`
            ].join("\r\n");
            fs.writeFileSync(runScriptFile, ps1Content, "utf-8");
          } else {
            const shContent = [
              `#!/usr/bin/env bash`,
              `set -e`,
              `echo "================================================="`,
              `echo "  CORTEX DELEGATION: ${args.role.toUpperCase()} -> ${targetCLI.toUpperCase()}"`,
              `echo "  Task ID: ${args.task_id}"`,
              `echo "================================================="`,
              `prompt=$(cat "${promptTmpFile}")`,
              `"${cliBin}" ${customArgs.join(" ")} "$prompt"`,
              `echo "================================================="`,
              `echo "  CORTEX DELEGATION FINISHED"`,
              `echo "================================================="`
            ].join("\n");
            fs.writeFileSync(runScriptFile, shContent, { encoding: "utf-8", mode: 0o755 });
          }
          logDelegation(context.directory, "INFO", `Created runner script: ${runScriptFile}`);

          // 2. Si se usa Herdr Workspace Multiplexer
          if (config.use_herdr) {
            const herdrBin = resolveBinary("herdr");
            logDelegation(context.directory, "INFO", `Using Herdr with binary '${herdrBin}'`);
            try {
              const splitDir = config.herdr_settings?.split_direction || "right";
              logDelegation(context.directory, "INFO", `Executing herdr pane split --direction ${splitDir}`);

              const splitOutput = execFileSync(herdrBin, [
                "pane", "split",
                "--direction", splitDir,
                "--cwd", context.directory,
                "--no-focus"
              ], {
                encoding: "utf-8",
                stdio: ["ignore", "pipe", "pipe"]
              });

              logDelegation(context.directory, "INFO", `Herdr split output: ${splitOutput.trim()}`);

              let paneId = "";
              try {
                const parsed = JSON.parse(splitOutput.trim());
                paneId = parsed?.result?.pane?.pane_id || parsed?.result?.pane_id || "";
              } catch {}

              if (paneId) {
                logDelegation(context.directory, "INFO", `Obtained pane_id '${paneId}' from Herdr`);

                const runCmd = isWin
                  ? `powershell -NoProfile -ExecutionPolicy Bypass -File "${runScriptFile}"`
                  : `bash "${runScriptFile}"`;

                logDelegation(context.directory, "INFO", `Executing in Herdr pane '${paneId}': ${runCmd}`);

                execFileSync(herdrBin, ["pane", "run", paneId, runCmd], {
                  encoding: "utf-8",
                  stdio: "pipe"
                });

                logDelegation(context.directory, "INFO", `Successfully dispatched execution into Herdr pane '${paneId}'`);

                return JSON.stringify({
                  delegated: true,
                  target: targetCLI,
                  multiplexer: "herdr",
                  status: "SPAWNED_HERDR_PANE",
                  pane_id: paneId,
                  prompt_file: promptTmpFile,
                  run_script: runScriptFile,
                  message: `Role '${args.role}' successfully delegated to ${targetCLI.toUpperCase()} in Herdr pane '${paneId}'.`
                }, null, 2);
              } else {
                logDelegation(context.directory, "WARN", `Could not determine pane_id from split output`);
              }
            } catch (err: any) {
              logDelegation(context.directory, "ERROR", `Herdr pane execution failed: ${err.message}`, { stack: err.stack });
              // Fallback to direct process if Herdr fails
            }
          }

          // 3. Delegación directa como subproceso CLI
          logDelegation(context.directory, "INFO", `Attempting direct subprocess delegation`);
          try {
            const proc = isWin
              ? spawn("powershell", ["-NoProfile", "-ExecutionPolicy", "Bypass", "-File", runScriptFile], {
                  cwd: context.directory,
                  stdio: ["ignore", "pipe", "pipe"]
                })
              : spawn("bash", [runScriptFile], {
                  cwd: context.directory,
                  stdio: ["ignore", "pipe", "pipe"]
                });

            let stdout = "";
            let stderr = "";
            proc.stdout?.on("data", (d) => { stdout += d.toString(); });
            proc.stderr?.on("data", (d) => { stderr += d.toString(); });

            await new Promise<void>((resolve, reject) => {
              proc.on("close", (code) => {
                if (code === 0) resolve();
                else reject(new Error(`Process exited with code ${code}: ${stderr || stdout}`));
              });
              proc.on("error", reject);
            });

            logDelegation(context.directory, "INFO", `Direct subprocess completed successfully`);

            return JSON.stringify({
              delegated: true,
              target: targetCLI,
              multiplexer: "direct_process",
              status: "COMPLETED",
              output: stdout.slice(-4000)
            }, null, 2);
          } catch (err: any) {
            logDelegation(context.directory, "ERROR", `Direct subprocess failed: ${err.message}`);
            return JSON.stringify({
              delegated: false,
              error: err.message,
              action: "USE_NATIVE_SUBAGENT",
              prompt_file: promptTmpFile
            }, null, 2);
          }
        }
      })
    }
  };
};

export default CortexDelegationBridge;
