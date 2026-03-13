package state

import (
	"testing"
)

func TestRecomputeState_AllNotStarted(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusNotStarted},
		{ID: "b", State: StatusNotStarted},
	}
	if got := RecomputeState(children); got != StatusNotStarted {
		t.Errorf("expected not_started, got %s", got)
	}
}

func TestRecomputeState_AllComplete(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusComplete},
		{ID: "b", State: StatusComplete},
	}
	if got := RecomputeState(children); got != StatusComplete {
		t.Errorf("expected complete, got %s", got)
	}
}

func TestRecomputeState_Mixed(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusComplete},
		{ID: "b", State: StatusNotStarted},
		{ID: "c", State: StatusInProgress},
	}
	if got := RecomputeState(children); got != StatusInProgress {
		t.Errorf("expected in_progress, got %s", got)
	}
}

func TestRecomputeState_AllNonCompleteBlocked(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusComplete},
		{ID: "b", State: StatusBlocked},
		{ID: "c", State: StatusBlocked},
	}
	if got := RecomputeState(children); got != StatusBlocked {
		t.Errorf("expected blocked, got %s", got)
	}
}

func TestRecomputeState_OneBlockedOthersAvailable(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusBlocked},
		{ID: "b", State: StatusNotStarted},
	}
	if got := RecomputeState(children); got != StatusInProgress {
		t.Errorf("expected in_progress, got %s", got)
	}
}

func TestRecomputeState_EmptyChildren(t *testing.T) {
	t.Parallel()
	if got := RecomputeState(nil); got != StatusNotStarted {
		t.Errorf("expected not_started for empty, got %s", got)
	}
}

func TestRecomputeState_AllBlocked(t *testing.T) {
	t.Parallel()
	children := []ChildRef{
		{ID: "a", State: StatusBlocked},
		{ID: "b", State: StatusBlocked},
	}
	if got := RecomputeState(children); got != StatusBlocked {
		t.Errorf("expected blocked, got %s", got)
	}
}
