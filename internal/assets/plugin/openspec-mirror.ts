/**
 * OpenSpec Mirror — OpenCode Plugin for Cortex-IA
 *
 * Automatically mirrors SDD contracts and task DAGs generated via ForgeSpec MCP
 * into human-readable Markdown files under `openspec/changes/<change_name>/`.
 *
 * Innovations:
 * - Auto-generates visual Mermaid sequence/flowchart diagrams from requirements & scenarios.
 * - Adds a CI/CD-ready Pull Request Summary block.
 */

import type { Plugin } from "@opencode-ai/plugin"
import { mkdir, writeFile } from "fs/promises"
import path from "path"

function safeJson(val: unknown): string {
  try {
    return JSON.stringify(val, null, 2)
  } catch {
    return String(val)
  }
}

function generateMermaidFromScenarios(changeName: string, requirements: unknown[]): string {
  if (!Array.isArray(requirements) || requirements.length === 0) return ""

  const lines: string[] = [
    "```mermaid",
    "sequenceDiagram",
    `    autonumber`,
    `    actor User`,
    `    participant System as ${changeName} System`,
    `    participant Oracle as Test Oracle / Validator`,
  ]

  let hasScenarios = false
  for (const req of requirements.slice(0, 5)) {
    if (typeof req === "object" && req !== null) {
      const title = (req as any).title ?? (req as any).id ?? "Requirement"
      lines.push(`    Note over User,System: ${title}`)
      lines.push(`    User->>System: Action / Input`)
      lines.push(`    System->>Oracle: Evaluate Invariants`)
      lines.push(`    Oracle-->>System: Validation PASS (Exit 0)`)
      lines.push(`    System-->>User: Observable Response`)
      hasScenarios = true
    }
  }

  lines.push("```\n")
  return hasScenarios ? `## Visual Scenario Flow\n\n${lines.join("\n")}\n` : ""
}

function generatePRSummaryBlock(changeName: string, artifactType: string): string {
  return `
> [!NOTE]
> ### 📋 CI/CD Pull Request Summary
> - **Change**: \`${changeName}\`
> - **Contract Type**: \`${artifactType.toUpperCase()}\`
> - **Authority**: ForgeSpec Direct-v1 & Cortex Persistent Memory
> - **Mirror Status**: Synced automatically by \`openspec-mirror\`
`
}

export const OpenSpecMirrorPlugin: Plugin = async ({ directory }) => {
  return {
    "tool.execute.after": async (input, output) => {
      try {
        const toolName = input.tool.toLowerCase()

        // 1. Mirror ForgeSpec SDD Artifacts (proposal, spec, design)
        if (toolName === "forgespec_sdd_save" || toolName === "sdd_save") {
          const args = input.args as {
            change_name?: string
            artifact_type?: string
            data?: Record<string, unknown> | string
            schema_version?: string
          }

          const changeName = args.change_name ?? "current"
          const artifactType = (args.artifact_type ?? "spec").toLowerCase()
          const data = args.data

          if (changeName && data) {
            const targetDir = path.join(directory, "openspec", "changes", changeName)
            await mkdir(targetDir, { recursive: true })

            const fileName = `${artifactType}.md`
            const filePath = path.join(targetDir, fileName)

            let mdContent = ""
            if (typeof data === "string") {
              mdContent = `# ${artifactType.toUpperCase()} — ${changeName}\n\n${data}\n\n${generatePRSummaryBlock(changeName, artifactType)}`
            } else if (typeof data === "object" && data !== null) {
              const title = (data.title as string) ?? `${artifactType.toUpperCase()} — ${changeName}`
              const overview = (data.overview as string) ?? (data.summary as string) ?? ""
              mdContent = `# ${title}\n\n${overview ? overview + "\n\n" : ""}`

              if (data.requirements && Array.isArray(data.requirements)) {
                mdContent += `## Requirements\n\n`
                for (const req of data.requirements) {
                  mdContent += `- ${typeof req === "string" ? req : safeJson(req)}\n`
                }
                mdContent += "\n"

                // Auto-generate Mermaid Diagram from requirements
                const mermaidDiagram = generateMermaidFromScenarios(changeName, data.requirements)
                if (mermaidDiagram) {
                  mdContent += mermaidDiagram
                }
              }

              if (data.acceptance_criteria && Array.isArray(data.acceptance_criteria)) {
                mdContent += `## Acceptance Criteria\n\n`
                for (const ac of data.acceptance_criteria) {
                  mdContent += `- [ ] ${typeof ac === "string" ? ac : safeJson(ac)}\n`
                }
                mdContent += "\n"
              }

              mdContent += generatePRSummaryBlock(changeName, artifactType)
              mdContent += `\n---\n*Mirrored from ForgeSpec SDD contract at ${new Date().toISOString()}*\n`
            }

            if (mdContent) {
              await writeFile(filePath, mdContent, "utf-8")
              console.info(`[openspec-mirror] Mirrored ${artifactType} with visual flow to ${filePath}`)
            }
          }
        }

        // 2. Mirror ForgeSpec Tasks (tasks.md)
        if (toolName === "forgespec_tb_add_task" || toolName === "tb_add_task") {
          const args = input.args as {
            board_id?: string
            task_id?: string
            title?: string
            description?: string
          }

          const boardId = args.board_id ?? "default"
          if (args.task_id && args.title) {
            const targetDir = path.join(directory, "openspec", "changes", boardId)
            await mkdir(targetDir, { recursive: true })
            const filePath = path.join(targetDir, "tasks.md")

            const taskEntry = `- [ ] **${args.task_id}**: ${args.title}\n  - ${args.description ?? "No description"}\n`
            let existing = ""
            try {
              const file = Bun.file(filePath)
              if (await file.exists()) {
                existing = await file.text()
              }
            } catch {}

            if (!existing) {
              existing = `# Tasks — ${boardId}\n\n`
            }

            if (!existing.includes(`**${args.task_id}**`)) {
              existing += taskEntry
              await writeFile(filePath, existing, "utf-8")
              console.info(`[openspec-mirror] Added task ${args.task_id} to ${filePath}`)
            }
          }
        }
      } catch (err) {
        console.warn(`[openspec-mirror] Mirroring skipped or failed:`, err)
      }
    },
  }
}

export default OpenSpecMirrorPlugin
