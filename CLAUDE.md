# claude-rig

Go CLI tool for managing multiple Claude Code configuration rigs.

## Architecture

Single-binary CLI, `package main` in `cmd/claude-rig/`. Three files:

- `main.go` — entrypoint, command dispatch (`switch` on `os.Args[1]`), usage text
- `commands.go` — all command implementations + helpers (~3500 lines, the workhorse)
- `paths.go` — filesystem paths, rig-specific item list, platform detection

No interfaces, no packages, no abstractions. Functions call functions.

## Key Concepts

- All state under `~/.claude-rig/` (rigs in `~/.claude-rig/rigs/<name>/`)
- Rig-specific items (settings, skills, plugins, agents, hooks) = real files
- `commands/` is isolatable — per-rig by default, can be shared via `share`
- New rigs isolate 21 items by default (conversations, history, sessions, etc.) — only telemetry/usage-data shared
- Everything else in `~/.claude/` = symlinked into each rig directory
- Auth files (`.credentials.json`, `.claude.json`, `statsig/`) shared via `--link-auth`
- `.claude.json` lives in `~/` not `~/.claude/` — special-cased in auth linking
- `CLAUDE_CONFIG_DIR` env var points Claude Code at the rig directory
- `launch` resolves binary via: pinned version → latest on disk (`~/.claude-rig/claude-latest`) → system symlink fallback
- `launch` uses `syscall.Exec` (replaces process, not subprocess)
- `rig.json` in rig dir controls isolation (`{"isolate": [...]}`), inheritance (`{"inherit": [...]}`),
  sync tracking (`{"synced_plugins": [...], "synced_mcp": [...]}`), and plugin MCP provenance (`{"plugin_mcp": {"server": "plugin@market"}}`)
- `syncPluginMCP` extracts `mcpServers` from installed plugins' `plugin.json` into the rig's `.mcp.json`
- `syncSharedSymlinks` skips items in rig.json isolate list
- `syncGlobalContents` symlinks entries from `~/.claude/{skills,agents,hooks,commands}/` into rig for inherited items
- `syncPlugins` copies plugins from global/source rig (symlinks cache dirs, rewrites manifest paths)
- `syncMCP` merges mcpServers from source `.claude.json` into rig's `.claude.json`
- `applyIsolation` helper shared by `cmdCreate` and `cloneFromDefault` for default isolation
- Inheritance = 3-layer: global (`~/.claude/`) → rig → project (`.claude/`). Rig-local files override inherited symlinks
- Version injected via ldflags from git tags
- Bundled plugin (`cmd/claude-rig/bundled/`) embedded in binary via `go:embed`, extracted
  to `~/.claude-rig/bundled-plugin/` on launch, loaded via `--plugin-dir`. Version-stamped
  — only re-extracts when binary version changes. Skills/agents shipped with claude-rig
  go here

## Command → Function Map

| Command          | Function           | Notes                                                            |
|------------------|--------------------|------------------------------------------------------------------|
| `init`           | `cmdInit`          | Shell integration, `.bashrc`/`.zshrc` detection                  |
| `create`         | `cmdCreate`        | Default isolation (21 items); `--no-isolate-defaults/--isolate-all` |
| `clone`          | `cmdClone`         | `cloneFromDefault` for `~/.claude/`, `cloneDir` for rig-to-rig   |
| `delete`         | `cmdDelete`        | Refuses to delete active rig                                     |
| `rename`         | `cmdRename`        | Renames rig dir, warns about .claude-rig files                   |
| `sync`           | `cmdSync`          | Symlinks + inherited + plugins + MCP + settings; `--no-plugins/--no-mcp/--no-settings/--from` |
| `update`         | `cmdUpdate`        | Forwards to `claude update`; warns about pinned rigs             |
| `versions`       | `cmdVersions`      | Lists version binaries on disk, marks current + pinned rigs      |
| `pin`            | `cmdPin`           | Pin rig to Claude version; disables auto-updater (`--rig`)       |
| `unpin`          | `cmdUnpin`         | Remove version pin; re-enables auto-updater (`--rig`)            |
| `list`           | `cmdList`          | `*` for running sessions, auth, skills, plugins, MCP, pinned ver |
| `launch`         | `cmdLaunch`        | Resolves `.claude-rig` file, sets env, `syscall.Exec`; respects pin |
| `rc`             | `cmdRC`            | Creates/reads `.claude-rig` project file, walks up dirs          |
| `link-auth`      | `cmdLinkAuth`      | Symlinks auth files, `--from` for cross-rig                      |
| `unlink-auth`    | `cmdUnlinkAuth`    | Copies auth files back to break sharing                          |
| `set-args`       | `cmdSetArgs`       | Global or per-rig default launch args                            |
| `show-args`      | `cmdShowArgs`      |                                                                  |
| `isolate`        | `cmdIsolate`       | Mark items as per-rig (remove symlink, create local)             |
| `share`          | `cmdShare`         | Reverse isolation (delete local, recreate symlink)               |
| `isolation`      | `cmdIsolation`     | Show isolation status for one or all rigs                        |
| `inherit`        | `cmdInherit`       | Enable global inheritance for skills/agents/hooks/commands       |
| `uninherit`      | `cmdUninherit`     | Disable inheritance, remove global symlinks                      |
| `diff`           | `cmdDiff`          | Compare two rigs: auth, settings, plugins, MCP, etc.             |
| `export`         | `cmdExport`        | tar.gz of rig-specific files; `--include-auth`, `--include-data` |
| `import`         | `cmdImport`        | Extract archive into new rig; `--link-auth` optional             |
| `status`         | `cmdStatus`        | Disk, sessions (/proc), last used; overview or single-rig detail |
| `plugin`         | `cmdPlugin`        | Forwards `claude plugin` to active rig; `--rig` to target other  |
| `update-plugins` | `cmdUpdatePlugins` | Parallel across rigs, calls `claude plugin` CLI                  |
| `settings`       | `cmdSettings`      | Manage default settings: `set/remove/list/override`; syncs to all rigs |
| `doctor`         | `cmdDoctor`        | Broken symlinks, inherited items, synced plugins/MCP health      |

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

```bash
go test ./...             # unit + integration tests
```

## Release

Tag push triggers GoReleaser via GitHub Actions (`.github/workflows/release.yml`).
Builds 5 binaries (linux/darwin × amd64/arm64, windows/amd64), publishes GitHub Release,
and pushes Homebrew formula to `edimuj/homebrew-tap`.

```bash
git tag v0.X.Y
git push origin v0.X.Y
```

Requires `HOMEBREW_TAP_TOKEN` secret on the repo (fine-grained PAT with Contents:write on homebrew-tap).

## Constraints

- stdlib only — zero external dependencies
- Single `package main` — no packages, no internal/
- Binary name = repo name = `claude-rig`
