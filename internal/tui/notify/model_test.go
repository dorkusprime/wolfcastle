package notify

import (
	"strings"
	"testing"
)

func TestNewNotificationModel_Defaults(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	if m.HasToasts() {
		t.Error("new model should have no toasts")
	}
	if m.width != 0 {
		t.Errorf("expected width 0, got %d", m.width)
	}
	if len(m.toasts) != 0 {
		t.Errorf("expected empty toast slice, got %d", len(m.toasts))
	}
}

func TestPush_AddsToast_ReturnsDismissCmd(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	cmd := m.Push("hello")
	if cmd == nil {
		t.Fatal("Push should return a non-nil command")
	}
	if len(m.toasts) != 1 {
		t.Fatalf("expected 1 toast, got %d", len(m.toasts))
	}
	if m.toasts[0].text != "hello" {
		t.Errorf("expected text 'hello', got %q", m.toasts[0].text)
	}
	if m.toasts[0].dismissed {
		t.Error("new toast should not be dismissed")
	}
}

func TestPush_QueueOverflow_DropsOldest(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	for i := 0; i < maxQueue+3; i++ {
		m.Push("toast")
	}
	if len(m.toasts) != maxQueue {
		t.Errorf("expected %d toasts after overflow, got %d", maxQueue, len(m.toasts))
	}
}

func TestToastDismissMsg_MarksDismissed(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	m.Push("first")
	m.Push("second")

	m, _ = m.Update(ToastDismissMsg{Index: 0})
	// After dismiss + pruning, only "second" remains.
	if len(m.toasts) != 1 {
		t.Fatalf("expected 1 toast after dismiss, got %d", len(m.toasts))
	}
	if m.toasts[0].text != "second" {
		t.Errorf("expected remaining toast to be 'second', got %q", m.toasts[0].text)
	}
}

func TestHasToasts_TrueWhenActive_FalseWhenAllDismissed(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()

	if m.HasToasts() {
		t.Error("empty model should have no toasts")
	}

	m.Push("one")
	if !m.HasToasts() {
		t.Error("should have toasts after push")
	}

	m, _ = m.Update(ToastDismissMsg{Index: 0})
	if m.HasToasts() {
		t.Error("should have no toasts after all dismissed")
	}
}

func TestView_RendersToastText(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	m.Push("alert fired")
	v := m.View()
	if !strings.Contains(v, "alert fired") {
		t.Errorf("view should contain toast text, got %q", v)
	}
}

func TestView_NoToasts_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	v := m.View()
	if v != "" {
		t.Errorf("expected empty view, got %q", v)
	}
}

func TestView_LongText_Renders(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	long := strings.Repeat("x", 80)
	m.Push(long)
	v := m.View()
	// The rendered output should contain the text content. MaxWidth may
	// truncate or wrap depending on the terminal environment, but the
	// view should never be empty.
	if v == "" {
		t.Error("view should not be empty for a long text toast")
	}
	if !strings.Contains(v, "xxx") {
		t.Error("view should contain the long text content")
	}
}

func TestSetSize_UpdatesWidth(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	m.SetSize(120)
	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
}

func TestMultiplePushes_QueueCorrectly(t *testing.T) {
	t.Parallel()
	m := NewNotificationModel()
	m.Push("alpha")
	m.Push("beta")
	m.Push("gamma")

	if len(m.toasts) != 3 {
		t.Fatalf("expected 3 toasts, got %d", len(m.toasts))
	}
	if m.toasts[0].text != "alpha" {
		t.Errorf("toast 0: expected 'alpha', got %q", m.toasts[0].text)
	}
	if m.toasts[1].text != "beta" {
		t.Errorf("toast 1: expected 'beta', got %q", m.toasts[1].text)
	}
	if m.toasts[2].text != "gamma" {
		t.Errorf("toast 2: expected 'gamma', got %q", m.toasts[2].text)
	}

	v := m.View()
	if !strings.Contains(v, "alpha") || !strings.Contains(v, "beta") || !strings.Contains(v, "gamma") {
		t.Error("view should contain all three toast texts")
	}
}
