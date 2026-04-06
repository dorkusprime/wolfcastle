package instance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setup(t *testing.T) string {
	t.Helper()
	raw := t.TempDir()
	// Resolve symlinks so paths match after EvalSymlinks in Register/Resolve.
	// macOS /tmp is a symlink to /private/var/folders/...
	dir, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatalf("resolving temp dir: %v", err)
	}
	RegistryDirOverride = filepath.Join(dir, "instances")
	t.Cleanup(func() { RegistryDirOverride = "" })
	return dir
}

func TestSlug(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"/users/wild/repo/wolfcastle/feat/auth", "users-wild-repo-wolfcastle-feat-auth"},
		{"/tmp/simple", "tmp-simple"},
		{"/", ""},
	}
	for _, tt := range tests {
		if got := Slug(tt.input); got != tt.want {
			t.Errorf("Slug(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRegister_CreatesFile(t *testing.T) {
	dir := setup(t)
	worktree := filepath.Join(dir, "repo")
	_ = os.MkdirAll(worktree, 0755)

	if err := Register(worktree, "feat/auth"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	slug := Slug(worktree)
	data, err := os.ReadFile(filepath.Join(dir, "instances", slug+".json"))
	if err != nil {
		t.Fatalf("reading instance file: %v", err)
	}
	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("unmarshaling: %v", err)
	}
	if entry.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", entry.PID, os.Getpid())
	}
	if entry.Branch != "feat/auth" {
		t.Errorf("Branch = %q, want %q", entry.Branch, "feat/auth")
	}
}

func TestDeregister_RemovesFile(t *testing.T) {
	dir := setup(t)
	worktree := filepath.Join(dir, "repo")
	_ = os.MkdirAll(worktree, 0755)

	if err := Register(worktree, "main"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := Deregister(worktree); err != nil {
		t.Fatalf("Deregister: %v", err)
	}

	slug := Slug(worktree)
	path := filepath.Join(dir, "instances", slug+".json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("instance file should be removed after Deregister")
	}
}

func TestDeregister_Idempotent(t *testing.T) {
	dir := setup(t)
	worktree := filepath.Join(dir, "repo")
	_ = os.MkdirAll(worktree, 0755)

	// Deregister without prior Register should not error.
	_ = os.MkdirAll(filepath.Join(dir, "instances"), 0755)
	if err := Deregister(worktree); err != nil {
		t.Fatalf("Deregister on missing file: %v", err)
	}
}

func TestList_FiltersDeadProcesses(t *testing.T) {
	dir := setup(t)
	regDir := filepath.Join(dir, "instances")
	_ = os.MkdirAll(regDir, 0755)

	// Write a stale entry with a PID that doesn't exist.
	stale := Entry{
		PID:       999999999,
		Worktree:  "/fake/stale",
		Branch:    "old",
		StartedAt: time.Now().UTC(),
	}
	data, _ := json.Marshal(stale)
	_ = os.WriteFile(filepath.Join(regDir, "fake-stale.json"), data, 0644)

	// Write a live entry with our own PID.
	live := Entry{
		PID:       os.Getpid(),
		Worktree:  "/fake/live",
		Branch:    "main",
		StartedAt: time.Now().UTC(),
	}
	data, _ = json.Marshal(live)
	_ = os.WriteFile(filepath.Join(regDir, "fake-live.json"), data, 0644)

	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 live entry, got %d", len(entries))
	}
	if entries[0].Worktree != "/fake/live" {
		t.Errorf("expected live entry, got %q", entries[0].Worktree)
	}

	// Stale file should have been cleaned.
	if _, err := os.Stat(filepath.Join(regDir, "fake-stale.json")); !os.IsNotExist(err) {
		t.Error("stale instance file should have been removed")
	}
}

func TestList_EmptyRegistry(t *testing.T) {
	setup(t)

	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestResolve_ExactMatch(t *testing.T) {
	dir := setup(t)
	regDir := filepath.Join(dir, "instances")
	_ = os.MkdirAll(regDir, 0755)

	worktree := filepath.Join(dir, "repo")
	_ = os.MkdirAll(worktree, 0755)

	entry := Entry{
		PID:       os.Getpid(),
		Worktree:  worktree,
		Branch:    "main",
		StartedAt: time.Now().UTC(),
	}
	data, _ := json.Marshal(entry)
	_ = os.WriteFile(filepath.Join(regDir, Slug(worktree)+".json"), data, 0644)

	got, err := Resolve(worktree)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Branch != "main" {
		t.Errorf("Branch = %q, want %q", got.Branch, "main")
	}
}

func TestResolve_Subdirectory(t *testing.T) {
	dir := setup(t)
	regDir := filepath.Join(dir, "instances")
	_ = os.MkdirAll(regDir, 0755)

	worktree := filepath.Join(dir, "repo")
	subdir := filepath.Join(worktree, "src", "pkg")
	_ = os.MkdirAll(subdir, 0755)

	entry := Entry{
		PID:       os.Getpid(),
		Worktree:  worktree,
		Branch:    "main",
		StartedAt: time.Now().UTC(),
	}
	data, _ := json.Marshal(entry)
	_ = os.WriteFile(filepath.Join(regDir, Slug(worktree)+".json"), data, 0644)

	got, err := Resolve(subdir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Worktree != worktree {
		t.Errorf("Worktree = %q, want %q", got.Worktree, worktree)
	}
}

func TestResolve_NoMatch(t *testing.T) {
	dir := setup(t)
	_ = os.MkdirAll(filepath.Join(dir, "instances"), 0755)
	unrelated := filepath.Join(dir, "elsewhere")
	_ = os.MkdirAll(unrelated, 0755)

	_, err := Resolve(unrelated)
	if err == nil {
		t.Error("expected error for unmatched directory")
	}
}

func TestResolve_LongestPrefixWins(t *testing.T) {
	dir := setup(t)
	regDir := filepath.Join(dir, "instances")
	_ = os.MkdirAll(regDir, 0755)

	parent := filepath.Join(dir, "repo")
	child := filepath.Join(dir, "repo", "nested")
	target := filepath.Join(dir, "repo", "nested", "deep")
	_ = os.MkdirAll(target, 0755)

	for _, wt := range []struct {
		path   string
		branch string
	}{
		{parent, "parent-branch"},
		{child, "child-branch"},
	} {
		entry := Entry{
			PID:       os.Getpid(),
			Worktree:  wt.path,
			Branch:    wt.branch,
			StartedAt: time.Now().UTC(),
		}
		data, _ := json.Marshal(entry)
		_ = os.WriteFile(filepath.Join(regDir, Slug(wt.path)+".json"), data, 0644)
	}

	got, err := Resolve(target)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Branch != "child-branch" {
		t.Errorf("expected child-branch (longest prefix), got %q", got.Branch)
	}
}

func TestResolve_BoundaryCheck(t *testing.T) {
	dir := setup(t)
	regDir := filepath.Join(dir, "instances")
	_ = os.MkdirAll(regDir, 0755)

	auth := filepath.Join(dir, "feat", "auth")
	authV2 := filepath.Join(dir, "feat", "auth-v2")
	_ = os.MkdirAll(auth, 0755)
	_ = os.MkdirAll(authV2, 0755)

	entry := Entry{
		PID:       os.Getpid(),
		Worktree:  auth,
		Branch:    "feat/auth",
		StartedAt: time.Now().UTC(),
	}
	data, _ := json.Marshal(entry)
	_ = os.WriteFile(filepath.Join(regDir, Slug(auth)+".json"), data, 0644)

	// auth-v2 should NOT match the auth instance.
	_, err := Resolve(authV2)
	if err == nil {
		t.Error("auth-v2 should not match auth instance (path boundary check)")
	}
}

func TestRegister_EvalSymlinksError(t *testing.T) {
	setup(t)
	// A path that doesn't exist causes EvalSymlinks to fail.
	err := Register("/nonexistent/path/that/will/fail", "feat/broken")
	if err == nil {
		t.Error("expected error for non-existent worktree path")
	}
}

func TestDeregister_EvalSymlinksFallback(t *testing.T) {
	dir := setup(t)
	regDir := filepath.Join(dir, "instances")
	_ = os.MkdirAll(regDir, 0755)

	// Write a file using the raw (non-resolved) path slug so Deregister
	// can find and remove it even when EvalSymlinks fails.
	rawPath := "/nonexistent/worktree"
	slug := Slug(rawPath)
	entry := Entry{PID: os.Getpid(), Worktree: rawPath, Branch: "test", StartedAt: time.Now().UTC()}
	data, _ := json.Marshal(entry)
	_ = os.WriteFile(filepath.Join(regDir, slug+".json"), data, 0644)

	// Deregister a path that doesn't exist on disk; EvalSymlinks fails
	// but the fallback to the raw path should still remove the file.
	if err := Deregister(rawPath); err != nil {
		t.Fatalf("Deregister with fallback: %v", err)
	}
	if _, err := os.Stat(filepath.Join(regDir, slug+".json")); !os.IsNotExist(err) {
		t.Error("expected instance file to be removed via fallback path")
	}
}

func TestList_SkipsNonJSON(t *testing.T) {
	dir := setup(t)
	regDir := filepath.Join(dir, "instances")
	_ = os.MkdirAll(regDir, 0755)

	// Create a non-JSON file and a subdirectory; both should be skipped.
	_ = os.WriteFile(filepath.Join(regDir, "readme.txt"), []byte("ignore"), 0644)
	_ = os.MkdirAll(filepath.Join(regDir, "subdir"), 0755)

	// Add one valid live entry.
	entry := Entry{PID: os.Getpid(), Worktree: "/fake/live", Branch: "main", StartedAt: time.Now().UTC()}
	data, _ := json.Marshal(entry)
	_ = os.WriteFile(filepath.Join(regDir, "live.json"), data, 0644)

	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestList_SkipsMalformedJSON(t *testing.T) {
	dir := setup(t)
	regDir := filepath.Join(dir, "instances")
	_ = os.MkdirAll(regDir, 0755)

	// Write a JSON file with invalid content.
	_ = os.WriteFile(filepath.Join(regDir, "bad.json"), []byte("{{{not json"), 0644)

	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for malformed JSON, got %d", len(entries))
	}
}

func TestResolve_EvalSymlinksError(t *testing.T) {
	setup(t)
	_, err := Resolve("/nonexistent/path/that/will/fail")
	if err == nil {
		t.Error("expected error for non-existent resolve path")
	}
}

func TestList_UnreadableFile(t *testing.T) {
	dir := setup(t)
	regDir := filepath.Join(dir, "instances")
	_ = os.MkdirAll(regDir, 0755)

	// Create a JSON file that can't be read.
	path := filepath.Join(regDir, "unreadable.json")
	_ = os.WriteFile(path, []byte(`{"pid":1}`), 0644)
	_ = os.Chmod(path, 0o000)
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	// Should skip the unreadable file without error.
	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestIsProcessRunning_DeadPID(t *testing.T) {
	t.Parallel()
	// PID 999999999 should not be running on any real system.
	if isProcessRunning(999999999) {
		t.Error("expected PID 999999999 to not be running")
	}
}

func TestIsProcessRunning_SelfPID(t *testing.T) {
	t.Parallel()
	if !isProcessRunning(os.Getpid()) {
		t.Error("expected own PID to be running")
	}
}

func TestIsSubpath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		child  string
		parent string
		want   bool
	}{
		{"/repo/feat/auth", "/repo/feat/auth", true},
		{"/repo/feat/auth/src", "/repo/feat/auth", true},
		{"/repo/feat/auth-v2", "/repo/feat/auth", false},
		{"/repo/feat/au", "/repo/feat/auth", false},
		{"/other/path", "/repo/feat/auth", false},
	}
	for _, tt := range tests {
		if got := isSubpath(tt.child, tt.parent); got != tt.want {
			t.Errorf("isSubpath(%q, %q) = %v, want %v", tt.child, tt.parent, got, tt.want)
		}
	}
}
