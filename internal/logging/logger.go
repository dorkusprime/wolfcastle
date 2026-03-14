package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Logger writes per-iteration NDJSON log files.
type Logger struct {
	LogDir    string
	Iteration int
	file      *os.File
}

// NewLogger creates a logger for the given log directory.
func NewLogger(logDir string) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}
	return &Logger{LogDir: logDir}, nil
}

// StartIteration creates a new log file for the current iteration.
// Closes any previously open file before starting a new one.
func (l *Logger) StartIteration() error {
	l.Close() // prevent file handle leak if called without Close()
	l.Iteration++
	filename := fmt.Sprintf("%04d-%s.jsonl", l.Iteration, time.Now().UTC().Format("20060102T15-04Z"))
	path := filepath.Join(l.LogDir, filename)

	var err error
	l.file, err = os.Create(path)
	return err
}

// Log writes a structured record to the current iteration's log file.
func (l *Logger) Log(record map[string]any) error {
	if l.file == nil {
		return fmt.Errorf("no active iteration")
	}
	record["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = l.file.Write(append(data, '\n'))
	return err
}

// AssistantWriter returns an io.Writer that formats each line written to it
// as an NDJSON record with type "assistant" and writes it to the current log file.
// Returns nil if no iteration is active.
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
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close closes the current iteration's log file.
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

// LatestLogFile returns the path to the most recent log file.
func LatestLogFile(logDir string) (string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return "", err
	}
	var logs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			logs = append(logs, e.Name())
		}
	}
	if len(logs) == 0 {
		return "", fmt.Errorf("no log files found")
	}
	sort.Strings(logs)
	return filepath.Join(logDir, logs[len(logs)-1]), nil
}

// EnforceRetention deletes old log files based on max count and age.
func EnforceRetention(logDir string, maxFiles int, maxAgeDays int) error {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}
	var logs []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			logs = append(logs, e)
		}
	}

	// Sort by name (which sorts by iteration number + timestamp)
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Name() < logs[j].Name()
	})

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	// Delete by age
	for _, l := range logs {
		info, err := l.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(logDir, l.Name()))
		}
	}

	// Re-read and delete by count
	entries, _ = os.ReadDir(logDir)
	logs = nil
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			logs = append(logs, e)
		}
	}
	if len(logs) > maxFiles {
		sort.Slice(logs, func(i, j int) bool {
			return logs[i].Name() < logs[j].Name()
		})
		for _, l := range logs[:len(logs)-maxFiles] {
			os.Remove(filepath.Join(logDir, l.Name()))
		}
	}

	return nil
}
