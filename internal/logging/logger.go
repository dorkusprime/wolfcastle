// Package logging provides per-iteration NDJSON log files for the
// Wolfcastle daemon. Each daemon iteration writes to its own .jsonl
// file, and log retention is enforced by file count and age.
// Console output is filtered by a configurable log level while NDJSON
// files always capture everything (ADR-012, ADR-037, ADR-046).
package logging

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// Logger writes per-iteration NDJSON log files and optionally mirrors
// human-readable output to a console writer.
type Logger struct {
	LogDir       string
	Iteration    int
	ConsoleLevel Level
	Console      io.Writer // nil disables console output

	file *os.File
}

// NewLogger creates a logger for the given log directory.
func NewLogger(logDir string) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}
	return &Logger{
		LogDir:       logDir,
		ConsoleLevel: LevelInfo,
		Console:      os.Stderr,
	}, nil
}

// StartIteration creates a new log file for the current iteration.
// Closes any previously open file before starting a new one.
func (l *Logger) StartIteration() error {
	l.Close() // prevent file handle leak if called without Close()
	l.Iteration++
	filename := fmt.Sprintf("%04d-%s.jsonl", l.Iteration, nowFunc().UTC().Format("20060102T15-04Z"))
	path := filepath.Join(l.LogDir, filename)

	var err error
	l.file, err = os.Create(path)
	if err != nil {
		return fmt.Errorf("creating log file %s: %w", filename, err)
	}
	return nil
}

// Log writes a structured record to the current iteration's log file.
// The level parameter is optional: if omitted, the record is logged at
// LevelInfo (backward compatible per ADR-046).
func (l *Logger) Log(record map[string]any, levels ...Level) error {
	if l.file == nil {
		return fmt.Errorf("no active iteration")
	}

	level := LevelInfo
	if len(levels) > 0 {
		level = levels[0]
	}

	record["timestamp"] = nowFunc().UTC().Format(time.RFC3339)
	record["level"] = level.String()
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshaling log record: %w", err)
	}
	if _, err := l.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("writing log record: %w", err)
	}

	// Mirror to console if the level meets the threshold (ADR-037).
	if l.Console != nil && level >= l.ConsoleLevel {
		l.writeConsole(record, level)
	}
	return nil
}

// writeConsole formats a record as a terse, human-readable line and
// writes it to the console writer.
func (l *Logger) writeConsole(record map[string]any, level Level) {
	prefix := strings.ToUpper(level.String())
	msg, _ := record["message"].(string)
	stage, _ := record["stage"].(string)

	if msg == "" {
		// Fall back to "type" field for records that use the old schema.
		msg, _ = record["type"].(string)
	}

	if stage != "" {
		_, _ = fmt.Fprintf(l.Console, "[%s] %s: %s\n", prefix, stage, msg)
	} else {
		_, _ = fmt.Fprintf(l.Console, "[%s] %s\n", prefix, msg)
	}
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

// LatestLogFile returns the path to the most recent log file. Both
// plain .jsonl and compressed .jsonl.gz files are considered, but
// uncompressed files take precedence when iteration numbers match.
func LatestLogFile(logDir string) (string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return "", fmt.Errorf("reading log directory: %w", err)
	}
	var logs []string
	for _, e := range entries {
		if !e.IsDir() && isLogFile(e.Name()) {
			logs = append(logs, e.Name())
		}
	}
	if len(logs) == 0 {
		return "", fmt.Errorf("no log files found")
	}
	sort.Strings(logs)
	return filepath.Join(logDir, logs[len(logs)-1]), nil
}

// isLogFile returns true for .jsonl and .jsonl.gz filenames.
func isLogFile(name string) bool {
	return strings.HasSuffix(name, ".jsonl") || strings.HasSuffix(name, ".jsonl.gz")
}

// EnforceRetention deletes old log files based on max count and age,
// then optionally compresses remaining files if compress is true.
func EnforceRetention(logDir string, maxFiles int, maxAgeDays int, opts ...RetentionOption) error {
	ro := retentionOpts{compress: false}
	for _, o := range opts {
		o(&ro)
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("reading log directory for retention: %w", err)
	}
	var logs []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && isLogFile(e.Name()) {
			logs = append(logs, e)
		}
	}

	// Sort by name (which sorts by iteration number + timestamp)
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Name() < logs[j].Name()
	})

	cutoff := nowFunc().AddDate(0, 0, -maxAgeDays)

	// Delete by age
	for _, l := range logs {
		info, err := l.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(logDir, l.Name()))
		}
	}

	// Re-read and delete by count
	entries, _ = os.ReadDir(logDir)
	logs = nil
	for _, e := range entries {
		if !e.IsDir() && isLogFile(e.Name()) {
			logs = append(logs, e)
		}
	}
	if len(logs) > maxFiles {
		sort.Slice(logs, func(i, j int) bool {
			return logs[i].Name() < logs[j].Name()
		})
		for _, l := range logs[:len(logs)-maxFiles] {
			_ = os.Remove(filepath.Join(logDir, l.Name()))
		}
	}

	// Compress surviving uncompressed files (excluding the newest, which
	// may still be actively written to).
	if ro.compress {
		entries, _ = os.ReadDir(logDir)
		var uncompressed []os.DirEntry
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
				uncompressed = append(uncompressed, e)
			}
		}
		// Keep the newest uncompressed file open — compress only older ones.
		if len(uncompressed) > 1 {
			sort.Slice(uncompressed, func(i, j int) bool {
				return uncompressed[i].Name() < uncompressed[j].Name()
			})
			for _, e := range uncompressed[:len(uncompressed)-1] {
				src := filepath.Join(logDir, e.Name())
				if err := compressFile(src); err != nil {
					// Non-fatal — the file simply stays uncompressed.
					continue
				}
			}
		}
	}

	return nil
}

// RetentionOption configures optional retention behaviour.
type RetentionOption func(*retentionOpts)

type retentionOpts struct {
	compress bool
}

// WithCompression enables gzip compression of old log files.
func WithCompression() RetentionOption {
	return func(o *retentionOpts) {
		o.compress = true
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
		if e.IsDir() || !isLogFile(e.Name()) {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(e.Name(), "%04d-", &n); err == nil && n > maxIter {
			maxIter = n
		}
	}
	return maxIter
}
