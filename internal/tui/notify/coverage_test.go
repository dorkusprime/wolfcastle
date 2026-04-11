package notify

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Push truncation paths
// ---------------------------------------------------------------------------

func TestPush_LongTextWithColon_FrontTruncatesValue(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	// Build a string like "Label: <very long value>"
	label := "Copied"
	value := strings.Repeat("x", 100)
	text := label + ": " + value
	m.Push(text)
	if len(m.toasts) != 1 {
		t.Fatalf("expected 1 toast, got %d", len(m.toasts))
	}
	got := m.toasts[0].text
	if !strings.HasPrefix(got, "Copied: ...") {
		t.Errorf("expected front-truncated value with '...', got %q", got)
	}
	if len(got) > maxWidth {
		t.Errorf("expected text within maxWidth, got length %d", len(got))
	}
}

func TestPush_LongTextWithoutColon_FrontTruncates(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	text := strings.Repeat("y", 100)
	m.Push(text)
	got := m.toasts[0].text
	if !strings.HasPrefix(got, "...") {
		t.Errorf("expected front-truncation with '...', got %q", got)
	}
	// The tail should be preserved
	if !strings.HasSuffix(got, "yyy") {
		t.Errorf("expected tail preserved, got %q", got)
	}
}

func TestPush_ExactLimitLength_NoTruncation(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	limit := maxWidth - 5
	text := strings.Repeat("z", limit)
	m.Push(text)
	got := m.toasts[0].text
	if got != text {
		t.Errorf("text at exact limit should not be truncated, got length %d", len(got))
	}
}

func TestPush_ColonNearEnd_FallsBackToPlainTruncation(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	// Colon is beyond the limit, so it won't match the idx < limit check
	text := strings.Repeat("a", 60) + ": short"
	m.Push(text)
	got := m.toasts[0].text
	if !strings.HasPrefix(got, "...") {
		t.Errorf("expected fallback truncation, got %q", got)
	}
}

func TestUpdate_UnhandledMsg_PassesThrough(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	m.Push("test")
	type customMsg struct{}
	m, cmd := m.Update(customMsg{})
	if cmd != nil {
		t.Error("expected nil cmd for unhandled message")
	}
	if len(m.toasts) != 1 {
		t.Errorf("toast should be unchanged, got %d", len(m.toasts))
	}
}

func TestDismiss_NonExistentID_NoPanic(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	m.Push("test")
	m, _ = m.Update(ToastDismissMsg{ID: 999})
	if len(m.toasts) != 1 {
		t.Errorf("expected toast to remain, got %d", len(m.toasts))
	}
}

func TestView_DismissedToasts_Excluded(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	m.Push("first")
	m.Push("second")
	// Dismiss the first
	m.toasts[0].dismissed = true
	v := m.View()
	if strings.Contains(v, "first") {
		t.Error("dismissed toast should not appear in view")
	}
	if !strings.Contains(v, "second") {
		t.Error("active toast should appear in view")
	}
}
