//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// execLaunch replaces the current process with the given binary (Unix execve).
func execLaunch(binPath string, args []string, env []string) error {
	return syscall.Exec(binPath, args, env)
}

// checkSymlinkSupport is a no-op on Unix — symlinks always work.
func checkSymlinkSupport() error {
	return nil
}

// shellWrapperTemplate is the function added to shell rc files for --rig support.
// %s is replaced with any extra default flags from an existing alias.
const shellWrapperTemplate = `
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

func detectShellRC() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	shell := os.Getenv("SHELL")
	switch {
	case strings.HasSuffix(shell, "/zsh"):
		return filepath.Join(home, ".zshrc")
	case strings.HasSuffix(shell, "/bash"):
		// Prefer .bashrc, fall back to .bash_profile on macOS
		bashrc := filepath.Join(home, ".bashrc")
		if _, err := os.Stat(bashrc); err == nil {
			return bashrc
		}
		profile := filepath.Join(home, ".bash_profile")
		if _, err := os.Stat(profile); err == nil {
			return profile
		}
		return bashrc
	default:
		return ""
	}
}

func hasShellIntegration(rcFile string) bool {
	data, err := os.ReadFile(rcFile)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "claude-rig")
}

func installShellIntegration(rcFile string) error {
	data, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return err
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
			// Comment out the old alias
			newLines = append(newLines, "# "+line+" # replaced by claude-rig wrapper")
		} else {
			newLines = append(newLines, line)
		}
	}

	wrapper := fmt.Sprintf(shellWrapperTemplate, extraFlags)
	output := strings.Join(newLines, "\n") + wrapper

	return os.WriteFile(rcFile, []byte(output), 0644)
}

// parseClaudeAlias extracts flags from an alias like: alias claude='claude --flag1 --flag2'
func parseClaudeAlias(line string) string {
	// Match: alias claude='claude ...' or alias claude="claude ..."
	for _, prefix := range []string{
		`alias claude='claude `,
		`alias claude="claude `,
	} {
		if strings.HasPrefix(line, prefix) {
			// Strip prefix and trailing quote
			rest := line[len(prefix):]
			if len(rest) > 0 {
				rest = rest[:len(rest)-1] // remove closing ' or "
			}
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// rigRunningSessions scans /proc for processes with CLAUDE_CONFIG_DIR matching the rig directory.
// Returns a slice of matching PIDs. Graceful no-op on non-Linux.
func rigRunningSessions(dir string) []int {
	procs, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	needle := []byte("CLAUDE_CONFIG_DIR=" + dir)
	var pids []int
	for _, p := range procs {
		if !p.IsDir() {
			continue
		}
		// Only look at numeric dirs (PIDs)
		pid := 0
		for _, c := range p.Name() {
			if c < '0' || c > '9' {
				pid = -1
				break
			}
			pid = pid*10 + int(c-'0')
		}
		if pid <= 0 {
			continue
		}
		env, err := os.ReadFile(filepath.Join("/proc", p.Name(), "environ"))
		if err != nil {
			continue
		}
		if bytesContains(env, needle) {
			pids = append(pids, pid)
		}
	}
	return pids
}

// bytesContains checks if haystack contains needle (for null-separated /proc environ).
func bytesContains(haystack, needle []byte) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		strings.Contains(string(haystack), string(needle))
}
