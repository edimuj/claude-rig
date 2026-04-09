package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// rigSpecificItems are directories/files that are unique per rig.
// Everything else in ~/.claude/ is shared via symlinks.
var rigSpecificItems = []string{
	"settings.json",
	"skills",
	"plugins",
	"agents",
	"hooks",
	"CLAUDE.md",
}

// authItems are files/directories needed to skip onboarding and reuse existing auth.
// .claude.json lives in ~ (not ~/.claude/) and holds onboarding/account state.
var authItems = []string{
	".credentials.json",
	"statsig",
}

// sharedItems are explicitly symlinked from the canonical ~/.claude/ into each rig.
// This list is discovered dynamically at rig creation time — anything in ~/.claude/
// that is NOT rig-specific gets symlinked.

func claudeHome() (string, error) {
	if env := os.Getenv("CLAUDE_CONFIG_DIR"); env != "" {
		// If user already overrides config dir, respect that as the canonical home
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// globalClaudeHome always returns ~/.claude/ regardless of CLAUDE_CONFIG_DIR.
// Used for inheriting global skills/agents/hooks/commands into rigs.
func globalClaudeHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

func rigHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude-rig"), nil
}

func rigsRoot() (string, error) {
	rig, err := rigHome()
	if err != nil {
		return "", err
	}
	rigsDir := filepath.Join(rig, "rigs")
	// Auto-migrate from old "profiles" directory
	oldDir := filepath.Join(rig, "profiles")
	if _, err := os.Stat(oldDir); err == nil {
		if _, err := os.Stat(rigsDir); os.IsNotExist(err) {
			os.Rename(oldDir, rigsDir)
		}
	}
	return rigsDir, nil
}

func rigDir(name string) (string, error) {
	root, err := rigsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}


// isolatableItems are shared items that can optionally be isolated per rig.
// Everything else in ~/.claude/ that isn't rig-specific or hidden gets symlinked.
var isolatableItems = []string{
	"history.jsonl",
	"conversations",
	"projects",
	"todos",
	"tasks",
	"file-history",
	"plans",
	"debug",
	"session-env",
	"sessions",
	"channels",
	"backups",
	"shell-snapshots",
	"cache",
	"stats-cache.json",
	"usage-data",
	"telemetry",
	"statusline",
	"chrome",
	"downloads",
	"paste-cache",
	"ide",
	"commands",
	"plugins",  // special: sync copies plugin cache + manifest, not file-level symlinks
	"mcp",      // special: sync merges mcpServers in .claude.json, not file-level
	"settings", // special: sync merges default-settings.json keys into settings.json
}

// defaultIsolatedItems are isolated automatically when creating a new rig.
// These are per-rig data/state items that shouldn't bleed across rigs.
// Only telemetry and usage-data remain shared (aggregate upstream tracking).
// Use --no-isolate-defaults to create a rig with everything shared instead.
var defaultIsolatedItems = []string{
	"conversations",
	"history.jsonl",
	"sessions",
	"channels",
	"tasks",
	"todos",
	"backups",
	"shell-snapshots",
	"projects",
	"plans",
	"paste-cache",
	"ide",
	"downloads",
	"debug",
	"commands",
	"file-history",
	"session-env",
	"cache",
	"stats-cache.json",
	"statusline",
	"chrome",
}

// inheritableItems are rig-specific directories whose contents can be inherited from ~/.claude/.
// Plugins are excluded — they're managed by `claude plugin install` and have internal state.
var inheritableItems = []string{
	"skills",
	"agents",
	"commands",
	"hooks",
}

func isRigSpecific(name string) bool {
	for _, item := range rigSpecificItems {
		if name == item {
			return true
		}
	}
	return false
}

func isInheritable(name string) bool {
	for _, item := range inheritableItems {
		if name == item {
			return true
		}
	}
	return false
}

func isIsolatable(name string) bool {
	for _, item := range isolatableItems {
		if name == item {
			return true
		}
	}
	return false
}

func defaultSettingsPath() string {
	home, err := rigHome()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "default-settings.json")
}

func blueprintsRoot() (string, error) {
	home, err := rigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "blueprints"), nil
}

func blueprintDir(name string) (string, error) {
	root, err := blueprintsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

func rigConfigPath(rigDir string) string {
	return filepath.Join(rigDir, "rig.json")
}

func claudeCodeBinary() string {
	if bin := os.Getenv("CLAUDE_BINARY"); bin != "" {
		return bin
	}
	if runtime.GOOS == "windows" {
		return "claude.exe"
	}
	return "claude"
}

// claudeVersionsDir discovers the directory containing Claude Code version binaries
// by following the symlink from the claude binary (e.g. ~/.local/bin/claude →
// ~/.local/share/claude/versions/2.1.92 → parent dir).
// Returns empty string if the binary is not a symlink or the path can't be resolved.
func claudeVersionsDir() string {
	binary := claudeCodeBinary()
	binPath, err := exec.LookPath(binary)
	if err != nil {
		return ""
	}
	target, err := os.Readlink(binPath)
	if err != nil {
		return "" // not a symlink
	}
	// Resolve relative symlink targets
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(binPath), target)
	}
	dir := filepath.Dir(target)
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	return dir
}

// claudeCurrentVersion returns the version the system claude symlink points to.
func claudeCurrentVersion() string {
	binary := claudeCodeBinary()
	binPath, err := exec.LookPath(binary)
	if err != nil {
		return ""
	}
	target, err := os.Readlink(binPath)
	if err != nil {
		return ""
	}
	return filepath.Base(target)
}

// compareVersions compares two version strings numerically (e.g. "2.1.9" < "2.1.85").
// Returns -1, 0, or 1.
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}
	for i := 0; i < maxLen; i++ {
		var na, nb int
		if i < len(partsA) {
			na, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			nb, _ = strconv.Atoi(partsB[i])
		}
		if na < nb {
			return -1
		}
		if na > nb {
			return 1
		}
	}
	return 0
}

// updateLatestLink ensures ~/.claude-rig/claude-latest points to the highest
// version binary on disk. Returns the resolved binary path, or "" if the
// versions directory can't be found.
func updateLatestLink() string {
	vDir := claudeVersionsDir()
	if vDir == "" {
		return ""
	}
	entries, err := os.ReadDir(vDir)
	if err != nil {
		return ""
	}
	var best string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) == 0 || name[0] < '0' || name[0] > '9' {
			continue
		}
		if best == "" || compareVersions(name, best) > 0 {
			best = name
		}
	}
	if best == "" {
		return ""
	}
	bestPath := filepath.Join(vDir, best)

	// Update the managed symlink
	rigBase, err := rigHome()
	if err != nil {
		return bestPath
	}
	link := filepath.Join(rigBase, "claude-latest")
	current, _ := os.Readlink(link)
	if current != bestPath {
		os.Remove(link)
		os.Symlink(bestPath, link)
	}
	return bestPath
}
