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

**Homebrew** (macOS / Linux):
```bash
brew install edimuj/tap/claude-rig
```

**Shell script** (macOS / Linux — detects OS and architecture):
```bash
curl -sSL https://raw.githubusercontent.com/edimuj/claude-rig/main/install.sh | sh
```

**PowerShell** (Windows):
```powershell
irm https://raw.githubusercontent.com/edimuj/claude-rig/main/install.ps1 | iex
```

**Go install** (requires Go):
```bash
go install github.com/edimuj/claude-rig/cmd/claude-rig@latest
```

**Download binary**: grab the latest release from [GitHub Releases](https://github.com/edimuj/claude-rig/releases) (macOS, Linux, Windows).

**Build from source**:
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

# Create a clean slate with nothing isolated (shared everything except always-isolated items)
claude-rig create cleanroom --link-auth --no-isolate-defaults
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

| Always per-rig | Per-rig, inheritable/syncable | Isolated by default | Shared by default |
|---|---|---|---|
| `settings.json` | `skills/` (inherit) | `conversations`, `history.jsonl`, `sessions` | `telemetry` |
| `CLAUDE.md` | `agents/` (inherit) | `channels`, `tasks`, `todos`, `backups` | `usage-data` |
| `.claude.json` | `hooks/` (inherit) | `shell-snapshots`, `projects`, `plans` | |
| | `commands/` (inherit or isolate/share) | `paste-cache`, `ide`, `downloads`, `debug` | |
| | `plugins/` (sync from global) | `file-history`, `session-env`, `cache` | |
| | `mcp` (sync from global) | `stats-cache.json`, `statusline`, `chrome` | |

New rigs isolate 21 items by default — only `telemetry` and `usage-data` remain shared. Skills, agents, hooks, and commands are per-rig but can **inherit** entries from global `~/.claude/` (see [Global Inheritance](#global-inheritance)). Plugins and MCP servers can be **synced** from global (see [Plugin & MCP Sync](#plugin--mcp-sync)).

Each rig gets its own `.claude.json` seeded from the global config on creation. MCP servers configured via `claude mcp add` go directly into the rig's `.claude.json` — no symlinks, no project-level files.

### Configurable Isolation

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

### Global Inheritance

Skills, agents, hooks, and commands defined in `~/.claude/` can be inherited by any rig. This gives you a 3-layer configuration stack — just like how `CLAUDE.md` works:

```
~/.claude/skills/            ← Global (inherited by rigs that opt in)
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

### Plugin & MCP Sync

The `sync` command keeps plugins and MCP servers aligned across rigs in addition to symlinks and inherited items.

**Plugins** are synced by symlinking cache directories from global `~/.claude/` (or another rig via `--from`) into the rig, then rewriting manifest paths to match. This avoids duplicating plugin data while keeping each rig's plugin state consistent.

**MCP servers** are merged from the global `~/.claude.json` into the rig's `.claude.json`. Local entries take precedence — if a rig defines its own version of an MCP server, the global one is skipped.

**Tracking:** Synced items are recorded in `rig.json` (`synced_plugins`, `synced_mcp`) so that `isolate` and `share` can cleanly add or remove them.

```bash
# Sync everything (symlinks, inherited items, plugins, MCP) for all rigs
claude-rig sync

# Sync a single rig
claude-rig sync myrig

# Sync from another rig instead of global
claude-rig sync myrig --from webdev

# Skip plugins or MCP during sync
claude-rig sync myrig --no-plugins
claude-rig sync myrig --no-mcp

# Skip inherited items during sync
claude-rig sync myrig --no-inherit

# Isolate plugins — removes synced plugins from the rig
claude-rig isolate myrig plugins

# Share them back — re-syncs plugins from global
claude-rig share myrig plugins

# Same for MCP servers
claude-rig isolate myrig mcp
claude-rig share myrig mcp
```

### Managing Plugins & Marketplaces

Use `claude-rig plugin` instead of `claude plugin` to ensure plugins are installed to the correct rig. It forwards all arguments to `claude plugin` with the right `CLAUDE_CONFIG_DIR` set.

```bash
# Add a plugin to the active rig
claude-rig plugin add edimuj/claude-mneme

# Add a marketplace
claude-rig plugin marketplace add github.com/anthropics/claude-plugins-official

# Update marketplaces and list plugins
claude-rig plugin marketplace update
claude-rig plugin list

# Remove a plugin
claude-rig plugin remove claude-mneme@claude-mneme

# Target a specific rig instead of the active one
claude-rig plugin add edimuj/claude-mneme --rig webdev
claude-rig plugin list --rig minimal
```

**Why not `claude plugin` directly?** When a rig is active, `CLAUDE_CONFIG_DIR` points Claude at the rig directory. But if you run `claude plugin add` from a fresh shell (without the rig env set), the plugin lands in `~/.claude/` — not your rig. `claude-rig plugin` always resolves the correct rig and sets the environment, so the plugin ends up where you expect.

## Commands

| Command | Description |
|---|---|
| `init` | Initialize claude-rig and install shell integration |
| `create <name>` | Create a new rig with default isolation (`--link-auth`, `--no-isolate-defaults`, `--isolate-all`) |
| `clone <src\|default> <dest>` | Clone a rig or `~/.claude/` config (`--link-auth` to share auth) |
| `delete <name>` | Delete a rig |
| `rename <old> <new>` | Rename a rig |
| `list` | List all rigs (`*` = running sessions) |
| `launch [name] [args]` | Launch Claude Code with a rig (flags forwarded to claude) |
| `rc [name]` | Show or create `.claude-rig` file for current directory |
| `link-auth <name>` | Link rig to shared auth (`--from <rig>` for cross-rig) |
| `unlink-auth <name>` | Remove shared auth so the rig gets its own |
| `set-args [name] <args>` | Set default launch args (global or per-rig) |
| `show-args [name]` | Show default launch args |
| `isolate <rig> <items>` | Isolate items per rig — supports files, dirs, `plugins`, `mcp` |
| `share <rig> <items>` | Reverse isolation — supports files, dirs, `plugins`, `mcp` |
| `isolation [rig]` | Show isolation status for one or all rigs |
| `inherit <items> [rig]` | Inherit global skills/agents/hooks/commands from `~/.claude/` |
| `uninherit <items> [rig]` | Stop inheriting (remove global symlinks) |
| `diff <rig1> <rig2>` | Compare two rigs (auth, settings, plugins, MCP, isolation, etc.) |
| `export <rig> [file]` | Export rig to `.tar.gz` (`--include-auth`, `--include-data`) |
| `import <file> <name>` | Import rig from archive (`--link-auth` to link auth after import) |
| `status [rig]` | Show rig status: disk usage, running sessions, last used |
| `plugin <subcommand>` | Run `claude plugin` commands on active rig (`--rig <name>` to target another) |
| `update-plugins [rigs]` | Update marketplace plugins across rigs (all if none specified) |
| `sync [rig]` | Sync symlinks, inherited items, plugins, MCP (all rigs if none given) |
| `update` | Update Claude Code (forwards to `claude update`) |
| `doctor` | Diagnose broken symlinks, plugin/MCP health, inherited items |

## How It Works

Each rig is a full config directory under `~/.claude-rig/rigs/<name>/`:

```
~/.claude-rig/rigs/webdev/
    .claude.json            ← Real file (MCP servers, onboarding state)
    CLAUDE.md               ← Real file (rig-specific instructions)
    settings.json           ← Real file (rig-specific config)
    rig.json                ← Isolation config, synced_plugins, synced_mcp tracking
    skills/                 ← Real directory
    plugins/                ← Real directory (synced from global by default)
    conversations/          ← Real dir (isolated by default)
    history.jsonl           ← Real file (isolated by default)
    telemetry/ → ~/.claude/ ← Symlink (shared by default)
    ...
```

On `launch`, claude-rig:

1. Sets `CLAUDE_CONFIG_DIR` to the rig directory
2. Loads global `~/.claude/CLAUDE.md` via `--add-dir`
3. Refreshes symlinks to pick up any new shared files
4. Replaces itself with Claude Code via `exec` (Unix) or spawns it as a child process (Windows)

Two Claude Code instances with different rigs run simultaneously without conflicts.

## Platform Support

- **Linux** — Full support
- **macOS** — Full support
- **Windows** — Requires Developer Mode (for symlinks). Session detection (`*` markers in `list`/`status`) is not available on Windows

### Windows

Windows requires **Developer Mode** enabled for symlink support. `claude-rig init` checks this automatically and fails fast with a clear message if symlinks aren't available.

**Shell integration** works with both PowerShell and Git Bash:

- **PowerShell** (5.x and 7+) — Installs a `claude` function wrapper in your PowerShell profile
- **Git Bash / MSYS2** — Detects `SHELL` env var and installs the same bash wrapper as Linux/macOS

**Launch behavior** differs slightly: on Unix, `claude-rig launch` replaces itself with Claude Code via `exec`. On Windows, it spawns Claude Code as a child process and forwards the exit code.

**Cross-compile from Linux/macOS:**

```bash
make build-windows    # produces claude-rig.exe (amd64)
```

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

## Known Limitations

### Remote Control

Claude Code's [Remote Control](https://code.claude.com/docs/en/remote-control) feature works with claude-rig but is fragile when multiple rigs have it enabled simultaneously. All rigs sharing auth (via `--link-auth`) use the same OAuth tokens, causing rate limiting and connection instability when multiple bridge sessions compete.

**Recommendation:** Enable `remoteControlAtStartup` on **one rig only**. The `doctor` command warns when multiple rigs have it enabled.

The underlying issue is a Claude Code bug — when the bridge WebSocket dies, it doesn't recover and the session becomes a zombie (visible in the app but unable to sync). This happens regardless of claude-rig but is more likely with shared auth.

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
make build-windows        # cross-compile for Windows (amd64)
make run ARGS="version"   # run without installing
make install              # install to ~/go/bin/
```

## License

MIT
