package daemon

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ── runExpandStage — SaveInbox error after expand ───────────────────

func TestRunExpandStage_SaveInboxError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	d := testDaemon(t)
	d.Config.Models["echo"] = config.ModelDef{Command: "printf", Args: []string{"## Item 1\\nExpanded text"}}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "expand.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item to expand", Timestamp: "2025-01-01T00:00:00Z"},
	}})

	// Lock the projects dir so SaveInbox (atomicWriteJSON) fails.
	projDir := d.Resolver.ProjectsDir()
	_ = os.Chmod(projDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(projDir, 0755) })

	stage := config.PipelineStage{Name: "expand", Model: "echo", PromptFile: "expand.md"}
	err := d.runExpandStage(context.Background(), stage)
	if err == nil {
		t.Error("expected error when SaveInbox fails after expand")
	}
}

// ── runFileStage — SaveInbox error after file ───────────────────────

func TestRunFileStage_SaveInboxError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	d := testDaemon(t)
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"filed output"}}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "file.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "expanded", Text: "item to file", Expanded: "expanded text", Timestamp: "2025-01-01T00:00:00Z"},
	}})

	// Lock the projects dir so SaveInbox (atomicWriteJSON) fails.
	projDir := d.Resolver.ProjectsDir()
	_ = os.Chmod(projDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(projDir, 0755) })

	stage := config.PipelineStage{Name: "file", Model: "echo", PromptFile: "file.md"}
	err := d.runFileStage(context.Background(), stage)
	if err == nil {
		t.Error("expected error when SaveInbox fails after file stage")
	}
}
