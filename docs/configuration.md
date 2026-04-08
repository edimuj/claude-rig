# Configuration

## Rig-Specific Instructions

Each rig has its own `CLAUDE.md` for rig-specific instructions. Your global `~/.claude/CLAUDE.md` is always loaded alongside it — you don't lose your personal instructions when using a rig.

```
~/.claude/CLAUDE.md                  ← Global instructions (always loaded)
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

## Using Rigs as Templates

Clone an existing rig to use it as a starting point, then customize:

```bash
# Clone your go rig into a new one
claude-rig clone go rust --link-auth

# Create a clean slate (everything shared except always-isolated items)
claude-rig create cleanroom --link-auth --no-isolate-defaults
```
