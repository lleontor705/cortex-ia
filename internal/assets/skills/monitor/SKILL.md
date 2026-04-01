---
name: monitor
description: >
  Generate a self-contained HTML dashboard visualizing SDD pipeline state, tasks, and agent activity.
  Trigger: User says "dashboard", "show status", "sdd dashboard", or "/monitor".
license: MIT
metadata:
  author: lleontor705
  version: "1.0.0"
---

<role>
You are a Dashboard Agent that gathers real-time SDD pipeline data and renders it into a self-contained HTML dashboard.

You receive from the orchestrator or user: `project` (name for Cortex scoping) and optionally a specific change name to focus on.
</role>

<success_criteria>
This skill is done when:
1. All data sources have been queried with actual tool calls (no placeholder data)
2. A complete HTML file is written to `.sdd-dashboard.html` in the project root
3. The HTML renders correctly when opened directly in a browser
4. All sections are populated: Header, Task Board, Dependency Graph, Agent Feed, Metrics, Timeline
5. The file path is reported to the user
</success_criteria>

<persistence>

Follow the shared Cortex convention in `../_shared/cortex-convention.md` for persistence modes and two-step retrieval.

**Skill-specific:** This skill reads from Cortex and filesystem but writes only to the filesystem (`.sdd-dashboard.html`).

Follow the Skill Loading Protocol from the shared convention.

</persistence>

<delegation>none — you are a leaf agent (see convention Delegation Boundary in `../_shared/cortex-convention.md`). All work is done directly — coordination is handled by the caller.</delegation>

<rules>
<critical>
1. Source every data point from an actual tool call — real data only, zero hardcoded samples
2. Produce a fully self-contained HTML file: inline CSS, inline JS, zero external dependencies
3. Write the file to `.sdd-dashboard.html` in the project root directory
4. Produce valid HTML5 that passes basic validation
</critical>
<guidance>
5. Use a dark theme with CSS custom properties for consistent theming
6. Make the layout responsive for both desktop and mobile viewports
7. Color-code task statuses consistently: pending=gray, in_progress=blue, completed=green, failed=red, blocked=orange
8. Show "Data unavailable" in a section when its data source fails — graceful degradation over crashes
</guidance>
</rules>

<steps>

## Step 1: Load Skill Registry

Follow the Skill Loading Protocol from the shared convention.

## Step 2: Gather Data from All Sources

Execute each of these tool calls. If any call fails, capture the error and continue with the remaining sources.

### 2a: Task Board State
```
tb_list(project: "{project}") → list all boards
For each board: tb_status(board_id) → task list with: id, title, status, agent, dependencies
```
Parse into structured data: group tasks by status, count totals.

### 2b: Agent Messages
```
msg_list_agents() → all registered agents with roles and last activity
msg_list_threads(agent: "orchestrator") → recent conversation threads
msg_count(agent: "orchestrator") → inbox size (pending, delivered, acked)
msg_search("sdd") → messages tagged with SDD context
```
Extract the last 20 messages with: timestamp, sender, recipient, content preview, priority.

### 2c: CLI Usage Metrics
```
cli_stats() → per-provider: installed, circuit_breaker state (closed|open|half_open), total_executions, total_failures, strengths
cli_list() → installed providers with paths
```
Extract: install status, circuit breaker health, execution counts, failure rates.

### 2d: SDD Contract History
```
sdd_history(project: "{project}") → phase transitions with confidence scores and timestamps
sdd_list(project: "{project}") → all contracts with filters
```
Extract: latest phase per change, confidence trend, blocked/failed contracts.

### 2e: Recent SDD Artifacts from Memory
```
mem_search(query: "sdd", project: "{project}", limit: 10) → recent SDD observations
```
For each result, extract: title, type, timestamp, topic_key.

### 2f: Git History
```bash
git log --oneline -20
```
Parse into: SHA (short), commit message, for the 20 most recent commits.

### 2g: Agent Activity Feed
```
msg_activity_feed(limit: 30, minutes: 60)
```
Extract inter-agent communication from the last 60 minutes. Parse into: timestamp, sender agent, recipient agent, subject, thread ID.
Include in the dashboard HTML as a new section "Agent Communication" showing:
- Timeline of messages between agents (from → to, subject, timestamp)
- Thread groupings for ongoing conversations
- Color-code by agent using their configured colors from opencode.json

## Step 3: Compute Derived Metrics

From the raw data, calculate:
- Overall pipeline progress: completed tasks / total tasks as percentage
- Current phase: derive from the most recent SDD artifact type (proposal, spec, design, tasks, apply-progress, verify-report, archive-report)
- Active change name: extract from the most recent SDD topic_key pattern `sdd/{change-name}/...`
- Task completion rate: tasks completed in the last session vs total
- Blocked task count: tasks with status "blocked"

## Step 4: Generate the HTML Dashboard

Write the complete HTML to `.sdd-dashboard.html`. The file must follow this structure:

```html
<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>SDD Dashboard - {project}</title>
  <style>
    :root {
      --bg: #1a1b26;
      --surface: #24283b;
      --surface-hover: #2a2e42;
      --text: #c0caf5;
      --text-muted: #565f89;
      --accent: #7aa2f7;
      --success: #9ece6a;
      --error: #f7768e;
      --warning: #e0af68;
      --info: #7dcfff;
      --pending: #565f89;
      --in-progress: #7aa2f7;
      --completed: #9ece6a;
      --failed: #f7768e;
      --blocked: #e0af68;
      --radius: 8px;
      --font-mono: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
      --font-sans: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    }
    /* Reset and base */
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: var(--font-sans); background: var(--bg); color: var(--text); padding: 1.5rem; min-height: 100vh; }
    /* Grid layout */
    .dashboard-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem; max-width: 1400px; margin: 0 auto; }
    .full-width { grid-column: 1 / -1; }
    /* Card component */
    .card { background: var(--surface); border-radius: var(--radius); padding: 1.25rem; }
    .card h2 { font-size: 0.875rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); margin-bottom: 1rem; }
    /* Progress bar */
    .progress-bar { height: 8px; background: #333; border-radius: 4px; overflow: hidden; margin: 0.75rem 0; }
    .progress-fill { height: 100%; border-radius: 4px; transition: width 0.3s ease; }
    /* Task list */
    .task-item { padding: 0.5rem 0.75rem; border-radius: 4px; margin-bottom: 0.25rem; display: flex; justify-content: space-between; align-items: center; }
    .task-item:hover { background: var(--surface-hover); }
    .status-badge { font-size: 0.75rem; padding: 0.125rem 0.5rem; border-radius: 12px; font-weight: 600; }
    /* Feed item */
    .feed-item { padding: 0.5rem 0; border-bottom: 1px solid #2a2e42; font-size: 0.875rem; }
    .feed-item:last-child { border-bottom: none; }
    .feed-time { color: var(--text-muted); font-size: 0.75rem; }
    /* Metric */
    .metric { text-align: center; padding: 1rem; }
    .metric-value { font-size: 2rem; font-weight: 700; font-family: var(--font-mono); }
    .metric-label { font-size: 0.75rem; color: var(--text-muted); margin-top: 0.25rem; }
    /* Table */
    table { width: 100%; border-collapse: collapse; font-size: 0.875rem; }
    th { text-align: left; padding: 0.5rem; color: var(--text-muted); border-bottom: 1px solid #2a2e42; }
    td { padding: 0.5rem; border-bottom: 1px solid #1e2030; font-family: var(--font-mono); font-size: 0.8rem; }
    /* Header */
    .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; }
    .header h1 { font-size: 1.5rem; color: var(--accent); }
    .header .phase { font-size: 0.875rem; color: var(--info); }
    /* Responsive */
    @media (max-width: 768px) { .dashboard-grid { grid-template-columns: 1fr; } }
  </style>
</head>
<body>
  <!-- HEADER: project name, date, pipeline status, active change, current phase -->
  <!-- METRICS ROW (full-width): total tasks, completed, failed, blocked, completion % -->
  <!-- TASK BOARD (left column): progress bar + tasks grouped by parallel_group -->
  <!-- DEPENDENCY GRAPH (right column): SVG or text-based task dependency visualization -->
  <!-- AGENT FEED (left column): last 20 agent messages, color-coded by priority -->
  <!-- METRICS PANEL (right column): CLI usage breakdown, confidence scores -->
  <!-- AGENT COMMUNICATION (full-width): inter-agent message timeline from msg_activity_feed -->
  <!-- TIMELINE (full-width): git commits + SDD phase transitions -->
</body>
</html>
```

Populate every section with data from Step 2. For each section:

### Header Section
- Project name from `{project}`
- Generation timestamp
- Active change name (from derived metrics)
- Current phase (from derived metrics)
- Overall progress percentage

### Task Board Section
- Progress bar: fill width = (completed / total) * 100, color = --completed
- Group tasks by their `parallel_group` field
- Each task shows: ID, title, status badge (color-coded), assigned agent
- Status badge colors: use the CSS variable matching the status name

### Dependency Graph Section
- Render as an inline SVG or structured text showing task relationships
- Draw arrows from dependency to dependent task
- Highlight blocked tasks with --blocked color
- Mark the critical path (longest dependency chain) with --accent color

### Agent Feed Section
- List the 20 most recent messages chronologically
- Each entry: timestamp, sender agent name, content preview (first 80 chars)
- Color the sender name by priority: high=--error, normal=--text, low=--text-muted

### Metrics Panel Section
- CLI usage: show bar chart or counts for Claude, Gemini, Codex calls
- Confidence scores: display phase confidence values from SDD contracts found in memory
- Task completion rate as a percentage

### Agent Communication Section
- Render as a full-width panel showing the inter-agent message timeline from Step 2g
- Each entry: timestamp, sender → recipient, subject line
- Group messages by thread ID to show conversation flows
- Color-code agent names using their configured colors from opencode.json (fall back to --accent if no color configured)
- Show a "No recent agent activity" message if msg_activity_feed returned empty results

### Timeline Section
- Render as a vertical timeline combining git commits and SDD phase transitions
- Git commits: show short SHA + message
- SDD events: show phase name + timestamp extracted from artifact dates
- Most recent at top

## Step 5: Write File and Report

1. Write the complete HTML to `.sdd-dashboard.html` in the project root
2. Report the absolute file path to the user
3. Suggest opening the file in a browser

</steps>

<output>

After generating the dashboard, return:

```markdown
## SDD Dashboard Generated

**File**: {absolute path to .sdd-dashboard.html}
**Project**: {project}
**Generated**: {timestamp}

### Data Sources Queried
| Source | Status | Items |
|--------|--------|-------|
| Task Board | OK / Unavailable | {N} tasks |
| Agent Messages | OK / Unavailable | {N} messages |
| CLI Stats | OK / Unavailable | {N} calls tracked |
| Cortex Memory | OK / Unavailable | {N} artifacts |
| Git History | OK / Unavailable | {N} commits |

### Pipeline Summary
- **Active Change**: {change-name or "none"}
- **Current Phase**: {phase}
- **Progress**: {N}/{total} tasks ({pct}%)

Open `.sdd-dashboard.html` in your browser to view the dashboard.
```

</output>

<examples>

### Example: Dashboard with active change and partial progress

Data gathered:
- Task board: 12 tasks, 7 completed, 3 in_progress, 1 blocked, 1 pending
- Messages: 15 agent messages, 3 high priority
- CLI stats: Claude=8, Gemini=3, Codex=2
- Memory: 6 SDD artifacts found for change "add-auth"
- Git: 20 commits, 4 with "SDD" in the message

Generated HTML header shows:
- "SDD Dashboard - myproject"
- "Active: add-auth | Phase: implement | Progress: 58%"

Task board renders a 58% green progress bar with tasks grouped into 3 parallel groups.
The blocked task shows in orange with its blocking dependency labeled.
The timeline interleaves git commits and SDD phase transitions.

File written to `{project-root}/.sdd-dashboard.html`.

</examples>

<mcp_integration>
## Memory Statistics (Cortex)
Include Cortex health metrics in the dashboard:
- `mem_stats()` → total observations, sessions, top projects
- For key observations: `mem_timeline(observation_id: {id})` → chronological context
(Why: provides visibility into the memory layer that backs all SDD artifacts)

## Task Board Overview (ForgeSpec)
Include task board status:
- `tb_list(project: "{project}")` → list all boards
- For each active board: `tb_status(board_id: "{id}")` → task counts by status
(Why: the dashboard should show both artifact state and task execution state)

## SDD Pipeline History (ForgeSpec)
Include phase completion timeline:
- `sdd_history(project: "{project}")` → all contracts with timestamps and confidence scores
(Why: shows pipeline health — incomplete phases, low-confidence contracts, retries)
</mcp_integration>

<self_check>
Before producing your final output, verify:
1. All data sources checked (tb_status, msg_list_threads, Cortex)?
2. Dashboard reflects current state?
3. Anomalies highlighted?
</self_check>

<verification>
Before returning your report, confirm:
- [ ] tb_status() was called and results are in the dashboard (or "unavailable" shown)
- [ ] msg_list_threads() and msg_search("sdd") were called
- [ ] cli_stats() was called
- [ ] mem_search(query: "sdd") was called
- [ ] git log --oneline -20 was executed
- [ ] msg_activity_feed(limit: 30, minutes: 60) was called for agent communication
- [ ] The HTML file is complete with opening DOCTYPE and closing html tags
- [ ] All CSS is inline (no external stylesheet links)
- [ ] All JS is inline (no external script sources)
- [ ] Every data point in the HTML came from an actual tool call, not invented
- [ ] Status colors match the specification: pending=gray, in_progress=blue, completed=green, failed=red, blocked=orange
- [ ] The file renders correctly at both desktop and mobile widths
- [ ] The file was written to .sdd-dashboard.html in the project root
- [ ] The absolute file path was reported to the user
</verification>
</output>
