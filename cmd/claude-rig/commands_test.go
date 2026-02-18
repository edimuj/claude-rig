package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// setupTestHome creates a temp HOME with minimal ~/.claude/ and ~/.claude-rig/rigs/ structure.
// Returns the home dir. CLAUDE_CONFIG_DIR is cleared so path helpers use HOME.
func setupTestHome(t *testing.T) string {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	os.MkdirAll(filepath.Join(home, ".claude"), 0755)
	os.MkdirAll(filepath.Join(home, ".claude-rig", "rigs"), 0755)
	return home
}

// createRigDir creates a rig directory with the standard rig-specific subdirs.
func createRigDir(t *testing.T, home, name string) string {
	t.Helper()
	dir := filepath.Join(home, ".claude-rig", "rigs", name)
	os.MkdirAll(dir, 0755)
	for _, item := range rigSpecificItems {
		if item == "settings.json" || item == "CLAUDE.md" {
			continue // files, not dirs
		}
		os.MkdirAll(filepath.Join(dir, item), 0755)
	}
	return dir
}

// --- Tier 1: Pure functions ---

func TestValidateRigName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"myrig", false},
		{"test-rig", false},
		{"rig123", false},
		{"", true},
		{"has/slash", true},
		{"has\\back", true},
		{"has.dot", true},
		{"has space", true},
		{"-leading-dash", true},
	}
	for _, tt := range tests {
		err := validateRigName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateRigName(%q) err=%v, wantErr=%v", tt.name, err, tt.wantErr)
		}
	}
}

func TestSetEnv(t *testing.T) {
	env := []string{"FOO=bar", "PATH=/usr/bin"}

	// Add new
	env = setEnv(env, "NEW", "val")
	if env[len(env)-1] != "NEW=val" {
		t.Errorf("setEnv add: got %v", env)
	}

	// Overwrite existing
	env = setEnv(env, "FOO", "baz")
	found := false
	for _, e := range env {
		if e == "FOO=baz" {
			found = true
		}
		if e == "FOO=bar" {
			t.Error("setEnv overwrite: old value still present")
		}
	}
	if !found {
		t.Error("setEnv overwrite: new value not found")
	}
}

func TestRemoveEnv(t *testing.T) {
	env := []string{"FOO=bar", "PATH=/usr/bin", "HOME=/home/x"}

	env = removeEnv(env, "PATH")
	for _, e := range env {
		if e == "PATH=/usr/bin" {
			t.Error("removeEnv: PATH still present")
		}
	}
	if len(env) != 2 {
		t.Errorf("removeEnv: expected len 2, got %d", len(env))
	}

	// Remove missing key — no panic, same length
	env = removeEnv(env, "MISSING")
	if len(env) != 2 {
		t.Errorf("removeEnv missing: expected len 2, got %d", len(env))
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0B"},
		{500, "500B"},
		{1024, "1K"},
		{1536, "1K"}, // 1.5K truncated to int
		{1048576, "1M"},
		{1572864, "1M"}, // 1.5M truncated
		{1073741824, "1.0G"},
		{1610612736, "1.5G"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatTimeAgo(t *testing.T) {
	now := time.Now()
	tests := []struct {
		offset time.Duration
		want   string
	}{
		{10 * time.Second, "just now"},
		{1 * time.Minute, "1m ago"},
		{5 * time.Minute, "5m ago"},
		{1 * time.Hour, "1h ago"},
		{3 * time.Hour, "3h ago"},
		{24 * time.Hour, "1d ago"},
		{72 * time.Hour, "3d ago"},
	}
	for _, tt := range tests {
		got := formatTimeAgo(now.Add(-tt.offset))
		if got != tt.want {
			t.Errorf("formatTimeAgo(-%v) = %q, want %q", tt.offset, got, tt.want)
		}
	}
}

func TestSetDiff(t *testing.T) {
	tests := []struct {
		name           string
		a, b           []string
		wantA, wantB   []string
	}{
		{"identical", []string{"a", "b"}, []string{"a", "b"}, nil, nil},
		{"disjoint", []string{"a"}, []string{"b"}, []string{"a"}, []string{"b"}},
		{"overlap", []string{"a", "b", "c"}, []string{"b", "c", "d"}, []string{"a"}, []string{"d"}},
		{"empty_both", nil, nil, nil, nil},
		{"empty_a", nil, []string{"x"}, nil, []string{"x"}},
		{"empty_b", []string{"x"}, nil, []string{"x"}, nil},
	}
	for _, tt := range tests {
		onlyA, onlyB := setDiff(tt.a, tt.b)
		if !sliceEqual(onlyA, tt.wantA) || !sliceEqual(onlyB, tt.wantB) {
			t.Errorf("setDiff[%s]: got (%v, %v), want (%v, %v)", tt.name, onlyA, onlyB, tt.wantA, tt.wantB)
		}
	}
}

func TestLastLine(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"single", "single"},
		{"first\nsecond\nthird", "third"},
		{"trailing\n\n", "trailing"},
		{"", ""},
		{"  spaced  ", "spaced"},
	}
	for _, tt := range tests {
		got := lastLine(tt.input)
		if got != tt.want {
			t.Errorf("lastLine(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRigConfigMethods(t *testing.T) {
	cfg := rigConfig{
		Isolate: []string{"conversations", "history.jsonl"},
		Inherit: []string{"skills", "hooks"},
	}

	if !cfg.isIsolated("conversations") {
		t.Error("isIsolated should find conversations")
	}
	if cfg.isIsolated("projects") {
		t.Error("isIsolated should not find projects")
	}
	if !cfg.isInherited("skills") {
		t.Error("isInherited should find skills")
	}
	if cfg.isInherited("agents") {
		t.Error("isInherited should not find agents")
	}

	// Empty config
	empty := rigConfig{}
	if empty.isIsolated("anything") {
		t.Error("empty config should not isolate anything")
	}
	if empty.isInherited("anything") {
		t.Error("empty config should not inherit anything")
	}
}

// --- Tier 2: Filesystem with temp dirs ---

func TestLoadSaveRigConfig(t *testing.T) {
	home := setupTestHome(t)
	dir := createRigDir(t, home, "testrig")

	// Missing file returns empty config
	cfg := loadRigConfig(dir)
	if len(cfg.Isolate) != 0 || len(cfg.Inherit) != 0 {
		t.Error("loadRigConfig missing file should return empty config")
	}

	// Round-trip
	cfg = rigConfig{
		Isolate: []string{"conversations"},
		Inherit: []string{"skills", "hooks"},
	}
	if err := saveRigConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	loaded := loadRigConfig(dir)
	if !sliceEqual(loaded.Isolate, cfg.Isolate) || !sliceEqual(loaded.Inherit, cfg.Inherit) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", loaded, cfg)
	}
}

func TestSyncSharedSymlinks(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	// Put some shared items in ~/.claude/
	os.MkdirAll(filepath.Join(claudeDir, "conversations"), 0755)
	os.WriteFile(filepath.Join(claudeDir, "history.jsonl"), []byte("{}"), 0644)

	// Also put a rig-specific item that should NOT be symlinked
	os.MkdirAll(filepath.Join(claudeDir, "skills"), 0755)

	// And a hidden file that should be skipped
	os.WriteFile(filepath.Join(claudeDir, ".hidden"), []byte("x"), 0644)

	if err := syncSharedSymlinks(rigDir); err != nil {
		t.Fatal(err)
	}

	// conversations and history.jsonl should be symlinked
	for _, name := range []string{"conversations", "history.jsonl"} {
		link := filepath.Join(rigDir, name)
		info, err := os.Lstat(link)
		if err != nil {
			t.Errorf("expected symlink for %s, got error: %v", name, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s should be a symlink", name)
		}
	}

	// skills (rig-specific) should NOT have a new symlink
	info, err := os.Lstat(filepath.Join(rigDir, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("skills should not be symlinked (rig-specific, already exists)")
	}

	// .hidden should not be symlinked
	if _, err := os.Lstat(filepath.Join(rigDir, ".hidden")); err == nil {
		t.Error(".hidden should not be symlinked")
	}
}

func TestSyncSharedSymlinksSkipsIsolated(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	os.MkdirAll(filepath.Join(claudeDir, "conversations"), 0755)
	os.MkdirAll(filepath.Join(claudeDir, "cache"), 0755)

	// Isolate conversations
	saveRigConfig(rigDir, rigConfig{Isolate: []string{"conversations"}})
	os.MkdirAll(filepath.Join(rigDir, "conversations"), 0755)

	if err := syncSharedSymlinks(rigDir); err != nil {
		t.Fatal(err)
	}

	// conversations should remain a real dir (not symlinked)
	info, _ := os.Lstat(filepath.Join(rigDir, "conversations"))
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("isolated conversations should not be symlinked")
	}

	// cache should be symlinked
	info, err := os.Lstat(filepath.Join(rigDir, "cache"))
	if err != nil {
		t.Fatal("cache should exist")
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("cache should be symlinked")
	}
}

func TestSyncGlobalContents(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	// Set up global skills
	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "global-skill.md"), []byte("# skill"), 0644)

	// Enable inheritance for skills
	saveRigConfig(rigDir, rigConfig{Inherit: []string{"skills"}})

	if err := syncGlobalContents(rigDir); err != nil {
		t.Fatal(err)
	}

	// Check global-skill.md is symlinked into rig's skills/
	link := filepath.Join(rigDir, "skills", "global-skill.md")
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal("global-skill.md should be symlinked into rig")
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("global-skill.md should be a symlink")
	}
	target, _ := os.Readlink(link)
	if target != filepath.Join(globalSkills, "global-skill.md") {
		t.Errorf("symlink target = %q, want %q", target, filepath.Join(globalSkills, "global-skill.md"))
	}
}

func TestSyncGlobalContentsSkipsLocalOverrides(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "override.md"), []byte("global"), 0644)

	// Create a local override in the rig
	os.WriteFile(filepath.Join(rigDir, "skills", "override.md"), []byte("local"), 0644)

	saveRigConfig(rigDir, rigConfig{Inherit: []string{"skills"}})
	syncGlobalContents(rigDir)

	// Local file should still be a real file, not overwritten
	info, _ := os.Lstat(filepath.Join(rigDir, "skills", "override.md"))
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("local override should not be replaced with symlink")
	}
	data, _ := os.ReadFile(filepath.Join(rigDir, "skills", "override.md"))
	if string(data) != "local" {
		t.Error("local file content should be preserved")
	}
}

func TestRemoveInheritedSymlinks(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "global.md"), []byte("g"), 0644)

	localSkills := filepath.Join(rigDir, "skills")

	// Create a global symlink and a local file
	os.Symlink(filepath.Join(globalSkills, "global.md"), filepath.Join(localSkills, "global.md"))
	os.WriteFile(filepath.Join(localSkills, "local.md"), []byte("l"), 0644)

	removeInheritedSymlinks(localSkills, globalSkills)

	// Global symlink should be removed
	if _, err := os.Lstat(filepath.Join(localSkills, "global.md")); err == nil {
		t.Error("global symlink should be removed")
	}
	// Local file should be preserved
	if _, err := os.Lstat(filepath.Join(localSkills, "local.md")); err != nil {
		t.Error("local file should be preserved")
	}
}

func TestRemoveStaleInheritedSymlinks(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "testrig")

	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)

	localSkills := filepath.Join(rigDir, "skills")

	// Create a symlink to a file that exists
	os.WriteFile(filepath.Join(globalSkills, "valid.md"), []byte("ok"), 0644)
	os.Symlink(filepath.Join(globalSkills, "valid.md"), filepath.Join(localSkills, "valid.md"))

	// Create a symlink to a file that does NOT exist (stale)
	os.Symlink(filepath.Join(globalSkills, "gone.md"), filepath.Join(localSkills, "gone.md"))

	removeStaleInheritedSymlinks(localSkills, globalSkills)

	// Valid symlink preserved
	if _, err := os.Lstat(filepath.Join(localSkills, "valid.md")); err != nil {
		t.Error("valid symlink should be preserved")
	}
	// Stale symlink removed
	if _, err := os.Lstat(filepath.Join(localSkills, "gone.md")); err == nil {
		t.Error("stale symlink should be removed")
	}
}

func TestDirEntryNames(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte(""), 0644)
	os.MkdirAll(filepath.Join(dir, "c"), 0755)

	names := dirEntryNames(dir)
	sort.Strings(names)
	want := []string{"a.txt", "b.txt", "c"}
	if !sliceEqual(names, want) {
		t.Errorf("dirEntryNames = %v, want %v", names, want)
	}

	// Missing dir returns nil
	if got := dirEntryNames("/nonexistent/path"); got != nil {
		t.Errorf("dirEntryNames missing dir = %v, want nil", got)
	}
}

// --- Tier 3: Integration workflows ---

func TestCreateInheritSyncUninherit(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "workflow")

	// Set up global skills and agents
	for _, item := range []string{"skills", "agents"} {
		d := filepath.Join(claudeDir, item)
		os.MkdirAll(d, 0755)
		os.WriteFile(filepath.Join(d, "global-"+item+".md"), []byte("content"), 0644)
	}

	// Inherit all
	cfg := rigConfig{Inherit: []string{"skills", "agents", "commands", "hooks"}}
	saveRigConfig(rigDir, cfg)
	syncGlobalContents(rigDir)

	// Verify global skills/agents symlinked
	for _, item := range []string{"skills", "agents"} {
		link := filepath.Join(rigDir, item, "global-"+item+".md")
		info, err := os.Lstat(link)
		if err != nil {
			t.Fatalf("expected symlink for global-%s.md: %v", item, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("global-%s.md should be a symlink", item)
		}
	}

	// Uninherit skills
	cfg.Inherit = []string{"agents", "commands", "hooks"}
	saveRigConfig(rigDir, cfg)
	removeInheritedSymlinks(
		filepath.Join(rigDir, "skills"),
		filepath.Join(claudeDir, "skills"),
	)

	// Skills global symlink removed
	if _, err := os.Lstat(filepath.Join(rigDir, "skills", "global-skills.md")); err == nil {
		t.Error("global-skills.md should be removed after uninherit")
	}
	// Agents still inherited
	if _, err := os.Lstat(filepath.Join(rigDir, "agents", "global-agents.md")); err != nil {
		t.Error("global-agents.md should still exist")
	}
}

func TestLaunchSyncPicksUpNewGlobalSkill(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "synctest")

	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "existing.md"), []byte("e"), 0644)

	cfg := rigConfig{Inherit: []string{"skills"}}
	saveRigConfig(rigDir, cfg)
	syncGlobalContents(rigDir)

	// Verify existing skill is synced
	if _, err := os.Lstat(filepath.Join(rigDir, "skills", "existing.md")); err != nil {
		t.Fatal("existing.md should be synced")
	}

	// Add a new global skill
	os.WriteFile(filepath.Join(globalSkills, "new-skill.md"), []byte("n"), 0644)

	// Re-sync (simulates launch sync)
	syncGlobalContents(rigDir)

	// New skill should now appear
	info, err := os.Lstat(filepath.Join(rigDir, "skills", "new-skill.md"))
	if err != nil {
		t.Fatal("new-skill.md should be synced after re-sync")
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("new-skill.md should be a symlink")
	}
}

func TestStaleSymlinkCleanup(t *testing.T) {
	home := setupTestHome(t)
	claudeDir := filepath.Join(home, ".claude")
	rigDir := createRigDir(t, home, "staletest")

	globalSkills := filepath.Join(claudeDir, "skills")
	os.MkdirAll(globalSkills, 0755)
	os.WriteFile(filepath.Join(globalSkills, "temp.md"), []byte("t"), 0644)

	cfg := rigConfig{Inherit: []string{"skills"}}
	saveRigConfig(rigDir, cfg)
	syncGlobalContents(rigDir)

	// Verify synced
	if _, err := os.Lstat(filepath.Join(rigDir, "skills", "temp.md")); err != nil {
		t.Fatal("temp.md should be synced")
	}

	// Remove the global source
	os.Remove(filepath.Join(globalSkills, "temp.md"))

	// Re-sync — stale cleanup happens inside syncGlobalContents
	syncGlobalContents(rigDir)

	// Stale symlink should be cleaned up
	if _, err := os.Lstat(filepath.Join(rigDir, "skills", "temp.md")); err == nil {
		t.Error("stale temp.md symlink should be removed after source deleted")
	}
}

// --- helpers ---

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
