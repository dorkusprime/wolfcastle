// Package logging provides per-iteration NDJSON log files for the
// Wolfcastle daemon. Each daemon iteration writes to its own .jsonl
// file, and log retention is enforced by file count and age.
// NDJSON files capture everything at all levels (ADR-012, ADR-046).
// Human-readable output is handled by the logrender package.
package logging

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Level represents a log severity tier (ADR-046).
type Level int

const (
	// LevelDebug captures stage skip reasons, inbox state checks, iteration
	// context details, and model output streaming.
	LevelDebug Level = iota
	// LevelInfo captures stage start/complete, iteration start, daemon
	// start/stop, expand/file item counts.
	LevelInfo
	// LevelWarn captures non-fatal stage errors, retry attempts, stale PID
	// detection, validation warnings.
	LevelWarn
	// LevelError captures fatal errors, invocation failures after retry
	// exhaustion, state corruption.
	LevelError
)

// levelNames maps Level values to their string representation.
var levelNames = map[Level]string{
	LevelDebug: "debug",
	LevelInfo:  "info",
	LevelWarn:  "warn",
	LevelError: "error",
}

// levelFromString maps a string to a Level. Returns LevelInfo for
// unrecognised values, matching the ADR-046 default.
var levelFromString = map[string]Level{
	"debug": LevelDebug,
	"info":  LevelInfo,
	"warn":  LevelWarn,
	"error": LevelError,
}

// ParseLevel converts a string to a Level. Returns LevelInfo and false
// if the string is not recognised.
func ParseLevel(s string) (Level, bool) {
	l, ok := levelFromString[strings.ToLower(s)]
	return l, ok
}

// String returns the lowercase name of the level.
func (l Level) String() string {
	if s, ok := levelNames[l]; ok {
		return s
	}
	return "info"
}

// nowFunc is the clock function used for timestamps and age comparisons.
// Tests replace it to control time.
var nowFunc = time.Now

// Logger writes per-iteration NDJSON log files.
type Logger struct {
	LogDir    string
	Iteration int
	TraceID   string // set by StartIterationWithPrefix, included in every log record

	defaultPrefix string // if set, StartIteration uses this instead of "iter"
	file          *os.File
}

// NewLogger creates a logger for the given log directory.
func NewLogger(logDir string) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}
	return &Logger{
		LogDir: logDir,
	}, nil
}

// Child creates a new Logger that shares the parent's LogDir but owns
// independent state: its Iteration counter starts at 0, its file handle
// is nil (opened on the first StartIteration call), and the given prefix
// is used as the default for trace IDs. The parent Logger is unmodified.
func (l *Logger) Child(prefix string) *Logger {
	return &Logger{
		LogDir:        l.LogDir,
		defaultPrefix: prefix,
	}
}

// StartIteration creates a new log file for the current iteration.
// Closes any previously open file before starting a new one. Uses
// the logger's default prefix for the trace ID, falling back to "iter".
func (l *Logger) StartIteration() error {
	prefix := l.defaultPrefix
	if prefix == "" {
		prefix = "iter"
	}
	return l.StartIterationWithPrefix(prefix)
}

// StartIterationWithPrefix creates a new log file and sets the trace
// ID to "{prefix}-{iteration}". Use "exec" for the execute loop and
// "intake" for the inbox goroutine so log records from concurrent
// goroutines are distinguishable.
func (l *Logger) StartIterationWithPrefix(prefix string) error {
	l.Close() // prevent file handle leak if called without Close()
	l.Iteration++
	l.TraceID = fmt.Sprintf("%s-%04d", prefix, l.Iteration)
	filename := fmt.Sprintf("%04d-%s-%s.jsonl", l.Iteration, prefix, nowFunc().UTC().Format("20060102T15-04Z"))
	path := filepath.Join(l.LogDir, filename)

	var err error
	l.file, err = os.Create(path)
	if err != nil {
		return fmt.Errorf("creating log file %s: %w", filename, err)
	}
	return nil
}

// LogIterationStart emits an iteration_start record that marks a session
// boundary. The log command uses these records to separate output into
// per-iteration sections. stageType is the kind of work ("execute",
// "plan", "intake") and nodeAddr is the tree address being worked on
// (empty string is acceptable for stages like intake that aren't
// node-scoped).
func (l *Logger) LogIterationStart(stageType, nodeAddr string) error {
	record := map[string]any{
		"type":      "iteration_start",
		"stage":     stageType,
		"iteration": l.Iteration,
	}
	if nodeAddr != "" {
		record["node"] = nodeAddr
	}
	return l.Log(record)
}

// Log writes a structured record to the current iteration's log file.
// The level parameter is optional: if omitted, the record is logged at
// LevelInfo (backward compatible per ADR-046).
//
// A nil receiver is treated as "no active iteration" so tests that
// build a Daemon directly without calling New — and any future caller
// that holds a nil *Logger reference — are safe from panics. The
// silent-drop canary still counts these and writes to stderr so the
// nil path isn't invisible.
func (l *Logger) Log(record map[string]any, levels ...Level) error {
	if l == nil || l.file == nil {
		// Silent-drop canary. Parallel mode introduced a class of
		// bugs where worker content was logged to a Logger whose
		// file had never been opened, and every call site's `_ =`
		// swallowed the "no active iteration" error. The next time
		// that regression ships, this line makes it visible in the
		// daemon.log tail instead of hiding behind empty log files.
		trace := ""
		if l != nil {
			trace = l.TraceID
		}
		fmt.Fprintf(os.Stderr, "wolfcastle: log record dropped (no active iteration): type=%v trace=%q\n",
			record["type"], trace)
		droppedRecords.Add(1)
		return fmt.Errorf("no active iteration")
	}

	level := LevelInfo
	if len(levels) > 0 {
		level = levels[0]
	}

	record["timestamp"] = nowFunc().UTC().Format(time.RFC3339)
	record["level"] = level.String()
	if l.TraceID != "" {
		record["trace"] = l.TraceID
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshaling log record: %w", err)
	}
	if _, err := l.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("writing log record: %w", err)
	}

	return nil
}

// AssistantWriter returns an io.Writer that formats each line written
// to it as an NDJSON record with type "assistant" at debug level and
// writes it to the current log file. Returns nil if no iteration is
// active.
func (l *Logger) AssistantWriter() io.Writer {
	if l.file == nil {
		return nil
	}
	return &assistantLogWriter{logger: l}
}

type assistantLogWriter struct {
	logger *Logger
}

func (w *assistantLogWriter) Write(p []byte) (int, error) {
	text := string(p)
	err := w.logger.Log(map[string]any{
		"type": "assistant",
		"text": text,
	}, LevelDebug)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close closes the current iteration's log file.
func (l *Logger) Close() {
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

// CurrentLogPath returns the path to the currently open log file, or
// an empty string if no iteration is active.
func (l *Logger) CurrentLogPath() string {
	if l.file == nil {
		return ""
	}
	return l.file.Name()
}

// LatestLogFile returns the path to the most recent log file from
// the daemon's exec stream specifically. Files are named
// <NNNN>-<prefix>-<timestamp>.jsonl[.gz] where prefix is one of
// exec, intake, inbox-init, etc. The exec stream is the canonical
// "what is the daemon doing" log; intake and inbox-init are
// administrative side-channels.
//
// Selection rules:
//
//  1. Among exec files, pick the one with the highest iteration
//     number. Plain .jsonl files take precedence over .jsonl.gz at
//     the same iteration (the .jsonl is the live, uncompressed file
//     and the .gz is the rotated archive).
//  2. If no exec files exist at all, fall back to lex-max across
//     all log files. This preserves the old behavior for log dirs
//     that contain only intake or inbox-init files (e.g. a daemon
//     that has only ever processed inbox events and never run an
//     execute iteration).
//
// The previous implementation used sort.Strings across all log
// files, which fails for log dirs that contain a mix of
// counter-prefixed traces because lex order is dominated by the
// leading-digit width: with exec at 0279 and inbox-init at 10168,
// "10168" sorts after "0279" and the wrong file wins.
func LatestLogFile(logDir string) (string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return "", fmt.Errorf("reading log directory: %w", err)
	}

	type execFile struct {
		name      string
		iteration int
		gz        bool
	}
	var execs []execFile
	var allLogs []string

	for _, e := range entries {
		if e.IsDir() || !IsLogFile(e.Name()) {
			continue
		}
		allLogs = append(allLogs, e.Name())
		iter, prefix, ok := parseLogFilename(e.Name())
		if !ok || prefix != "exec" {
			continue
		}
		execs = append(execs, execFile{
			name:      e.Name(),
			iteration: iter,
			gz:        strings.HasSuffix(e.Name(), ".gz"),
		})
	}

	if len(execs) > 0 {
		// Sort by iteration descending, plain .jsonl before .jsonl.gz
		// at the same iteration.
		sort.Slice(execs, func(i, j int) bool {
			if execs[i].iteration != execs[j].iteration {
				return execs[i].iteration > execs[j].iteration
			}
			return !execs[i].gz && execs[j].gz
		})
		return filepath.Join(logDir, execs[0].name), nil
	}

	if len(allLogs) == 0 {
		return "", fmt.Errorf("no log files found")
	}
	sort.Strings(allLogs)
	return filepath.Join(logDir, allLogs[len(allLogs)-1]), nil
}

// parseLogFilename pulls the iteration number and trace prefix out
// of a log filename of the form <NNNN>-<prefix>-<timestamp>.jsonl[.gz].
// Returns ok=false for filenames that don't fit the pattern.
func parseLogFilename(name string) (iteration int, prefix string, ok bool) {
	// Strip the .jsonl[.gz] suffix.
	base := name
	switch {
	case strings.HasSuffix(base, ".jsonl.gz"):
		base = strings.TrimSuffix(base, ".jsonl.gz")
	case strings.HasSuffix(base, ".jsonl"):
		base = strings.TrimSuffix(base, ".jsonl")
	default:
		return 0, "", false
	}
	// Split into <NNNN>-<prefix>-<timestamp>.
	parts := strings.SplitN(base, "-", 3)
	if len(parts) < 3 {
		return 0, "", false
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}
	// The prefix may itself contain a hyphen (inbox-init), in which
	// case parts[1] is "inbox" and parts[2] starts with "init-...".
	// Detect this by checking whether parts[2] starts with "init-".
	prefix = parts[1]
	if prefix == "inbox" && strings.HasPrefix(parts[2], "init-") {
		prefix = "inbox-init"
	}
	return n, prefix, true
}

// IsLogFile reports whether name looks like a log filename (.jsonl or .jsonl.gz).
func IsLogFile(name string) bool {
	return strings.HasSuffix(name, ".jsonl") || strings.HasSuffix(name, ".jsonl.gz")
}

// EnforceRetention deletes old log files based on max count and age,
// then optionally compresses remaining files if compress is true.
func EnforceRetention(logDir string, maxFiles int, maxAgeDays int, opts ...RetentionOption) error {
	ro := retentionOpts{compress: false, quietWindow: compressionQuietWindow}
	for _, o := range opts {
		o(&ro)
	}

	now := nowFunc()
	quietCutoff := now.Add(-ro.quietWindow)

	// collect captures every log file along with its mtime. Sorting by
	// mtime — rather than filename — is what keeps parallel workers
	// safe: a worker logger's filename starts with "0001-..." because
	// its child iteration counter is fresh, so an alphabetical sort
	// treats brand-new worker files as the oldest in the directory
	// and retention would happily delete or compress them first.
	collect := func() []logFileInfo {
		entries, err := os.ReadDir(logDir)
		if err != nil {
			return nil
		}
		out := make([]logFileInfo, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !IsLogFile(e.Name()) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			out = append(out, logFileInfo{name: e.Name(), modTime: info.ModTime()})
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i].modTime.Before(out[j].modTime)
		})
		return out
	}

	logs := collect()
	if logs == nil {
		return fmt.Errorf("reading log directory for retention: unreadable")
	}

	ageCutoff := now.AddDate(0, 0, -maxAgeDays)

	// Delete by age. Files still in the quiet window are skipped so an
	// active worker's file can never be removed out from under it.
	for _, l := range logs {
		if l.modTime.After(quietCutoff) {
			continue
		}
		if l.modTime.Before(ageCutoff) {
			_ = os.Remove(filepath.Join(logDir, l.name))
		}
	}

	// Re-read and delete by count (oldest mtime first, skipping any
	// file still in the quiet window).
	logs = collect()
	if len(logs) > maxFiles {
		stale := make([]logFileInfo, 0, len(logs))
		for _, l := range logs {
			if l.modTime.After(quietCutoff) {
				continue
			}
			stale = append(stale, l)
		}
		// Delete from the oldest stale files until either the budget
		// is met or we run out of stale candidates. Fresh files are
		// never removed even if that leaves the dir temporarily
		// over budget — retention will catch up on the next tick.
		overBy := len(logs) - maxFiles
		if overBy > len(stale) {
			overBy = len(stale)
		}
		for _, l := range stale[:overBy] {
			_ = os.Remove(filepath.Join(logDir, l.name))
		}
	}

	// Compress surviving uncompressed files, skipping any file whose
	// mtime is within compressionQuietWindow. In parallel mode several
	// workers write concurrently, and compressing a file still held
	// open by another worker unlinks it mid-write and silently loses
	// the content. We also keep the single mtime-newest stale file
	// uncompressed so the sequential daemon's tail stays readable on
	// disk without a gzip round-trip.
	if ro.compress {
		logs = collect()
		var stale []logFileInfo
		for _, l := range logs {
			if !strings.HasSuffix(l.name, ".jsonl") {
				continue
			}
			if l.modTime.After(quietCutoff) {
				continue
			}
			stale = append(stale, l)
		}
		if len(stale) > 1 {
			// collect() sorted by mtime ascending, so the mtime-newest
			// stale file is at the end — skip it.
			for _, l := range stale[:len(stale)-1] {
				src := filepath.Join(logDir, l.name)
				if err := compressFile(src); err != nil {
					// Non-fatal: the file simply stays uncompressed.
					continue
				}
			}
		}
	}

	return nil
}

// logFileInfo is the minimal view of a log file used by retention: we
// only need the basename (to open/delete it) and the mtime (to sort and
// gate against the compression quiet window).
type logFileInfo struct {
	name    string
	modTime time.Time
}

// compressionQuietWindow is the duration a log file must be idle (no
// writes) before retention is allowed to compress or delete it. In
// parallel mode, compressing or unlinking a file still held open by a
// worker orphans the content, so retention backs off until the file
// has been quiescent.
const compressionQuietWindow = 30 * time.Second

// droppedRecords counts records dropped because Logger.Log was called
// without an active iteration file. Exposed via DroppedRecords for
// daemon health checks and test assertions; incremented atomically
// because workers hold independent Logger instances.
var droppedRecords atomic.Uint64

// DroppedRecords returns the total number of log records silently
// dropped because no iteration file was open when Log was invoked.
// A healthy daemon should return 0 at all times; any non-zero value
// means a code path is calling Log on an uninitialized Logger.
func DroppedRecords() uint64 {
	return droppedRecords.Load()
}

// RetentionOption configures optional retention behaviour.
type RetentionOption func(*retentionOpts)

type retentionOpts struct {
	compress    bool
	quietWindow time.Duration
}

// WithCompression enables gzip compression of old log files.
func WithCompression() RetentionOption {
	return func(o *retentionOpts) {
		o.compress = true
	}
}

// WithQuietWindow overrides the default quiet window that retention
// uses to decide when a file is safe to compress or delete. The daemon
// relies on the default (see compressionQuietWindow) so active workers
// never have their open files removed; tests that want retention to
// operate on fresh fixtures can pass WithQuietWindow(0).
func WithQuietWindow(d time.Duration) RetentionOption {
	return func(o *retentionOpts) {
		o.quietWindow = d
	}
}

// compressFile gzip-compresses src and removes the original on success.
func compressFile(src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	dst := src + ".gz"
	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		_ = gz.Close()
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := gz.Close(); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	_ = in.Close()
	return os.Remove(src)
}

// WatchForNewFiles polls logDir for the appearance of a log file with a
// higher iteration number than currentPath. It blocks until a new file
// appears or the done channel is closed. Returns the path to the new
// file, or an empty string if done was closed first.
func WatchForNewFiles(logDir string, currentPath string, done <-chan struct{}, pollInterval time.Duration) string {
	for {
		select {
		case <-done:
			return ""
		default:
		}
		latest, err := LatestLogFile(logDir)
		if err == nil && latest != currentPath {
			return latest
		}
		select {
		case <-done:
			return ""
		case <-time.After(pollInterval):
		}
	}
}

// IterationFromDir scans existing log files in logDir and returns the
// highest iteration number found, so a new Logger can resume numbering.
func IterationFromDir(logDir string) int {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 0
	}
	maxIter := 0
	for _, e := range entries {
		if e.IsDir() || !IsLogFile(e.Name()) {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(e.Name(), "%04d-", &n); err == nil && n > maxIter {
			maxIter = n
		}
	}
	return maxIter
}
