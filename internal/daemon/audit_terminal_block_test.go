package daemon

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestIsTerminalBlock_BlockedWithSuperseded(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusBlocked, BlockedReason: "Superseded by newer approach"}
	if !isTerminalBlock(task) {
		t.Error("expected true for 'superseded' reason")
	}
}

func TestIsTerminalBlock_BlockedWithAlreadyDone(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusBlocked, BlockedReason: "Already done in previous iteration"}
	if !isTerminalBlock(task) {
		t.Error("expected true for 'already done' reason")
	}
}

func TestIsTerminalBlock_BlockedWithAlreadyCompleted(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusBlocked, BlockedReason: "This was already completed by task-0003"}
	if !isTerminalBlock(task) {
		t.Error("expected true for 'already completed' reason")
	}
}

func TestIsTerminalBlock_BlockedWithNoLongerNeeded(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusBlocked, BlockedReason: "No longer needed after refactor"}
	if !isTerminalBlock(task) {
		t.Error("expected true for 'no longer needed' reason")
	}
}

func TestIsTerminalBlock_BlockedWithReplacedBy(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusBlocked, BlockedReason: "Replaced by task-0005"}
	if !isTerminalBlock(task) {
		t.Error("expected true for 'replaced by' reason")
	}
}

func TestIsTerminalBlock_BlockedWithDoneIn(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusBlocked, BlockedReason: "Done in parent node"}
	if !isTerminalBlock(task) {
		t.Error("expected true for 'done in' reason")
	}
}

func TestIsTerminalBlock_BlockedWithDoneDirectly(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusBlocked, BlockedReason: "Done directly by orchestrator"}
	if !isTerminalBlock(task) {
		t.Error("expected true for 'done directly' reason")
	}
}

func TestIsTerminalBlock_BlockedWithDecomposedInto(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusBlocked, BlockedReason: "Decomposed into subtasks"}
	if !isTerminalBlock(task) {
		t.Error("expected true for 'decomposed into' reason")
	}
}

func TestIsTerminalBlock_BlockedWithDecomposition(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusBlocked, BlockedReason: "Blocked due to decomposition of parent"}
	if !isTerminalBlock(task) {
		t.Error("expected true for 'decomposition' reason")
	}
}

func TestIsTerminalBlock_NotBlocked(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusInProgress, BlockedReason: "superseded"}
	if isTerminalBlock(task) {
		t.Error("expected false when state is not blocked")
	}
}

func TestIsTerminalBlock_BlockedGenericReason(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusBlocked, BlockedReason: "Waiting for API key"}
	if isTerminalBlock(task) {
		t.Error("expected false for a genuine blocking reason")
	}
}

func TestIsTerminalBlock_BlockedEmptyReason(t *testing.T) {
	t.Parallel()
	task := state.Task{State: state.StatusBlocked, BlockedReason: ""}
	if isTerminalBlock(task) {
		t.Error("expected false for empty reason")
	}
}
