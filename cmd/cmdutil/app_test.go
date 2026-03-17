package cmdutil

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// tokenize
// ---------------------------------------------------------------------------

func TestTokenize_BasicWords(t *testing.T) {
	tokens := tokenize("Hello World 42")
	want := []string{"hello", "world", "42"}
	if len(tokens) != len(want) {
		t.Fatalf("got %v, want %v", tokens, want)
	}
	for i, tok := range tokens {
		if tok != want[i] {
			t.Errorf("token[%d] = %q, want %q", i, tok, want[i])
		}
	}
}

func TestTokenize_StripsPunctuation(t *testing.T) {
	tokens := tokenize("hello, world! foo-bar")
	for _, tok := range tokens {
		for _, r := range tok {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				t.Errorf("token %q contains non-alphanumeric rune %q", tok, r)
			}
		}
	}
}

func TestTokenize_DropsSingleChar(t *testing.T) {
	tokens := tokenize("a b cc dd")
	for _, tok := range tokens {
		if len(tok) < 2 {
			t.Errorf("single-char token %q should have been dropped", tok)
		}
	}
}

func TestTokenize_Empty(t *testing.T) {
	tokens := tokenize("")
	if len(tokens) != 0 {
		t.Errorf("expected empty, got %v", tokens)
	}
}

// ---------------------------------------------------------------------------
// isStopWord
// ---------------------------------------------------------------------------

func TestIsStopWord_True(t *testing.T) {
	stops := []string{"the", "and", "for", "with", "this", "that", "project", "audit"}
	for _, w := range stops {
		if !isStopWord(w) {
			t.Errorf("expected %q to be a stop word", w)
		}
	}
}

func TestIsStopWord_False(t *testing.T) {
	words := []string{"database", "authentication", "refactor", "endpoint"}
	for _, w := range words {
		if isStopWord(w) {
			t.Errorf("expected %q not to be a stop word", w)
		}
	}
}

// ---------------------------------------------------------------------------
// bigrams
// ---------------------------------------------------------------------------

func TestBigrams_Basic(t *testing.T) {
	bg := bigrams([]string{"abc"})
	if !bg["ab"] || !bg["bc"] {
		t.Errorf("expected bigrams ab, bc; got %v", bg)
	}
	if len(bg) != 2 {
		t.Errorf("expected 2 bigrams, got %d", len(bg))
	}
}

func TestBigrams_SkipsStopWords(t *testing.T) {
	bg := bigrams([]string{"the", "database"})
	if bg["th"] || bg["he"] {
		t.Errorf("stop word 'the' should not contribute bigrams")
	}
	if !bg["da"] {
		t.Errorf("expected bigram 'da' from 'database'")
	}
}

func TestBigrams_EmptyInput(t *testing.T) {
	bg := bigrams(nil)
	if len(bg) != 0 {
		t.Errorf("expected empty bigram set, got %v", bg)
	}
}

func TestBigrams_SingleCharToken(t *testing.T) {
	bg := bigrams([]string{"x"})
	if len(bg) != 0 {
		t.Errorf("expected no bigrams from single char, got %v", bg)
	}
}

// ---------------------------------------------------------------------------
// jaccardSimilarity
// ---------------------------------------------------------------------------

func TestJaccardSimilarity_Identical(t *testing.T) {
	a := map[string]bool{"ab": true, "bc": true}
	b := map[string]bool{"ab": true, "bc": true}
	score := jaccardSimilarity(a, b)
	if score != 1.0 {
		t.Errorf("identical sets should have score 1.0, got %f", score)
	}
}

func TestJaccardSimilarity_Disjoint(t *testing.T) {
	a := map[string]bool{"ab": true, "bc": true}
	b := map[string]bool{"xy": true, "yz": true}
	score := jaccardSimilarity(a, b)
	if score != 0.0 {
		t.Errorf("disjoint sets should have score 0.0, got %f", score)
	}
}

func TestJaccardSimilarity_Partial(t *testing.T) {
	a := map[string]bool{"ab": true, "bc": true, "cd": true}
	b := map[string]bool{"ab": true, "bc": true, "xy": true}
	expected := 2.0 / 4.0
	score := jaccardSimilarity(a, b)
	if math.Abs(score-expected) > 0.001 {
		t.Errorf("expected %f, got %f", expected, score)
	}
}

func TestJaccardSimilarity_BothEmpty(t *testing.T) {
	score := jaccardSimilarity(map[string]bool{}, map[string]bool{})
	if score != 0.0 {
		t.Errorf("both empty should give 0.0, got %f", score)
	}
}

func TestJaccardSimilarity_OneEmpty(t *testing.T) {
	a := map[string]bool{"ab": true}
	score := jaccardSimilarity(a, map[string]bool{})
	if score != 0.0 {
		t.Errorf("one empty should give 0.0, got %f", score)
	}
}

// ---------------------------------------------------------------------------
// significantTerms
// ---------------------------------------------------------------------------

func TestSignificantTerms_FiltersStopWords(t *testing.T) {
	terms := significantTerms([]string{"the", "database", "authentication", "for", "ab"})
	if terms["the"] || terms["for"] {
		t.Error("stop words should not be in significant terms")
	}
	if !terms["database"] || !terms["authentication"] {
		t.Error("non-stop words should be in significant terms")
	}
	if terms["ab"] {
		t.Error("short terms (len<=2) should be excluded")
	}
}

func TestSignificantTerms_Empty(t *testing.T) {
	terms := significantTerms(nil)
	if len(terms) != 0 {
		t.Errorf("expected empty, got %v", terms)
	}
}

// ---------------------------------------------------------------------------
// FindWolfcastleDir
// ---------------------------------------------------------------------------

func TestFindWolfcastleDir_Found(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	if err := os.MkdirAll(wcDir, 0755); err != nil {
		t.Fatal(err)
	}

	// CWD must be the directory containing .wolfcastle (no walking up)
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	app := &App{}
	found, err := app.FindWolfcastleDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Resolve symlinks (macOS /var -> /private/var)
	resolvedWcDir, _ := filepath.EvalSymlinks(wcDir)
	resolvedFound, _ := filepath.EvalSymlinks(found)
	if resolvedFound != resolvedWcDir {
		t.Errorf("got %q, want %q", resolvedFound, resolvedWcDir)
	}
}

func TestFindWolfcastleDir_DoesNotWalkUp(t *testing.T) {
	tmp := t.TempDir()
	// Create .wolfcastle in the parent
	if err := os.MkdirAll(filepath.Join(tmp, ".wolfcastle"), 0755); err != nil {
		t.Fatal(err)
	}
	// CWD is a subdirectory without .wolfcastle
	sub := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(sub)

	app := &App{}
	_, err := app.FindWolfcastleDir()
	if err == nil {
		t.Fatal("expected error: should not walk up to find .wolfcastle in ancestor")
	}
}

func TestFindWolfcastleDir_NotFound(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	app := &App{}
	_, err := app.FindWolfcastleDir()
	if err == nil {
		t.Error("expected error when no .wolfcastle dir exists")
	}
}

// ---------------------------------------------------------------------------
// RequireResolver
// ---------------------------------------------------------------------------

func TestRequireResolver_NilResolver(t *testing.T) {
	app := &App{}
	err := app.RequireResolver()
	if err == nil {
		t.Error("expected error when resolver is nil")
	}
}

func TestRequireResolver_WithResolver(t *testing.T) {
	app := &App{
		Resolver: &tree.Resolver{
			WolfcastleDir: "/tmp/fake",
			Namespace:     "test",
		},
	}
	err := app.RequireResolver()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// overlapMatch / compareNamespace
// ---------------------------------------------------------------------------

func TestCompareNamespace_FindsOverlap(t *testing.T) {
	tmp := t.TempDir()

	// Write a project .md file
	content := "database migration schema refactoring for postgresql"
	if err := os.WriteFile(filepath.Join(tmp, "db-migration.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	newText := "database migration schema upgrade postgresql"
	newBigrams := bigrams(tokenize(newText))
	newTerms := significantTerms(tokenize(newText))

	var matches []overlapMatch
	compareNamespace(tmp, "alice", newBigrams, newTerms, 0.1, &matches)

	if len(matches) == 0 {
		t.Error("expected at least one overlap match for similar content")
	}
}

func TestCompareNamespace_NoOverlap(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "unrelated.md"), []byte("quantum physics photon entanglement"), 0644); err != nil {
		t.Fatal(err)
	}

	newBigrams := bigrams(tokenize("database migration postgresql"))
	newTerms := significantTerms(tokenize("database migration postgresql"))

	var matches []overlapMatch
	compareNamespace(tmp, "bob", newBigrams, newTerms, 0.5, &matches)

	if len(matches) != 0 {
		t.Errorf("expected no matches for unrelated content, got %d", len(matches))
	}
}

func TestCompareNamespace_SkipsNonMd(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "state.json"), []byte("database migration"), 0644); err != nil {
		t.Fatal(err)
	}

	newBigrams := bigrams(tokenize("database migration"))
	newTerms := significantTerms(tokenize("database migration"))

	var matches []overlapMatch
	compareNamespace(tmp, "charlie", newBigrams, newTerms, 0.0, &matches)

	if len(matches) != 0 {
		t.Error("should skip non-.md files")
	}
}

func TestCompareNamespace_SkipsEmptyFiles(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "empty.md"), []byte("   "), 0644); err != nil {
		t.Fatal(err)
	}

	newBigrams := bigrams(tokenize("anything"))
	newTerms := significantTerms(tokenize("anything"))

	var matches []overlapMatch
	compareNamespace(tmp, "dave", newBigrams, newTerms, 0.0, &matches)

	if len(matches) != 0 {
		t.Error("should skip empty/whitespace-only files")
	}
}

func TestCompareNamespace_RecursesSubdirs(t *testing.T) {
	sub := filepath.Join(t.TempDir(), "sub")
	_ = os.MkdirAll(sub, 0755)
	if err := os.WriteFile(filepath.Join(sub, "nested.md"), []byte("database migration schema"), 0644); err != nil {
		t.Fatal(err)
	}

	newBigrams := bigrams(tokenize("database migration schema"))
	newTerms := significantTerms(tokenize("database migration schema"))

	var matches []overlapMatch
	compareNamespace(filepath.Dir(sub), "eve", newBigrams, newTerms, 0.1, &matches)

	if len(matches) == 0 {
		t.Error("should recurse into subdirectories")
	}
}

// ---------------------------------------------------------------------------
// LoadConfig
// ---------------------------------------------------------------------------

func TestLoadConfig_Success(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	_ = os.MkdirAll(filepath.Join(wcDir, "system", "base"), 0755)
	_ = os.MkdirAll(filepath.Join(wcDir, "system", "local"), 0755)

	// Use Defaults() to get a valid config, then marshal
	cfg := config.Defaults()
	cfg.Identity = &config.IdentityConfig{User: "tester", Machine: "box"}
	cfgData, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(filepath.Join(wcDir, "system", "base", "config.json"), cfgData, 0644)

	localJSON := `{"identity": {"user": "tester", "machine": "box"}}`
	_ = os.WriteFile(filepath.Join(wcDir, "system", "local", "config.json"), []byte(localJSON), 0644)

	// Create projects dir for namespace
	ns := "tester-box"
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	// Write a root index so the resolver can init
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(`{"nodes":{}}`), 0644)

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()

	// cd into the directory containing .wolfcastle (no walking up)
	_ = os.Chdir(tmp)

	a := &App{}
	err := a.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if a.Cfg == nil {
		t.Error("expected config to be loaded")
	}
	// Resolver may or may not init depending on identity; at minimum WolfcastleDir is set
	resolvedWC, _ := filepath.EvalSymlinks(wcDir)
	resolvedApp, _ := filepath.EvalSymlinks(a.WolfcastleDir)
	if resolvedApp != resolvedWC {
		t.Errorf("WolfcastleDir = %q, want %q", resolvedApp, resolvedWC)
	}
}

func TestLoadConfig_NoWolfcastleDir(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	a := &App{}
	err := a.LoadConfig()
	if err == nil {
		t.Error("expected error when no .wolfcastle exists")
	}
}

// ---------------------------------------------------------------------------
// CheckOverlap
// ---------------------------------------------------------------------------

func TestCheckOverlap_DisabledConfig(t *testing.T) {
	a := &App{} // nil Cfg
	// Should not panic
	a.CheckOverlap("test", "description")
}

func TestCheckOverlap_NilResolver(t *testing.T) {
	a := &App{
		Cfg: &config.Config{
			OverlapAdvisory: config.OverlapConfig{Enabled: true, Threshold: 0.3},
		},
	}
	// Should return silently when resolver is nil
	a.CheckOverlap("test", "description")
}

func TestCheckOverlap_NotEnabled(t *testing.T) {
	a := &App{
		Cfg: &config.Config{
			OverlapAdvisory: config.OverlapConfig{Enabled: false},
		},
		Resolver: &tree.Resolver{WolfcastleDir: "/tmp/fake", Namespace: "test"},
	}
	// Should return without error
	a.CheckOverlap("test", "description")
}

func TestCheckOverlap_EmptyProject(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "me-dev"
	_ = os.MkdirAll(filepath.Join(wcDir, "system", "projects", ns), 0755)

	a := &App{
		WolfcastleDir: wcDir,
		Cfg: &config.Config{
			OverlapAdvisory: config.OverlapConfig{Enabled: true, Threshold: 0.3},
		},
		Resolver: &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns},
	}
	// No other namespaces, should not panic
	a.CheckOverlap("database migration", "migrate the database schema")
}

func TestCheckOverlap_FindsMatch(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "me-dev"
	_ = os.MkdirAll(filepath.Join(wcDir, "system", "projects", ns), 0755)

	// Create another engineer's namespace with similar project
	otherDir := filepath.Join(wcDir, "system", "projects", "alice-dev")
	_ = os.MkdirAll(otherDir, 0755)
	_ = os.WriteFile(filepath.Join(otherDir, "database-migration.md"),
		[]byte("database migration schema upgrade postgresql"), 0644)

	a := &App{
		WolfcastleDir: wcDir,
		Cfg: &config.Config{
			OverlapAdvisory: config.OverlapConfig{Enabled: true, Threshold: 0.1},
		},
		Resolver: &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns},
	}
	// Should not panic, should detect overlap silently
	a.CheckOverlap("database migration", "database migration schema upgrade postgresql")
}

// ---------------------------------------------------------------------------
// Completions
// ---------------------------------------------------------------------------

func TestCompleteNodeAddresses_NilResolver(t *testing.T) {
	// Run from a temp dir to avoid picking up .wolfcastle/ from the repo
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()

	a := &App{}
	fn := CompleteNodeAddresses(a)
	addrs, directive := fn(nil, nil, "")
	if addrs != nil {
		t.Errorf("expected nil addrs, got %v", addrs)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Error("expected NoFileComp directive")
	}
}

func TestCompleteTaskAddresses_NilResolver(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmp)
	defer func() { _ = os.Chdir(origDir) }()

	a := &App{}
	fn := CompleteTaskAddresses(a)
	addrs, directive := fn(nil, nil, "")
	if addrs != nil {
		t.Errorf("expected nil addrs, got %v", addrs)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Error("expected NoFileComp directive")
	}
}

func TestCompleteNodeAddresses_WithResolver(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	idxJSON := `{"nodes":{"my-node":{"name":"My Node","type":"leaf","state":"not_started","address":"my-node","children":[]}}}`
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(idxJSON), 0644)

	a := &App{
		WolfcastleDir: wcDir,
		Resolver:      &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns},
	}
	fn := CompleteNodeAddresses(a)
	addrs, _ := fn(nil, nil, "")
	if len(addrs) != 1 {
		t.Fatalf("expected 1 address, got %d", len(addrs))
	}
	if addrs[0] != "my-node" {
		t.Errorf("expected 'my-node', got %q", addrs[0])
	}
}

func TestConfigNotReady_Error(t *testing.T) {
	e := &ConfigNotReady{}
	if e.Error() != "config not ready" {
		t.Errorf("unexpected error message: %s", e.Error())
	}
}

func TestLoadRootIndexForCompletion_AlreadyLoaded(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	idxJSON := `{"nodes":{}}`
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(idxJSON), 0644)

	a := &App{
		WolfcastleDir: wcDir,
		Resolver:      &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns},
	}
	idx, err := loadRootIndexForCompletion(a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx == nil {
		t.Error("expected non-nil index")
	}
}

func TestResolverForCompletion_AlreadyLoaded(t *testing.T) {
	r := &tree.Resolver{WolfcastleDir: "/tmp/fake", Namespace: "test"}
	a := &App{Resolver: r}
	got, err := resolverForCompletion(a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != r {
		t.Error("expected the same resolver")
	}
}

func TestResolverForCompletion_NilResolverNoConfig(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmp)

	a := &App{}
	_, err := resolverForCompletion(a)
	if err == nil {
		t.Error("expected error when resolver and config are both unavailable")
	}
}

func TestCompleteTaskAddresses_WithResolverAndLeaf(t *testing.T) {
	tmp := t.TempDir()
	wcDir := filepath.Join(tmp, ".wolfcastle")
	ns := "test-dev"
	projDir := filepath.Join(wcDir, "system", "projects", ns)
	_ = os.MkdirAll(projDir, 0755)

	// Create a root index with a leaf node
	idxJSON := `{"nodes":{"my-node":{"name":"My Node","type":"leaf","state":"in_progress","address":"my-node","children":[]}}}`
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), []byte(idxJSON), 0644)

	// Create node state with a task
	nodeDir := filepath.Join(projDir, "my-node")
	_ = os.MkdirAll(nodeDir, 0755)
	nodeJSON := `{
		"id": "my-node",
		"name": "My Node",
		"type": "leaf",
		"state": "in_progress",
		"tasks": [{"id":"task-0001","description":"do work","state":"not_started"}],
		"audit": {"status": "pending", "breadcrumbs": [], "gaps": [], "escalations": []}
	}`
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), []byte(nodeJSON), 0644)

	a := &App{
		WolfcastleDir: wcDir,
		Resolver:      &tree.Resolver{WolfcastleDir: wcDir, Namespace: ns},
	}
	fn := CompleteTaskAddresses(a)
	addrs, _ := fn(nil, nil, "")
	// Should contain both the node address and the task address
	foundNode := false
	foundTask := false
	for _, addr := range addrs {
		if addr == "my-node" {
			foundNode = true
		}
		if addr == "my-node/task-0001" {
			foundTask = true
		}
	}
	if !foundNode {
		t.Error("expected node address in completions")
	}
	if !foundTask {
		t.Error("expected task address in completions")
	}
}
