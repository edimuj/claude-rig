# claude-rig

Go CLI tool for managing multiple Claude Code configuration profiles.

## Architecture

Single-binary CLI. Source lives in `cmd/claude-rig/` as `package main` (standard Go layout).

- `cmd/claude-rig/main.go` — entrypoint, command dispatch, usage text
- `cmd/claude-rig/commands.go` — all command implementations + helpers
- `cmd/claude-rig/paths.go` — filesystem paths, profile-specific item list, platform detection

## Key Concepts

- Profiles live in `~/.claude-profiles/<name>/`
- Profile-specific items (settings, skills, plugins, agents, commands, hooks, mcp.json) are real files
- Everything else in `~/.claude/` is symlinked into each profile directory
- `CLAUDE_CONFIG_DIR` env var is used to point Claude Code at the profile directory
- `launch` uses `syscall.Exec` to replace the process (not subprocess)

## Build & Run

```bash
make build        # produces ./claude-rig
make install      # moves to ~/go/bin/
```

## Conventions

- Error handling: early returns, guard clauses
- No external dependencies (stdlib only)
- Binary name = repo name = `claude-rig`
