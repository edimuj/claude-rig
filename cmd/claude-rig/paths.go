package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// rigSpecificItems are directories/files that are unique per rig.
// Everything else in ~/.claude/ is shared via symlinks.
var rigSpecificItems = []string{
	"settings.json",
	"skills",
	"plugins",
	"agents",
	"commands",
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

func activeRigFile() (string, error) {
	rig, err := rigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(rig, ".active"), nil
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
	"shell-snapshots",
	"cache",
	"stats-cache.json",
	"usage-data",
	"telemetry",
	"statusline",
	"chrome",
	"downloads",
	"paste-cache",
}

func isRigSpecific(name string) bool {
	for _, item := range rigSpecificItems {
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
