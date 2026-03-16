package cmdutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/tree"
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

	// App has a resolver but set it to nil to simulate broken state
	a := &App{
		WolfcastleDir: wcDir,
		Resolver:      nil, // broken: no resolver
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

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

	a := &App{
		WolfcastleDir: wcDir,
		Cfg: &config.Config{
			OverlapAdvisory: config.OverlapConfig{Enabled: true, Threshold: 0.1},
		},
		Resolver: &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns},
	}

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

	// Has resolver for loadRootIndex but we'll test the resolver-for-completion path
	// by providing a resolver that works for index but simulating a scenario where
	// resolverForCompletion's fallback is needed
	a := &App{
		WolfcastleDir: wcDir,
		Resolver:      &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns},
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
