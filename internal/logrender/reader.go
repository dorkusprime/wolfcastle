package logrender

import (
	"bufio"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ReplayReader reads NDJSON records from a fixed set of log files and delivers
// them through a channel. Handles both plain .jsonl and compressed .jsonl.gz
// files transparently. Malformed lines are silently skipped.
type ReplayReader struct {
	files []string
}

// NewReplayReader creates a reader that will replay records from the given
// file paths in order. Files are read sequentially; each file's lines are
// parsed and emitted before moving to the next.
func NewReplayReader(files []string) *ReplayReader {
	return &ReplayReader{files: files}
}

// Records returns a channel of parsed records. The channel closes when all
// files have been read. The caller should range over the channel.
func (r *ReplayReader) Records() <-chan Record {
	ch := make(chan Record, 64)
	go func() {
		defer close(ch)
		for _, path := range r.files {
			r.readFile(path, ch)
		}
	}()
	return ch
}

// readFile opens a single log file (decompressing .gz if needed), scans it
// line by line, and sends parsed records to ch. Malformed lines are dropped.
func (r *ReplayReader) readFile(path string, ch chan<- Record) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	var reader io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return
		}
		defer gz.Close()
		reader = gz
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		rec, err := ParseRecord(line)
		if err != nil {
			continue
		}
		ch <- rec
	}
}

// FollowReader tails a log directory, polling for new files and new lines at a
// configurable interval. It detects new iteration files as they appear and
// streams records from them in order. Cancelling the context stops the reader.
type FollowReader struct {
	dir      string
	interval time.Duration
}

// NewFollowReader creates a reader that tails logDir for new NDJSON content.
// The poll interval controls how frequently the reader checks for new lines
// and new files. A zero or negative interval defaults to 200ms.
func NewFollowReader(logDir string, interval time.Duration) *FollowReader {
	if interval <= 0 {
		interval = 200 * time.Millisecond
	}
	return &FollowReader{dir: logDir, interval: interval}
}

// Records returns a channel of parsed records that stays open until ctx is
// cancelled. New records appear as the daemon appends lines to existing files
// or creates new iteration files.
func (r *FollowReader) Records(ctx context.Context) <-chan Record {
	ch := make(chan Record, 64)
	go r.poll(ctx, ch)
	return ch
}

// fileState tracks read progress for a single log file.
type fileState struct {
	path   string
	offset int64
}

// poll is the main loop: it scans the log directory for files, reads new
// content from each, and sleeps between cycles until the context expires.
func (r *FollowReader) poll(ctx context.Context, ch chan<- Record) {
	defer close(ch)

	tracked := make(map[string]*fileState)
	var orderedPaths []string

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run one cycle immediately before waiting on the ticker.
	r.cycle(tracked, &orderedPaths, ch)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.cycle(tracked, &orderedPaths, ch)
		}
	}
}

// cycle performs a single poll iteration: discover new files, then read new
// content from every tracked file in order.
func (r *FollowReader) cycle(tracked map[string]*fileState, orderedPaths *[]string, ch chan<- Record) {
	// Discover any new log files in the directory.
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && isLogFile(e.Name()) && !strings.HasSuffix(e.Name(), ".gz") {
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

	for _, name := range names {
		fullPath := filepath.Join(r.dir, name)
		if _, ok := tracked[fullPath]; !ok {
			fs := &fileState{path: fullPath}
			tracked[fullPath] = fs
			*orderedPaths = append(*orderedPaths, fullPath)
		}
	}

	// Read new content from each tracked file in iteration order.
	for _, p := range *orderedPaths {
		fs := tracked[p]
		r.readNewLines(fs, ch)
	}
}

// readNewLines reads any content appended since the last read and sends
// parsed records to ch. Only complete lines (terminated by \n) are consumed;
// a partial trailing line is left for the next poll cycle.
func (r *FollowReader) readNewLines(fs *fileState, ch chan<- Record) {
	f, err := os.Open(fs.path)
	if err != nil {
		return
	}
	defer f.Close()

	// Read all new bytes from the last-known offset.
	if _, err := f.Seek(fs.offset, io.SeekStart); err != nil {
		return
	}
	data, err := io.ReadAll(f)
	if err != nil || len(data) == 0 {
		return
	}

	// Process only complete lines. If the last chunk doesn't end with a
	// newline, it's a partial write; leave it for the next cycle.
	consumed := 0
	for len(data[consumed:]) > 0 {
		nl := indexOf(data[consumed:], '\n')
		if nl < 0 {
			break // partial line, stop here
		}
		line := string(data[consumed : consumed+nl])
		consumed += nl + 1 // skip past the newline

		if line == "" || line == "\r" {
			continue
		}
		// Trim trailing \r for Windows-style line endings.
		if line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		rec, err := ParseRecord(line)
		if err != nil {
			continue
		}
		ch <- rec
	}
	fs.offset += int64(consumed)
}

// indexOf returns the index of the first occurrence of b in data, or -1.
func indexOf(data []byte, b byte) int {
	for i, v := range data {
		if v == b {
			return i
		}
	}
	return -1
}
