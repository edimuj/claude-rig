<p align="center">
  <img src="assets/claude-rig-raccoon-256.png" alt="claude-rig mascot" width="180" />
</p>

# claude-rig

**Run multiple Claude Code configurations side by side.**

<p align="center">
  <img src="assets/demo.gif" alt="claude-rig demo" width="800" />
</p>

## The Problem

Claude Code keeps all configuration in a single `~/.claude/` directory. That's fine until you need:

- **Multiple subscriptions** — Juggling 2–5 Max accounts? You have to log out and back in every time you switch. There's no way to run two subscriptions simultaneously.
- **API and subscription side by side** — Want API access for one project and your Max subscription for another? Same problem — one config, one auth.
- **Different tools per project** — Your web project needs Tailwind skills and a database MCP server. Your CLI tool needs none of that. But every Claude Code session loads the same plugins, the same MCP servers, the same hooks.
- **Testing without risk** — Want to see if a new MCP server is eating your context window or introducing errors? You'd have to disable everything else, test, then re-enable. No way to isolate the experiment.

## What claude-rig Does

Each configuration becomes its own isolated **rig** — with its own settings, skills, plugins, agents, commands, hooks, MCP servers, and instructions. Auth can be shared across rigs or kept separate. Run them side by side in different terminals. No conflicts.

```bash
# Terminal 1: your Max subscription, minimal setup
claude --rig=minimal

# Terminal 2: different Max account, full web dev stack
claude --rig=webdev

# Terminal 3: API access, experimental MCP server you're testing
claude --rig=experiment
```

## What It Doesn't Do

- **Doesn't modify Claude Code.** Uses the official `CLAUDE_CONFIG_DIR` environment variable and `--add-dir` flag. No patches, no forks.
- **Doesn't manage Claude Code installation.** Install Claude Code separately — claude-rig just manages its configuration.
- **Doesn't replace project-level config.** Your project's `CLAUDE.md` and `.claude/` still work as normal. Rigs handle user-level configuration.
- **No external dependencies.** Single Go binary, stdlib only.

## Install

```bash
go install github.com/edimuj/claude-rig/cmd/claude-rig@latest
```

Or build from source:

```bash
git clone https://github.com/edimuj/claude-rig.git
cd claude-rig
make install
```

## Quick Start

```bash
# One-time setup
claude-rig init

# Create your first rig
claude-rig create minimal

# Or clone your existing ~/.claude/ config as a starting point
claude-rig clone default webdev --link-auth

# Launch explicitly
claude-rig launch webdev

# Or bind to a project and just run claude-rig
cd ~/projects/my-app
claude-rig rc webdev
claude-rig                # picks up the rig automatically
```

## Rig-Specific Instructions

Each rig has its own `CLAUDE.md` for rig-specific instructions. Your global `~/.claude/CLAUDE.md` is always loaded alongside it — you don't lose your personal instructions when using a rig.

```
~/.claude/CLAUDE.md              ← Global instructions (always loaded)
~/.claude-rig/rigs/webdev/CLAUDE.md  ← Rig-specific additions
```

This means you can give each rig its own personality, tool preferences, or coding conventions without duplicating your global setup.

## Per-Project Rigs

Drop a `.claude-rig` file in any project and it automatically uses the right rig:

```bash
# One-time setup per project
cd ~/projects/my-app
claude-rig rc webdev      # creates .claude-rig with rig=webdev

# From now on — no flags, no rig names, just:
claude-rig
```

Any flags are forwarded straight to Claude Code, so you can resume sessions, set prompts, or pass any other flags without specifying the rig name:

```bash
claude-rig --resume           # resume last session
claude-rig --resume abc123    # resume a specific session
claude-rig -p "fix the tests" # pass a prompt
```

The `.claude-rig` file contains one line (`rig=webdev`) and walks up the directory tree, so every subdirectory inherits it. Different projects, different rigs, zero friction:

```
~/projects/
    my-webapp/.claude-rig    → rig=fullstack
    cli-tool/.claude-rig     → rig=minimal
    experiment/.claude-rig   → rig=sandbox
```

## Shell Integration

Run `claude-rig init` to install a shell wrapper that adds `--rig` support directly to the `claude` command:

```bash
claude --rig=webdev
claude --rig=minimal --dangerously-skip-permissions
```

Or set up simple aliases:

```bash
alias claude-minimal='claude-rig launch minimal'
alias claude-webdev='claude-rig launch webdev'
```

## What's Isolated vs. Shared

| Always isolated per rig | Shared by default (configurable) |
|---|---|
| `settings.json` | `history.jsonl`, `conversations` |
| `CLAUDE.md` (rig instructions) | `projects`, `todos`, `tasks` |
| `.claude.json` (MCP servers, state) | `file-history`, `plans`, `cache` |
| `skills/`, `plugins/`, `agents/` | `debug`, `session-env`, `telemetry` |
| `commands/`, `hooks/` | All other `~/.claude/` files |

Each rig gets its own `.claude.json` seeded from the global config on creation. MCP servers configured via `claude mcp add` go directly into the rig's `.claude.json` — no symlinks, no project-level files. Plugins installed within a rig session stay within that rig.

### Configurable Isolation

By default, items like history, conversations, and projects are shared across rigs via symlinks. You can isolate any of them per rig:

```bash
# Give a rig its own private history and conversations
claude-rig isolate myrig history.jsonl conversations projects

# See what's isolated vs shared
claude-rig isolation myrig

# Change your mind — share them again
claude-rig share myrig history.jsonl

# Or isolate at creation time
claude-rig create cleanroom --isolate history.jsonl,conversations,projects
```

Isolation config lives in `rig.json` inside the rig directory. When an item is isolated, the symlink is replaced with a local empty file or directory — the rig gets its own independent copy from that point on.

## Commands

| Command | Description |
|---|---|
| `init` | Initialize claude-rig and install shell integration |
| `create <name>` | Create a new rig (`--link-auth` to reuse existing auth) |
| `clone <src\|default> <dest>` | Clone a rig or `~/.claude/` config (`--link-auth` to share auth) |
| `delete <name>` | Delete a rig |
| `list` | List all rigs with auth status and item counts |
| `launch [name] [args]` | Launch Claude Code with a rig (flags forwarded to claude) |
| `use <name>` | Set the active rig |
| `current` | Show the active rig |
| `rc [name]` | Show or create `.claude-rig` file for current directory |
| `link-auth <name>` | Link rig to shared auth (`--from <rig>` for cross-rig) |
| `unlink-auth <name>` | Remove shared auth so the rig gets its own |
| `set-args [name] <args>` | Set default launch args (global or per-rig) |
| `show-args [name]` | Show default launch args |
| `isolate <rig> <items>` | Isolate items per rig (no sharing via symlinks) |
| `share <rig> <items>` | Reverse isolation (delete local, recreate symlink) |
| `isolation [rig]` | Show isolation status for one or all rigs |
| `status [rig]` | Show rig status: disk usage, running sessions, last used |
| `update-plugins [rigs]` | Update marketplace plugins across rigs (all if none specified) |
| `doctor` | Diagnose broken symlinks and missing items |

## How It Works

Each rig is a full config directory under `~/.claude-rig/rigs/<name>/`:

```
~/.claude-rig/rigs/webdev/
    .claude.json            ← Real file (MCP servers, onboarding state)
    CLAUDE.md               ← Real file (rig-specific instructions)
    settings.json           ← Real file (rig-specific config)
    rig.json                ← Real file (isolation config, optional)
    skills/                 ← Real directory
    plugins/                ← Real directory
    history.jsonl           ← Real file (if isolated) or symlink (if shared)
    conversations/          ← Real dir (if isolated) or symlink (if shared)
    todos/ → ~/.claude/     ← Symlink (shared)
    ...
```

On `launch`, claude-rig:

1. Sets `CLAUDE_CONFIG_DIR` to the rig directory
2. Loads global `~/.claude/CLAUDE.md` via `--add-dir`
3. Refreshes symlinks to pick up any new shared files
4. Replaces itself with Claude Code via `exec` (no wrapper process)

Two Claude Code instances with different rigs run simultaneously without conflicts.

## Platform Support

- **Linux** — Full support
- **macOS** — Full support
- **Windows** — Requires Developer Mode (for symlinks)

## Backup & Restore

Your rig configurations live in `~/.claude-rig/`. To version-control and back them up:

```bash
cd ~/.claude-rig
git init && git branch -m main
```

Add a `.gitignore` to skip plugins (reinstallable), auth tokens, and runtime symlinks:

```gitignore
# Plugins — reinstallable via update-plugins
rigs/*/plugins/

# Auth tokens and backup files
rigs/*/.claude.json
*.backup.*

# Runtime symlinks (recreated by claude-rig)
rigs/*/cache
rigs/*/chrome
rigs/*/conversations
rigs/*/.credentials.json
rigs/*/debug
rigs/*/downloads
rigs/*/file-history
rigs/*/history.jsonl
rigs/*/paste-cache
rigs/*/personal.md
rigs/*/plans
rigs/*/projects
rigs/*/session-env
rigs/*/shell-snapshots
rigs/*/stats-cache.json
rigs/*/statsig
rigs/*/statusline
rigs/*/tasks
rigs/*/telemetry
rigs/*/todos
rigs/*/tokenlean.md
rigs/*/usage-data
```

Then commit and push to a private repo:

```bash
git add -A && git commit -m "rig configurations"
git remote add origin git@github.com:you/your-rig-config.git
git push -u origin main
```

**What gets tracked:** `CLAUDE.md`, `settings.json`, `mcp.json`, `skills/`, `agents/`, `commands/`, `hooks/` — the stuff you'd actually lose.

**Restore on a new machine:**

```bash
git clone git@github.com:you/your-rig-config.git ~/.claude-rig
claude-rig link-auth <rig>        # reconnect auth for each rig
claude-rig update-plugins         # reinstall marketplace plugins
```

## Also Check Out

More open-source tools for the Claude Code workflow:

| Project | Description |
|---|---|
| [tokenlean](https://github.com/edimuj/tokenlean) | Lean CLI tools for AI agents — reduce context, save tokens |
| [claude-mneme](https://github.com/edimuj/claude-mneme) | Automatic session memory — every session picks up where the last left off |
| [vexscan](https://github.com/edimuj/vexscan) | Security scanner for AI agent plugins, skills, MCPs, and configs |
| [claude-workshop](https://github.com/edimuj/claude-workshop) | Collection of useful plugins and tools for Claude Code |
| [claude-simple-status](https://github.com/edimuj/claude-simple-status) | No-frills statusline showing model, context, and quota usage |

## Development

```bash
make build                # build binary
make run ARGS="version"   # run without installing
make install              # install to ~/go/bin/
```

## License

MIT
