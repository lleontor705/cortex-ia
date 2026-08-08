package sdd

import (
	"github.com/lleontor705/cortex-ia/internal/components/filemerge"
)

// OrganicRoutingDirective is the canonical system prompt section injected by cortex-ia
// into target agent system prompts (e.g. ~/.claude/CLAUDE.md, opencode.json).
//
// The external AI agent runtime executes this directive during conversation to organically
// select the smallest useful implementation route (Direct Inline, Delegated Direct, or Optional SDD).
const OrganicRoutingDirective = `## Organic Implementation Routing

Ask for the outcome. Act as the orchestrator to dynamically select the smallest useful implementation route:

1. **Direct Inline (1–3 files)**:
   - Use when deciding or verifying requires **1–3 files**, or the change is **one mechanical, already-understood file** with no research or unresolved design decision.
   - Keep the action direct inline. Do NOT create SDD proposal/spec/design/task files, phase attempts, or synthetic SDD state.

2. **Delegated Direct (4+ files)**:
   - Use when understanding requires **4+ files**, reading prepares a write, broad research is needed, or a writer must change **2+ non-trivial files**.
   - Delegate the action to a focused sub-agent without creating SDD state or phase artifacts.

3. **Optional SDD**:
   - Use ONLY when the work has substantial ambiguity, or durable proposal, spec, design, and task artifacts would materially reduce uncertainty.
   - Propose SDD to the user; select it only after an explicit request or an accepted proposal.`

// InjectOrganicRouting applies the Organic Routing directive to target prompt content
// using marker-based injection (<!-- cortex-ia:organic-routing -->).
func InjectOrganicRouting(content string) string {
	return filemerge.InjectMarkdownSection(content, "organic-routing", OrganicRoutingDirective)
}
