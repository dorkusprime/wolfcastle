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
