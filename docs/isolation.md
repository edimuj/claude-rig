# Isolation & Inheritance

## What's Isolated vs. Shared

| Always per-rig | Per-rig, inheritable/syncable | Isolated by default | Shared by default |
|---|---|---|---|
| `settings.json` | `skills/` (inherit) | `conversations`, `history.jsonl`, `sessions` | `telemetry` |
| `CLAUDE.md` | `agents/` (inherit) | `channels`, `tasks`, `todos`, `backups` | `usage-data` |
| `.claude.json` | `hooks/` (inherit) | `shell-snapshots`, `projects`, `plans` | |
| | `commands/` (inherit or isolate/share) | `paste-cache`, `ide`, `downloads`, `debug` | |
| | `plugins/` (sync from global) | `file-history`, `session-env`, `cache` | |
| | `mcp` (sync from global) | `stats-cache.json`, `statusline`, `chrome` | |

New rigs isolate 21 items by default — only `telemetry` and `usage-data` remain shared. Skills, agents, hooks, and commands are per-rig but can inherit entries from global `~/.claude/`. Plugins and MCP servers can be synced from global (see [sync.md](sync.md)).

Each rig gets its own `.claude.json` seeded from the global config on creation. MCP servers configured via `claude mcp add` go directly into the rig's `.claude.json` — no symlinks, no project-level files.

## Configurable Isolation

New rigs come with sensible defaults already isolated. You can further isolate shared items or un-isolate defaults:

```bash
# See what's isolated vs shared
claude-rig isolation myrig

# Share something that's isolated by default
claude-rig share myrig conversations

# Isolate something that's currently shared
claude-rig isolate myrig telemetry

# Create with no default isolation (everything shared except always-isolated items)
claude-rig create bare --no-isolate-defaults --link-auth

# Create with everything isolated
claude-rig create fortress --isolate-all --link-auth
```

Isolation config lives in `rig.json` inside the rig directory. When an item is isolated, the symlink is replaced with a local empty file or directory — the rig gets its own independent copy from that point on.

## Global Inheritance

Skills, agents, hooks, and commands defined in `~/.claude/` can be inherited by any rig. This gives you a 3-layer configuration stack — just like how `CLAUDE.md` works:

```
~/.claude/skills/                ← Global (inherited by rigs that opt in)
~/.claude-rig/rigs/go/skills/    ← Rig-specific (overrides global by name)
myproject/.claude/skills/        ← Project-level (native Claude Code discovery)
```

```bash
# Inherit all global skills, agents, hooks, and commands
claude-rig inherit --all myrig

# Or pick what to inherit
claude-rig inherit skills agents myrig

# Stop inheriting
claude-rig uninherit skills myrig
claude-rig uninherit --all myrig

# Or set up at creation time
claude-rig create myrig --inherit-all --link-auth
```

Rig-specific files always win — if both `~/.claude/skills/foo/` and the rig have a `skills/foo/`, the rig's version is used. Inherited entries are symlinks; rig-specific entries are real files/directories.
