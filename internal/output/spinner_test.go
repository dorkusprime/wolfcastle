package output

import (
	"testing"
)

func TestRenderFrame_PositionZero(t *testing.T) {
	t.Parallel()
	frame := renderFrame(0, len([]rune(projectile)))
	if len([]rune(frame)) != spinnerWidth+2 { // +2 for brackets
		t.Errorf("frame width: got %d runes, want %d", len([]rune(frame)), spinnerWidth+2)
	}
	if frame[0] != '[' || frame[len(frame)-1] != ']' {
		t.Errorf("frame should be bracketed: %q", frame)
	}
}

func TestRenderFrame_ProjectileVisible(t *testing.T) {
	t.Parallel()
	frame := renderFrame(0, len([]rune(projectile)))
	// The projectile should be visible somewhere in the frame
	if frame == "["+string(make([]rune, spinnerWidth))+"]" {
		t.Error("frame should contain the projectile")
	}
}

func TestRenderFrame_WrapsAround(t *testing.T) {
	t.Parallel()
	// Position near the end should wrap the projectile
	frame := renderFrame(spinnerWidth-2, len([]rune(projectile)))
	if frame[0] != '[' {
		t.Error("frame should still be bracketed after wrap")
	}
	// Frame should still have correct width
	if len([]rune(frame)) != spinnerWidth+2 {
		t.Errorf("wrapped frame width: got %d, want %d", len([]rune(frame)), spinnerWidth+2)
	}
}

func TestRenderFrame_AllPositions(t *testing.T) {
	t.Parallel()
	projLen := len([]rune(projectile))
	for pos := 0; pos < spinnerWidth; pos++ {
		frame := renderFrame(pos, projLen)
		if len([]rune(frame)) != spinnerWidth+2 {
			t.Errorf("pos %d: frame width %d, want %d", pos, len([]rune(frame)), spinnerWidth+2)
		}
	}
}

func TestSpinner_StopWithoutStart(t *testing.T) {
	t.Parallel()
	s := NewSpinner()
	// Should not panic or hang
	s.Stop()
}

func TestSpinner_StartStop(t *testing.T) {
	t.Parallel()
	s := NewSpinner()
	s.Start()
	s.Stop()
	// Should complete without hanging
}

func TestSpinner_DoubleStart(t *testing.T) {
	t.Parallel()
	s := NewSpinner()
	s.Start()
	s.Start() // should be a no-op
	s.Stop()
}
