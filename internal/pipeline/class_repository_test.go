package pipeline_test

import (
	"os"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

func TestClassRepository_Resolve_SimpleKey(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/bugfix.md", "You are a bug-fixing agent.")

	env.Classes.Reload(map[string]config.ClassDef{
		"bugfix": {Description: "Fix bugs"},
	})

	got, err := env.Classes.Resolve("bugfix")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got != "You are a bug-fixing agent." {
		t.Errorf("expected prompt content, got %q", got)
	}
}

func TestClassRepository_Resolve_HierarchicalFallback_Hyphen(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/lang.md", "You handle language tasks.")

	env.Classes.Reload(map[string]config.ClassDef{
		"lang-go": {Description: "Go language tasks"},
	})

	got, err := env.Classes.Resolve("lang-go")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got != "You handle language tasks." {
		t.Errorf("expected fallback to lang.md, got %q", got)
	}
}

func TestClassRepository_Resolve_HierarchicalFallback_Slash(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/typescript.md", "You handle TypeScript.")

	env.Classes.Reload(map[string]config.ClassDef{
		"typescript/react": {Description: "React TypeScript tasks"},
	})

	got, err := env.Classes.Resolve("typescript/react")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got != "You handle TypeScript." {
		t.Errorf("expected fallback to typescript.md, got %q", got)
	}
}

func TestClassRepository_Resolve_ExactKeyBeforeFallback(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/lang.md", "generic lang").
		WithPrompt("classes/lang-go.md", "specific go")

	env.Classes.Reload(map[string]config.ClassDef{
		"lang-go": {Description: "Go"},
	})

	got, err := env.Classes.Resolve("lang-go")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got != "specific go" {
		t.Errorf("expected exact match over fallback, got %q", got)
	}
}

func TestClassRepository_Resolve_UnknownKeyErrors(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	env.Classes.Reload(map[string]config.ClassDef{
		"bugfix": {Description: "Fix bugs"},
	})

	_, err := env.Classes.Resolve("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown class key")
	}
	if !strings.HasPrefix(err.Error(), "classes:") {
		t.Errorf("expected error prefixed with 'classes:', got: %v", err)
	}
}

func TestClassRepository_Resolve_KeyExistsButPromptMissing(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	env.Classes.Reload(map[string]config.ClassDef{
		"orphan": {Description: "No prompt file anywhere"},
	})

	_, err := env.Classes.Resolve("orphan")
	if err == nil {
		t.Fatal("expected error when prompt file missing from all tiers")
	}
	if !strings.HasPrefix(err.Error(), "classes:") {
		t.Errorf("expected error prefixed with 'classes:', got: %v", err)
	}
}

func TestClassRepository_Resolve_KeyWithParentButBothMissing(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	env.Classes.Reload(map[string]config.ClassDef{
		"lang-go": {Description: "Go tasks"},
	})

	_, err := env.Classes.Resolve("lang-go")
	if err == nil {
		t.Fatal("expected error when both exact and fallback prompts are missing")
	}
	if !strings.Contains(err.Error(), "fallback") {
		t.Errorf("expected error to mention fallback, got: %v", err)
	}
}

// ── Subdirectory assembly tests ──────────────────────────────────────────

func TestClassRepository_Resolve_SubdirectoryAssembly(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/bugfix.md", "You are a bug-fixing agent.").
		WithPrompt("classes/bugfix/voice.md", "Be concise.").
		WithPrompt("classes/bugfix/tools.md", "Use grep.")

	env.Classes.Reload(map[string]config.ClassDef{
		"bugfix": {Description: "Fix bugs"},
	})

	got, err := env.Classes.Resolve("bugfix")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	// Fragments are sorted lexicographically: tools.md before voice.md.
	want := "You are a bug-fixing agent.\nUse grep.\nBe concise."
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestClassRepository_Resolve_SubdirectoryAssembly_NoSubdir(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/bugfix.md", "You are a bug-fixing agent.")

	env.Classes.Reload(map[string]config.ClassDef{
		"bugfix": {Description: "Fix bugs"},
	})

	got, err := env.Classes.Resolve("bugfix")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got != "You are a bug-fixing agent." {
		t.Errorf("expected unmodified content, got %q", got)
	}
}

func TestClassRepository_Resolve_SubdirectoryAssembly_WithFallback(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/typescript.md", "You handle TypeScript.").
		WithPrompt("classes/typescript/style.md", "Follow Airbnb style guide.")

	env.Classes.Reload(map[string]config.ClassDef{
		"typescript/react": {Description: "React TypeScript tasks"},
	})

	got, err := env.Classes.Resolve("typescript/react")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	// Falls back to typescript.md, assembles subdirectory from typescript/.
	want := "You handle TypeScript.\nFollow Airbnb style guide."
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestClassRepository_Resolve_SubdirectoryAssembly_MultipleFiles(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/writing.md", "You are a writing assistant.").
		WithPrompt("classes/writing/audience.md", "Write for developers.").
		WithPrompt("classes/writing/format.md", "Use markdown.").
		WithPrompt("classes/writing/tone.md", "Be direct.").
		WithPrompt("classes/writing/voice.md", "Active voice only.")

	env.Classes.Reload(map[string]config.ClassDef{
		"writing": {Description: "Writing tasks"},
	})

	got, err := env.Classes.Resolve("writing")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	// Four fragments appended in lexicographic order: audience, format, tone, voice.
	want := "You are a writing assistant.\nWrite for developers.\nUse markdown.\nBe direct.\nActive voice only."
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestClassRepository_Resolve_SubdirectoryAssembly_ScanErrorIgnored(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/fragile.md", "Main content.").
		WithPrompt("classes/fragile/placeholder.md", "This will be unreadable.")

	// Make the subdirectory unreadable so ListFragments returns an I/O error
	// rather than an empty result.
	subdirPath := env.Tiers.BasePath("prompts/classes/fragile")
	if err := os.Chmod(subdirPath, 0o000); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(subdirPath, 0o755) })

	env.Classes.Reload(map[string]config.ClassDef{
		"fragile": {Description: "Class with broken subdirectory"},
	})

	got, err := env.Classes.Resolve("fragile")
	if err != nil {
		t.Fatalf("Resolve() should succeed despite unreadable subdirectory, got error: %v", err)
	}
	if got != "Main content." {
		t.Errorf("expected main content only, got %q", got)
	}
}

func TestClassRepository_Reload_ReplacesMap(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/alpha.md", "alpha prompt").
		WithPrompt("classes/beta.md", "beta prompt")

	env.Classes.Reload(map[string]config.ClassDef{
		"alpha": {Description: "Alpha"},
	})

	got, err := env.Classes.Resolve("alpha")
	if err != nil {
		t.Fatalf("Resolve(alpha) error: %v", err)
	}
	if got != "alpha prompt" {
		t.Errorf("expected alpha prompt, got %q", got)
	}

	// Reload with a different map; alpha should vanish, beta should appear.
	env.Classes.Reload(map[string]config.ClassDef{
		"beta": {Description: "Beta"},
	})

	_, err = env.Classes.Resolve("alpha")
	if err == nil {
		t.Fatal("expected error for alpha after reload replaced it")
	}

	got, err = env.Classes.Resolve("beta")
	if err != nil {
		t.Fatalf("Resolve(beta) after reload error: %v", err)
	}
	if got != "beta prompt" {
		t.Errorf("expected beta prompt, got %q", got)
	}
}

func TestClassRepository_List_SortedKeys(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	env.Classes.Reload(map[string]config.ClassDef{
		"charlie": {Description: "C"},
		"alpha":   {Description: "A"},
		"bravo":   {Description: "B"},
	})

	got := env.Classes.List()
	if len(got) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(got))
	}
	if got[0] != "alpha" || got[1] != "bravo" || got[2] != "charlie" {
		t.Errorf("expected [alpha bravo charlie], got %v", got)
	}
}

func TestClassRepository_List_EmptyAfterConstruction(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	got := env.Classes.List()
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestClassRepository_Validate_IdentifiesMissingPrompts(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/present.md", "I exist.")

	env.Classes.Reload(map[string]config.ClassDef{
		"present": {Description: "Has a prompt"},
		"absent":  {Description: "No prompt file"},
	})

	missing := env.Classes.Validate()
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing, got %d: %v", len(missing), missing)
	}
	if missing[0] != "absent" {
		t.Errorf("expected 'absent' in missing list, got %v", missing)
	}
}

func TestClassRepository_Validate_AllPresent(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/one.md", "first").
		WithPrompt("classes/two.md", "second")

	env.Classes.Reload(map[string]config.ClassDef{
		"one": {Description: "One"},
		"two": {Description: "Two"},
	})

	missing := env.Classes.Validate()
	if len(missing) != 0 {
		t.Errorf("expected no missing classes, got %v", missing)
	}
}

func TestClassRepository_Validate_FallbackCountsAsPresent(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/lang.md", "language prompt")

	env.Classes.Reload(map[string]config.ClassDef{
		"lang-go":     {Description: "Go"},
		"lang-python": {Description: "Python"},
	})

	missing := env.Classes.Validate()
	if len(missing) != 0 {
		t.Errorf("expected fallback to satisfy validation, got missing: %v", missing)
	}
}
