package tree

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Temporary local message types until the shared tui message package is
// wired through the top-level model. These mirror the definitions in
// internal/tui/messages.go and should be replaced with those once the
// app shell routes messages into sub-models.

// StateUpdatedMsg signals that the root index has changed on disk.
type StateUpdatedMsg struct {
	Index *state.RootIndex
}

// NodeUpdatedMsg signals that a single node's state has been refreshed.
type NodeUpdatedMsg struct {
	Address string
	Node    *state.NodeState
}

// LoadNodeMsg is an internal command result carrying a freshly-read node.
type LoadNodeMsg struct {
	Address string
	Node    *state.NodeState
	Err     error
}

// TreeRow is a single visible line in the flattened tree view.
type TreeRow struct {
	Addr       string
	Name       string
	Depth      int
	NodeType   state.NodeType
	Status     state.NodeStatus
	IsTask     bool
	Expandable bool
	IsExpanded bool
}

// TreeModel is the sub-model that owns the project tree panel.
type TreeModel struct {
	index          *state.RootIndex
	nodes          map[string]*state.NodeState
	cacheExpiry    map[string]time.Time
	flatList       []TreeRow
	cursor         int
	scrollTop      int
	expanded       map[string]bool
	focused        bool
	width          int
	height         int
	currentTarget  string
	searchMatches  map[int]bool
}

// NewTreeModel returns an initialized TreeModel with empty maps.
func NewTreeModel() TreeModel {
	return TreeModel{
		nodes:       make(map[string]*state.NodeState),
		cacheExpiry: make(map[string]time.Time),
		expanded:    make(map[string]bool),
	}
}

// Key bindings. Mirrors the TreeKeyMap in the parent tui package so the
// tree sub-model is self-contained for now.
var keys = struct {
	MoveDown key.Binding
	MoveUp   key.Binding
	Expand   key.Binding
	Collapse key.Binding
	Top      key.Binding
	Bottom   key.Binding
}{
	MoveDown: key.NewBinding(key.WithKeys("j", "down")),
	MoveUp:   key.NewBinding(key.WithKeys("k", "up")),
	Expand:   key.NewBinding(key.WithKeys("enter", "l", "right")),
	Collapse: key.NewBinding(key.WithKeys("esc", "h", "left")),
	Top:      key.NewBinding(key.WithKeys("g")),
	Bottom:   key.NewBinding(key.WithKeys("G")),
}

// Update processes incoming messages and returns the updated model plus any
// commands that should fire next.
func (m TreeModel) Update(msg tea.Msg) (TreeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if !m.focused {
			return m, nil
		}
		return m.handleKey(msg)

	case StateUpdatedMsg:
		m.index = msg.Index
		m.buildFlatList()
		m.clampCursor()
		m.scrollIntoCursor()
		return m, nil

	case NodeUpdatedMsg:
		m.nodes[msg.Address] = msg.Node
		m.cacheExpiry[msg.Address] = time.Now().Add(30 * time.Second)
		m.buildFlatList()
		m.clampCursor()
		m.scrollIntoCursor()
		return m, nil

	case LoadNodeMsg:
		if msg.Err != nil {
			return m, nil
		}
		m.nodes[msg.Address] = msg.Node
		m.cacheExpiry[msg.Address] = time.Now().Add(30 * time.Second)
		m.buildFlatList()
		m.clampCursor()
		m.scrollIntoCursor()
		return m, nil
	}

	return m, nil
}

func (m TreeModel) handleKey(msg tea.KeyPressMsg) (TreeModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.MoveDown):
		if m.cursor < len(m.flatList)-1 {
			m.cursor++
		}
		m.scrollIntoCursor()

	case key.Matches(msg, keys.MoveUp):
		if m.cursor > 0 {
			m.cursor--
		}
		m.scrollIntoCursor()

	case key.Matches(msg, keys.Expand):
		return m.handleExpand()

	case key.Matches(msg, keys.Collapse):
		return m.handleCollapse(), nil

	case key.Matches(msg, keys.Top):
		m.cursor = 0
		m.scrollIntoCursor()

	case key.Matches(msg, keys.Bottom):
		if len(m.flatList) > 0 {
			m.cursor = len(m.flatList) - 1
		}
		m.scrollIntoCursor()
	}

	return m, nil
}

func (m TreeModel) handleExpand() (TreeModel, tea.Cmd) {
	if len(m.flatList) == 0 {
		return m, nil
	}
	row := m.flatList[m.cursor]

	if row.IsTask {
		return m, nil
	}

	if m.expanded[row.Addr] {
		// Already expanded; collapse instead.
		delete(m.expanded, row.Addr)
		m.buildFlatList()
		m.clampCursor()
		m.scrollIntoCursor()
		return m, nil
	}

	m.expanded[row.Addr] = true

	// For leaf nodes we may need to load the NodeState from disk to get
	// tasks. If the cache is stale or missing, fire a command.
	if row.NodeType == state.NodeLeaf {
		if _, ok := m.nodes[row.Addr]; !ok {
			m.buildFlatList()
			m.scrollIntoCursor()
			addr := row.Addr
			return m, func() tea.Msg {
				return LoadNodeMsg{Address: addr}
			}
		}
	}

	m.buildFlatList()
	m.scrollIntoCursor()
	return m, nil
}

func (m TreeModel) handleCollapse() TreeModel {
	if len(m.flatList) == 0 {
		return m
	}
	row := m.flatList[m.cursor]

	if m.expanded[row.Addr] {
		delete(m.expanded, row.Addr)
		m.cacheExpiry[row.Addr] = time.Now().Add(30 * time.Second)
		m.buildFlatList()
		m.clampCursor()
		m.scrollIntoCursor()
		return m
	}

	// Already collapsed (or a task): jump cursor to parent.
	idx := m.parentOf(row.Addr)
	if idx >= 0 {
		m.cursor = idx
		m.scrollIntoCursor()
	}
	return m
}

// SetSize updates the viewport dimensions.
func (m *TreeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused marks whether the tree panel currently owns keyboard input.
func (m *TreeModel) SetFocused(focused bool) {
	m.focused = focused
}

// SetIndex replaces the root index and rebuilds the flat list.
func (m *TreeModel) SetIndex(index *state.RootIndex) {
	m.index = index
	m.buildFlatList()
	m.clampCursor()
	m.scrollIntoCursor()
}

// SetCurrentTarget sets the address of the node the daemon is working on.
func (m *TreeModel) SetCurrentTarget(addr string) {
	m.currentTarget = addr
}

// SetSearchMatches replaces the set of row indices that should be
// highlighted as search matches. Pass nil to clear highlighting.
func (m *TreeModel) SetSearchMatches(matches map[int]bool) {
	m.searchMatches = matches
}

// SetCursor moves the cursor to the given row index (clamped to bounds)
// and scrolls it into view.
func (m *TreeModel) SetCursor(row int) {
	m.cursor = row
	m.clampCursor()
	m.scrollIntoCursor()
}

// CleanCache removes cached node entries whose expiry time has passed.
func (m *TreeModel) CleanCache() {
	now := time.Now()
	for addr, exp := range m.cacheExpiry {
		if now.After(exp) {
			delete(m.nodes, addr)
			delete(m.cacheExpiry, addr)
		}
	}
}

// FlatList returns the current flattened tree rows. The returned slice
// should be treated as read-only.
func (m *TreeModel) FlatList() []TreeRow {
	return m.flatList
}

// SelectedAddr returns the address of the row under the cursor, or empty
// if the list is empty.
func (m *TreeModel) SelectedAddr() string {
	if m.cursor >= 0 && m.cursor < len(m.flatList) {
		return m.flatList[m.cursor].Addr
	}
	return ""
}

// buildFlatList walks the index tree, respecting expand state, and produces
// the ordered slice of TreeRows that the renderer will draw.
func (m *TreeModel) buildFlatList() {
	if m.index == nil {
		m.flatList = nil
		return
	}
	m.flatList = m.flatList[:0]
	for _, addr := range m.index.Root {
		m.appendNode(addr)
	}
}

func (m *TreeModel) appendNode(addr string) {
	entry, ok := m.index.Nodes[addr]
	if !ok {
		return
	}

	expandable := entry.Type == state.NodeOrchestrator && len(entry.Children) > 0
	// Leaf nodes are expandable if they might have tasks.
	if entry.Type == state.NodeLeaf {
		expandable = true
	}

	isExpanded := m.expanded[addr]

	m.flatList = append(m.flatList, TreeRow{
		Addr:       addr,
		Name:       entry.Name,
		Depth:      entry.DecompositionDepth,
		NodeType:   entry.Type,
		Status:     entry.State,
		Expandable: expandable,
		IsExpanded: isExpanded,
	})

	if !isExpanded {
		return
	}

	// Orchestrators: recurse into children.
	if entry.Type == state.NodeOrchestrator {
		for _, child := range entry.Children {
			m.appendNode(child)
		}
	}

	// Leaves: show tasks from cached NodeState.
	if entry.Type == state.NodeLeaf {
		if ns, ok := m.nodes[addr]; ok {
			for _, task := range ns.Tasks {
				m.flatList = append(m.flatList, TreeRow{
					Addr:   addr + "/" + task.ID,
					Name:   task.Title,
					Depth:  entry.DecompositionDepth + 1,
					Status: task.State,
					IsTask: true,
				})
			}
		}
	}
}

// scrollIntoCursor adjusts scrollTop so the cursor row is visible.
func (m *TreeModel) scrollIntoCursor() {
	if m.height <= 0 {
		return
	}
	if m.cursor < m.scrollTop {
		m.scrollTop = m.cursor
	}
	if m.cursor >= m.scrollTop+m.height {
		m.scrollTop = m.cursor - m.height + 1
	}
}

// clampCursor keeps the cursor within bounds after the flat list changes.
func (m *TreeModel) clampCursor() {
	if len(m.flatList) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.flatList) {
		m.cursor = len(m.flatList) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// parentOf finds the flatList index of the parent node for the given
// address. Returns -1 if no parent is found.
func (m *TreeModel) parentOf(addr string) int {
	if m.index == nil {
		return -1
	}

	// For task rows (addr contains a task ID suffix beyond the node address),
	// the parent is the leaf node itself.
	for nodeAddr := range m.index.Nodes {
		prefix := nodeAddr + "/"
		if len(addr) > len(prefix) && addr[:len(prefix)] == prefix {
			// addr is a task under nodeAddr
			for i, row := range m.flatList {
				if row.Addr == nodeAddr {
					return i
				}
			}
			return -1
		}
	}

	// For nodes, use the index's Parent field.
	if entry, ok := m.index.Nodes[addr]; ok && entry.Parent != "" {
		for i, row := range m.flatList {
			if row.Addr == entry.Parent {
				return i
			}
		}
	}
	return -1
}
