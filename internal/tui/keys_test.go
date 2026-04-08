package tui

import (
	"testing"
)

func TestGlobalKeyMap_BindingsNonEmpty(t *testing.T) {
	t.Parallel()
	bindings := []struct {
		name string
		keys []string
	}{
		{"Quit", GlobalKeyMap.Quit.Keys()},
		{"ForceQuit", GlobalKeyMap.ForceQuit.Keys()},
		{"ToggleTree", GlobalKeyMap.ToggleTree.Keys()},
		{"CycleFocus", GlobalKeyMap.CycleFocus.Keys()},
		{"Refresh", GlobalKeyMap.Refresh.Keys()},
		{"ToggleHelp", GlobalKeyMap.ToggleHelp.Keys()},
		{"Search", GlobalKeyMap.Search.Keys()},
		{"Copy", GlobalKeyMap.Copy.Keys()},
	}
	for _, b := range bindings {
		if len(b.keys) == 0 {
			t.Errorf("GlobalKeyMap.%s has no keys bound", b.name)
		}
	}
}

func TestTreeKeyMap_BindingsNonEmpty(t *testing.T) {
	t.Parallel()
	bindings := []struct {
		name string
		keys []string
	}{
		{"MoveDown", TreeKeyMap.MoveDown.Keys()},
		{"MoveUp", TreeKeyMap.MoveUp.Keys()},
		{"Expand", TreeKeyMap.Expand.Keys()},
		{"Collapse", TreeKeyMap.Collapse.Keys()},
		{"Top", TreeKeyMap.Top.Keys()},
		{"Bottom", TreeKeyMap.Bottom.Keys()},
	}
	for _, b := range bindings {
		if len(b.keys) == 0 {
			t.Errorf("TreeKeyMap.%s has no keys bound", b.name)
		}
	}
}

func TestSearchKeyMap_BindingsNonEmpty(t *testing.T) {
	t.Parallel()
	bindings := []struct {
		name string
		keys []string
	}{
		{"Confirm", SearchKeyMap.Confirm.Keys()},
		{"Cancel", SearchKeyMap.Cancel.Keys()},
		{"NextMatch", SearchKeyMap.NextMatch.Keys()},
		{"PrevMatch", SearchKeyMap.PrevMatch.Keys()},
	}
	for _, b := range bindings {
		if len(b.keys) == 0 {
			t.Errorf("SearchKeyMap.%s has no keys bound", b.name)
		}
	}
}

func TestHelpKeyMap_BindingsNonEmpty(t *testing.T) {
	t.Parallel()
	bindings := []struct {
		name string
		keys []string
	}{
		{"Dismiss", HelpKeyMap.Dismiss.Keys()},
		{"ScrollDown", HelpKeyMap.ScrollDown.Keys()},
		{"ScrollUp", HelpKeyMap.ScrollUp.Keys()},
	}
	for _, b := range bindings {
		if len(b.keys) == 0 {
			t.Errorf("HelpKeyMap.%s has no keys bound", b.name)
		}
	}
}

func TestWelcomeKeyMap_BindingsNonEmpty(t *testing.T) {
	t.Parallel()
	bindings := []struct {
		name string
		keys []string
	}{
		{"MoveDown", WelcomeKeyMap.MoveDown.Keys()},
		{"MoveUp", WelcomeKeyMap.MoveUp.Keys()},
		{"Enter", WelcomeKeyMap.Enter.Keys()},
		{"Back", WelcomeKeyMap.Back.Keys()},
		{"Top", WelcomeKeyMap.Top.Keys()},
		{"Bottom", WelcomeKeyMap.Bottom.Keys()},
		{"Quit", WelcomeKeyMap.Quit.Keys()},
	}
	for _, b := range bindings {
		if len(b.keys) == 0 {
			t.Errorf("WelcomeKeyMap.%s has no keys bound", b.name)
		}
	}
}

func TestGlobalKeyMap_HelpText(t *testing.T) {
	t.Parallel()
	// Verify help text is set on each binding.
	h := GlobalKeyMap.Quit.Help()
	if h.Key == "" || h.Desc == "" {
		t.Error("GlobalKeyMap.Quit missing help text")
	}
}

func TestTreeKeyMap_HelpText(t *testing.T) {
	t.Parallel()
	h := TreeKeyMap.MoveDown.Help()
	if h.Key == "" || h.Desc == "" {
		t.Error("TreeKeyMap.MoveDown missing help text")
	}
}

func TestSearchKeyMap_HelpText(t *testing.T) {
	t.Parallel()
	h := SearchKeyMap.Confirm.Help()
	if h.Key == "" || h.Desc == "" {
		t.Error("SearchKeyMap.Confirm missing help text")
	}
}

func TestHelpKeyMap_HelpText(t *testing.T) {
	t.Parallel()
	h := HelpKeyMap.Dismiss.Help()
	if h.Key == "" || h.Desc == "" {
		t.Error("HelpKeyMap.Dismiss missing help text")
	}
}

func TestWelcomeKeyMap_HelpText(t *testing.T) {
	t.Parallel()
	h := WelcomeKeyMap.Quit.Help()
	if h.Key == "" || h.Desc == "" {
		t.Error("WelcomeKeyMap.Quit missing help text")
	}
}
