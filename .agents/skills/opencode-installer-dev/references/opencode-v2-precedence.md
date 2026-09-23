# OpenCode v2 Configuration Precedence & Merging Matrix

This reference documents the merging behavior and precedence hierarchy for configuration files in OpenCode v2 (`opencode2`).

## Precedence Hierarchy (Highest to Lowest)

```
[Priority 1 - Highest] Command-Line Flags
  e.g. opencode2 --model nan/deepseek-v4-flash --log-level debug

[Priority 2] Current Working Directory .opencode/
  <cwd>/.opencode/opencode.json(c)

[Priority 3] Current Working Directory Root
  <cwd>/opencode.json(c)

[Priority 4] Ancestor Directories Upward (nearest to farthest)
  <parent>/.opencode/opencode.json(c)
  <parent>/opencode.json(c)

[Priority 5] Global User Configuration Directory
  ~/.config/opencode/opencode.json(c)
  ~/.config/opencode/cli.json (TUI / appearance settings)

[Priority 6 - Lowest] Built-in Defaults
  Embedded within the opencode2 binary
```

## Array & Object Merging Invariants

| Key in Config | Data Type | Merge Behavior |
| :--- | :--- | :--- |
| `permissions` | Array of objects | Prepend higher-priority rules; the LAST matching rule takes effect (broad rules first, specific exceptions after) |
| `plugins` | Array of strings | Union with deduplication (preserving order) |
| `skills` | Array of paths/URLs | Union with deduplication |
| `mcp.servers` | Object of servers | Deep merge; higher priority overrides server with same name |
| `providers` | Object of providers | Deep merge; higher priority overrides provider/model entries |
| `theme` | Object / String | Replaced entirely by higher priority config |
| `compaction` | Object | Deep merge of numeric settings |
