---
name: claude-rig-awareness
description: >
  Understanding claude-rig — a tool that manages multiple Claude Code configurations
  ("rigs") in parallel. MUST be consulted before modifying any Claude Code configuration:
  installing/removing plugins, adding/configuring MCP servers, editing settings.json,
  editing .claude.json, editing CLAUDE.md, adding skills, hooks, agents, or commands.
  Also trigger when the user mentions "rig", "claude-rig", or asks about config layers,
  or when the user mentions "plugins", "MCP", "skills", "hooks", "agents", or
  "commands" in a configuration context, or when you're about to write to ~/.claude/,
  settings.json, .claude.json, or any config file inside a rig directory. Without this
  skill, you will likely modify the wrong location or put config in the wrong file.
---

# claude-rig Awareness

claude-rig lets users run multiple independent Claude Code configurations side by side.
Each configuration is called a **rig**. When a rig is active, Claude Code doesn't use
`~/.claude/` directly — it uses the rig's directory instead.

## How to detect the active rig

Check the `CLAUDE_CONFIG_DIR` environment variable. If set, you're running inside a rig.

```
echo $CLAUDE_CONFIG_DIR
# Example: /home/user/.claude-rig/rigs/go
```

**If `CLAUDE_CONFIG_DIR` is set, that directory replaces `~/.claude/` for everything.**
Settings, plugins, skills, hooks, agents, commands, CLAUDE.md — all of it lives there,
not in `~/.claude/`.

If the variable is unset, there's no active rig and `~/.claude/` is the config dir as
usual.

## The 3-layer config model

Configuration resolves in three layers, most specific wins:

| Layer   | Location                                           | Scope                  |
|---------|----------------------------------------------------|------------------------|
| Global  | `~/.claude/`                                       | Shared across all rigs |
| Rig     | `~/.claude-rig/rigs/<name>/` (`CLAUDE_CONFIG_DIR`) | Per-rig override       |
| Project | `.claude/` in the project working dir              | Per-project override   |

Rig-specific items (settings, plugins, skills, hooks, agents) are real files that override
the global ones. New rigs also isolate per-rig data by default (conversations, history,
sessions, etc.). Remaining shared items are symlinks back to `~/.claude/`. Rigs can
inherit skills/agents/hooks/commands from global, and sync plugins + MCP servers.

### Where things live (with active rig)

| Item             | Location                              |
|------------------|---------------------------------------|
| `settings.json`  | `$CLAUDE_CONFIG_DIR/settings.json`    |
| Plugins          | `$CLAUDE_CONFIG_DIR/plugins/`         |
| Skills           | `$CLAUDE_CONFIG_DIR/skills/`          |
| Hooks            | `$CLAUDE_CONFIG_DIR/hooks/`           |
| Agents           | `$CLAUDE_CONFIG_DIR/agents/`          |
| Commands         | `$CLAUDE_CONFIG_DIR/commands/`        |
| CLAUDE.md (rig)  | `$CLAUDE_CONFIG_DIR/CLAUDE.md`        |
| MCP servers      | `$CLAUDE_CONFIG_DIR/.claude.json`     |
| Auth files       | Often symlinked — shared across rigs  |
| Rig config       | `$CLAUDE_CONFIG_DIR/rig.json`         |

## Installing and managing plugins

**CRITICAL: Never run `claude plugin` directly. Always use `claude-rig plugin` instead.**

`claude-rig plugin` forwards all arguments to `claude plugin` but sets `CLAUDE_CONFIG_DIR`
to the correct rig directory first. Running `claude plugin` directly from a shell that
doesn't have the rig env set will install to `~/.claude/` instead of the active rig.

```bash
# Install a plugin to the active rig
claude-rig plugin add edimuj/claude-mneme

# Add a private/custom marketplace
claude-rig plugin marketplace add github.com/anthropics/claude-plugins-official

# Update all marketplace indexes
claude-rig plugin marketplace update

# List installed plugins
claude-rig plugin list

# Remove a plugin
claude-rig plugin remove claude-mneme@claude-mneme

# Update a specific plugin
claude-rig plugin update claude-mneme@claude-mneme

# Target a different rig (not the active one)
claude-rig plugin add edimuj/claude-mneme --rig webdev
claude-rig plugin list --rig minimal
```

The `--rig <name>` flag can go anywhere in the args. Without it, the active rig is
resolved from: `CLAUDE_CONFIG_DIR` env var > `.claude-rig` RC file in the project.

### Bulk plugin updates across all rigs

```bash
# Update all marketplace plugins in all rigs (parallel)
claude-rig update-plugins

# Update plugins for specific rigs only
claude-rig update-plugins go minimal
```

This refreshes marketplaces and updates every installed plugin in each rig. Runs in
parallel across rigs. Also cleans stale orphan markers that Claude Code sometimes leaves
on active plugin cache entries.

## Managing MCP servers

MCP servers live in the rig's `.claude.json` (not `settings.json`). Inside an active rig
session, `claude mcp add` already respects `CLAUDE_CONFIG_DIR` and targets the correct
rig. Use it normally:

```bash
claude mcp add gopls -- gopls -remote=auto
```

For cross-rig MCP distribution, use `claude-rig sync` (see below).

## Adding skills, agents, hooks, commands

These are rig-specific directories. To add them to a rig:

- **Skills**: create a directory under `$CLAUDE_CONFIG_DIR/skills/<name>/` with a SKILL.md
- **Agents**: create a `.md` file under `$CLAUDE_CONFIG_DIR/agents/`
- **Commands**: create a `.md` file under `$CLAUDE_CONFIG_DIR/commands/`
- **Hooks**: configure in `$CLAUDE_CONFIG_DIR/settings.json` under `"hooks"`

To share global items from `~/.claude/` into a rig, use inheritance (see below).

## Syncing rigs (`claude-rig sync`)

The `sync` command keeps rigs in shape. It runs four sync steps:

1. **Shared symlinks** — refreshes symlinks from `~/.claude/` for shared items
2. **Inherited items** — syncs skills/agents/hooks/commands from global `~/.claude/`
   into rigs that have inheritance enabled (symlinks individual entries)
3. **Plugins** — syncs plugins from global (or another rig) by symlinking cache dirs
   and rewriting manifest paths
4. **MCP servers** — merges `mcpServers` from the global `~/.claude.json` into the
   rig's `.claude.json` (local entries take precedence, never overwritten)

```bash
# Sync all rigs (all four steps)
claude-rig sync

# Sync a single rig
claude-rig sync myrig

# Sync from another rig instead of global ~/.claude/
claude-rig sync myrig --from webdev

# Skip specific sync steps
claude-rig sync myrig --no-plugins     # skip plugin sync
claude-rig sync myrig --no-mcp         # skip MCP sync
claude-rig sync myrig --no-inherit     # skip inherited items sync
```

Sync is idempotent — safe to run repeatedly. It respects isolation: if plugins or MCP
are in the rig's isolate list, those sync steps are skipped automatically.

## Inheritance

Skills, agents, hooks, and commands from `~/.claude/` can be inherited by any rig.
Inherited entries appear as symlinks inside the rig's directory. Rig-local entries
(real files) always take precedence over inherited ones.

```bash
# Inherit everything from global
claude-rig inherit --all myrig

# Inherit specific categories
claude-rig inherit skills agents myrig

# Stop inheriting
claude-rig uninherit skills myrig
claude-rig uninherit --all myrig

# Set up at creation time
claude-rig create myrig --inherit-all --link-auth
```

Inheritable items: `skills`, `agents`, `commands`, `hooks`.

## Isolation

New rigs isolate 21 items by default (conversations, history, sessions, channels,
tasks, todos, backups, shell-snapshots, projects, plans, paste-cache, ide, downloads,
debug, commands, file-history, session-env, cache, stats-cache.json, statusline, chrome).
Only `telemetry` and `usage-data` remain shared.

```bash
# See what's isolated vs shared + content category counts
claude-rig isolation myrig

# Isolate a currently shared item
claude-rig isolate myrig telemetry

# Share a currently isolated item (replaces local with symlink to global)
claude-rig share myrig conversations

# Isolate or share plugins and MCP (special handling)
claude-rig isolate myrig plugins     # removes synced plugins, skips future syncs
claude-rig share myrig plugins       # re-syncs plugins from global
claude-rig isolate myrig mcp         # removes synced MCP servers
claude-rig share myrig mcp           # re-syncs MCP from global
```

Isolation state is tracked in `rig.json` inside the rig directory.

## Diagnosing where things come from

**When you need to know where a skill, plugin, agent, command, or MCP server comes from
(local to the rig, inherited from global, or synced), use `claude-rig isolation`.**

```bash
# Overview: isolation status + content counts (local vs inherited/synced)
claude-rig isolation myrig

# Filter to specific categories
claude-rig isolation myrig --skills
claude-rig isolation myrig --plugins --mcp
claude-rig isolation myrig --agents --commands

# Show every individual entry with its source
claude-rig isolation myrig --skills --details
claude-rig isolation myrig --plugins --details
claude-rig isolation myrig --details              # everything with details

# Check all rigs at once
claude-rig isolation --plugins --details
```

Example output with `--skills --details`:
```
go:
  skills: 24 total (0 local, 24 inherited)
    add-feature                    inherited (global)
    claude-rig-awareness           inherited (global)
    ...
```

Example output with `--plugins --details`:
```
go:
  plugins: 6 total (4 local, 2 synced)
    claude-mneme@claude-mneme      local
    gopls-lsp@claude-plugins-official local
    plugin-dev@claude-plugins-official synced (from /home/user/.claude/plugins/cache)
    ...
```

Sources shown:
- **local** — installed directly in the rig
- **inherited (global)** — symlinked from `~/.claude/` via inheritance
- **synced** / **synced (from ...)** — copied/symlinked from global or another rig via sync

Use this to answer questions like "why does this rig have skill X?", "is this plugin
local or synced?", "what's shared vs isolated?". Always check before telling the user
something is missing — it might be inherited or synced from elsewhere.

## Creating and managing rigs

```bash
# Create with defaults (21 items isolated, shared auth optional)
claude-rig create myrig --link-auth

# Create with everything shared
claude-rig create myrig --link-auth --no-isolate-defaults

# Create with everything isolated
claude-rig create myrig --link-auth --isolate-all

# Create with inheritance from global
claude-rig create myrig --link-auth --inherit-all

# Clone existing config as starting point
claude-rig clone default webdev --link-auth    # clone from ~/.claude/
claude-rig clone go rust --link-auth           # clone from another rig

# Delete, rename
claude-rig delete myrig
claude-rig rename oldname newname
```

## Launching and project binding

```bash
# Launch explicitly
claude-rig launch webdev

# Bind a project to a rig (creates .claude-rig file)
cd ~/projects/my-app
claude-rig rc webdev

# Auto-launch from .claude-rig file
claude-rig                    # picks up rig from .claude-rig
claude-rig --resume           # forward flags to claude
claude-rig -p "fix the tests" # pass prompt
```

## Diagnostics

```bash
# Health check across all rigs
claude-rig doctor

# Compare two rigs
claude-rig diff go minimal

# Overview of all rigs (disk, sessions, plugins, MCP)
claude-rig status
claude-rig status go          # detailed single-rig view

# List rigs (* = running sessions)
claude-rig list
```

`doctor` checks: broken symlinks, missing rig-specific files, inherited item health,
synced plugin cache validity, synced MCP server presence, and multiple rigs with
Remote Control enabled.

## Auth management

Auth files (`.credentials.json`, `.claude.json`, `statsig/`) can be shared across
rigs or kept separate.

```bash
# Share auth from global ~/.claude/
claude-rig link-auth myrig

# Share auth from another rig
claude-rig link-auth myrig --from webdev

# Give rig its own auth (copies auth files, breaks sharing)
claude-rig unlink-auth myrig
```

## Export and import

```bash
# Export rig to portable archive
claude-rig export myrig                           # → myrig.tar.gz
claude-rig export myrig --include-auth            # include credentials
claude-rig export myrig --include-data            # include conversations/history

# Import on another machine
claude-rig import myrig.tar.gz myrig-restored --link-auth
```

## Before modifying Claude Code config: check the rig

When asked to install a plugin, change a setting, add a skill, edit CLAUDE.md,
or modify any Claude Code configuration:

1. **Check `CLAUDE_CONFIG_DIR`** — if set, you're in a rig
2. **Ask which level** the user wants the change at:
   - **This project only** → `.claude/` in the project dir
   - **This rig** → the path in `CLAUDE_CONFIG_DIR`
   - **All rigs / global** → `~/.claude/`
3. **Default suggestion**: the rig level (since that's what they're actively using),
   but always confirm if they didn't specifically mention project, global or rig.

### Common mistakes to avoid

- **Never run `claude plugin` directly** — always use `claude-rig plugin`
- **Don't write to `~/.claude/settings.json`** when a rig is active — use
  `$CLAUDE_CONFIG_DIR/settings.json`
- **Don't install plugins to `~/.claude/plugins/`** — use `claude-rig plugin add`
- **Don't edit `~/.claude/CLAUDE.md`** for rig-specific changes — edit
  `$CLAUDE_CONFIG_DIR/CLAUDE.md`
- **Don't look in `~/.claude/` and report "not found"** when the config lives
  in the rig directory
- **Don't run `claude mcp add` from a shell without the rig env** — inside a rig
  session it's fine, but from a plain shell it targets `~/.claude.json`
- **Never put MCP servers in `.mcp.json` or `settings.json`** — Claude Code reads
  MCP config from `.claude.json` only. The correct location for a rig is
  `$CLAUDE_CONFIG_DIR/.claude.json` under the `"mcpServers"` key. A `.mcp.json`
  file in the rig dir is ignored, and `settings.json` is for settings, not servers
