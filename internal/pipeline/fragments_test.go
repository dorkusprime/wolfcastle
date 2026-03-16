package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTiers(t *testing.T, dir string) {
	t.Helper()
	for _, tier := range []string{"system/base", "system/custom", "system/local"} {
		if err := os.MkdirAll(filepath.Join(dir, tier, "rules"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, tier, "prompts"), 0755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestResolveFragment_ReturnsBaseContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "test.md"), []byte("base content"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveFragment(dir, "prompts/test.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "base content" {
		t.Errorf("expected %q, got %q", "base content", got)
	}
}

func TestResolveFragment_CustomReplacesBase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "test.md"), []byte("base"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "custom", "prompts", "test.md"), []byte("custom"), 0644)

	got, err := ResolveFragment(dir, "prompts/test.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "custom" {
		t.Errorf("expected %q, got %q", "custom", got)
	}
}

func TestResolveFragment_LocalReplacesBoth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "test.md"), []byte("base"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "custom", "prompts", "test.md"), []byte("custom"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "local", "prompts", "test.md"), []byte("local"), 0644)

	got, err := ResolveFragment(dir, "prompts/test.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "local" {
		t.Errorf("expected %q, got %q", "local", got)
	}
}

func TestResolveFragment_ErrorWhenNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_, err := ResolveFragment(dir, "prompts/missing.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestResolveAllFragments_DiscoversMergesAcrossTiers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	// base has a.md and b.md
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "rules", "a.md"), []byte("base-a"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "rules", "b.md"), []byte("base-b"), 0644)

	// custom overrides a.md, adds c.md
	_ = os.WriteFile(filepath.Join(dir, "system", "custom", "rules", "a.md"), []byte("custom-a"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "custom", "rules", "c.md"), []byte("custom-c"), 0644)

	contents, err := ResolveAllFragments(dir, "rules", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(contents) != 3 {
		t.Fatalf("expected 3 fragments, got %d", len(contents))
	}
	// Sorted: a.md, b.md, c.md
	// a.md should be custom override
	if contents[0] != "custom-a" {
		t.Errorf("a.md: expected %q, got %q", "custom-a", contents[0])
	}
	if contents[1] != "base-b" {
		t.Errorf("b.md: expected %q, got %q", "base-b", contents[1])
	}
	if contents[2] != "custom-c" {
		t.Errorf("c.md: expected %q, got %q", "custom-c", contents[2])
	}
}

func TestResolveAllFragments_RespectsExcludeList(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_ = os.WriteFile(filepath.Join(dir, "system", "base", "rules", "a.md"), []byte("a"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "rules", "b.md"), []byte("b"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "rules", "c.md"), []byte("c"), 0644)

	contents, err := ResolveAllFragments(dir, "rules", nil, []string{"b.md"})
	if err != nil {
		t.Fatal(err)
	}

	if len(contents) != 2 {
		t.Fatalf("expected 2 fragments, got %d", len(contents))
	}
	if contents[0] != "a" {
		t.Errorf("expected %q, got %q", "a", contents[0])
	}
	if contents[1] != "c" {
		t.Errorf("expected %q, got %q", "c", contents[1])
	}
}

func TestResolveAllFragments_ErrorsOnMissingInclude(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_ = os.WriteFile(filepath.Join(dir, "system", "base", "rules", "a.md"), []byte("a"), 0644)

	_, err := ResolveAllFragments(dir, "rules", []string{"a.md", "missing.md"}, nil)
	if err == nil {
		t.Error("expected error for missing include entry")
	}
}

func TestResolveAllFragments_RespectsIncludeListOrdering(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_ = os.WriteFile(filepath.Join(dir, "system", "base", "rules", "a.md"), []byte("a"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "rules", "b.md"), []byte("b"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "rules", "c.md"), []byte("c"), 0644)

	// Include list specifies order: c, a (skipping b)
	contents, err := ResolveAllFragments(dir, "rules", []string{"c.md", "a.md"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(contents) != 2 {
		t.Fatalf("expected 2 fragments, got %d", len(contents))
	}
	if contents[0] != "c" {
		t.Errorf("first: expected %q, got %q", "c", contents[0])
	}
	if contents[1] != "a" {
		t.Errorf("second: expected %q, got %q", "a", contents[1])
	}
}

// ── ResolvePromptTemplate tests ──────────────────────────────────────────

func TestResolvePromptTemplate_PlainString(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "hello.md"), []byte("Hello, world!"), 0644)

	got, err := ResolvePromptTemplate(dir, "hello.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Hello, world!" {
		t.Errorf("expected %q, got %q", "Hello, world!", got)
	}
}

func TestResolvePromptTemplate_WithTemplateVars(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "greet.md"),
		[]byte("Hello, {{.Name}}! You are {{.Age}} years old."), 0644)

	ctx := struct {
		Name string
		Age  int
	}{"Alice", 30}

	got, err := ResolvePromptTemplate(dir, "greet.md", ctx)
	if err != nil {
		t.Fatal(err)
	}
	expected := "Hello, Alice! You are 30 years old."
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestResolvePromptTemplate_CustomOverridesBase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "msg.md"), []byte("base: {{.Val}}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "custom", "prompts", "msg.md"), []byte("custom: {{.Val}}"), 0644)

	got, err := ResolvePromptTemplate(dir, "msg.md", struct{ Val string }{"test"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "custom: test" {
		t.Errorf("expected %q, got %q", "custom: test", got)
	}
}

func TestResolvePromptTemplate_LocalOverridesAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "msg.md"), []byte("base"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "custom", "prompts", "msg.md"), []byte("custom"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "system", "local", "prompts", "msg.md"), []byte("local"), 0644)

	got, err := ResolvePromptTemplate(dir, "msg.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "local" {
		t.Errorf("expected %q, got %q", "local", got)
	}
}

func TestResolvePromptTemplate_ErrorOnMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_, err := ResolvePromptTemplate(dir, "missing.md", nil)
	if err == nil {
		t.Error("expected error for missing prompt file")
	}
}

func TestResolvePromptTemplate_ErrorOnBadTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "bad.md"), []byte("{{.Foo"), 0644)

	_, err := ResolvePromptTemplate(dir, "bad.md", struct{ Foo string }{"x"})
	if err == nil {
		t.Error("expected error for malformed template")
	}
}

func TestResolvePromptTemplate_DecompositionTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	tmpl := "Break {{.NodeAddr}} into sub-tasks."
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "decomposition.md"), []byte(tmpl), 0644)

	ctx := DecompositionContext{NodeAddr: "project/auth"}
	got, err := ResolvePromptTemplate(dir, "decomposition.md", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Break project/auth into sub-tasks." {
		t.Errorf("got %q", got)
	}
}

func TestResolvePromptTemplate_ContextHeadersTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	tmpl := "Failed {{.FailureCount}} times. Threshold: {{.DecompThreshold}}"
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "context-headers.md"), []byte(tmpl), 0644)

	ctx := FailureHeaderContext{
		FailureCount:    5,
		DecompThreshold: 10,
		MaxDecompDepth:  3,
		CurrentDepth:    1,
		HardCap:         50,
	}
	got, err := ResolvePromptTemplate(dir, "context-headers.md", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Failed 5 times. Threshold: 10" {
		t.Errorf("got %q", got)
	}
}
