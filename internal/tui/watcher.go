package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/dorkusprime/wolfcastle/internal/instance"
	"github.com/dorkusprime/wolfcastle/internal/logging"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/fsnotify/fsnotify"
)

// Watcher monitors the filesystem for changes to state files, instance
// registrations, and log output, translating raw filesystem events into
// typed Bubbletea messages. It uses fsnotify where available and falls
// back to polling when the OS watcher cannot be initialized.
type Watcher struct {
	watcher     *fsnotify.Watcher
	store       *state.Store
	logDir      string
	logFile     string
	logOffset   int64
	instanceDir string
	debounce    *time.Timer
	maxSlide    *time.Timer
	pending     map[string]bool
	program     *tea.Program
	done        chan struct{}
	mu          sync.Mutex
	useFsnotify bool

	// polling state
	indexMtime    time.Time
	instanceMtime time.Time
	logFileSize   int64
	lineBuf       string // incomplete trailing line from last read
}

// NewWatcher creates a Watcher that will observe the given store's state
// files, the instance registry directory, and log output. The watcher is
// inert until Start or StartPolling is called.
func NewWatcher(store *state.Store, logDir, instanceDir string) *Watcher {
	return &Watcher{
		store:       store,
		logDir:      logDir,
		instanceDir: instanceDir,
		pending:     make(map[string]bool),
		done:        make(chan struct{}),
		useFsnotify: true,
	}
}

// Start initializes the fsnotify watcher, adds watches on the relevant
// paths, and launches the event-processing goroutine. If fsnotify cannot
// be initialized, it logs to stderr and disables filesystem watching (the
// caller should use StartPolling as the fallback).
func (w *Watcher) Start(program *tea.Program) error {
	w.program = program

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "wolfcastle: fsnotify init failed: %v\n", err)
		w.useFsnotify = false
		return nil
	}
	w.watcher = fsw

	// Watch the instance registry directory.
	if w.instanceDir != "" {
		_ = fsw.Add(w.instanceDir)
	}

	// Watch the directory containing state.json (the index). Watching the
	// parent directory catches both modifications and atomic-rename writes.
	indexPath := w.store.IndexPath()
	indexDir := filepath.Dir(indexPath)
	_ = fsw.Add(indexDir)

	// Watch the log directory for new file creation.
	if w.logDir != "" {
		_ = fsw.Add(w.logDir)
	}

	// If a log file already exists, watch it for appended lines.
	if latest, err := logging.LatestLogFile(w.logDir); err == nil {
		w.logFile = latest
		if info, err := os.Stat(latest); err == nil {
			w.logOffset = info.Size()
		}
		_ = fsw.Add(latest)
	}

	go w.loop()
	return nil
}

// loop reads fsnotify events and errors, feeding them through the
// debounce/maxSlide machinery before flushing as Bubbletea messages.
func (w *Watcher) loop() {
	for {
		select {
		case <-w.done:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.mu.Lock()
			w.pending[event.Name] = true
			w.resetDebounce()
			w.mu.Unlock()

		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// fsnotify errors are transient; nothing actionable here.
		}
	}
}

// resetDebounce resets the 100ms debounce timer and, if the 500ms max-slide
// timer has not yet started, kicks it off. Must be called with w.mu held.
func (w *Watcher) resetDebounce() {
	if w.debounce != nil {
		w.debounce.Stop()
	}
	w.debounce = time.AfterFunc(100*time.Millisecond, w.flushFromTimer)

	if w.maxSlide == nil {
		w.maxSlide = time.AfterFunc(500*time.Millisecond, w.flushFromTimer)
	}
}

// flushFromTimer is invoked by either the debounce or maxSlide timer.
// It grabs the lock, snapshots and clears the pending set, then sends
// messages outside the lock to avoid deadlocking with the tea runtime.
func (w *Watcher) flushFromTimer() {
	w.mu.Lock()
	if len(w.pending) == 0 {
		w.mu.Unlock()
		return
	}
	paths := make(map[string]bool, len(w.pending))
	for k, v := range w.pending {
		paths[k] = v
	}
	w.pending = make(map[string]bool)
	if w.debounce != nil {
		w.debounce.Stop()
		w.debounce = nil
	}
	if w.maxSlide != nil {
		w.maxSlide.Stop()
		w.maxSlide = nil
	}
	w.mu.Unlock()

	w.flush(paths)
}

// flush processes a batch of changed paths and dispatches the appropriate
// Bubbletea messages. All program.Send calls happen without any lock held.
func (w *Watcher) flush(paths map[string]bool) {
	indexPath := w.store.IndexPath()
	indexDir := filepath.Dir(indexPath)

	sentIndex := false
	sentInstances := false
	sentNodes := make(map[string]bool)

	for p := range paths {
		switch {
		// Index file itself, or any file in its directory that could be the
		// atomically-renamed replacement.
		case p == indexPath || filepath.Dir(p) == indexDir && strings.HasSuffix(p, "state.json"):
			if !sentIndex {
				sentIndex = true
				idx, err := w.store.ReadIndex()
				if err != nil {
					w.program.Send(ErrorMsg{
						Filename: "state.json",
						Message:  "State corruption detected: state.json. Run wolfcastle doctor.",
					})
				} else {
					w.program.Send(StateUpdatedMsg{Index: idx})
				}
			}

		// Instance registry directory.
		case w.instanceDir != "" && strings.HasPrefix(p, w.instanceDir):
			if !sentInstances {
				sentInstances = true
				if entries, err := instance.List(); err == nil {
					w.program.Send(InstancesUpdatedMsg{Instances: entries})
				}
			}

		// Per-node state files. The path pattern is:
		// <store.Dir()>/<addr segments...>/state.json
		case strings.HasPrefix(p, w.store.Dir()) && strings.HasSuffix(p, "state.json") && p != indexPath:
			addr := w.nodeAddrFromPath(p)
			if addr != "" && !sentNodes[addr] {
				sentNodes[addr] = true
				node, err := w.store.ReadNode(addr)
				if err != nil {
					w.program.Send(ErrorMsg{
						Filename: addr + "/state.json",
						Message:  fmt.Sprintf("Unreadable: %s/state.json. Run wolfcastle doctor.", addr),
					})
				} else {
					w.program.Send(NodeUpdatedMsg{Address: addr, Node: node})
				}
			}

		// Log directory: a new file may have appeared.
		case w.logDir != "" && filepath.Dir(p) == w.logDir && p != w.logFile:
			if latest, err := logging.LatestLogFile(w.logDir); err == nil && latest != w.logFile {
				oldFile := w.logFile
				w.mu.Lock()
				w.logFile = latest
				w.logOffset = 0
				w.lineBuf = ""
				w.mu.Unlock()

				// Start watching the new file, stop watching the old one.
				if w.watcher != nil {
					_ = w.watcher.Add(latest)
					if oldFile != "" {
						_ = w.watcher.Remove(oldFile)
					}
				}
				w.program.Send(NewLogFileMsg{Path: latest})
			}

		// Current log file was modified: read new lines.
		case p == w.logFile:
			if lines := w.readNewLogLines(); len(lines) > 0 {
				coerced := make([]any, len(lines))
				for i, l := range lines {
					coerced[i] = l
				}
				w.program.Send(LogLinesMsg{Lines: coerced})
			}
		}
	}
}

// nodeAddrFromPath extracts the tree address from a node state.json path
// by stripping the store directory prefix and the trailing /state.json.
func (w *Watcher) nodeAddrFromPath(p string) string {
	rel, err := filepath.Rel(w.store.Dir(), p)
	if err != nil {
		return ""
	}
	// rel looks like "a/b/c/state.json"
	addr := strings.TrimSuffix(rel, string(filepath.Separator)+"state.json")
	if addr == rel {
		return ""
	}
	// Normalize to forward slashes for the address.
	return filepath.ToSlash(addr)
}

// StartPolling launches a goroutine that checks for changes every two
// seconds, serving as the fallback when fsnotify is unavailable.
func (w *Watcher) StartPolling(program *tea.Program) {
	w.program = program

	// Seed initial mtime/size values so the first tick doesn't spuriously
	// re-send everything.
	if info, err := os.Stat(w.store.IndexPath()); err == nil {
		w.indexMtime = info.ModTime()
	}
	if info, err := os.Stat(w.instanceDir); err == nil {
		w.instanceMtime = info.ModTime()
	}
	if w.logFile == "" {
		if latest, err := logging.LatestLogFile(w.logDir); err == nil {
			w.logFile = latest
		}
	}
	if w.logFile != "" {
		if info, err := os.Stat(w.logFile); err == nil {
			w.logOffset = info.Size()
			w.logFileSize = info.Size()
		}
	}

	go w.pollLoop()
}

func (w *Watcher) pollLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			w.pollTick()
		}
	}
}

func (w *Watcher) pollTick() {
	// Check index mtime.
	if info, err := os.Stat(w.store.IndexPath()); err == nil {
		if info.ModTime() != w.indexMtime {
			w.indexMtime = info.ModTime()
			idx, err := w.store.ReadIndex()
			if err != nil {
				w.program.Send(ErrorMsg{
					Filename: "state.json",
					Message:  "State corruption detected: state.json. Run wolfcastle doctor.",
				})
			} else {
				w.program.Send(StateUpdatedMsg{Index: idx})
			}
		}
	}

	// Check instance directory mtime.
	if w.instanceDir != "" {
		if info, err := os.Stat(w.instanceDir); err == nil {
			if info.ModTime() != w.instanceMtime {
				w.instanceMtime = info.ModTime()
				if entries, err := instance.List(); err == nil {
					w.program.Send(InstancesUpdatedMsg{Instances: entries})
				}
			}
		}
	}

	// Check for new log files.
	if w.logDir != "" {
		if latest, err := logging.LatestLogFile(w.logDir); err == nil && latest != w.logFile {
			w.mu.Lock()
			w.logFile = latest
			w.logOffset = 0
			w.logFileSize = 0
			w.lineBuf = ""
			w.mu.Unlock()
			w.program.Send(NewLogFileMsg{Path: latest})
		}
	}

	// Check current log file size.
	if w.logFile != "" {
		if info, err := os.Stat(w.logFile); err == nil {
			if info.Size() != w.logFileSize {
				w.logFileSize = info.Size()
				if lines := w.readNewLogLines(); len(lines) > 0 {
					coerced := make([]any, len(lines))
					for i, l := range lines {
						coerced[i] = l
					}
					w.program.Send(LogLinesMsg{Lines: coerced})
				}
			}
		}
	}

	w.program.Send(PollTickMsg{})
}

// AddNodeWatch adds an fsnotify watch on a specific node's state.json,
// typically called when the user expands a node in the tree view.
func (w *Watcher) AddNodeWatch(addr string) {
	if w.watcher == nil {
		return
	}
	p, err := w.store.NodePath(addr)
	if err != nil {
		return
	}
	// Watch the directory rather than the file itself, since atomic
	// writes create a new inode.
	_ = w.watcher.Add(filepath.Dir(p))
}

// RemoveNodeWatch removes the fsnotify watch for a node's state directory,
// typically called when the user collapses a node.
func (w *Watcher) RemoveNodeWatch(addr string) {
	if w.watcher == nil {
		return
	}
	p, err := w.store.NodePath(addr)
	if err != nil {
		return
	}
	_ = w.watcher.Remove(filepath.Dir(p))
}

// Stop tears down the watcher: signals the goroutines to exit, closes the
// fsnotify handle, and cancels any pending timers.
func (w *Watcher) Stop() {
	select {
	case <-w.done:
		return // already stopped
	default:
		close(w.done)
	}

	w.mu.Lock()
	if w.debounce != nil {
		w.debounce.Stop()
		w.debounce = nil
	}
	if w.maxSlide != nil {
		w.maxSlide.Stop()
		w.maxSlide = nil
	}
	w.mu.Unlock()

	if w.watcher != nil {
		_ = w.watcher.Close()
	}
}

// readNewLogLines reads from logOffset to EOF in the current log file and
// returns only complete lines. Any trailing incomplete line is buffered
// internally for the next read.
func (w *Watcher) readNewLogLines() []string {
	w.mu.Lock()
	logFile := w.logFile
	offset := w.logOffset
	buf := w.lineBuf
	w.mu.Unlock()

	if logFile == "" {
		return nil
	}

	f, err := os.Open(logFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil
	}

	scanner := bufio.NewScanner(f)
	var lines []string
	var bytesRead int64

	for scanner.Scan() {
		line := scanner.Text()
		bytesRead += int64(len(scanner.Bytes())) + 1 // +1 for the newline
		lines = append(lines, buf+line)
		buf = ""
	}

	// If the scanner stopped with an error or at EOF, any remaining data
	// in the scanner's buffer is an incomplete line. We detect this by
	// checking whether the file position advanced past what we consumed.
	newOffset := offset + bytesRead

	// Check if there's trailing data after the last complete line.
	if info, err := f.Stat(); err == nil && info.Size() > newOffset {
		// Read the trailing bytes as an incomplete line buffer.
		trailing := make([]byte, info.Size()-newOffset)
		if _, err := f.ReadAt(trailing, newOffset); err == nil {
			buf = string(trailing)
			newOffset = info.Size()
		}
	}

	w.mu.Lock()
	w.logOffset = newOffset
	w.lineBuf = buf
	w.mu.Unlock()

	return lines
}
