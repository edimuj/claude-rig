//go:build !windows

package main

import "testing"

func TestEnvironHasVar(t *testing.T) {
	// NUL-separated /proc environ buffer with the "go" rig active.
	environ := []byte("PATH=/usr/bin\x00CLAUDE_CONFIG_DIR=/home/u/.claude-rig/rigs/go\x00TERM=xterm\x00")

	tests := []struct {
		name   string
		needle string
		want   bool
	}{
		{"exact match", "CLAUDE_CONFIG_DIR=/home/u/.claude-rig/rigs/go", true},
		{"prefix must not match longer dir", "CLAUDE_CONFIG_DIR=/home/u/.claude-rig/rigs/g", false},
		{"unrelated rig", "CLAUDE_CONFIG_DIR=/home/u/.claude-rig/rigs/gold", false},
		{"first entry", "PATH=/usr/bin", true},
		{"last entry (trailing NUL)", "TERM=xterm", true},
		{"absent", "CLAUDE_CONFIG_DIR=/nope", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := environHasVar(environ, tt.needle); got != tt.want {
				t.Errorf("environHasVar(%q) = %v, want %v", tt.needle, got, tt.want)
			}
		})
	}

	// Reverse of the prefix bug: a "gold" rig's environ must not be matched by the "go" needle.
	goldEnviron := []byte("CLAUDE_CONFIG_DIR=/home/u/.claude-rig/rigs/gold\x00")
	if environHasVar(goldEnviron, "CLAUDE_CONFIG_DIR=/home/u/.claude-rig/rigs/go") {
		t.Error(`"go" needle wrongly matched a "gold" rig environ (prefix bug regression)`)
	}
}
