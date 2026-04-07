package tui

import "charm.land/bubbles/v2/key"

type GlobalKeys struct {
	Quit       key.Binding
	ForceQuit  key.Binding
	Dashboard  key.Binding
	ToggleTree key.Binding
	CycleFocus key.Binding
	Refresh    key.Binding
	ToggleHelp key.Binding
	Search     key.Binding
	Copy       key.Binding
}

var GlobalKeyMap = GlobalKeys{
	Quit:       key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	ForceQuit:  key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	Dashboard:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "dash")),
	ToggleTree: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tree")),
	CycleFocus: key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "focus")),
	Refresh:    key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
	ToggleHelp: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Search:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Copy:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy")),
}

type TreeKeys struct {
	MoveDown key.Binding
	MoveUp   key.Binding
	Expand   key.Binding
	Collapse key.Binding
	Top      key.Binding
	Bottom   key.Binding
}

var TreeKeyMap = TreeKeys{
	MoveDown: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	MoveUp:   key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Expand:   key.NewBinding(key.WithKeys("enter", "l", "right"), key.WithHelp("Enter/l", "expand")),
	Collapse: key.NewBinding(key.WithKeys("esc", "h", "left"), key.WithHelp("Esc/h", "collapse")),
	Top:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
	Bottom:   key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
}

type SearchKeys struct {
	Confirm   key.Binding
	Cancel    key.Binding
	NextMatch key.Binding
	PrevMatch key.Binding
}

var SearchKeyMap = SearchKeys{
	Confirm:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "confirm")),
	Cancel:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "cancel")),
	NextMatch: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next")),
	PrevMatch: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev")),
}

type HelpKeys struct {
	Dismiss    key.Binding
	ScrollDown key.Binding
	ScrollUp   key.Binding
}

var HelpKeyMap = HelpKeys{
	Dismiss:    key.NewBinding(key.WithKeys("?", "esc"), key.WithHelp("?/Esc", "close")),
	ScrollDown: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	ScrollUp:   key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
}

type WelcomeKeys struct {
	MoveDown key.Binding
	MoveUp   key.Binding
	Enter    key.Binding
	Back     key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Quit     key.Binding
}

var WelcomeKeyMap = WelcomeKeys{
	MoveDown: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	MoveUp:   key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Enter:    key.NewBinding(key.WithKeys("enter", "l", "right"), key.WithHelp("Enter", "select")),
	Back:     key.NewBinding(key.WithKeys("h", "left", "backspace"), key.WithHelp("h/←", "back")),
	Top:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
	Bottom:   key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
