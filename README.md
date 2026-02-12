<p align="center">
  <img src="assets/claude-rig-raccoon-256.png" alt="claude-rig mascot" width="180" />
</p>

# claude-rig

Manage multiple Claude Code configuration rigs with full isolation for concurrent use.

Run `claude --rig minimal` in one terminal and `claude --rig webappdev` in another — each with its own settings, skills, plugins, MCP servers, and agents — while sharing authentication, memory, and session history.

## How It Works

Each rig gets its own config directory under `~/.claude-rig/rigs/`. Rig-specific files (settings, skills, plugins, agents, commands, hooks, MCP config) live as real files in the rig directory. Shared files (CLAUDE.md, credentials, sessions, todos) are symlinked back to `~/.claude/`, so all rigs share authentication and memory.

```
~/.claude/                          # Canonical home (shared)
    CLAUDE.md                       # Personal memory — shared
    credentials.json                # Auth — shared
    sessions/                       # History — shared

~/.claude-rig/
    .active                             # Current rig marker
    rigs/
        minimal/
        settings.json               # Rig-specific
        skills/                     # Rig-specific
        plugins/                    # Rig-specific
        agents/                     # Rig-specific
        CLAUDE.md → ~/.claude/CLAUDE.md    # Symlink to shared
    webappdev/
        settings.json               # Different config
        skills/                     # Different skills
        plugins/                    # Different plugins
        ...
```

## Install

```bash
go install github.com/edimuj/claude-rig/cmd/claude-rig@v0.7.0
```

Or build from source:

```bash
git clone https://github.com/edimuj/claude-rig.git
cd claude-rig
make build
```

## Development

```bash
make build                # build binary
make run ARGS="version"   # run without installing
make install              # install to ~/go/bin/
```

## Quick Start

```bash
# Initialize the rig system
claude-rig init

# Create rigs
claude-rig create minimal
claude-rig create webappdev

# Launch Claude Code with a specific rig
claude-rig launch webappdev

# In another terminal, run a different rig simultaneously
claude-rig launch minimal
```

## Project Rigs

Bind a rig to a project directory with a `.claude-rig` file so you don't have to type the rig name every time:

```bash
cd ~/projects/my-app

# Create the .claude-rig file
claude-rig rc webappdev

# Now just run claude-rig — it picks up the rig automatically
claude-rig
```

The file is a simple key=value format:

```
rig=webappdev
```

Lookup walks from the current directory up to `$HOME`, so subdirectories inherit the project's rig. Both `claude-rig` (no subcommand) and `claude-rig launch` (no rig arg) will use the RC file. An explicit `claude-rig launch other` still overrides it.

## Commands

| Command | Description |
|---|---|
| `init` | Initialize the rig system (`~/.claude-rig/rigs/`) |
| `create <name>` | Create a new rig (`--link-auth` to reuse existing auth) |
| `clone <src> <dest>` | Clone a rig (`--link-auth` to use shared auth) |
| `link-auth <name>` | Link rig to shared auth (`--from <rig>` for cross-rig) |
| `unlink-auth <name>` | Remove shared auth, rig will use its own |
| `list` | List all rigs with auth status and item counts |
| `use <name>` | Set the active rig |
| `current` | Show currently active rig |
| `delete <name>` | Delete a rig (with confirmation) |
| `launch [name] [args]` | Launch Claude Code with the given rig (or from `.claude-rig` file) |
| `rc [name]` | Show or create `.claude-rig` file for current directory |
| `set-args [name] <args>` | Set default launch args (global if no name, per-rig if given) |
| `show-args [name]` | Show default launch args |
| `doctor` | Check rigs for broken symlinks and missing items |

## Shell Integration

Add to `~/.bashrc` or `~/.zshrc`:

```bash
# Quick aliases per rig
alias claude-minimal='claude-rig launch minimal'
alias claude-webdev='claude-rig launch webappdev'

# Or a wrapper that supports --rig=<name>
claude() {
  for arg in "$@"; do
    if [[ "$arg" == --rig=* ]]; then
      claude-rig launch "${arg#--rig=}" "${@//$arg/}"
      return
    fi
  done
  command claude "$@"
}
```

Then use naturally:

```bash
claude --rig=webappdev
claude --rig=minimal --dangerously-skip-permissions
```

## What's Isolated vs. Shared

| Isolated per rig | Shared across rigs |
|---|---|
| `settings.json` | `CLAUDE.md` (memory) |
| `skills/` | `credentials.json` (auth) |
| `plugins/` | `sessions/` (history) |
| `agents/` | `todos/` |
| `commands/` | All other files |
| `hooks/` | |
| `mcp.json` | |

## Environment Variables

| Variable | Description |
|---|---|
| `CLAUDE_BINARY` | Override the Claude Code binary name/path |
| `CLAUDE_CONFIG_DIR` | (Used internally — set automatically by launch) |

## How It Works Under the Hood

The tool leverages Claude Code's `CLAUDE_CONFIG_DIR` environment variable. Each rig is a full config directory where:

1. **Rig-specific files** are real files unique to each rig
2. **Shared files** are symlinks pointing back to `~/.claude/`
3. On `launch`, symlinks are refreshed to pick up any new shared files
4. Claude Code is exec'd with `CLAUDE_CONFIG_DIR` pointing to the rig directory

This means two Claude Code instances with different rigs can run simultaneously without any conflicts.

## Platform Support

- **Linux**: Full support
- **macOS**: Full support
- **Windows**: Requires Developer Mode enabled (for symlink support)

## License

MIT
