package tui

import (
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/instance"
)

func TestGlyphForStatus_Complete(t *testing.T) {
	t.Parallel()
	g := GlyphForStatus("complete")
	if g == "" || g == "?" {
		t.Errorf("expected a glyph for complete, got %q", g)
	}
}

func TestGlyphForStatus_InProgress(t *testing.T) {
	t.Parallel()
	g := GlyphForStatus("in_progress")
	if g == "" || g == "?" {
		t.Errorf("expected a glyph for in_progress, got %q", g)
	}
}

func TestGlyphForStatus_NotStarted(t *testing.T) {
	t.Parallel()
	g := GlyphForStatus("not_started")
	if g == "" || g == "?" {
		t.Errorf("expected a glyph for not_started, got %q", g)
	}
}

func TestGlyphForStatus_Blocked(t *testing.T) {
	t.Parallel()
	g := GlyphForStatus("blocked")
	if g == "" || g == "?" {
		t.Errorf("expected a glyph for blocked, got %q", g)
	}
}

func TestGlyphForStatus_Unknown(t *testing.T) {
	t.Parallel()
	g := GlyphForStatus("nonexistent_status")
	if g != "?" {
		t.Errorf("expected '?' for unknown status, got %q", g)
	}
}

func TestGlyphForAuditStatus_Passed(t *testing.T) {
	t.Parallel()
	g := GlyphForAuditStatus("passed")
	if g == "" || g == "?" {
		t.Errorf("expected a glyph for passed, got %q", g)
	}
}

func TestGlyphForAuditStatus_InProgress(t *testing.T) {
	t.Parallel()
	g := GlyphForAuditStatus("in_progress")
	if g == "" || g == "?" {
		t.Errorf("expected a glyph for in_progress, got %q", g)
	}
}

func TestGlyphForAuditStatus_Pending(t *testing.T) {
	t.Parallel()
	g := GlyphForAuditStatus("pending")
	if g == "" || g == "?" {
		t.Errorf("expected a glyph for pending, got %q", g)
	}
}

func TestGlyphForAuditStatus_Failed(t *testing.T) {
	t.Parallel()
	g := GlyphForAuditStatus("failed")
	if g == "" || g == "?" {
		t.Errorf("expected a glyph for failed, got %q", g)
	}
}

func TestGlyphForAuditStatus_Unknown(t *testing.T) {
	t.Parallel()
	g := GlyphForAuditStatus("nonexistent_status")
	if g != "?" {
		t.Errorf("expected '?' for unknown audit status, got %q", g)
	}
}

func TestNodeStatusGlyphs_AllFourStatuses(t *testing.T) {
	t.Parallel()
	expected := []string{"complete", "in_progress", "not_started", "blocked"}
	for _, status := range expected {
		sg, ok := NodeStatusGlyphs[status]
		if !ok {
			t.Errorf("NodeStatusGlyphs missing key %q", status)
			continue
		}
		if sg.Glyph == "" {
			t.Errorf("NodeStatusGlyphs[%q].Glyph is empty", status)
		}
		if sg.Color == nil {
			t.Errorf("NodeStatusGlyphs[%q].Color is nil", status)
		}
	}
	if len(NodeStatusGlyphs) != 4 {
		t.Errorf("expected exactly 4 entries in NodeStatusGlyphs, got %d", len(NodeStatusGlyphs))
	}
}

func TestAuditStatusGlyphs_AllFourStatuses(t *testing.T) {
	t.Parallel()
	expected := []string{"passed", "in_progress", "pending", "failed"}
	for _, status := range expected {
		sg, ok := AuditStatusGlyphs[status]
		if !ok {
			t.Errorf("AuditStatusGlyphs missing key %q", status)
			continue
		}
		if sg.Glyph == "" {
			t.Errorf("AuditStatusGlyphs[%q].Glyph is empty", status)
		}
		if sg.Color == nil {
			t.Errorf("AuditStatusGlyphs[%q].Color is nil", status)
		}
	}
	if len(AuditStatusGlyphs) != 4 {
		t.Errorf("expected exactly 4 entries in AuditStatusGlyphs, got %d", len(AuditStatusGlyphs))
	}
}

func TestGlyphForStatus_AllStatusesReturnNonEmpty(t *testing.T) {
	t.Parallel()
	for status := range NodeStatusGlyphs {
		g := GlyphForStatus(status)
		if g == "" {
			t.Errorf("GlyphForStatus(%q) returned empty string", status)
		}
	}
}

func TestGlyphForAuditStatus_AllStatusesReturnNonEmpty(t *testing.T) {
	t.Parallel()
	for status := range AuditStatusGlyphs {
		g := GlyphForAuditStatus(status)
		if g == "" {
			t.Errorf("GlyphForAuditStatus(%q) returned empty string", status)
		}
	}
}

func TestInstanceLabel_DirAndBranchMatch(t *testing.T) {
	t.Parallel()
	inst := instance.Entry{
		PID:      42,
		Worktree: "/path/to/main",
		Branch:   "main",
	}
	if got := InstanceLabel(inst); got != "main" {
		t.Fatalf("expected 'main', got %q", got)
	}
}

func TestInstanceLabel_DirDiffersFromBranch(t *testing.T) {
	t.Parallel()
	inst := instance.Entry{
		PID:      42,
		Worktree: "/path/to/wc-tui-test",
		Branch:   "feat/tui",
	}
	if got := InstanceLabel(inst); got != "wc-tui-test (feat/tui)" {
		t.Fatalf("expected 'wc-tui-test (feat/tui)', got %q", got)
	}
}

func TestInstanceLabel_EmptyBranchUsesDir(t *testing.T) {
	t.Parallel()
	inst := instance.Entry{
		PID:      42,
		Worktree: "/path/to/somewhere",
		Branch:   "",
	}
	if got := InstanceLabel(inst); got != "somewhere" {
		t.Fatalf("expected 'somewhere', got %q", got)
	}
}

func TestInstanceLabel_EmptyWorktreeFallbackBranch(t *testing.T) {
	t.Parallel()
	inst := instance.Entry{
		PID:      42,
		Worktree: "",
		Branch:   "feat/something",
	}
	if got := InstanceLabel(inst); got != "feat/something" {
		t.Fatalf("expected 'feat/something', got %q", got)
	}
}

func TestInstanceLabel_EmptyWorktreeAndBranchFallbackPID(t *testing.T) {
	t.Parallel()
	inst := instance.Entry{
		PID:      1234,
		Worktree: "",
		Branch:   "",
	}
	if got := InstanceLabel(inst); got != "pid:1234" {
		t.Fatalf("expected 'pid:1234', got %q", got)
	}
}

func TestInstanceLabel_DotWorktreeFallback(t *testing.T) {
	t.Parallel()
	// filepath.Base(".") == ".", which triggers the fallback branch.
	inst := instance.Entry{
		PID:      99,
		Worktree: ".",
		Branch:   "main",
	}
	if got := InstanceLabel(inst); got != "main" {
		t.Fatalf("expected 'main', got %q", got)
	}
}
