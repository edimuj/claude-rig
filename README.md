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

- **Multiple subscriptions** — Juggling 2-5 Max accounts? You have to log out and back in every time you switch. There's no way to run two subscriptions simultaneously.
- **API and subscription side by side** — Want API access for one project and your Max subscription for another? Same problem — one config, one auth.
- **Different tools per project** — Your web project needs Tailwind skills and a database MCP server. Your CLI tool needs none of that. But every Claude Code session loads the same plugins, the same MCP servers, the same hooks.
- **Testing without risk** — Want to see if a new MCP server is eating your context window? You'd have to disable everything else, test, then re-enable. No way to isolate the experiment.

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

Download binaries: [GitHub Releases](https://github.com/edimuj/claude-rig/releases) — macOS, Linux, Windows.

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
| `init` | Initialize claude-rig and install shell integration |
| `create <name>` | Create a new rig (`--link-auth`, `--no-isolate-defaults`, `--isolate-all`) |
| `clone <src\|default> <dest>` | Clone a rig or `~/.claude/` config |
| `delete <name>` | Delete a rig |
| `rename <old> <new>` | Rename a rig |
| `list` | List all rigs (`*` = running sessions) |
| `launch [name] [args]` | Launch Claude Code with a rig |
| `rc [name]` | Show or create `.claude-rig` file for current directory |
| `sync [rig]` | Sync symlinks, inherited items, plugins, MCP, settings |
| `settings <sub>` | Manage default settings applied to all rigs |
| `isolate <rig> <items>` | Isolate items per rig |
| `share <rig> <items>` | Reverse isolation |
| `isolation [rig]` | Show isolation status for one or all rigs |
| `inherit <items> [rig]` | Inherit global skills/agents/hooks/commands |
| `uninherit <items> [rig]` | Stop inheriting |
| `link-auth <name>` | Link rig to shared auth |
| `unlink-auth <name>` | Remove shared auth |
| `set-args [name] <args>` | Set default launch args (global or per-rig) |
| `show-args [name]` | Show default launch args |
| `plugin <subcommand>` | Run `claude plugin` commands on the correct rig |
| `update-plugins [rigs]` | Update marketplace plugins across rigs |
| `versions` | List installed Claude Code versions on disk |
| `pin <version>` | Pin rig to a specific Claude Code version |
| `unpin` | Remove version pin |
| `update` | Update Claude Code |
| `blueprint <sub>` | Shareable rig specs: `create`, `apply`, `inspect`, `list`, `pack` |
| `diff <rig1> <rig2>` | Compare two rigs |
| `export <rig> [file]` | Export rig to `.tar.gz` |
| `import <file> <name>` | Import rig from archive |
| `status [rig]` | Show rig status: disk, sessions, last used |
| `doctor` | Diagnose broken symlinks, plugin/MCP health |

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

On `launch`, claude-rig resolves the binary (pinned or latest on disk), sets `CLAUDE_CONFIG_DIR` to the rig directory, loads global `~/.claude/CLAUDE.md` via `--add-dir`, refreshes symlinks, then replaces itself with Claude Code via `exec`. Two instances with different rigs run simultaneously without conflicts.

## Blueprints — Share Your Setup

You've spent hours dialing in your Claude Code setup. The right plugins, MCP servers, skills, settings, CLAUDE.md instructions. Then someone asks "what's your setup?" and you have... nothing. No way to export it. No way to share it. You end up writing a wall of text listing everything manually.

Blueprints fix that. One command extracts your rig into a portable spec. Another command recreates it from scratch.

```bash
# Extract your setup
claude-rig blueprint create my-setup --from go

# Anyone can recreate it — from GitHub, a file, or a local directory
claude-rig blueprint apply edimuj/my-setup --link-auth
```

Unlike `export`/`import` (which creates full file archives for backup), blueprints are lightweight specs. Plugins are listed as install references, not binary blobs. The whole thing is a small JSON file plus any custom skills or agents you've written. Push it to GitHub and your followers, teammates, or future self can spin up your exact setup in one command.

**Use cases:**

- Share your setup on social media — "here's the rig I use for all my Go projects"
- Onboard new team members — one blueprint, everyone's on the same tools
- Reproduce your setup on a new machine without remembering what you installed
- Publish alongside a blog post or tutorial so readers can follow along with your exact config
- Keep a library of blueprints for different types of work — web, mobile, data, DevOps

See [docs/blueprints.md](docs/blueprints.md) for the full reference.

## Platform Support

- **Linux** — Full support
- **macOS** — Full support
- **Windows** — Requires Developer Mode (for symlinks). See [docs/windows.md](docs/windows.md).

## Documentation

| Topic | Doc |
|---|---|
| Per-project RC files, shell integration, CLAUDE.md, templates | [docs/configuration.md](docs/configuration.md) |
| Isolation model, configurable isolation, global inheritance | [docs/isolation.md](docs/isolation.md) |
| Plugin sync, MCP sync, default settings, version management | [docs/sync.md](docs/sync.md) |
| Blueprints — share your Claude Code setup with anyone | [docs/blueprints.md](docs/blueprints.md) |
| Export, import, status, diff | [docs/export-import.md](docs/export-import.md) |
| Windows setup, known limitations | [docs/windows.md](docs/windows.md) |

## Also Check Out

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
