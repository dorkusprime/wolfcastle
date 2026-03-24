package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Store provides coordinated read and mutation access to the three
// kinds of state files in a Wolfcastle engineer namespace: per-node state,
// the root index, and the inbox. Reads are lock-free (atomic writes
// guarantee complete reads). Mutations acquire an advisory file lock,
// re-read the file, apply a caller-supplied callback, and write atomically.
type Store struct {
	dir     string        // namespace/projects directory
	timeout time.Duration // lock acquisition timeout
}

// NewStore creates a Store rooted at the given namespace directory
// (e.g., .wolfcastle/projects/wild-macbook-pro). The timeout governs how
// long lock acquisition will block before giving up.
func NewStore(namespaceDir string, timeout time.Duration) *Store {
	return &Store{
		dir:     namespaceDir,
		timeout: timeout,
	}
}

// Dir returns the namespace directory backing this store.
func (s *Store) Dir() string {
	return s.dir
}

// Timeout returns the lock timeout for this store.
func (s *Store) Timeout() time.Duration {
	return s.timeout
}

// ── Read operations ─────────────────────────────────────────────────────
// No lock required: atomic writes guarantee a complete file on disk.

// ReadNode loads a node's state from disk. Returns a default NodeState
// if the file does not exist.
func (s *Store) ReadNode(addr string) (*NodeState, error) {
	p, err := s.nodePath(addr)
	if err != nil {
		return nil, err
	}
	ns, err := LoadNodeState(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newDefaultNodeState(), nil
		}
		return nil, err
	}
	return ns, nil
}

func newDefaultNodeState() *NodeState {
	return &NodeState{Version: 1, Audit: AuditState{
		Status:      AuditPending,
		Breadcrumbs: []Breadcrumb{},
		Gaps:        []Gap{},
		Escalations: []Escalation{},
	}}
}

// ReadIndex loads the root index from disk. Returns a fresh empty index
// if the file does not exist.
func (s *Store) ReadIndex() (*RootIndex, error) {
	idx, err := LoadRootIndex(s.indexPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewRootIndex(), nil
		}
		return nil, err
	}
	return idx, nil
}

// ReadInbox loads the inbox from disk. Returns an empty InboxFile if the
// file does not exist.
func (s *Store) ReadInbox() (*InboxFile, error) {
	return LoadInbox(s.inboxPath())
}

// ReadScopeLocks loads the scope lock table from disk. Returns an empty
// table (Version:1, empty Locks) if the file does not exist.
func (s *Store) ReadScopeLocks() (*ScopeLockTable, error) {
	return LoadScopeLocks(s.scopeLocksPath())
}

// ── Mutation operations ─────────────────────────────────────────────────
// Each acquires the namespace lock, reads the current value, calls the
// callback, and writes the result back atomically.

// MutateNode locks the namespace, loads the node state, applies fn,
// saves the result, and propagates the new state up through parent
// nodes and the root index. Every state change is automatically
// reflected in the full tree. If fn returns an error the write is
// skipped and the error propagated.
func (s *Store) MutateNode(addr string, fn func(*NodeState) error) error {
	p, err := s.nodePath(addr)
	if err != nil {
		return err
	}
	lock := NewFileLock(s.dir, s.timeout)
	return lock.WithLock(func() error {
		ns, err := LoadNodeState(p)
		if err != nil {
			return err
		}
		if err := fn(ns); err != nil {
			return err
		}
		if err := SaveNodeState(p, ns); err != nil {
			return err
		}

		// Propagate state up through parents and root index.
		idx, idxErr := LoadRootIndex(s.indexPath())
		if idxErr != nil {
			// No index yet (e.g., during scaffolding). Skip propagation.
			return nil
		}

		loadNode := func(a string) (*NodeState, error) {
			np, err := s.nodePath(a)
			if err != nil {
				return nil, err
			}
			return LoadNodeState(np)
		}
		saveNode := func(a string, n *NodeState) error {
			np, err := s.nodePath(a)
			if err != nil {
				return err
			}
			return SaveNodeState(np, n)
		}

		if err := Propagate(addr, ns.State, idx, loadNode, saveNode); err != nil {
			// Propagation failure is non-fatal for the mutation itself.
			return nil
		}
		return SaveRootIndex(s.indexPath(), idx)
	})
}

// MutateIndex locks the namespace, loads the root index (or an empty
// default), applies fn, and saves the result.
func (s *Store) MutateIndex(fn func(*RootIndex) error) error {
	lock := NewFileLock(s.dir, s.timeout)
	return lock.WithLock(func() error {
		idx, err := LoadRootIndex(s.indexPath())
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				idx = NewRootIndex()
			} else {
				return err
			}
		}
		if err := fn(idx); err != nil {
			return err
		}
		return SaveRootIndex(s.indexPath(), idx)
	})
}

// MutateScopeLocks locks the namespace, loads the scope lock table (or an
// empty default), applies fn, and saves the result.
func (s *Store) MutateScopeLocks(fn func(*ScopeLockTable) error) error {
	lock := NewFileLock(s.dir, s.timeout)
	return lock.WithLock(func() error {
		t, err := LoadScopeLocks(s.scopeLocksPath())
		if err != nil {
			return err
		}
		if err := fn(t); err != nil {
			return err
		}
		return SaveScopeLocks(s.scopeLocksPath(), t)
	})
}

// MutateInbox locks the namespace, loads the inbox (or an empty default),
// applies fn, and saves the result.
func (s *Store) MutateInbox(fn func(*InboxFile) error) error {
	lock := NewFileLock(s.dir, s.timeout)
	return lock.WithLock(func() error {
		f, err := LoadInbox(s.inboxPath())
		if err != nil {
			return err
		}
		if err := fn(f); err != nil {
			return err
		}
		return SaveInbox(s.inboxPath(), f)
	})
}

// WithLock acquires the namespace lock and runs fn with exclusive access.
// Use this for complex multi-file operations (e.g., propagation that
// touches both the index and multiple node state files).
func (s *Store) WithLock(fn func() error) error {
	lock := NewFileLock(s.dir, s.timeout)
	return lock.WithLock(fn)
}

// ── Path helpers ────────────────────────────────────────────────────────

// IndexPath returns the absolute path to the root index file (state.json)
// in this namespace. Useful for callers that need raw I/O inside WithLock.
func (s *Store) IndexPath() string {
	return s.indexPath()
}

// NodePath returns the absolute path to a node's state.json file. Useful
// for callers that need raw I/O inside WithLock. Returns an error if the
// address is invalid.
func (s *Store) NodePath(addr string) (string, error) {
	return s.nodePath(addr)
}

// InboxPath returns the absolute path to the inbox file in this namespace.
func (s *Store) InboxPath() string {
	return s.inboxPath()
}

// ScopeLocksPath returns the absolute path to the scope-locks.json file.
func (s *Store) ScopeLocksPath() string {
	return s.scopeLocksPath()
}

func (s *Store) nodePath(addr string) (string, error) {
	if addr == "" {
		return "", fmt.Errorf("node address cannot be empty")
	}
	parts := strings.Split(addr, "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, " \t\n") {
			return "", fmt.Errorf("invalid address segment %q in %q", p, addr)
		}
	}
	return filepath.Join(s.dir, filepath.Join(parts...), "state.json"), nil
}

func (s *Store) indexPath() string {
	return filepath.Join(s.dir, "state.json")
}

func (s *Store) inboxPath() string {
	return filepath.Join(s.dir, "inbox.json")
}

func (s *Store) scopeLocksPath() string {
	return filepath.Join(s.dir, "scope-locks.json")
}
