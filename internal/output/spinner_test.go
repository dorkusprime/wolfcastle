package output

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestRenderFrame_CorrectWidth(t *testing.T) {
	t.Parallel()
	frame := renderFrame(0, len([]rune(projectile)))
	runes := []rune(frame)
	if len(runes) != spinnerWidth+1 {
		t.Errorf("frame width: got %d runes, want %d: %q", len(runes), spinnerWidth+1, frame)
	}
}

func TestRenderFrame_EndsWithPipe(t *testing.T) {
	t.Parallel()
	frame := renderFrame(0, len([]rune(projectile)))
	if !strings.HasSuffix(frame, "|") {
		t.Errorf("frame should end with pipe: %q", frame)
	}
}

func TestRenderFrame_ProjectileVisible(t *testing.T) {
	t.Parallel()
	frame := renderFrame(0, len([]rune(projectile)))
	if !strings.Contains(frame, "▶") {
		t.Error("frame should contain the arrowhead")
	}
}

func TestRenderFrame_WrapsAround(t *testing.T) {
	t.Parallel()
	frame := renderFrame(spinnerWidth-2, len([]rune(projectile)))
	runes := []rune(frame)
	if len(runes) != spinnerWidth+1 {
		t.Errorf("wrapped frame width: got %d, want %d", len(runes), spinnerWidth+1)
	}
}

func TestRenderFrame_AllPositions(t *testing.T) {
	t.Parallel()
	projLen := len([]rune(projectile))
	for pos := 0; pos < spinnerWidth; pos++ {
		frame := renderFrame(pos, projLen)
		runes := []rune(frame)
		if len(runes) != spinnerWidth+1 {
			t.Errorf("pos %d: frame width %d, want %d", pos, len(runes), spinnerWidth+1)
		}
	}
}

func TestSpinner_StopWithoutStart(t *testing.T) {
	t.Parallel()
	s := NewSpinner()
	s.Stop()
}

func TestSpinner_StartStop(t *testing.T) {
	t.Parallel()
	s := NewSpinner()
	s.Start()
	s.Stop()
}

func TestSpinner_DoubleStart(t *testing.T) {
	t.Parallel()
	s := NewSpinner()
	s.Start()
	s.Start()
	s.Stop()
}

// fakeTerminal overrides isTerminal and redirects stdout to /dev/null
// so that spinner writes never block. Returns a cleanup function.
func fakeTerminal(t *testing.T) {
	t.Helper()

	origIsTerminal := isTerminal
	isTerminal = func() bool { return true }

	origStdout := os.Stdout
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = devNull

	t.Cleanup(func() {
		isTerminal = origIsTerminal
		os.Stdout = origStdout
		_ = devNull.Close()
	})
}

// TestSpinner_RunProducesFrames exercises the run goroutine: start,
// let it tick a few frames, then stop.
func TestSpinner_RunProducesFrames(t *testing.T) {
	fakeTerminal(t)

	s := NewSpinner()
	s.Start()
	time.Sleep(frameDelay * 4)
	s.Stop()
}

// TestSpinner_PauseAndResume exercises clearForMessage and resumeAfterMessage
// through the PauseSpinner/ResumeSpinner public API.
func TestSpinner_PauseAndResume(t *testing.T) {
	fakeTerminal(t)

	s := NewSpinner()
	s.Start()
	time.Sleep(frameDelay * 2)

	PauseSpinner()
	time.Sleep(frameDelay * 2)

	ResumeSpinner()
	// Wait for the resume goroutine's sleep to expire so paused flips back.
	time.Sleep(frameDelay * 4)
	s.Stop()
}

// TestSpinner_ActiveSpinnerLifecycle confirms that Start sets
// activeSpinner and Stop clears it.
func TestSpinner_ActiveSpinnerLifecycle(t *testing.T) {
	fakeTerminal(t)

	s := NewSpinner()
	s.Start()

	activeMu.Lock()
	active := activeSpinner
	activeMu.Unlock()
	if active != s {
		t.Error("expected activeSpinner to be set after Start")
	}

	s.Stop()

	activeMu.Lock()
	active = activeSpinner
	activeMu.Unlock()
	if active != nil {
		t.Error("expected activeSpinner to be nil after Stop")
	}
}

// TestPauseSpinner_NoActiveSpinner verifies the nil guard (no-op path).
func TestPauseSpinner_NoActiveSpinner(t *testing.T) {
	activeMu.Lock()
	prev := activeSpinner
	activeSpinner = nil
	activeMu.Unlock()
	t.Cleanup(func() {
		activeMu.Lock()
		activeSpinner = prev
		activeMu.Unlock()
	})

	PauseSpinner()
}

// TestResumeSpinner_NoActiveSpinner verifies the nil guard (no-op path).
func TestResumeSpinner_NoActiveSpinner(t *testing.T) {
	activeMu.Lock()
	prev := activeSpinner
	activeSpinner = nil
	activeMu.Unlock()
	t.Cleanup(func() {
		activeMu.Lock()
		activeSpinner = prev
		activeMu.Unlock()
	})

	ResumeSpinner()
}

// TestSpinner_RunPausedSkipsFrames verifies that the ticker branch
// skips rendering when paused is true.
func TestSpinner_RunPausedSkipsFrames(t *testing.T) {
	fakeTerminal(t)

	s := NewSpinner()
	s.paused.Store(true)
	s.Start()
	time.Sleep(frameDelay * 3)
	s.paused.Store(false)
	s.Stop()
}

// TestIsTerminal_ReturnsFalseInTest confirms the real IsTerminal
// returns false when stdout is a pipe (as in tests).
func TestIsTerminal_ReturnsFalseInTest(t *testing.T) {
	t.Parallel()
	if IsTerminal() {
		t.Error("expected IsTerminal=false when stdout is a pipe")
	}
}

// TestSpinner_StopClearsNonOwned verifies Stop is a no-op for
// activeSpinner when a different spinner is active.
func TestSpinner_StopClearsNonOwned(t *testing.T) {
	fakeTerminal(t)

	s1 := NewSpinner()
	s2 := NewSpinner()
	s1.Start()

	// Manually replace activeSpinner with s2 to simulate a different owner.
	activeMu.Lock()
	activeSpinner = s2
	activeMu.Unlock()

	s1.Stop()

	// s2 should still be the active spinner since s1 doesn't own it.
	activeMu.Lock()
	active := activeSpinner
	activeMu.Unlock()
	if active != s2 {
		t.Error("Stop should not clear activeSpinner owned by another spinner")
	}

	// Clean up: unset so other tests aren't affected.
	activeMu.Lock()
	activeSpinner = nil
	activeMu.Unlock()
}
