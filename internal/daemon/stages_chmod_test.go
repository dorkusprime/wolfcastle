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

// ── runIntakeStage — SaveInbox error after intake ───────────────────

func TestRunIntakeStage_SaveInboxError_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test not reliable on Windows")
	}

	d := testDaemon(t)
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"filed output"}}
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()
	writePromptFile(t, d.WolfcastleDir, "intake.md")

	inboxPath := filepath.Join(d.Resolver.ProjectsDir(), "inbox.json")
	writeJSON(t, inboxPath, &state.InboxFile{Items: []state.InboxItem{
		{Status: "new", Text: "item to process", Timestamp: "2025-01-01T00:00:00Z"},
	}})

	// Lock the projects dir so SaveInbox (atomicWriteJSON) fails.
	projDir := d.Resolver.ProjectsDir()
	_ = os.Chmod(projDir, 0555)
	t.Cleanup(func() { _ = os.Chmod(projDir, 0755) })

	stage := config.PipelineStage{Name: "intake", Model: "echo", PromptFile: "intake.md"}
	err := d.runIntakeStage(context.Background(), stage)
	if err == nil {
		t.Error("expected error when SaveInbox fails after intake")
	}
}
