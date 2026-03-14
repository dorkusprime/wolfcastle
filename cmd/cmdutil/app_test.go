package cmdutil

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/tree"
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
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
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

	sub := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(sub)

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

func TestFindWolfcastleDir_NotFound(t *testing.T) {
	tmp := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmp)

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
	os.MkdirAll(sub, 0755)
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
