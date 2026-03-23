package pipeline_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// stubResolver is a tierfs.Resolver that returns injected errors for specific
// operations. Non-error calls delegate to a real resolver.
type stubResolver struct {
	tierfs.Resolver
	writeBaseErr  error
	resolveErr    error
	resolveAllErr error
}

func (s *stubResolver) WriteBase(relPath string, data []byte) error {
	if s.writeBaseErr != nil {
		return s.writeBaseErr
	}
	return s.Resolver.WriteBase(relPath, data)
}

func (s *stubResolver) Resolve(relPath string) ([]byte, error) {
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}
	return s.Resolver.Resolve(relPath)
}

func (s *stubResolver) ResolveAll(subdir string) (map[string][]byte, error) {
	if s.resolveAllErr != nil {
		return nil, s.resolveAllErr
	}
	return s.Resolver.ResolveAll(subdir)
}

// --- WriteBase error path ---

func TestWriteBase_PropagatesWriteError(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	stub := &stubResolver{
		Resolver:     env.Tiers,
		writeBaseErr: errors.New("disk full"),
	}
	repo := pipeline.NewPromptRepositoryWithTiers(stub)

	err := repo.WriteBase("prompts/test.md", []byte("data"))
	if err == nil {
		t.Fatal("expected error from WriteBase")
	}
	if !strings.Contains(err.Error(), "write-base") {
		t.Errorf("error should wrap with 'write-base' context, got: %v", err)
	}
}

// --- WriteAllBase error paths ---

func TestWriteAllBase_WriteBaseFailureMidLoop(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	stub := &stubResolver{
		Resolver:     env.Tiers,
		writeBaseErr: errors.New("permission denied"),
	}
	repo := pipeline.NewPromptRepositoryWithTiers(stub)

	templates := fstest.MapFS{
		"prompts/one.md": {Data: []byte("first")},
	}

	err := repo.WriteAllBase(templates)
	if err == nil {
		t.Fatal("expected error from WriteAllBase when WriteBase fails")
	}
	if !strings.Contains(err.Error(), "write-base") {
		t.Errorf("error should mention 'write-base', got: %v", err)
	}
}

// errFS implements fs.FS but returns an error for any file read.
type errFS struct{}

func (errFS) Open(name string) (fs.File, error) {
	return nil, &os.PathError{Op: "open", Path: name, Err: errors.New("synthetic read error")}
}

func TestWriteAllBase_WalkError(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)

	// An fs.FS whose Open always fails triggers the WalkDir error path.
	err := repo.WriteAllBase(errFS{})
	if err == nil {
		t.Fatal("expected error from WriteAllBase when walk itself fails")
	}
}

func TestWriteAllBase_ReadFileError(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)

	// Create an fstest.MapFS where the file entry exists but the data read
	// is sabotaged by using an unreadable mode. fstest.MapFS doesn't enforce
	// permissions, so instead we build a real directory with a file that
	// can't be read.
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 has no effect on Windows")
	}

	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "bad.md")
	if err := os.WriteFile(fpath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fpath, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(fpath, 0o644) }()

	err := repo.WriteAllBase(os.DirFS(tmpDir))
	if err == nil {
		t.Fatal("expected error from WriteAllBase when ReadFile fails")
	}
	if !strings.Contains(err.Error(), "read embedded") {
		t.Errorf("error should mention 'read embedded', got: %v", err)
	}
}

// --- Resolve template execution error ---

func TestResolve_TemplateExecutionError(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("exec-fail.md", "Hello, {{.MissingMethod}}!")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)

	// Passing a type that doesn't have .MissingMethod triggers an execution error.
	_, err := repo.Resolve("exec-fail", struct{}{})
	if err == nil {
		t.Fatal("expected error for template execution failure")
	}
	if !strings.Contains(err.Error(), "execute template") {
		t.Errorf("error should mention 'execute template', got: %v", err)
	}
}

// --- ListFragments ResolveAll error ---

func TestListFragments_ResolveAllError(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	stub := &stubResolver{
		Resolver:      env.Tiers,
		resolveAllErr: errors.New("storage unavailable"),
	}
	repo := pipeline.NewPromptRepositoryWithTiers(stub)

	_, err := repo.ListFragments("rules", nil, nil)
	if err == nil {
		t.Fatal("expected error from ListFragments when ResolveAll fails")
	}
	if !strings.Contains(err.Error(), "list-fragments") {
		t.Errorf("error should mention 'list-fragments', got: %v", err)
	}
}

// --- Build: nodeDir .md file injection ---

func TestBuild_InjectsPerTaskMDContent(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Do the thing", State: state.StatusInProgress},
		},
	}

	// Write a per-task .md file in a temp directory.
	nodeDir := t.TempDir()
	mdContent := "## Detailed Plan\n\nStep 1: Read everything.\nStep 2: Write everything."
	if err := os.WriteFile(filepath.Join(nodeDir, "task-0001.md"), []byte(mdContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj/node", nodeDir, ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "Detailed Plan") {
		t.Error("per-task .md content should be injected into context")
	}
	if !strings.Contains(got, "Step 1: Read everything.") {
		t.Error("per-task .md file body should appear in output")
	}
}

func TestBuild_SkipsMissingPerTaskMD(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "No md file", State: state.StatusInProgress},
		},
	}

	// nodeDir exists but has no .md file for the task.
	nodeDir := t.TempDir()

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj/node", nodeDir, ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Should still produce valid output with task context.
	if !strings.Contains(got, "**Task:** proj/node/task-0001") {
		t.Error("task should still appear when .md file is missing")
	}
}

func TestBuild_EmptyPerTaskMDIgnored(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Empty md", State: state.StatusInProgress},
		},
	}

	nodeDir := t.TempDir()
	// Write an empty (whitespace-only) .md file.
	if err := os.WriteFile(filepath.Join(nodeDir, "task-0001.md"), []byte("   \n  "), 0o644); err != nil {
		t.Fatal(err)
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj/node", nodeDir, ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "**Task:** proj/node/task-0001") {
		t.Error("task should still render when .md file is empty")
	}
}

// --- ClassRepository.Resolve: non-ErrNotExist error on exact key ---

func TestClassResolve_NonNotExistErrorOnExactKey(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 has no effect on Windows")
	}

	env := testutil.NewEnvironment(t)
	env.Classes.Reload(map[string]config.ClassDef{
		"broken": {Description: "Has unreadable prompt"},
	})

	// Write the prompt file, then make it unreadable.
	tierDirs := env.Tiers.TierDirs()
	classDir := filepath.Join(tierDirs[0], "prompts", "classes")
	if err := os.MkdirAll(classDir, 0o755); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(classDir, "broken.md")
	if err := os.WriteFile(unreadable, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(unreadable, 0o644) }()

	_, err := env.Classes.Resolve("broken")
	if err == nil {
		t.Fatal("expected error for unreadable class prompt file")
	}
	if !strings.Contains(err.Error(), "resolve") {
		t.Errorf("error should mention 'resolve', got: %v", err)
	}
}

// --- ClassRepository.Resolve: non-ErrNotExist error on fallback parent ---

func TestClassResolve_NonNotExistErrorOnFallbackParent(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 has no effect on Windows")
	}

	env := testutil.NewEnvironment(t)
	env.Classes.Reload(map[string]config.ClassDef{
		"lang-go": {Description: "Go language tasks"},
	})

	// No exact key file (lang-go.md), but parent (lang.md) exists but
	// is unreadable, triggering the non-ErrNotExist path on fallback.
	tierDirs := env.Tiers.TierDirs()
	classDir := filepath.Join(tierDirs[0], "prompts", "classes")
	if err := os.MkdirAll(classDir, 0o755); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(classDir, "lang.md")
	if err := os.WriteFile(unreadable, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(unreadable, 0o644) }()

	_, err := env.Classes.Resolve("lang-go")
	if err == nil {
		t.Fatal("expected error for unreadable fallback class prompt")
	}
	if !strings.Contains(err.Error(), "resolve fallback") {
		t.Errorf("error should mention 'resolve fallback', got: %v", err)
	}
}

// --- ResolveAllFragments: ReadFile error on a discovered fragment ---

func TestResolveAllFragments_ReadFileError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 has no effect on Windows")
	}

	dir := t.TempDir()
	rulesDir := filepath.Join(dir, "system", "base", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	unreadable := filepath.Join(rulesDir, "broken.md")
	if err := os.WriteFile(unreadable, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(unreadable, 0o644) }()

	_, err := pipeline.ResolveAllFragments(dir, "rules", nil, nil)
	if err == nil {
		t.Fatal("expected error when fragment file is unreadable")
	}
	if !strings.Contains(err.Error(), "resolving fragments") {
		t.Errorf("error should mention 'resolving fragments', got: %v", err)
	}
}

// --- ResolveAllFragments: include references missing fragment ---

func TestResolveAllFragments_IncludeReferencesMissingFragment(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	rulesDir := filepath.Join(dir, "system", "base", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "exists.md"), []byte("here"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := pipeline.ResolveAllFragments(dir, "rules", []string{"exists.md", "ghost.md"}, nil)
	if err == nil {
		t.Fatal("expected error for include list entry not found in any tier")
	}
	if !strings.Contains(err.Error(), "ghost.md") {
		t.Errorf("error should name the missing fragment, got: %v", err)
	}
}

// --- ResolvePromptTemplate: invalid template syntax ---

func TestResolvePromptTemplate_InvalidSyntax(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	promptsDir := filepath.Join(dir, "system", "base", "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "bad.md"), []byte("{{.Broken"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := pipeline.ResolvePromptTemplate(dir, "bad.md", struct{}{})
	if err == nil {
		t.Fatal("expected error for invalid template syntax")
	}
	if !strings.Contains(err.Error(), "parsing prompt template") {
		t.Errorf("error should mention 'parsing prompt template', got: %v", err)
	}
}

// --- ResolvePromptTemplate: template execution error ---

func TestResolvePromptTemplate_ExecutionError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	promptsDir := filepath.Join(dir, "system", "base", "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "execfail.md"), []byte("{{.NoSuchField}}"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := pipeline.ResolvePromptTemplate(dir, "execfail.md", struct{}{})
	if err == nil {
		t.Fatal("expected error for template execution failure")
	}
	if !strings.Contains(err.Error(), "executing prompt template") {
		t.Errorf("error should mention 'executing prompt template', got: %v", err)
	}
}

// --- AssemblePrompt: lightweight stage with missing prompt ---

func TestAssemblePrompt_LightweightStageMissingPrompt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := config.Defaults()
	skip := true
	stage := config.PipelineStage{
		PromptFile:         "nonexistent.md",
		SkipPromptAssembly: &skip,
	}

	_, err := pipeline.AssemblePrompt(dir, cfg, stage, "")
	if err == nil {
		t.Fatal("expected error for missing lightweight stage prompt")
	}
}

// --- AssemblePrompt: fragment resolution error ---

func TestAssemblePrompt_FragmentResolutionError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 has no effect on Windows")
	}

	dir := t.TempDir()

	// Create an unreadable rule fragment so ResolveAllFragments fails.
	rulesDir := filepath.Join(dir, "system", "base", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(rulesDir, "bad.md")
	if err := os.WriteFile(unreadable, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(unreadable, 0o644) }()

	cfg := config.Defaults()
	stage := config.PipelineStage{
		PromptFile: "stages/execute.md",
	}

	_, err := pipeline.AssemblePrompt(dir, cfg, stage, "")
	if err == nil {
		t.Fatal("expected error when rule fragment is unreadable")
	}
	if !strings.Contains(err.Error(), "resolving rule fragments") {
		t.Errorf("error should mention 'resolving rule fragments', got: %v", err)
	}
}

// --- AssemblePrompt: full stage with missing stage prompt ---

func TestAssemblePrompt_FullStageMissingPrompt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// No rule fragments, but stage prompt file is missing.
	cfg := config.Defaults()
	stage := config.PipelineStage{
		PromptFile: "nonexistent.md",
	}

	_, err := pipeline.AssemblePrompt(dir, cfg, stage, "")
	if err == nil {
		t.Fatal("expected error for missing stage prompt")
	}
}

// --- BuildPlanningContext: blocked task with reason ---

func TestBuildPlanningContext_BlockedTaskWithReason(t *testing.T) {
	t.Parallel()

	ns := &state.NodeState{
		Type:  state.NodeOrchestrator,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{
				ID:            "task-0001",
				Description:   "Blocked task",
				State:         state.StatusBlocked,
				BlockedReason: "waiting on upstream",
			},
		},
	}

	got := pipeline.BuildPlanningContext("proj/orch", ns, "failure")
	if !strings.Contains(got, "blocked: waiting on upstream") {
		t.Error("blocked task reason should appear in planning context")
	}
}
