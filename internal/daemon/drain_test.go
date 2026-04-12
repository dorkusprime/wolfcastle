package daemon

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ═══════════════════════════════════════════════════════════════════════════
// Repository drain file operations
// ═══════════════════════════════════════════════════════════════════════════

func TestDrainFile_WriteHasRemove(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	repo := d.repo()

	if repo.HasDrainFile() {
		t.Error("drain file should not exist initially")
	}

	if err := repo.WriteDrainFile(); err != nil {
		t.Fatalf("WriteDrainFile: %v", err)
	}
	if !repo.HasDrainFile() {
		t.Error("drain file should exist after write")
	}

	if err := repo.RemoveDrainFile(); err != nil {
		t.Fatalf("RemoveDrainFile: %v", err)
	}
	if repo.HasDrainFile() {
		t.Error("drain file should not exist after remove")
	}
}

func TestRemoveDrainFile_NoFile(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)

	if err := d.repo().RemoveDrainFile(); err != nil {
		t.Fatalf("RemoveDrainFile on missing file should return nil: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RunOnce picks up drain file and sets flag
// ═══════════════════════════════════════════════════════════════════════════

func TestRunOnce_DrainFileSetsDrainingFlag(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	_ = d.Logger.StartIteration()
	defer d.Logger.Close()

	// Write a drain file.
	if err := d.repo().WriteDrainFile(); err != nil {
		t.Fatalf("writing drain file: %v", err)
	}

	// Set up a minimal empty tree.
	idx := state.NewRootIndex()
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	result, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce error: %v", err)
	}

	if !d.draining {
		t.Error("expected draining=true after drain file detected")
	}
	if d.repo().HasDrainFile() {
		t.Error("drain file should be removed after detection")
	}

	// Empty tree, no work: returns NoWork. Drain exit happens in Run loop.
	if result != IterationNoWork {
		t.Errorf("expected IterationNoWork, got %v", result)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Run loop exits on NoWork when draining
// ═══════════════════════════════════════════════════════════════════════════

func TestRun_DrainExitsOnNoWork(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Git.VerifyBranch = false

	// Empty tree, no work.
	idx := state.NewRootIndex()
	writeJSON(t, filepath.Join(d.Store.Dir(), "state.json"), idx)

	// Write drain file before starting.
	if err := d.repo().WriteDrainFile(); err != nil {
		t.Fatalf("writing drain file: %v", err)
	}

	err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !d.draining {
		t.Error("expected draining=true after Run")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Run loop exits after work when draining
// ═══════════════════════════════════════════════════════════════════════════

func TestRun_DrainExitsAfterWork(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.Config.Pipeline.Planning.Enabled = true
	d.Config.Pipeline.Planning.Model = "echo"
	d.Config.Models["echo"] = config.ModelDef{Command: "echo", Args: []string{"WOLFCASTLE_COMPLETE"}}
	d.Config.Git.VerifyBranch = false

	projDir := d.Store.Dir()

	// Set up an orchestrator that needs planning (will count as work).
	ns := state.NewNodeState("orch", "Orch", state.NodeOrchestrator)
	ns.NeedsPlanning = true
	ns.State = state.StatusNotStarted
	writeJSON(t, filepath.Join(projDir, "orch", "state.json"), ns)

	idx := state.NewRootIndex()
	idx.Root = []string{"orch"}
	idx.Nodes["orch"] = state.IndexEntry{
		Name: "Orch", Type: state.NodeOrchestrator, State: state.StatusNotStarted, Address: "orch",
	}
	writeJSON(t, filepath.Join(projDir, "state.json"), idx)

	writePromptFile(t, d.WolfcastleDir, "stages/plan-initial.md")

	// Write drain file.
	if err := d.repo().WriteDrainFile(); err != nil {
		t.Fatalf("writing drain file: %v", err)
	}

	err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !d.draining {
		t.Error("expected draining=true")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Parallel fillSlots respects draining
// ═══════════════════════════════════════════════════════════════════════════

func TestFillSlots_SkipsWhenDraining(t *testing.T) {
	t.Parallel()
	d := testDaemon(t)
	d.draining = true
	pd := NewParallelDispatcher(d, 4)
	d.dispatcher = pd

	idx := state.NewRootIndex()
	launched := pd.fillSlots(context.Background(), idx)
	if launched != 0 {
		t.Errorf("fillSlots should return 0 when draining, got %d", launched)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// getDaemonStatus with drain file
// ═══════════════════════════════════════════════════════════════════════════

func TestDrainFileStatus(t *testing.T) {
	d := testDaemon(t)
	repo := d.repo()

	// Register the current process in the instance registry so IsAlive returns true.
	regDir := t.TempDir()
	old := instance.RegistryDirOverride
	instance.RegistryDirOverride = regDir
	defer func() { instance.RegistryDirOverride = old }()

	if err := instance.Register(d.RepoDir, "test-branch"); err != nil {
		t.Fatalf("instance.Register: %v", err)
	}
	defer func() { _ = instance.Deregister(d.RepoDir) }()

	if repo.HasDrainFile() {
		t.Error("drain file should not exist")
	}

	if err := repo.WriteDrainFile(); err != nil {
		t.Fatalf("writing drain file: %v", err)
	}

	if !repo.HasDrainFile() {
		t.Error("drain file should exist")
	}

	// IsAlive should still return true (drain doesn't kill the process).
	if !repo.IsAlive() {
		t.Error("daemon should still be alive while draining")
	}
}
