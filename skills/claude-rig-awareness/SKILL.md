---
name: claude-rig-awareness
description: >
  Understanding claude-rig — a tool that manages multiple Claude Code configurations
  ("rigs") in parallel. MUST be consulted before modifying any Claude Code configuration:
  installing/removing plugins, editing settings.json, editing CLAUDE.md, adding skills,
  hooks, agents, or commands. Also trigger when the user mentions "rig", "claude-rig",
  or asks about config layers, or when you're about to write to ~/.claude/ or
  settings.json. Without this skill, you will likely modify the wrong location.
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

Most files in the rig directory are **symlinks** back to `~/.claude/` — they share
global state. Rig-specific items (settings, plugins, skills, hooks, agents, commands)
are real files that override the global ones. Rigs can inherit (symlink) some of these from global.

## Before modifying Claude Code config: ASK the user

When a user asks you to install a plugin, change a setting, add a skill, edit CLAUDE.md,
or modify any Claude Code configuration:

1. **Check `CLAUDE_CONFIG_DIR`** — if set, you're in a rig
2. **Ask which level** they want the change at:
   - **This project only** → `.claude/` in the project dir
   - **This rig** → the path in `CLAUDE_CONFIG_DIR`
   - **All rigs / global** → `~/.claude/`
3. **Default suggestion**: the rig level (since that's what they're actively using),
   but always confirm if they didn't specifically mentioned project, global or rig.

### Common mistakes to avoid

- **Don't write to `~/.claude/settings.json`** when a rig is active — the rig has its
  own `settings.json` at `$CLAUDE_CONFIG_DIR/settings.json`
- **Don't install plugins to `~/.claude/plugins/`** — use `$CLAUDE_CONFIG_DIR/plugins/`
- **Don't edit `~/.claude/CLAUDE.md`** for rig-specific changes — edit
  `$CLAUDE_CONFIG_DIR/CLAUDE.md`
- **Don't look in `~/.claude/` and report "not found"** when the config actually lives
  in the rig directory

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
| Auth files       | Often symlinked — shared across rigs  |

## Inheritance and isolation

Rigs can **inherit** items from global `~/.claude/` (skills, agents, hooks, commands).
Inherited items appear as symlinks in the rig directory. Rigs can also **isolate** items,
meaning they have their own local copy instead of a symlink.

A `rig.json` file in the rig directory controls this:
```json
{
  "isolate": ["hooks"],
  "inherit": ["skills", "commands"]
}
```

When you see a symlink in the rig directory, it's inherited from global. When it's a
real file, it's rig-local.
