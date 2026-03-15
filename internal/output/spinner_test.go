package output

import (
	"strings"
	"testing"
)

func TestRenderFrame_CorrectWidth(t *testing.T) {
	t.Parallel()
	frame := renderFrame(0, len([]rune(projectile)))
	// Frame is: track content + "|" suffix (no left bracket)
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
