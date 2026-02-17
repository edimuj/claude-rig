# claude-rig

Go CLI tool for managing multiple Claude Code configuration rigs.

## Architecture

Single-binary CLI, `package main` in `cmd/claude-rig/`. Three files:

- `main.go` — entrypoint, command dispatch (`switch` on `os.Args[1]`), usage text
- `commands.go` — all command implementations + helpers (~2000 lines, the workhorse)
- `paths.go` — filesystem paths, rig-specific item list, platform detection

No interfaces, no packages, no abstractions. Functions call functions.

## Key Concepts

- All state under `~/.claude-rig/` (rigs in `~/.claude-rig/rigs/<name>/`)
- Rig-specific items (settings, skills, plugins, agents, commands, hooks, mcp.json) = real files
- Everything else in `~/.claude/` = symlinked into each rig directory
- Auth files (`.credentials.json`, `.claude.json`, `statsig/`) shared via `--link-auth`
- `.claude.json` lives in `~/` not `~/.claude/` — special-cased in auth linking
- `CLAUDE_CONFIG_DIR` env var points Claude Code at the rig directory
- `launch` uses `syscall.Exec` (replaces process, not subprocess)
- `rig.json` in rig dir controls per-rig isolation (`{"isolate": ["history.jsonl", ...]}`)
- `syncSharedSymlinks` skips items in rig.json isolate list
- Version injected via ldflags from git tags

## Command → Function Map

| Command          | Function           | Notes                                                          |
|------------------|--------------------|----------------------------------------------------------------|
| `init`           | `cmdInit`          | Shell integration, `.bashrc`/`.zshrc` detection                |
| `create`         | `cmdCreate`        | Seeds `.claude.json`, creates rig-specific dirs                |
| `clone`          | `cmdClone`         | `cloneFromDefault` for `~/.claude/`, `cloneDir` for rig-to-rig |
| `delete`         | `cmdDelete`        | Refuses to delete active rig                                   |
| `list`           | `cmdList`          | Auth status, item counts, MCP server counts                    |
| `launch`         | `cmdLaunch`        | Resolves `.claude-rig` file, sets env, `syscall.Exec`          |
| `use`            | `cmdUse`           | Writes `.active` file                                          |
| `current`        | `cmdCurrent`       | Reads `.active` file                                           |
| `rc`             | `cmdRC`            | Creates/reads `.claude-rig` project file, walks up dirs        |
| `link-auth`      | `cmdLinkAuth`      | Symlinks auth files, `--from` for cross-rig                    |
| `unlink-auth`    | `cmdUnlinkAuth`    | Copies auth files back to break sharing                        |
| `set-args`       | `cmdSetArgs`       | Global or per-rig default launch args                          |
| `show-args`      | `cmdShowArgs`      |                                                                |
| `isolate`        | `cmdIsolate`       | Mark items as per-rig (remove symlink, create local)           |
| `share`          | `cmdShare`         | Reverse isolation (delete local, recreate symlink)             |
| `isolation`      | `cmdIsolation`     | Show isolation status for one or all rigs                      |
| `status`         | `cmdStatus`        | Disk, sessions (/proc), last used; overview or single-rig detail |
| `update-plugins` | `cmdUpdatePlugins` | Parallel across rigs, calls `claude plugin` CLI                |
| `doctor`         | `cmdDoctor`        | Checks broken symlinks, missing items                          |

## Adding a New Command

1. Add function `cmdFoo(args []string) error` in `commands.go`
2. Add `case "foo":` in the `switch` in `main.go`
3. Add to `printUsage()` in `main.go`
4. Add to Commands table in `README.md`

## Build & Test

```bash
make build                # produces ./claude-rig (version from git tags)
make install              # moves to ~/go/bin/
make run ARGS="version"   # run without installing
```

No test suite — manual testing against real `~/.claude-rig/` state.

## Constraints

- stdlib only — zero external dependencies
- Single `package main` — no packages, no internal/
- Binary name = repo name = `claude-rig`
