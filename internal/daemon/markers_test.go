package daemon

import (
	"strings"
	"testing"
)

func TestParseMarkers_Breadcrumb(t *testing.T) {
	t.Parallel()
	var got string
	ParseMarkers("WOLFCASTLE_BREADCRUMB: step one done", MarkerCallbacks{
		OnBreadcrumb: func(text string) { got = text },
	})
	if got != "step one done" {
		t.Errorf("expected 'step one done', got %q", got)
	}
}

func TestParseMarkers_BreadcrumbEmpty(t *testing.T) {
	t.Parallel()
	called := false
	ParseMarkers("WOLFCASTLE_BREADCRUMB:  ", MarkerCallbacks{
		OnBreadcrumb: func(text string) { called = true },
	})
	if called {
		t.Error("empty breadcrumb should not invoke callback")
	}
}

func TestParseMarkers_BreadcrumbNilCallback(t *testing.T) {
	t.Parallel()
	// Should not panic when callback is nil
	ParseMarkers("WOLFCASTLE_BREADCRUMB: some text", MarkerCallbacks{})
}

func TestParseMarkers_Gap(t *testing.T) {
	t.Parallel()
	var got string
	ParseMarkers("WOLFCASTLE_GAP: missing error handling", MarkerCallbacks{
		OnGap: func(desc string) { got = desc },
	})
	if got != "missing error handling" {
		t.Errorf("expected 'missing error handling', got %q", got)
	}
}

func TestParseMarkers_GapEmpty(t *testing.T) {
	t.Parallel()
	called := false
	ParseMarkers("WOLFCASTLE_GAP:  ", MarkerCallbacks{
		OnGap: func(desc string) { called = true },
	})
	if called {
		t.Error("empty gap should not invoke callback")
	}
}

func TestParseMarkers_FixGap(t *testing.T) {
	t.Parallel()
	var got string
	ParseMarkers("WOLFCASTLE_FIX_GAP: gap-n1-3", MarkerCallbacks{
		OnFixGap: func(gapID string) { got = gapID },
	})
	if got != "gap-n1-3" {
		t.Errorf("expected 'gap-n1-3', got %q", got)
	}
}

func TestParseMarkers_Scope(t *testing.T) {
	t.Parallel()
	var got string
	ParseMarkers("WOLFCASTLE_SCOPE: refactor auth module", MarkerCallbacks{
		OnScope: func(desc string) { got = desc },
	})
	if got != "refactor auth module" {
		t.Errorf("expected 'refactor auth module', got %q", got)
	}
}

func TestParseMarkers_ScopeEmpty(t *testing.T) {
	t.Parallel()
	called := false
	ParseMarkers("WOLFCASTLE_SCOPE:  ", MarkerCallbacks{
		OnScope: func(desc string) { called = true },
	})
	if called {
		t.Error("empty scope should not invoke callback")
	}
}

func TestParseMarkers_ScopeFiles(t *testing.T) {
	t.Parallel()
	var got string
	ParseMarkers("WOLFCASTLE_SCOPE_FILES: auth.go|login.go", MarkerCallbacks{
		OnScopeFiles: func(raw string) { got = raw },
	})
	if got != "auth.go|login.go" {
		t.Errorf("expected 'auth.go|login.go', got %q", got)
	}
}

func TestParseMarkers_ScopeSystems(t *testing.T) {
	t.Parallel()
	var got string
	ParseMarkers("WOLFCASTLE_SCOPE_SYSTEMS: api|database", MarkerCallbacks{
		OnScopeSystems: func(raw string) { got = raw },
	})
	if got != "api|database" {
		t.Errorf("expected 'api|database', got %q", got)
	}
}

func TestParseMarkers_ScopeCriteria(t *testing.T) {
	t.Parallel()
	var got string
	ParseMarkers("WOLFCASTLE_SCOPE_CRITERIA: tests pass|lint clean", MarkerCallbacks{
		OnScopeCriteria: func(raw string) { got = raw },
	})
	if got != "tests pass|lint clean" {
		t.Errorf("expected 'tests pass|lint clean', got %q", got)
	}
}

func TestParseMarkers_Summary(t *testing.T) {
	t.Parallel()
	var got string
	ParseMarkers("WOLFCASTLE_SUMMARY: all tests pass now", MarkerCallbacks{
		OnSummary: func(text string) { got = text },
	})
	if got != "all tests pass now" {
		t.Errorf("expected 'all tests pass now', got %q", got)
	}
}

func TestParseMarkers_SummaryEmpty(t *testing.T) {
	t.Parallel()
	called := false
	ParseMarkers("WOLFCASTLE_SUMMARY:  ", MarkerCallbacks{
		OnSummary: func(text string) { called = true },
	})
	if called {
		t.Error("empty summary should not invoke callback")
	}
}

func TestParseMarkers_ResolveEsc(t *testing.T) {
	t.Parallel()
	var got string
	ParseMarkers("WOLFCASTLE_RESOLVE_ESCALATION: esc-42", MarkerCallbacks{
		OnResolveEsc: func(id string) { got = id },
	})
	if got != "esc-42" {
		t.Errorf("expected 'esc-42', got %q", got)
	}
}

func TestParseMarkers_Complete(t *testing.T) {
	t.Parallel()
	called := false
	ParseMarkers("some output\nWOLFCASTLE_COMPLETE\nmore", MarkerCallbacks{
		OnComplete: func() { called = true },
	})
	if !called {
		t.Error("expected OnComplete callback")
	}
}

func TestParseMarkers_Blocked(t *testing.T) {
	t.Parallel()
	var got string
	ParseMarkers("WOLFCASTLE_BLOCKED: dependency missing", MarkerCallbacks{
		OnBlocked: func(reason string) { got = reason },
	})
	if got != "dependency missing" {
		t.Errorf("expected 'dependency missing', got %q", got)
	}
}

func TestParseMarkers_BlockedNoReason(t *testing.T) {
	t.Parallel()
	called := false
	ParseMarkers("WOLFCASTLE_BLOCKED", MarkerCallbacks{
		OnBlocked: func(reason string) { called = true },
	})
	if !called {
		t.Error("expected OnBlocked callback even without reason")
	}
}

func TestParseMarkers_Yield(t *testing.T) {
	t.Parallel()
	called := false
	ParseMarkers("WOLFCASTLE_YIELD", MarkerCallbacks{
		OnYield: func() { called = true },
	})
	if !called {
		t.Error("expected OnYield callback")
	}
}

func TestParseMarkers_MultipleMarkers(t *testing.T) {
	t.Parallel()
	var breadcrumbs []string
	var gapCount int
	var summary string
	input := strings.Join([]string{
		"WOLFCASTLE_BREADCRUMB: first",
		"random text",
		"WOLFCASTLE_BREADCRUMB: second",
		"WOLFCASTLE_GAP: missing coverage",
		"WOLFCASTLE_SUMMARY: done",
	}, "\n")

	ParseMarkers(input, MarkerCallbacks{
		OnBreadcrumb: func(text string) { breadcrumbs = append(breadcrumbs, text) },
		OnGap:        func(desc string) { gapCount++ },
		OnSummary:    func(text string) { summary = text },
	})

	if len(breadcrumbs) != 2 {
		t.Errorf("expected 2 breadcrumbs, got %d", len(breadcrumbs))
	}
	if gapCount != 1 {
		t.Errorf("expected 1 gap, got %d", gapCount)
	}
	if summary != "done" {
		t.Errorf("expected summary 'done', got %q", summary)
	}
}

func TestParseMarkers_NoMarkers(t *testing.T) {
	t.Parallel()
	called := false
	ParseMarkers("just some plain text\nno markers here", MarkerCallbacks{
		OnBreadcrumb: func(text string) { called = true },
		OnGap:        func(desc string) { called = true },
		OnComplete:   func() { called = true },
	})
	if called {
		t.Error("no callbacks should fire for text without markers")
	}
}

func TestParseMarkers_EmptyInput(t *testing.T) {
	t.Parallel()
	called := false
	ParseMarkers("", MarkerCallbacks{
		OnComplete: func() { called = true },
	})
	if called {
		t.Error("no callbacks should fire for empty input")
	}
}

func TestParseMarkers_ScopeBeforeScopeFiles(t *testing.T) {
	t.Parallel()
	// WOLFCASTLE_SCOPE_FILES should not match WOLFCASTLE_SCOPE due to
	// ordering in the switch — SCOPE_FILES has a longer prefix.
	var scopeDesc string
	var filesRaw string
	input := "WOLFCASTLE_SCOPE: auth\nWOLFCASTLE_SCOPE_FILES: a.go|b.go"
	ParseMarkers(input, MarkerCallbacks{
		OnScope:      func(desc string) { scopeDesc = desc },
		OnScopeFiles: func(raw string) { filesRaw = raw },
	})
	if scopeDesc != "auth" {
		t.Errorf("expected scope 'auth', got %q", scopeDesc)
	}
	if filesRaw != "a.go|b.go" {
		t.Errorf("expected files 'a.go|b.go', got %q", filesRaw)
	}
}
