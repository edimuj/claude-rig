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

## Why claude-rig

Most tools in this space swap credential files between accounts. claude-rig isolates the entire configuration — and lets you run them simultaneously.

| | claude-rig | Credential switchers |
|---|---|---|
| Run multiple configs simultaneously | Yes | No — must close Claude first |
| Isolated settings, plugins, MCP, hooks | Yes | No — auth only |
| Choose what's shared vs isolated per rig | Yes | No |
| Per-project auto-selection | Yes | No |
| Status, diff, export/import | Yes | No |
| Dependencies | None (single binary) | Varies |

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

### Using Rigs as Templates

Clone an existing rig to use it as a starting point, then customize:

```bash
# Clone your go rig into a new one
claude-rig clone go rust --link-auth

# Create a fully isolated rig — its own history, conversations, projects
claude-rig create cleanroom --link-auth --isolate history.jsonl,conversations,projects
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
| `list` | List all rigs (`*` = running sessions) |
| `launch [name] [args]` | Launch Claude Code with a rig (flags forwarded to claude) |
| `rc [name]` | Show or create `.claude-rig` file for current directory |
| `link-auth <name>` | Link rig to shared auth (`--from <rig>` for cross-rig) |
| `unlink-auth <name>` | Remove shared auth so the rig gets its own |
| `set-args [name] <args>` | Set default launch args (global or per-rig) |
| `show-args [name]` | Show default launch args |
| `isolate <rig> <items>` | Isolate items per rig (no sharing via symlinks) |
| `share <rig> <items>` | Reverse isolation (delete local, recreate symlink) |
| `isolation [rig]` | Show isolation status for one or all rigs |
| `diff <rig1> <rig2>` | Compare two rigs (auth, settings, plugins, MCP, isolation, etc.) |
| `export <rig> [file]` | Export rig to `.tar.gz` (`--include-auth`, `--include-data`) |
| `import <file> <name>` | Import rig from archive (`--link-auth` to link auth after import) |
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

## Export & Import

Portable `.tar.gz` archives for backup, machine migration, or sharing setups with teammates.

```bash
# Export a rig (settings, skills, plugins, agents, commands, hooks, MCP config)
claude-rig export webdev                          # → webdev.tar.gz
claude-rig export webdev ~/backups/webdev.tar.gz  # explicit path
claude-rig export webdev --include-auth           # include auth credentials
claude-rig export webdev --include-data           # include isolated conversations/history

# Import on another machine (or as a new rig)
claude-rig import webdev.tar.gz webdev-restored
claude-rig import webdev.tar.gz webdev-restored --link-auth  # link local auth after import
```

**Default export includes:** settings, CLAUDE.md, skills, plugins, agents, commands, hooks, MCP config, isolation config.
**Excluded by default:** auth credentials (use `--include-auth`), symlinked shared data (recreated on import), isolated data files (use `--include-data`).

### Git-Based Backup (Alternative)

For version-controlled backups of `~/.claude-rig/`, see the `.gitignore` patterns in previous releases. The export/import commands are simpler for most use cases.

## Rig Status

See what's going on across all your rigs at a glance:

```bash
$ claude-rig status
* go                 auth: linked  plugins: 5  mcp: 1  isolated: 0  disk: 44K   running:7  last: just now
  minimal            auth: linked  plugins: 5  mcp: 0  isolated: 0  disk: 37K   last: 1d ago
  webdev             auth: own     plugins: 3  mcp: 2  isolated: 3  disk: 1.2M  running:2  last: 3h ago
```

Drill into a single rig for details:

```bash
$ claude-rig status go
Rig: go
  Auth:       linked
  Skills:     1
  Plugins:    5
  MCP:        1
  Isolated:   none
  Disk:       44K (real: 44K, symlinked: 451B)
  Sessions:   2 running (PID 12345, 67890)
  Last used:  just now
  Path:       /home/user/.claude-rig/rigs/go
```

## Rig Diff

Compare two rigs to see what's different — settings, plugins, MCP servers, skills, agents, isolation config, and more:

```bash
$ claude-rig diff go minimal
  Auth:       same (linked)
  Settings:   2 differences (enabledPlugins, hooks)
  Plugins:    same (5)
  Skills:     go has 1, minimal has 0 | only go: checkpoint
  Agents:     go has 1, minimal has 0 | only go: go-reviewer.md
  Commands:   same (0)
  Hooks:      same (0)
  MCP:        go has 1, minimal has 0 | only go: gopls
  Isolation:  same (none)
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
