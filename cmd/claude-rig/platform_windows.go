//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// execLaunch spawns the binary as a child process and waits for it to exit.
// On Windows, syscall.Exec (execve) is not available, so we use exec.Command instead.
func execLaunch(binPath string, args []string, env []string) error {
	cmd := exec.Command(binPath, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}

// checkSymlinkSupport probes whether symlinks work on this Windows system.
// Symlinks require Developer Mode or elevated privileges on Windows.
func checkSymlinkSupport() error {
	tmp := filepath.Join(os.TempDir(), "claude-rig-symlink-test")
	target := filepath.Join(os.TempDir(), "claude-rig-symlink-target")

	// Clean up any leftovers
	os.Remove(tmp)
	os.Remove(target)

	if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
		return fmt.Errorf("cannot write temp file: %w", err)
	}
	defer os.Remove(target)

	err := os.Symlink(target, tmp)
	if err != nil {
		os.Remove(target)
		return fmt.Errorf("symlinks not available — enable Developer Mode in Windows Settings > Privacy & Security > For Developers")
	}
	os.Remove(tmp)
	return nil
}

// PowerShell wrapper template for --rig support.
const powershellWrapperTemplate = `
# claude-rig: --rig flag support
Remove-Alias claude -ErrorAction SilentlyContinue 2>$null
function claude {
  foreach ($arg in $args) {
    if ($arg -match '^--rig=(.+)$') {
      $rigName = $Matches[1]
      $rest = $args | Where-Object { $_ -ne $arg }
      claude-rig launch $rigName @rest
      return
    }
  }
  & claude.exe%s @args
}
`

// Bash wrapper template for Git Bash / MSYS2 users on Windows.
const bashWrapperTemplate = `
# claude-rig: --rig flag support
unalias claude 2>/dev/null
claude() {
  for arg in "$@"; do
    if [[ "$arg" == --rig=* ]]; then
      claude-rig launch "${arg#--rig=}" "${@//$arg/}"
      return
    fi
  done
  command claude%s "$@"
}
`

// shellWrapperTemplate is unused directly — installShellIntegration picks the right one.
// Kept to satisfy the cross-platform contract (cmdInit references it indirectly).
const shellWrapperTemplate = powershellWrapperTemplate

func detectShellRC() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Git Bash / MSYS2 — check SHELL env var
	shell := os.Getenv("SHELL")
	if strings.HasSuffix(shell, "/bash") || strings.HasSuffix(shell, "/zsh") {
		bashrc := filepath.Join(home, ".bashrc")
		if _, err := os.Stat(bashrc); err == nil {
			return bashrc
		}
		profile := filepath.Join(home, ".bash_profile")
		if _, err := os.Stat(profile); err == nil {
			return profile
		}
		// Neither exists — create .bashrc
		return bashrc
	}

	// PowerShell 7+ (pwsh)
	pwsh7 := filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	if _, err := os.Stat(pwsh7); err == nil {
		return pwsh7
	}

	// Windows PowerShell 5.x
	pwsh5 := filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
	if _, err := os.Stat(pwsh5); err == nil {
		return pwsh5
	}

	// If neither exists, prefer pwsh 7 path (create it)
	pwsh7Dir := filepath.Join(home, "Documents", "PowerShell")
	if _, err := os.Stat(pwsh7Dir); err == nil {
		return pwsh7
	}

	// Fall back to 5.x path
	return pwsh5
}

func hasShellIntegration(rcFile string) bool {
	data, err := os.ReadFile(rcFile)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "claude-rig")
}

// isBashRC returns true if the rc file is a bash/zsh-style file (not PowerShell).
func isBashRC(rcFile string) bool {
	base := filepath.Base(rcFile)
	return base == ".bashrc" || base == ".bash_profile" || base == ".zshrc"
}

func installShellIntegration(rcFile string) error {
	data, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(rcFile), 0755); err != nil {
		return err
	}

	// Pick the right template
	tmpl := powershellWrapperTemplate
	if isBashRC(rcFile) {
		tmpl = bashWrapperTemplate
	}

	// Detect existing claude alias and extract its flags
	extraFlags := ""
	lines := strings.Split(string(data), "\n")
	var newLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if aliasFlags := parseClaudeAlias(trimmed); aliasFlags != "" {
			extraFlags = " " + aliasFlags
			fmt.Printf("Found existing alias: %s\n", trimmed)
			fmt.Printf("Folding flags into wrapper: %s\n", aliasFlags)
			newLines = append(newLines, "# "+line+" # replaced by claude-rig wrapper")
		} else {
			newLines = append(newLines, line)
		}
	}

	wrapper := fmt.Sprintf(tmpl, extraFlags)
	output := strings.Join(newLines, "\n") + wrapper

	return os.WriteFile(rcFile, []byte(output), 0644)
}

// parseClaudeAlias extracts flags from a shell alias.
// Handles both bash (alias claude='claude ...') and PowerShell (Set-Alias) syntax.
func parseClaudeAlias(line string) string {
	// Bash-style: alias claude='claude --flag1 --flag2'
	for _, prefix := range []string{
		`alias claude='claude `,
		`alias claude="claude `,
	} {
		if strings.HasPrefix(line, prefix) {
			rest := line[len(prefix):]
			if len(rest) > 0 {
				rest = rest[:len(rest)-1] // remove closing ' or "
			}
			return strings.TrimSpace(rest)
		}
	}
	// PowerShell Set-Alias — too many variations to reliably extract flags
	return ""
}

// rigRunningSessions returns nil on Windows.
// Reading other processes' environment variables on Windows requires cgo or x/sys/windows,
// which violates the stdlib-only constraint.
func rigRunningSessions(dir string) []int {
	return nil
}
