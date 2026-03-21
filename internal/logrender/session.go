package logrender

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Session represents a single daemon run: a sequence of log files starting
// from iteration 1 and continuing through consecutive iterations until the
// next iteration-1 file (or end of directory).
type Session struct {
	// Files holds the log file paths in iteration order (ascending).
	Files []string
}

// ListSessions scans logDir for NDJSON log files, groups them into sessions
// by finding iteration-1 boundaries, and returns sessions ordered
// most-recent-first. Both .jsonl and .jsonl.gz files are included.
func ListSessions(logDir string) ([]Session, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, fmt.Errorf("reading log directory: %w", err)
	}

	// Collect log filenames and sort by timestamp first, iteration second.
	// Filenames are NNNN-YYYYMMDDTHH-MMZZ.jsonl; sorting by the timestamp
	// portion (index 5 onward) groups files from the same daemon run together,
	// and within a run the iteration number provides the correct ordering.
	var names []string
	for _, e := range entries {
		if !e.IsDir() && isLogFile(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool {
		ti, tj := timestampKey(names[i]), timestampKey(names[j])
		if ti != tj {
			return ti < tj
		}
		return parseIteration(names[i]) < parseIteration(names[j])
	})

	// Walk the sorted list. Every file whose iteration number is 1 starts a
	// new session; all subsequent files belong to that session until the next
	// iteration-1 file.
	var sessions []Session
	for _, name := range names {
		iter := parseIteration(name)
		if iter == 1 || len(sessions) == 0 {
			sessions = append(sessions, Session{})
		}
		sessions[len(sessions)-1].Files = append(
			sessions[len(sessions)-1].Files,
			filepath.Join(logDir, name),
		)
	}

	// Reverse so the most recent session is first.
	for i, j := 0, len(sessions)-1; i < j; i, j = i+1, j-1 {
		sessions[i], sessions[j] = sessions[j], sessions[i]
	}

	return sessions, nil
}

// ResolveSession returns the session at the given index, where 0 is the most
// recent session, 1 is the previous, and so on. Returns an error if the index
// is out of range or if no sessions exist.
func ResolveSession(logDir string, index int) (Session, error) {
	sessions, err := ListSessions(logDir)
	if err != nil {
		return Session{}, err
	}
	if len(sessions) == 0 {
		return Session{}, fmt.Errorf("no log sessions found")
	}
	if index < 0 || index >= len(sessions) {
		return Session{}, fmt.Errorf("session index %d out of range (0..%d)", index, len(sessions)-1)
	}
	return sessions[index], nil
}

// isLogFile returns true for filenames ending in .jsonl or .jsonl.gz.
func isLogFile(name string) bool {
	return strings.HasSuffix(name, ".jsonl") || strings.HasSuffix(name, ".jsonl.gz")
}

// timestampKey extracts the timestamp portion of a log filename for sorting.
// Given "0001-20260321T18-04Z.jsonl", it returns "20260321T18-04Z.jsonl"
// (everything after the first hyphen). Falls back to the full name if the
// expected format isn't found.
func timestampKey(name string) string {
	if idx := strings.IndexByte(name, '-'); idx >= 0 && idx < len(name)-1 {
		return name[idx+1:]
	}
	return name
}

// parseIteration extracts the leading iteration number from a log filename.
// Filenames follow the pattern NNNN-YYYYMMDDTHH-MMZZ.jsonl where NNNN is
// a zero-padded 4-digit iteration counter. Returns 0 if the name doesn't
// match the expected format.
func parseIteration(name string) int {
	if len(name) < 4 {
		return 0
	}
	n := 0
	for i := 0; i < 4; i++ {
		c := name[i]
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
