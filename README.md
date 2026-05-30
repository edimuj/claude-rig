<p align="center">
  <img src="assets/claude-rig-raccoon-256.png" alt="claude-rig mascot" width="180" />
</p>

<h1 align="center">claude-rig</h1>

<p align="center">
  <strong>Run multiple Claude Code configurations side by side.</strong>
</p>

<p align="center">
  <a href="https://github.com/edimuj/claude-rig/releases"><img src="https://img.shields.io/github/v/release/edimuj/claude-rig?style=flat-square" alt="Release"></a>
  <a href="https://github.com/edimuj/claude-rig/blob/main/LICENSE"><img src="https://img.shields.io/github/license/edimuj/claude-rig?style=flat-square" alt="License"></a>
  <a href="https://github.com/edimuj/claude-rig"><img src="https://img.shields.io/github/stars/edimuj/claude-rig?style=flat-square" alt="Stars"></a>
  <img src="https://img.shields.io/badge/Go-stdlib%20only-00ADD8?style=flat-square&logo=go" alt="Go stdlib only">
</p>

<p align="center">
  <img src="assets/demo.gif" alt="claude-rig demo" width="800" />
</p>

## The Problem

Claude Code keeps all configuration in a single `~/.claude/` directory. That works until you need:

- **Multiple subscriptions** -- You have 2-5 Max accounts and have to log out and back in every time you switch. No way to run two subscriptions at the same time.
- **API and subscription side by side** -- Want API access for one project and your Max subscription for another? Same deal -- one config, one auth.
- **Different tools per project** -- Your web project needs Tailwind skills and a database MCP server. Your CLI tool needs none of that. But every session loads the same plugins, MCP servers, and hooks.
- **Testing without risk** -- Want to try a new MCP server without it leaking into your production setup? You'd have to disable everything else, test, then re-enable. No isolation.

## What claude-rig Does

Each configuration becomes its own isolated **rig** -- separate settings, skills, plugins, agents, commands, hooks, MCP servers, and instructions. Auth can be shared across rigs or kept separate. Run them side by side in different terminals.

```bash
# Terminal 1: your Max subscription, minimal setup
claude --rig=minimal

# Terminal 2: different Max account, full web dev stack
claude --rig=webdev

# Terminal 3: API access, experimental MCP server you're testing
claude --rig=experiment
```

## Why claude-rig

Most tools in this space swap credential files between accounts. claude-rig isolates the entire configuration -- and lets you run them at the same time.

| | claude-rig | Credential switchers |
|---|---|---|
| Run multiple configs simultaneously | Yes | No -- must close Claude first |
| Isolated settings, plugins, MCP, hooks | Yes | No -- auth only |
| Choose what's shared vs isolated per rig | Yes | No |
| Per-project auto-selection | Yes | No |
| Status, diff, export/import | Yes | No |
| Dependencies | None (single binary) | Varies |

## What It Doesn't Do

- **Doesn't modify Claude Code.** Uses the official `CLAUDE_CONFIG_DIR` environment variable and `--add-dir` flag. No patches, no forks.
- **Doesn't manage Claude Code installation.** Install Claude Code yourself -- claude-rig manages its configuration.
- **Doesn't replace project-level config.** Your project's `CLAUDE.md` and `.claude/` still work normally. Rigs handle user-level configuration.
- **No external dependencies.** Single Go binary, stdlib only.

## Install

**Homebrew** (macOS / Linux):
```bash
brew install edimuj/tap/claude-rig
```

**Shell script** (macOS / Linux):
```bash
curl -sSL https://raw.githubusercontent.com/edimuj/claude-rig/main/install.sh | sh
```

**PowerShell** (Windows):
```powershell
irm https://raw.githubusercontent.com/edimuj/claude-rig/main/install.ps1 | iex
```

**Go install**:
```bash
go install github.com/edimuj/claude-rig/cmd/claude-rig@latest
```

Download binaries: [GitHub Releases](https://github.com/edimuj/claude-rig/releases) -- macOS, Linux, Windows.

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
claude-rig          # picks up the rig automatically
```

## Commands

| Command | Description |
|---|---|
| `agents [rig] [args]` | Launch Claude Code in agents (orchestrator) mode |
| `blueprint <sub>` | Shareable rig specs: `create`, `apply`, `inspect`, `list`, `pack` |
| `clone <src\|default> <dest>` | Clone a rig or `~/.claude/` config |
| `config <sub>` | Manage claude-rig preferences (`set`, `get`, `remove`, `list`) |
| `create <name>` | Create a new rig (`--link-auth`, `--no-isolate-defaults`, `--isolate-all`) |
| `delete <name>` | Delete a rig |
| `diff <rig1> <rig2>` | Compare two rigs |
| `doctor [--fix]` | Diagnose and fix broken symlinks, plugin/MCP health |
| `export <rig> [file]` | Export rig to `.tar.gz` |
| `import <file> <name>` | Import rig from archive |
| `inherit <items> [rig]` | Inherit global skills/agents/hooks/commands |
| `init` | Initialize claude-rig and install shell integration |
| `isolate <rig> <items>` | Isolate items per rig |
| `isolation [rig]` | Show isolation status for one or all rigs (`--json`) |
| `launch [name] [args]` | Launch Claude Code with a rig |
| `link-auth <name>` | Link rig to shared auth |
| `list` | List all rigs (`*` = running sessions; `--json` for machine output) |
| `pin <version>` | Pin rig to a specific Claude Code version |
| `plugin <subcommand>` | Run `claude plugin` commands on the correct rig |
| `rc [name]` | Show or create `.claude-rig` file for current directory |
| `rename <old> <new>` | Rename a rig |
| `set-args [name] <args>` | Set default launch args (global or per-rig) |
| `settings <sub>` | Manage default settings applied to all rigs |
| `share <rig> <items>` | Reverse isolation |
| `show-args [name]` | Show default launch args |
| `status [rig]` | Show rig status: disk, sessions, last used (`--json`) |
| `sync [rig]` | Sync symlinks, inherited items, plugins, MCP, settings |
| `uninherit <items> [rig]` | Stop inheriting |
| `unlink-auth <name>` | Remove shared auth |
| `unpin` | Remove version pin |
| `update` | Update Claude Code |
| `update-plugins [-j N] [rigs]` | Update marketplace plugins across rigs (default: 2 concurrent) |
| `versions` | List installed Claude Code versions on disk |

## How It Works

Each rig is a full config directory under `~/.claude-rig/rigs/<name>/`:

```
~/.claude-rig/rigs/webdev/
    .claude.json            # real file (MCP servers, onboarding state)
    CLAUDE.md               # real file (rig-specific instructions)
    settings.json           # real file (rig-specific config)
    rig.json                # isolation config, synced plugins/MCP, overrides
    skills/                 # real directory
    plugins/                # real directory (synced from global by default)
    conversations/          # real dir (isolated by default)
    history.jsonl           # real file (isolated by default)
    telemetry/ -> ~/.claude/ # symlink (shared by default)
    ...
```

On `launch`, claude-rig resolves the binary (pinned or latest on disk), sets `CLAUDE_CONFIG_DIR` to the rig directory, loads global `~/.claude/CLAUDE.md` via `--add-dir`, refreshes symlinks, then replaces itself with Claude Code via `exec`. Two instances with different rigs can run at the same time without conflicts.

## Blueprints -- Share Your Setup

You've spent hours dialing in your Claude Code setup. The right plugins, MCP servers, skills, settings, CLAUDE.md instructions. Then someone asks "what's your setup?" and you have nothing to hand them. No export, no shareable format. You end up writing a wall of text listing everything by hand.

Blueprints fix that. One command extracts your rig into a portable spec. Another recreates it from scratch.

```bash
# Extract your setup
claude-rig blueprint create my-setup --from go

# Anyone can recreate it -- from GitHub, a file, or a local directory
claude-rig blueprint apply edimuj/my-setup --link-auth
```

Unlike `export`/`import` (which creates full file archives for backup), blueprints are lightweight specs. Plugins are listed as install references, not binary blobs. The whole thing is a small JSON file plus any custom skills or agents you've written. Push it to GitHub and your followers, teammates, or future self can spin up your exact setup in one command.

**What you can do with blueprints:**

- Share your setup on social media -- "here's the rig I use for all my Go projects"
- Onboard new team members with one command instead of a wiki page
- Reproduce your setup on a new machine without remembering what you installed
- Publish alongside a blog post or tutorial so readers can follow along with your exact config
- Keep a library of blueprints for different types of work -- web, mobile, data, DevOps

See [docs/blueprints.md](docs/blueprints.md) for the full reference.

## Platform Support

- **Linux** -- Full support
- **macOS** -- Full support
- **Windows** -- Requires Developer Mode (for symlinks). See [docs/windows.md](docs/windows.md).

## Documentation

| Topic | Doc |
|---|---|
| Per-project RC files, shell integration, CLAUDE.md, templates | [docs/configuration.md](docs/configuration.md) |
| Isolation model, configurable isolation, global inheritance | [docs/isolation.md](docs/isolation.md) |
| Plugin sync, MCP sync, default settings, version management | [docs/sync.md](docs/sync.md) |
| Blueprints -- share your Claude Code setup with anyone | [docs/blueprints.md](docs/blueprints.md) |
| Export, import, status, diff | [docs/export-import.md](docs/export-import.md) |
| Windows setup, known limitations | [docs/windows.md](docs/windows.md) |

## Also Check Out

| Project | Description |
|---|---|
| [tokenlean](https://github.com/edimuj/tokenlean) | Lean CLI tools for AI agents -- reduce context, save tokens |
| [claude-mneme](https://github.com/edimuj/claude-mneme) | Automatic session memory -- every session picks up where the last left off |
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
