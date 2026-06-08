# Sync, Settings & Versions

The `sync` command keeps rigs aligned with global config: symlinks, inherited items, plugins, MCP servers, and default settings.

```bash
# Sync everything for all rigs
claude-rig sync

# Sync a single rig
claude-rig sync myrig

# Skip specific sync steps
claude-rig sync myrig --no-plugins
claude-rig sync myrig --no-mcp
claude-rig sync myrig --no-inherit
claude-rig sync myrig --no-settings

# Sync from another rig instead of global
claude-rig sync myrig --from webdev
```

---

## Plugin Sync

Plugins are synced by symlinking cache directories from global `~/.claude/` (or another rig via `--from`) into the rig, then rewriting manifest paths to match. This avoids duplicating plugin data while keeping each rig's plugin state consistent.

Synced plugins are recorded in `rig.json` (`synced_plugins`) so that `isolate` and `share` can cleanly add or remove them.

```bash
# Isolate plugins — removes synced plugins from the rig
claude-rig isolate myrig plugins

# Share them back — re-syncs plugins from global
claude-rig share myrig plugins
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

---

## MCP Server Sync

MCP servers are reconciled from the global `~/.claude.json` into the rig's `.claude.json`. Servers claude-rig owns — those it synced earlier, tracked in `rig.json` (`synced_mcp`) — are kept in lockstep with the source:

- **Added** — a source server missing from the rig is copied in and marked owned.
- **Updated** — an owned server whose source definition changed is overwritten (so edits to a global server propagate to every rig).
- **Removed** — an owned server the source no longer defines is deleted from the rig.

Entries a rig defines itself (not in `synced_mcp`) always take precedence and are never touched, even if the source later defines a server of the same name.

Preview the reconcile without writing:

```bash
# Show which MCP servers sync would add/update/remove
claude-rig sync --dry-run
claude-rig sync myrig --dry-run
```

```bash
# Isolate MCP — removes synced servers from the rig
claude-rig isolate myrig mcp

# Share them back — re-syncs from global
claude-rig share myrig mcp
```

---

## Default Settings

Store base settings that apply to all rigs. Each rig keeps its own `settings.json` — defaults are merged in at sync time, not symlinked. This means you can define once and propagate everywhere, while still allowing per-rig overrides.

Defaults are stored in `~/.claude-rig/default-settings.json`.

```bash
# Set a default — applies immediately to all rigs
claude-rig settings set theme dark
claude-rig settings set env.SOME_VAR value

# Show current defaults
claude-rig settings list

# Remove a default — strips it from all rigs (respects overrides)
claude-rig settings remove theme

# Pin a per-rig value that sync won't clobber
claude-rig settings override theme light --rig minimal
```

Keys use **dot-notation** for nested values: `env.FOO`, `permissions.allow.Bash`. Values are parsed as JSON first, then fall back to a plain string — so `true`, `42`, and `"text"` all work naturally.

**Merge semantics:** Each leaf key is applied individually. If a default sets `env.FOO`, the rig's existing `env.BAR` is untouched. Arrays are atomic — a default `allowedTools` replaces the rig's array unless overridden.

**Overrides** are tracked in `rig.json` (`settings_overrides`). A key in that list is never touched by sync — not on `claude-rig sync`, not on `settings set`. Remove an override by editing `rig.json` directly.

**Isolation:** `claude-rig isolate myrig settings` blocks all settings sync for that rig.

---

## Version Management

claude-rig manages its own version resolution independently from the system `claude` symlink. This prevents a common problem: Claude Code's auto-updater in one rig can re-point the system symlink to a different version, silently affecting every other rig.

**How it works:** On launch, claude-rig scans `~/.local/share/claude/versions/` (or equivalent), picks the highest version, and maintains its own symlink at `~/.claude-rig/claude-latest`. All unpinned rigs use this — never the system symlink. The managed symlink only moves forward; it never downgrades.

```bash
# See installed versions — * marks what unpinned rigs use
claude-rig versions
#   2.1.85  (symlink)     # system symlink points here
#   2.1.91
# * 2.1.92  (latest)      # what unpinned rigs actually use

# Update Claude Code (downloads new version, refreshes latest link)
claude-rig update
```

### Version Pinning

Pin individual rigs to specific Claude Code versions — useful for testing new releases safely or keeping a stable setup while experimenting elsewhere.

```bash
# Pin a rig to a specific version
claude-rig pin 2.1.85
claude-rig pin 2.1.85 --rig staging

# Remove the pin (rig uses latest on disk again)
claude-rig unpin
claude-rig unpin --rig staging
```

When a rig is pinned, `launch` uses that exact binary instead of the latest. The auto-updater is also disabled for pinned rigs so a running session won't silently upgrade itself. Pinned rigs are not affected by `claude-rig update`, and you'll see a reminder about them after each update.

Pinned binaries are copied to `~/.claude-rig/versions/` so they survive Claude Code's own cleanup of old versions. When you unpin, the preserved copy is automatically removed (unless another rig still pins the same version). `claude-rig versions` shows preserved copies and `doctor` warns if a pinned binary goes missing.

