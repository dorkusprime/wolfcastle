package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewStateStore(t *testing.T) {
	t.Parallel()
	s := NewStateStore("/tmp/test", 3*time.Second)
	if s.Dir() != "/tmp/test" {
		t.Errorf("expected dir /tmp/test, got %s", s.Dir())
	}
	if s.Timeout() != 3*time.Second {
		t.Errorf("expected timeout 3s, got %v", s.Timeout())
	}
}

// ── ReadNode ────────────────────────────────────────────────────────────

func TestReadNode_MissingFile_ReturnsDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	ns, err := s.ReadNode("some-node")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ns.Version != 1 {
		t.Errorf("default version should be 1, got %d", ns.Version)
	}
	if ns.Audit.Status != AuditPending {
		t.Errorf("default audit status should be pending, got %s", ns.Audit.Status)
	}
}

func TestReadNode_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	// Write a node state manually
	nodeDir := filepath.Join(dir, "my-node")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	ns := NewNodeState("my-node", "My Node", NodeLeaf)
	ns.State = StatusInProgress
	if err := SaveNodeState(filepath.Join(nodeDir, "state.json"), ns); err != nil {
		t.Fatal(err)
	}

	got, err := s.ReadNode("my-node")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.State != StatusInProgress {
		t.Errorf("expected in_progress, got %s", got.State)
	}
	if got.Name != "My Node" {
		t.Errorf("expected 'My Node', got %q", got.Name)
	}
}

func TestReadNode_NestedAddress(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	nodeDir := filepath.Join(dir, "parent", "child")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	ns := NewNodeState("child", "Child", NodeLeaf)
	if err := SaveNodeState(filepath.Join(nodeDir, "state.json"), ns); err != nil {
		t.Fatal(err)
	}

	got, err := s.ReadNode("parent/child")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "child" {
		t.Errorf("expected id 'child', got %q", got.ID)
	}
}

func TestReadNode_InvalidAddress(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	_, err := s.ReadNode("INVALID ADDRESS")
	if err == nil {
		t.Error("expected error for invalid address")
	}
}

// ── ReadIndex ───────────────────────────────────────────────────────────

func TestReadIndex_MissingFile_ReturnsDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	idx, err := s.ReadIndex()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx.Version != 1 {
		t.Errorf("default version should be 1, got %d", idx.Version)
	}
	if idx.Nodes == nil {
		t.Error("nodes map should be initialized")
	}
}

func TestReadIndex_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	idx := NewRootIndex()
	idx.Nodes["test"] = IndexEntry{Name: "Test", Type: NodeLeaf, State: StatusNotStarted, Address: "test"}
	if err := SaveRootIndex(filepath.Join(dir, "state.json"), idx); err != nil {
		t.Fatal(err)
	}

	got, err := s.ReadIndex()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := got.Nodes["test"]; !ok {
		t.Error("expected 'test' node in index")
	}
}

// ── ReadInbox ───────────────────────────────────────────────────────────

func TestReadInbox_MissingFile_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	f, err := s.ReadInbox()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Items) != 0 {
		t.Errorf("expected empty items, got %d", len(f.Items))
	}
}

func TestReadInbox_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	f := &InboxFile{Items: []InboxItem{
		{Timestamp: "2026-01-01T00:00:00Z", Text: "hello", Status: "new"},
	}}
	if err := SaveInbox(filepath.Join(dir, "inbox.json"), f); err != nil {
		t.Fatal(err)
	}

	got, err := s.ReadInbox()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Text != "hello" {
		t.Errorf("unexpected inbox contents: %+v", got.Items)
	}
}

// ── MutateNode ──────────────────────────────────────────────────────────

func TestMutateNode_ModifiesState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	// Seed a node
	nodeDir := filepath.Join(dir, "my-node")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	ns := NewNodeState("my-node", "My Node", NodeLeaf)
	ns.Tasks = []Task{{ID: "task-0001", State: StatusNotStarted}}
	if err := SaveNodeState(filepath.Join(nodeDir, "state.json"), ns); err != nil {
		t.Fatal(err)
	}

	err := s.MutateNode("my-node", func(ns *NodeState) error {
		ns.State = StatusInProgress
		for i := range ns.Tasks {
			if ns.Tasks[i].ID == "task-0001" {
				ns.Tasks[i].State = StatusInProgress
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("MutateNode error: %v", err)
	}

	got, _ := s.ReadNode("my-node")
	if got.State != StatusInProgress {
		t.Errorf("expected in_progress, got %s", got.State)
	}
	if got.Tasks[0].State != StatusInProgress {
		t.Errorf("expected task in_progress, got %s", got.Tasks[0].State)
	}
}

func TestMutateNode_MissingFile_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	err := s.MutateNode("new-node", func(ns *NodeState) error {
		ns.Name = "Created"
		return nil
	})
	if err == nil {
		t.Error("expected error when node file does not exist")
	}
}

func TestMutateNode_CallbackError_AbortsWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	// Seed a node
	nodeDir := filepath.Join(dir, "my-node")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	ns := NewNodeState("my-node", "My Node", NodeLeaf)
	ns.State = StatusNotStarted
	if err := SaveNodeState(filepath.Join(nodeDir, "state.json"), ns); err != nil {
		t.Fatal(err)
	}

	sentinel := errors.New("abort")
	err := s.MutateNode("my-node", func(ns *NodeState) error {
		ns.State = StatusComplete // this should NOT be persisted
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}

	got, _ := s.ReadNode("my-node")
	if got.State != StatusNotStarted {
		t.Errorf("state should remain not_started after aborted mutation, got %s", got.State)
	}
}

// ── MutateIndex ─────────────────────────────────────────────────────────

func TestMutateIndex_AddsNode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	// Seed an empty index
	if err := SaveRootIndex(filepath.Join(dir, "state.json"), NewRootIndex()); err != nil {
		t.Fatal(err)
	}

	err := s.MutateIndex(func(idx *RootIndex) error {
		idx.Nodes["my-node"] = IndexEntry{
			Name: "My Node", Type: NodeLeaf, State: StatusNotStarted, Address: "my-node",
		}
		idx.Root = append(idx.Root, "my-node")
		return nil
	})
	if err != nil {
		t.Fatalf("MutateIndex error: %v", err)
	}

	got, _ := s.ReadIndex()
	if _, ok := got.Nodes["my-node"]; !ok {
		t.Error("expected 'my-node' in index after mutation")
	}
}

func TestMutateIndex_CallbackError_AbortsWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	if err := SaveRootIndex(filepath.Join(dir, "state.json"), NewRootIndex()); err != nil {
		t.Fatal(err)
	}

	sentinel := errors.New("abort")
	err := s.MutateIndex(func(idx *RootIndex) error {
		idx.Nodes["bad"] = IndexEntry{Name: "Bad"}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}

	got, _ := s.ReadIndex()
	if _, ok := got.Nodes["bad"]; ok {
		t.Error("aborted mutation should not persist")
	}
}

// ── MutateInbox ─────────────────────────────────────────────────────────

func TestMutateInbox_MarksItemsFiled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	f := &InboxFile{Items: []InboxItem{
		{Timestamp: "t1", Text: "item1", Status: "new"},
		{Timestamp: "t2", Text: "item2", Status: "new"},
	}}
	if err := SaveInbox(filepath.Join(dir, "inbox.json"), f); err != nil {
		t.Fatal(err)
	}

	err := s.MutateInbox(func(f *InboxFile) error {
		for i := range f.Items {
			f.Items[i].Status = "filed"
		}
		return nil
	})
	if err != nil {
		t.Fatalf("MutateInbox error: %v", err)
	}

	got, _ := s.ReadInbox()
	for i, item := range got.Items {
		if item.Status != "filed" {
			t.Errorf("item %d: expected 'filed', got %q", i, item.Status)
		}
	}
}

func TestMutateInbox_CallbackError_AbortsWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	f := &InboxFile{Items: []InboxItem{
		{Timestamp: "t1", Text: "item1", Status: "new"},
	}}
	if err := SaveInbox(filepath.Join(dir, "inbox.json"), f); err != nil {
		t.Fatal(err)
	}

	sentinel := errors.New("abort")
	err := s.MutateInbox(func(f *InboxFile) error {
		f.Items[0].Status = "filed"
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel, got %v", err)
	}

	got, _ := s.ReadInbox()
	if got.Items[0].Status != "new" {
		t.Errorf("aborted mutation should not persist, got %q", got.Items[0].Status)
	}
}

// ── Concurrency ─────────────────────────────────────────────────────────

func TestMutateNode_Concurrent_NoDataLoss(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 10*time.Second)

	// Seed with a node that has tasks
	nodeDir := filepath.Join(dir, "my-node")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	ns := NewNodeState("my-node", "My Node", NodeLeaf)
	if err := SaveNodeState(filepath.Join(nodeDir, "state.json"), ns); err != nil {
		t.Fatal(err)
	}

	// 10 goroutines each add a task
	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = s.MutateNode("my-node", func(ns *NodeState) error {
				ns.Tasks = append(ns.Tasks, Task{
					ID:    fmt.Sprintf("task-%04d", idx+1),
					State: StatusNotStarted,
				})
				return nil
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d error: %v", i, err)
		}
	}

	got, err := s.ReadNode("my-node")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tasks) != n {
		t.Errorf("expected %d tasks, got %d (data lost)", n, len(got.Tasks))
	}
}

func TestMutateInbox_Concurrent_NoItemLoss(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 10*time.Second)

	// Seed empty inbox
	if err := SaveInbox(filepath.Join(dir, "inbox.json"), &InboxFile{}); err != nil {
		t.Fatal(err)
	}

	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = s.MutateInbox(func(f *InboxFile) error {
				f.Items = append(f.Items, InboxItem{
					Timestamp: fmt.Sprintf("t%d", idx),
					Text:      fmt.Sprintf("item-%d", idx),
					Status:    "new",
				})
				return nil
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d error: %v", i, err)
		}
	}

	got, err := s.ReadInbox()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != n {
		t.Errorf("expected %d items, got %d (data lost)", n, len(got.Items))
	}
}

// ── WithLock ────────────────────────────────────────────────────────────

func TestWithLock_ExclusiveAccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	err := s.WithLock(func() error {
		// Just verify we can hold the lock and do work
		return SaveRootIndex(filepath.Join(dir, "state.json"), NewRootIndex())
	})
	if err != nil {
		t.Fatalf("WithLock error: %v", err)
	}

	idx, err := s.ReadIndex()
	if err != nil {
		t.Fatalf("ReadIndex error: %v", err)
	}
	if idx.Version != 1 {
		t.Errorf("expected version 1, got %d", idx.Version)
	}
}

func TestMutateIndex_MissingFile_CreatesDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	err := s.MutateIndex(func(idx *RootIndex) error {
		idx.RootName = "test"
		return nil
	})
	if err != nil {
		t.Fatalf("MutateIndex error: %v", err)
	}

	got, _ := s.ReadIndex()
	if got.RootName != "test" {
		t.Errorf("expected 'test', got %q", got.RootName)
	}
}

func TestMutateInbox_MissingFile_CreatesDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStateStore(dir, 5*time.Second)

	err := s.MutateInbox(func(f *InboxFile) error {
		f.Items = append(f.Items, InboxItem{Status: "new", Text: "hello"})
		return nil
	})
	if err != nil {
		t.Fatalf("MutateInbox error: %v", err)
	}

	got, _ := s.ReadInbox()
	if len(got.Items) != 1 || got.Items[0].Text != "hello" {
		t.Errorf("unexpected inbox: %+v", got.Items)
	}
}
