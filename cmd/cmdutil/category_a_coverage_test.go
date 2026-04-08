package cmdutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ---------------------------------------------------------------------------
// completions.go: CompleteTaskAddresses with broken resolver
// ---------------------------------------------------------------------------

func TestCompleteTaskAddresses_BrokenResolver(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	// Valid root index
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(`{"nodes":{"my-node":{"name":"My Node","type":"leaf","state":"not_started","address":"my-node","children":[]}}}`), 0644)

	// App has no resolver to simulate broken state
	a := &App{}

	t.Chdir(tmp)

	fn := CompleteTaskAddresses(a)
	addrs, _ := fn(nil, nil, "")
	// With nil resolver, should fallback to LoadConfig which will fail
	// (no config files) and return nil
	if addrs != nil {
		t.Errorf("expected nil addrs with broken resolver, got %v", addrs)
	}
}

// ---------------------------------------------------------------------------
// app.go: compareNamespace with all-stop-words .md file (empty bigrams)
// ---------------------------------------------------------------------------

func TestCompareNamespace_AllStopWordsMdFile(t *testing.T) {
	tmp := t.TempDir()

	// Create a .md file containing only stop words
	_ = os.WriteFile(filepath.Join(tmp, "stopwords.md"), []byte("the and for with this that from"), 0644)

	newBigrams := bigrams(tokenize("database migration postgresql"))
	newTerms := significantTerms(tokenize("database migration postgresql"))

	var matches []overlapMatch
	compareNamespace(tmp, "alice", newBigrams, newTerms, 0.0, &matches)

	// The stop-words-only file should produce empty bigrams and be skipped
	if len(matches) != 0 {
		t.Errorf("expected no matches for all-stop-words file, got %d", len(matches))
	}
}

// ---------------------------------------------------------------------------
// app.go: jaccardSimilarity with both-empty guard
// ---------------------------------------------------------------------------

func TestJaccardSimilarity_BothNil(t *testing.T) {
	score := jaccardSimilarity(nil, nil)
	if score != 0.0 {
		t.Errorf("both nil should give 0.0, got %f", score)
	}
}

// ---------------------------------------------------------------------------
// app.go: CheckOverlap with all stop words producing empty bigrams
// ---------------------------------------------------------------------------

func TestCheckOverlap_AllStopWordsInput(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "me-dev"
	_ = os.MkdirAll(filepath.Join(wcDir, "system", "projects", ns), 0755)

	// Create another namespace
	otherDir := filepath.Join(wcDir, "system", "projects", "other-dev")
	_ = os.MkdirAll(otherDir, 0755)
	_ = os.WriteFile(filepath.Join(otherDir, "proj.md"), []byte("database migration schema"), 0644)

	a := &App{}

	// Input that is entirely stop words produces empty bigrams, should bail early
	a.CheckOverlap("the and for", "the and for with this that")
}

// ---------------------------------------------------------------------------
// completions.go: CompleteTaskAddresses where loadRootIndexForCompletion
// succeeds but resolverForCompletion fails
// ---------------------------------------------------------------------------

func TestCompleteTaskAddresses_IndexSucceedsResolverFails(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(`{"nodes":{"my-node":{"name":"My Node","type":"leaf","state":"not_started","address":"my-node","children":[]}}}`), 0644)

	a := &App{
		State: state.NewStore(projDir, state.DefaultLockTimeout),
	}

	fn := CompleteTaskAddresses(a)
	addrs, _ := fn(nil, nil, "")

	// Should succeed and return at least the node address
	found := false
	for _, addr := range addrs {
		if addr == "my-node" {
			found = true
		}
	}
	if !found {
		t.Error("expected node address in completions")
	}
}

// ---------------------------------------------------------------------------
// app.go: RequireIdentity
// ---------------------------------------------------------------------------

func TestRequireIdentity_NilIdentity(t *testing.T) {
	a := &App{}
	err := a.RequireIdentity()
	if err == nil {
		t.Error("expected error when Identity is nil")
	}
}

func TestRequireIdentity_WithIdentity(t *testing.T) {
	a := &App{
		Identity: &config.Identity{User: "test", Machine: "dev", Namespace: "test-dev"},
	}
	err := a.RequireIdentity()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// completions.go: storeForCompletion fallback paths
// ---------------------------------------------------------------------------

func TestStoreForCompletion_AlreadyLoaded(t *testing.T) {
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "projects")
	_ = os.MkdirAll(projDir, 0755)

	ss := state.NewStore(projDir, state.DefaultLockTimeout)
	a := &App{State: ss}
	got, err := storeForCompletion(a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ss {
		t.Error("expected same Store back")
	}
}

func TestStoreForCompletion_FallbackConfigFails(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	a := &App{}
	_, err := storeForCompletion(a)
	if err == nil {
		t.Error("expected error when LoadConfig fails")
	}
}

func TestStoreForCompletion_FallbackStateNil(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	_ = os.MkdirAll(filepath.Join(wcDir, "system", "base"), 0755)

	// Config loads but no identity => State stays nil
	cfg := config.Defaults()
	cfgData, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(wcDir, "system", "base", "config.json"), cfgData, 0644)

	t.Chdir(tmp)

	a := &App{}
	_, err := storeForCompletion(a)
	if err == nil {
		t.Error("expected error when State stays nil after config load")
	}
}

func TestStoreForCompletion_FallbackSuccess(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "tester-box"
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)
	_ = os.MkdirAll(filepath.Join(wcDir, "system", "local"), 0755)

	cfgJSON := `{"identity": {"user": "tester", "machine": "box"}}`
	_ = os.WriteFile(filepath.Join(wcDir, "system", "local", "config.json"), []byte(cfgJSON), 0644)
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(`{"nodes":{}}`), 0644)

	t.Chdir(tmp)

	a := &App{}
	got, err := storeForCompletion(a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Error("expected non-nil Store")
	}
}

// ---------------------------------------------------------------------------
// completions.go: CompleteTaskAddresses with invalid address in index
// ---------------------------------------------------------------------------

func TestCompleteTaskAddresses_InvalidAddressInIndex(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	// Address with uppercase chars fails ParseAddress (invalid slug)
	idxJSON := `{"nodes":{"INVALID NODE":{"name":"Bad","type":"leaf","state":"not_started","address":"INVALID NODE","children":[]}}}`
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(idxJSON), 0644)

	a := &App{
		State: state.NewStore(projDir, state.DefaultLockTimeout),
	}
	fn := CompleteTaskAddresses(a)
	addrs, _ := fn(nil, nil, "")

	// The invalid address still shows up in node listing (it's from the index),
	// but no task sub-addresses should appear since ParseAddress fails
	for _, addr := range addrs {
		if addr != "INVALID NODE" {
			t.Errorf("unexpected address %q; only the raw node address expected", addr)
		}
	}
}

// ---------------------------------------------------------------------------
// app.go: compareNamespace with unreadable .md file
// ---------------------------------------------------------------------------

func TestCompareNamespace_UnreadableMdFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	tmp := t.TempDir()

	// Create a .md file that can't be read
	mdPath := filepath.Join(tmp, "secret.md")
	_ = os.WriteFile(mdPath, []byte("database migration"), 0644)
	_ = os.Chmod(mdPath, 0000)
	defer func() { _ = os.Chmod(mdPath, 0644) }()

	newBigrams := bigrams(tokenize("database migration"))
	newTerms := significantTerms(tokenize("database migration"))

	var matches []overlapMatch
	compareNamespace(tmp, "alice", newBigrams, newTerms, 0.0, &matches)

	// Should skip unreadable files without panicking
	if len(matches) != 0 {
		t.Errorf("expected no matches for unreadable file, got %d", len(matches))
	}
}

// ---------------------------------------------------------------------------
// app.go: InitFromDir — coverage for the new --instance entrypoint
// ---------------------------------------------------------------------------

// TestInitFromDir_MissingDirectory verifies that an absent .wolfcastle
// produces a clear error and leaves the App unmutated.
func TestInitFromDir_MissingDirectory(t *testing.T) {
	tmp := t.TempDir()
	a := &App{}
	err := a.InitFromDir(filepath.Join(tmp, "no-such-worktree"))
	if err == nil {
		t.Fatal("expected error for missing .wolfcastle directory")
	}
	if a.Config != nil {
		t.Error("Config should remain nil after a failed InitFromDir")
	}
}

// TestInitFromDir_FileInsteadOfDirectory verifies that a .wolfcastle
// path that exists but isn't a directory is rejected.
func TestInitFromDir_FileInsteadOfDirectory(t *testing.T) {
	tmp := t.TempDir()
	// Create a regular file at .wolfcastle instead of a directory.
	if err := os.WriteFile(filepath.Join(tmp, ".wolfcastle"), []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	if err := a.InitFromDir(tmp); err == nil {
		t.Fatal("expected error for .wolfcastle that is not a directory")
	}
}

// TestInitFromDir_LoadConfigFails verifies that a malformed config
// surfaces as an error rather than silently leaving the App in a
// half-initialized state.
func TestInitFromDir_LoadConfigFails(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	if err := os.MkdirAll(filepath.Join(wcDir, "system", "base"), 0755); err != nil {
		t.Fatal(err)
	}
	// Write invalid JSON so config.Load fails.
	if err := os.WriteFile(filepath.Join(wcDir, "system", "base", "config.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	if err := a.InitFromDir(tmp); err == nil {
		t.Fatal("expected error when config fails to load")
	}
}

// TestInitFromDir_NoIdentityIsNotFatal verifies the documented
// contract: identity-not-configured is acceptable, Config gets wired
// up, but Identity and State stay nil for commands to surface a
// clearer error via RequireIdentity.
func TestInitFromDir_NoIdentityIsNotFatal(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	if err := os.MkdirAll(filepath.Join(wcDir, "system", "base"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfgData, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wcDir, "system", "base", "config.json"), cfgData, 0644); err != nil {
		t.Fatal(err)
	}

	a := &App{}
	if err := a.InitFromDir(tmp); err != nil {
		t.Fatalf("InitFromDir should not fail when identity is missing: %v", err)
	}
	if a.Config == nil {
		t.Error("Config should be wired up after successful InitFromDir")
	}
	if a.Daemon == nil {
		t.Error("Daemon repository should be wired up after successful InitFromDir")
	}
	if a.Prompts == nil {
		t.Error("Prompts repository should be wired up after successful InitFromDir")
	}
	if a.Git == nil {
		t.Error("Git provider should be wired up after successful InitFromDir")
	}
	if a.Classes == nil {
		t.Error("Classes repository should be wired up after successful InitFromDir")
	}
	if a.Identity != nil {
		t.Error("Identity should remain nil when local config has no identity")
	}
	if a.State != nil {
		t.Error("State should remain nil when identity is not configured")
	}
}

// TestInitFromDir_WithIdentityWiresState verifies the happy path:
// when identity is configured in the local tier, InitFromDir wires up
// every repository AND populates Identity + State.
func TestInitFromDir_WithIdentityWiresState(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	if err := os.MkdirAll(filepath.Join(wcDir, "system", "base"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wcDir, "system", "local"), 0755); err != nil {
		t.Fatal(err)
	}
	cfgJSON := `{"identity": {"user": "tester", "machine": "box"}}`
	if err := os.WriteFile(filepath.Join(wcDir, "system", "local", "config.json"), []byte(cfgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	a := &App{}
	if err := a.InitFromDir(tmp); err != nil {
		t.Fatalf("InitFromDir failed: %v", err)
	}
	if a.Identity == nil {
		t.Fatal("Identity should be populated when local config has one")
	}
	if a.State == nil {
		t.Fatal("State should be populated when identity is configured")
	}
}

// TestInitFromDir_PointsAtSpecifiedDirectoryNotCWD is the regression
// for the whole reason InitFromDir exists: when called with a
// directory that is NOT the current working directory, the resulting
// App points at that directory's repositories, not the CWD's.
func TestInitFromDir_PointsAtSpecifiedDirectoryNotCWD(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// Set up a .wolfcastle in a temp dir, then chdir somewhere else.
	target := t.TempDir()
	wcDir := filepath.Join(target, ".wolfcastle")
	if err := os.MkdirAll(filepath.Join(wcDir, "system", "base"), 0755); err != nil {
		t.Fatal(err)
	}
	cfgJSON := `{"version": 1}`
	if err := os.WriteFile(filepath.Join(wcDir, "system", "base", "config.json"), []byte(cfgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	other := t.TempDir()
	t.Chdir(other)

	a := &App{}
	if err := a.InitFromDir(target); err != nil {
		t.Fatalf("InitFromDir(target) failed: %v", err)
	}
	if a.Config == nil {
		t.Fatal("Config not wired")
	}
	gotRoot, err := filepath.EvalSymlinks(a.Config.Root())
	if err != nil {
		t.Fatal(err)
	}
	wantRoot, err := filepath.EvalSymlinks(wcDir)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != wantRoot {
		t.Errorf("Config root = %q, want %q (the App should point at the target, not the CWD)", gotRoot, wantRoot)
	}
}
