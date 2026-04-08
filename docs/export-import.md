# Export, Import, Status & Diff

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

For version-controlled backups of `~/.claude-rig/`, the rig directories themselves are portable — just be careful to exclude auth files from version control.

---

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

---

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
