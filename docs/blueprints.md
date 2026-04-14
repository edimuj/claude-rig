# Blueprints

## The Problem

Right now, there's no good way to share a Claude Code setup. You've got your plugins, MCP servers, skills, custom agents, settings, CLAUDE.md instructions — all tuned over weeks of use. Then someone on X asks "what plugins do you use?" or a new teammate joins and needs the same tooling. Your options: write a long list of manual steps, or tell them to figure it out.

Blueprints change that. A blueprint is a portable, declarative spec of your rig — dotfiles for AI coding. It captures *what* your rig has (not the binary data), so anyone can recreate it from scratch in one command.

```bash
# Extract your setup into a blueprint
claude-rig blueprint create go-poweruser --from go

# Push to GitHub, share the link — anyone can recreate it:
claude-rig blueprint apply edimuj/go-poweruser --link-auth
```

### What you can do with blueprints

- **Share your setup publicly** — Push to GitHub, drop the link on X/Reddit/your blog. Followers run one command and get your exact config
- **Onboard teammates** — One blueprint per team role. New hire runs `blueprint apply` and they're set up in seconds, not hours
- **Reproduce across machines** — Moving to a new laptop? Your blueprint library comes with you
- **Pair with tutorials** — Writing a "how I use Claude Code for Go development" post? Include the blueprint so readers can follow along with your actual tools
- **Maintain setup libraries** — Keep blueprints for different types of work (web, mobile, data, DevOps) and spin up purpose-built rigs as needed

### Blueprint vs Export

Blueprints are not backups. `export`/`import` creates a full tar.gz archive of your rig files — good for backup and migration, but heavy and not meant for sharing. Blueprints are small (a JSON manifest + any custom skill/agent files you wrote), and plugins are listed as install references rather than copied. Think of it like the difference between sharing a `package.json` vs zipping your entire `node_modules/`.

---

## Blueprint Format

A blueprint is a directory containing a `blueprint.json` manifest and optional supporting files:

```
my-blueprint/
  blueprint.json          # manifest (required)
  CLAUDE.md               # rig instructions (optional)
  skills/                 # skill files (optional)
  agents/                 # agent files (optional)
  hooks/                  # hook files (optional)
  commands/               # command files (optional)
```

### blueprint.json

All fields except `name` are optional. Omitted fields use defaults or are skipped.

```json
{
  "name": "go-poweruser",
  "description": "Go development rig with gopls MCP and code review tools",
  "version": "1",
  "author": "edimuj",
  "marketplaces": {
    "claude-plugins-official": { "source": "github", "repo": "anthropics/claude-plugins-official" },
    "my-marketplace": { "source": "github", "repo": "edimuj/my-marketplace" }
  },
  "plugins": [
    "gopls-lsp@claude-plugins-official",
    "my-plugin@my-marketplace"
  ],
  "mcp_servers": {
    "gopls": { "command": "gopls-mcp", "args": ["--stdio"] }
  },
  "settings": {
    "env": { "GO111MODULE": "on" },
    "model": "opus"
  },
  "isolation": ["conversations", "sessions", "history.jsonl"],
  "inherit": ["skills"],
  "args": "--dangerously-skip-permissions"
}
```

| Field | Description |
|---|---|
| `name` | Blueprint name (required). Used as default rig name on apply |
| `description` | Human-readable description |
| `version` | Blueprint version (for your own tracking) |
| `author` | Author name |
| `marketplaces` | Marketplace name -> source info. Registered before plugins are installed |
| `plugins` | Plugin keys in `plugin@marketplace` format. Installed fresh via `claude plugin install` |
| `mcp_servers` | MCP server configs, written to `.claude.json` |
| `settings` | Settings merged into `settings.json` |
| `isolation` | Items to isolate per rig. Defaults to standard isolation if omitted |
| `inherit` | Items to inherit from global `~/.claude/` (skills, agents, hooks, commands) |
| `args` | Default launch arguments |

---

## Creating a Blueprint

Extract from an existing rig:

```bash
# From the active rig (inside a launched rig session)
claude-rig blueprint create my-blueprint

# From the rig bound to the current directory (.claude-rig RC file)
cd ~/projects/my-app
claude-rig blueprint create my-blueprint

# From a named rig
claude-rig blueprint create go-bp --from go
```

**What gets captured:**

- **Marketplaces** — source type and repo from `known_marketplaces.json` (portable, no local paths)
- **Plugins** — install keys from `installed_plugins.json` (not binary data — installed fresh on apply)
- **MCP servers** — from `.claude.json`, excluding plugin-provided servers (those come with the plugin)
- **Settings** — full `settings.json` contents
- **Isolation** — from `rig.json`
- **Inheritance** — from `rig.json`
- **Launch args** — from `default-args` file
- **CLAUDE.md** — copied if non-empty
- **Skills, agents, hooks, commands** — real files only (inherited symlinks are skipped)

Blueprints are stored in `~/.claude-rig/blueprints/<name>/`.

---

## Applying a Blueprint

Create a new rig from a blueprint:

```bash
# From the local blueprint library
claude-rig blueprint apply go-bp

# Custom rig name + auth linking
claude-rig blueprint apply go-bp --as my-go-rig --link-auth

# From a local directory
claude-rig blueprint apply ./path/to/blueprint/

# From a packed .tar.gz
claude-rig blueprint apply go-bp.blueprint.tar.gz

# From GitHub (clones the repo, looks for blueprint.json)
claude-rig blueprint apply edimuj/go-poweruser

# Skip plugin installation (useful when sharing across teams with different plugin access)
claude-rig blueprint apply go-bp --skip-plugins
```

### Source Resolution

Sources are resolved in this order:

1. **Local directory** — path to a directory containing `blueprint.json`
2. **Local archive** — `.tar.gz` or `.tgz` file (extracted to temp dir)
3. **Blueprint library** — `~/.claude-rig/blueprints/<name>/`
4. **GitHub** — `user/repo` pattern, cloned via `gh repo clone` (looks for `blueprint.json` in root, then `.claude-rig/` subdirectory)

### What Happens on Apply

1. Creates the rig directory with standard structure
2. Applies isolation (from blueprint, or defaults if not specified)
3. Sets up shared symlinks to `~/.claude/`
4. Links auth if `--link-auth` is passed
5. Copies CLAUDE.md, skills, agents, hooks, commands from blueprint
6. Writes settings to `settings.json`
7. Writes MCP servers to `.claude.json`
8. Configures inheritance and syncs global contents
9. Sets default launch args
10. Registers marketplaces via `claude plugin marketplace add`
11. Installs plugins via `claude plugin install`
12. Applies default settings on top (from `~/.claude-rig/default-settings.json`)

If the claude binary isn't available or a plugin/marketplace fails, it warns and continues. You can install them later with `claude-rig plugin`.

---

## Other Commands

### Inspect

Preview what a blueprint contains without applying it:

```bash
claude-rig blueprint inspect go-bp
claude-rig blueprint inspect ./path/to/blueprint/
claude-rig blueprint inspect edimuj/go-poweruser
```

Shows: name, description, author, version, marketplaces, plugins, MCP servers, settings (flattened with dot notation), isolation, inheritance, args, file counts, and a CLAUDE.md preview.

### List

List blueprints in the local library:

```bash
claude-rig blueprint list
```

### Pack

Pack a blueprint into a single `.tar.gz` file for sharing:

```bash
claude-rig blueprint pack go-bp
# produces go-bp.blueprint.tar.gz

claude-rig blueprint pack go-bp custom-name.tar.gz
```

---

## Sharing Blueprints

### As a GitHub repo

Commit the blueprint directory (with `blueprint.json` at the root) to a GitHub repo. Others apply it with:

```bash
claude-rig blueprint apply user/repo --link-auth
```

For repos where the blueprint lives in a subdirectory, place it in `.claude-rig/` — the resolver checks there as a fallback.

### As a file

Pack and share the `.tar.gz`:

```bash
claude-rig blueprint pack my-bp
# Send my-bp.blueprint.tar.gz to someone

# They run:
claude-rig blueprint apply ./my-bp.blueprint.tar.gz --as my-rig --link-auth
```

---

## Blueprint vs Export — Quick Reference

| | Blueprint | Export |
|---|---|---|
| Purpose | Share setup | Backup/migrate |
| Format | Declarative spec (JSON + files) | Full file archive |
| Plugins | Install references (installed fresh) | Binary data (copied) |
| Marketplaces | Source repos (registered fresh) | Not included |
| MCP servers | Config only | Config only |
| Size | Small (KB) | Large (MB) |
| Shareable | Yes (GitHub, file, directory) | Not really |
| Auth data | Never included | Optional (`--include-auth`) |
| Session data | Never included | Optional (`--include-data`) |
