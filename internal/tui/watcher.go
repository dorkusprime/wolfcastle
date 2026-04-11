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
//
// Events are delivered through an outbound channel rather than a direct
// program.Send call so that the watcher does not need a *tea.Program
// reference. The model owns the channel and drives delivery via a
// recursive tea.Cmd that wraps each event in a WatcherMsg envelope.
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
	events      chan<- tea.Msg
	done        chan struct{}
	mu          sync.Mutex
	useFsnotify bool

	// subscribed tracks which leaf addresses already have an
	// fsnotify subscription via AddNodeWatch + an initial cache
	// load via NodeUpdatedMsg. EagerPrefetchAndSubscribe consults
	// this set so it can be called repeatedly without re-emitting
	// load events for already-subscribed leaves. The set is what
	// makes the function idempotent and lets the watcher pick up
	// leaves that the daemon adds to the index after startup
	// (decomposition, planning passes, etc).
	subscribed map[string]bool

	// polling state
	indexMtime    time.Time
	instanceMtime time.Time
	logFileSize   int64
	lineBuf       string // incomplete trailing line from last read
}

// NewWatcher creates a Watcher that will observe the given store's state
// files, the instance registry directory, and log output, and emit events
// to the supplied channel. The watcher is inert until Start or
// StartPolling is called.
func NewWatcher(store *state.Store, logDir, instanceDir string, events chan<- tea.Msg) *Watcher {
	return &Watcher{
		store:       store,
		logDir:      logDir,
		instanceDir: instanceDir,
		events:      events,
		pending:     make(map[string]bool),
		done:        make(chan struct{}),
		useFsnotify: true,
		subscribed:  make(map[string]bool),
	}
}

// emit performs a non-blocking send on the events channel, wrapping the
// payload in a WatcherMsg envelope. The wrapper lets the model dispatch
// every watcher-sourced event through a single Update branch that also
// reschedules the next channel drain. If the consumer is slow and the
// channel buffer is full the event is dropped rather than stalling the
// watcher goroutine; the next mtime/poll cycle will resend an equivalent
// state update.
func (w *Watcher) emit(msg tea.Msg) {
	if w.events == nil {
		return
	}
	select {
	case w.events <- WatcherMsg{Inner: msg}:
	default:
	}
}

// Start initializes the fsnotify watcher, adds watches on the relevant
// paths, and launches the event-processing goroutine. Returns the
// fsnotify init error if the OS watcher cannot be created, so the
// caller can fall back to StartPolling. Returns an error and is a
// no-op if the store is nil — defensive guard so tests can construct
// the model without a fully wired backing store.
func (w *Watcher) Start() error {
	if w.store == nil {
		return fmt.Errorf("watcher: store is nil")
	}
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		w.useFsnotify = false
		return fmt.Errorf("fsnotify init: %w", err)
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
		_ = fsw.Add(latest)
		// Emit the tail of the file so the LogViewModel has content
		// to show the moment the user switches to the log view.
		// LoadInitialLogTail also sets logOffset = file size so the
		// next incremental read picks up from the current end of
		// file, not from the beginning.
		w.LoadInitialLogTail(defaultLogTailLines)
	}

	go w.loop()
	return nil
}

// defaultLogTailLines is the number of lines the watcher loads from
// the existing log file at startup. The LogViewModel has its own
// 10000-line cap so any value below that just controls how much
// historical context the user sees on first switch.
const defaultLogTailLines = 500

// LoadInitialLogTail reads the last maxLines from the currently
// seeded w.logFile and emits them as a LogLinesMsg via the events
// channel, then advances w.logOffset to the end of file so the next
// incremental read produces only new content.
//
// Skips files ending in .jsonl.gz; the live exec log is always the
// plain .jsonl (rotated archives are .gz). If the latest file is
// gz-only because the daemon has stopped between iterations, we
// emit nothing rather than feeding gzipped binary into the
// LogViewModel's NDJSON parser.
func (w *Watcher) LoadInitialLogTail(maxLines int) {
	w.mu.Lock()
	logFile := w.logFile
	w.mu.Unlock()

	if logFile == "" || strings.HasSuffix(logFile, ".gz") {
		return
	}

	f, err := os.Open(logFile)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return
	}

	// Read every complete line from the file. For multi-megabyte
	// logs this is wasteful but the daemon's per-iteration log
	// files cap out around a few hundred KB in practice. If that
	// ever becomes a real concern, replace this with a backwards
	// chunked read that stops once it has gathered maxLines.
	scanner := bufio.NewScanner(f)
	// Allow long NDJSON lines that contain embedded prompts.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Trim to the last maxLines.
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	w.mu.Lock()
	w.logOffset = info.Size()
	w.lineBuf = ""
	w.mu.Unlock()

	if len(lines) > 0 {
		w.emit(LogLinesMsg{Lines: lines})
	}
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
// Bubbletea messages. All emit calls happen without any lock held.
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
					w.emit(ErrorMsg{
						Filename: "state.json",
						Message:  "State corruption detected: state.json. Run wolfcastle doctor.",
					})
				} else {
					w.emit(StateUpdatedMsg{Index: idx})
					// The index just changed. The daemon may have
					// added new leaves via decomposition; subscribe
					// to any that don't already have an fsnotify
					// watch and load their initial state into the
					// model's cache. EagerPrefetchAndSubscribe is
					// idempotent (consults w.subscribed), so this
					// is safe to call after every index update.
					_ = w.EagerPrefetchAndSubscribe()
				}
			}

		// Instance registry directory.
		case w.instanceDir != "" && strings.HasPrefix(p, w.instanceDir):
			if !sentInstances {
				sentInstances = true
				if entries, err := instance.List(); err == nil {
					w.emit(InstancesUpdatedMsg{Instances: entries})
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
					w.emit(ErrorMsg{
						Filename: addr + "/state.json",
						Message:  fmt.Sprintf("Unreadable: %s/state.json. Run wolfcastle doctor.", addr),
					})
				} else {
					w.emit(NodeUpdatedMsg{Address: addr, Node: node})
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
				w.emit(NewLogFileMsg{Path: latest})
			}

		// Current log file was modified: read new lines.
		case p == w.logFile:
			if lines := w.readNewLogLines(); len(lines) > 0 {
				w.emit(LogLinesMsg{Lines: lines})
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
// No-op if the store is nil.
func (w *Watcher) StartPolling() {
	if w.store == nil {
		return
	}
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
		// Tail-load existing content so the LogViewModel has
		// something to show on first switch even when the daemon
		// isn't writing right now. LoadInitialLogTail also seeds
		// logOffset to file size for incremental polling reads.
		w.LoadInitialLogTail(defaultLogTailLines)
		if info, err := os.Stat(w.logFile); err == nil {
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
				w.emit(ErrorMsg{
					Filename: "state.json",
					Message:  "State corruption detected: state.json. Run wolfcastle doctor.",
				})
			} else {
				w.emit(StateUpdatedMsg{Index: idx})
			}
		}
	}

	// Check instance directory mtime.
	if w.instanceDir != "" {
		if info, err := os.Stat(w.instanceDir); err == nil {
			if info.ModTime() != w.instanceMtime {
				w.instanceMtime = info.ModTime()
				if entries, err := instance.List(); err == nil {
					w.emit(InstancesUpdatedMsg{Instances: entries})
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
			w.emit(NewLogFileMsg{Path: latest})
		}
	}

	// Check current log file size.
	if w.logFile != "" {
		if info, err := os.Stat(w.logFile); err == nil {
			if info.Size() != w.logFileSize {
				w.logFileSize = info.Size()
				if lines := w.readNewLogLines(); len(lines) > 0 {
					w.emit(LogLinesMsg{Lines: lines})
				}
			}
		}
	}
	// PollTickMsg is owned by the model's own tea.Tick scheduler; the
	// watcher does not emit it. The model uses tea.Tick to drive
	// detect-entry-state and pollState refreshes on a fixed cadence
	// regardless of whether filesystem activity triggered a watcher
	// event.
}

// AddNodeWatch adds an fsnotify watch on a specific node's state.json,
// typically called from EagerPrefetchAndSubscribe at startup so that
// subsequent state.json rewrites by the daemon trigger NodeUpdatedMsg
// events into the channel.
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

// EagerPrefetchAndSubscribe walks every leaf in the store's index,
// reads its NodeState from disk, emits a NodeUpdatedMsg per leaf
// (populating the model's tree cache via the events channel), and
// adds an fsnotify subscription for each leaf so subsequent state
// rewrites by the daemon flow through the same channel.
//
// This is the bootstrap step that makes per-task state visible
// without requiring the user to manually expand each leaf, and
// keeps the cache fresh for the rest of the session via fsnotify.
// Failures to read individual leaves are tolerated: a corrupt or
// in-flight state.json is logged-and-skipped, and the next watcher
// event will retry on its own.
//
// **Idempotent.** The function consults w.subscribed and skips any
// address that already has a subscription, so it's safe to call
// repeatedly. The flush handler invokes it after every index update
// so leaves added by daemon decomposition (which appear in the
// index after watcher startup) get subscribed and cached the moment
// they show up. Without this re-call, eager prefetch would only
// cover leaves that existed at TUI launch — newly-decomposed leaves
// would have no fsnotify subscription and their tasks would never
// reach the cache.
//
// Safe to call before or after Start/StartPolling. When called
// before fsnotify init, the AddNodeWatch calls become no-ops and
// only the eager-load half runs; when called after, both halves
// run as intended.
func (w *Watcher) EagerPrefetchAndSubscribe() error {
	if w.store == nil {
		return fmt.Errorf("watcher: store is nil")
	}
	idx, err := w.store.ReadIndex()
	if err != nil {
		return fmt.Errorf("reading index: %w", err)
	}
	if idx == nil {
		return nil
	}
	for addr, entry := range idx.Nodes {
		if entry.Type != state.NodeLeaf {
			continue
		}
		w.mu.Lock()
		alreadySubscribed := w.subscribed[addr]
		w.mu.Unlock()
		if alreadySubscribed {
			continue
		}
		node, err := w.store.ReadNode(addr)
		if err != nil {
			// Skip leaves whose state files are missing or corrupt.
			// The next fsnotify event will retry on its own once
			// the daemon writes a clean version. Don't mark as
			// subscribed yet so a retry happens.
			continue
		}
		w.emit(NodeUpdatedMsg{Address: addr, Node: node})
		w.AddNodeWatch(addr)
		w.mu.Lock()
		w.subscribed[addr] = true
		w.mu.Unlock()
	}
	return nil
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
	defer func() { _ = f.Close() }()

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
