# Documentation assets

Images used by the README and the documentation site.

| Folder | Contents |
|---|---|
| `brand/` | Hero banner and logos |
| `diagrams/` | Explanatory diagrams, one concept per file |

## Diagram index

| File | Explains | Used in |
|---|---|---|
| `diagrams/architecture-overview.svg` | Hosts, the binary and the three planes | README, docs home, Architecture |
| `diagrams/task-lifecycle.svg` | Task states and the command behind each transition | README, Architecture, Planning |
| `diagrams/install-pipeline.svg` | Plan, lock, backup, apply, verify, rollback | README, Architecture, Security and recovery |
| `diagrams/host-targets.svg` | What `install --target` writes for each host | README, Supported platforms |
| `diagrams/planning-flow.svg` | When a request needs a plan, and the path from plan to done | README, Planning |
| `diagrams/multi-agent-orchestration.svg` | The orchestrator and the five role controllers | README, Agents |
| `diagrams/cortex-vs-cortexia.svg` | Cortex (MCP server) compared with Cortex-IA | README, Cortex memory |

## Rules for diagrams

1. **SVG, hand-editable, self-contained.** No external fonts, images or scripts. GitHub renders
   SVG through an image tag, so anything external is dropped.
2. **Own background.** Each diagram paints its own dark panel, so it reads the same on GitHub's
   light and dark themes.
3. **Accessible.** Set `role="img"` with a `<title>` and a `<desc>` that states what the diagram
   shows, and give every `<img>` tag meaningful `alt` text. A reader who cannot see the image
   should still get the point.
4. **One concept per diagram.** If a diagram needs a paragraph to explain its legend, split it.
5. **True to the code.** Names, states and paths must match the source. When a command, state or
   path changes, update the diagram in the same change.
6. **System fonts only.** Use the sans-serif and monospace stacks already in the files; do not
   rely on emoji, which render differently on every platform.
7. **Markdown image syntax inside `docs/`.** Write `![alt](../assets/diagrams/file.svg)` so the docs site rewrites the path for nested pages. Raw `<img>` tags are for the root README only.
8. **Text stays inside its box.** Render the file and look at it before committing
   (on macOS: `qlmanage -t -s 1600 -o /tmp <file.svg>`).

## Shared palette

| Role | Color |
|---|---|
| Panel background | `#0B1220` |
| Card background / border | `#111A2E` / `#334155` |
| Primary text / secondary text | `#F1F5F9` / `#93A4BF` |
| Work authority, ready | `#38BDF8` |
| Specification, in progress | `#A78BFA` |
| Evidence, done | `#34D399` |
| Review, caution | `#FBBF24` |
| Blocked, failure | `#FB7185` |

Color never carries meaning alone: every colored element also has a text label.
