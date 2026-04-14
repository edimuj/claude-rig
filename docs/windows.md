# Windows Support

## Requirements

Windows requires **Developer Mode** enabled for symlink support. `claude-rig init` checks this automatically and fails fast with a clear message if symlinks aren't available.

Enable Developer Mode: **Settings > System > For developers > Developer Mode > On**

## Shell Integration

Works with both PowerShell and Git Bash:

- **PowerShell** (5.x and 7+) — Installs a `claude` function wrapper in your PowerShell profile
- **Git Bash / MSYS2** — Detects the `SHELL` env var and installs the same bash wrapper as Linux/macOS

## Launch Behavior

On Unix, `claude-rig launch` replaces itself with Claude Code via `exec`. On Windows, it spawns Claude Code as a child process and forwards the exit code.

Session detection (`*` markers in `list` and `status`) is not available on Windows.

## Cross-Compiling

Build a Windows binary from Linux/macOS:

```bash
make build-windows    # produces claude-rig.exe (amd64)
```

---

## Known Limitations

### Remote Control

Claude Code's [Remote Control](https://code.claude.com/docs/en/remote-control) feature works with claude-rig but is fragile when multiple rigs have it enabled simultaneously. All rigs sharing auth (via `--link-auth`) use the same OAuth tokens, causing rate limiting and connection instability when multiple bridge sessions compete.

**Recommendation:** Enable `remoteControlAtStartup` on **one rig only**. The `doctor` command warns when multiple rigs have it enabled.

The underlying issue is a Claude Code bug — when the bridge WebSocket dies, it doesn't recover and the session becomes a zombie (visible in the app but unable to sync). This happens regardless of claude-rig but is more likely with shared auth.
