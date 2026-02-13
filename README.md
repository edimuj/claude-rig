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

That's it. The file contains one line (`rig=webdev`) and walks up the directory tree, so every subdirectory inherits it. Different projects, different rigs, zero friction:

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

| Isolated per rig | Shared across rigs |
|---|---|
| `settings.json` | `~/.claude/CLAUDE.md` (global memory) |
| `CLAUDE.md` (rig instructions) | `credentials.json` (auth) |
| `skills/` | `sessions/` (history) |
| `plugins/` (enabled set) | `todos/` |
| `agents/` | All other `~/.claude/` files |
| `commands/` | |
| `hooks/` | |
| `mcp.json` | |

> **MCP servers:** Claude Code only discovers MCP config from `~/.claude.json` (global) or `.mcp.json` (project-level) — it does not read MCP config from `CLAUDE_CONFIG_DIR`. To work around this, `claude-rig launch` automatically symlinks the rig's `mcp.json` as `.mcp.json` in the current working directory. If a real `.mcp.json` already exists in the project, it is left untouched and a warning is shown.
>
> **Plugins:** Installing a plugin from within a rig session stays within that rig — the plugin cache, manifest, and enablement all go to the rig's config directory. Plugins installed in one rig don't appear in other rigs.

## Commands

| Command | Description |
|---|---|
| `init` | Initialize claude-rig and install shell integration |
| `create <name>` | Create a new rig (`--link-auth` to reuse existing auth) |
| `clone <src\|default> <dest>` | Clone a rig or `~/.claude/` config (`--link-auth` to share auth) |
| `delete <name>` | Delete a rig |
| `list` | List all rigs with auth status and item counts |
| `launch [name] [args]` | Launch Claude Code with a rig |
| `use <name>` | Set the active rig |
| `current` | Show the active rig |
| `rc [name]` | Show or create `.claude-rig` file for current directory |
| `link-auth <name>` | Link rig to shared auth (`--from <rig>` for cross-rig) |
| `unlink-auth <name>` | Remove shared auth so the rig gets its own |
| `set-args [name] <args>` | Set default launch args (global or per-rig) |
| `show-args [name]` | Show default launch args |
| `doctor` | Diagnose broken symlinks and missing items |

## How It Works

Each rig is a full config directory under `~/.claude-rig/rigs/<name>/`:

```
~/.claude-rig/rigs/webdev/
    CLAUDE.md               ← Real file (rig-specific instructions)
    settings.json           ← Real file (rig-specific config)
    skills/                 ← Real directory
    plugins/                ← Real directory
    mcp.json                ← Real file
    sessions/ → ~/.claude/  ← Symlink (shared history)
    todos/ → ~/.claude/     ← Symlink (shared)
    ...
```

On `launch`, claude-rig:

1. Sets `CLAUDE_CONFIG_DIR` to the rig directory
2. Symlinks the rig's `mcp.json` as `.mcp.json` in the working directory
3. Loads global `~/.claude/CLAUDE.md` via `--add-dir`
4. Refreshes symlinks to pick up any new shared files
5. Replaces itself with Claude Code via `exec` (no wrapper process)

Two Claude Code instances with different rigs run simultaneously without conflicts.

## Platform Support

- **Linux** — Full support
- **macOS** — Full support
- **Windows** — Requires Developer Mode (for symlinks)

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
