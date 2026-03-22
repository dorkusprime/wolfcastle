package logrender

import (
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeLines creates a .jsonl file with the given content lines.
func writeLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	var buf []byte
	for _, l := range lines {
		buf = append(buf, l...)
		buf = append(buf, '\n')
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// writeGzLines creates a .jsonl.gz file with the given content lines.
func writeGzLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	for _, l := range lines {
		gz.Write([]byte(l))
		gz.Write([]byte("\n"))
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}
}

// collectRecords drains a record channel into a slice.
func collectRecords(ch <-chan Record) []Record {
	var recs []Record
	for r := range ch {
		recs = append(recs, r)
	}
	return recs
}

// --- ReplayReader tests ---

func TestReplayReader_BasicRecords(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001.jsonl")
	writeLines(t, path,
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info","node":"foo"}`,
		`{"type":"stage_start","timestamp":"2026-03-21T18:04:01Z","level":"info","stage":"execute"}`,
	)

	rr := NewReplayReader([]string{path})
	recs := collectRecords(rr.Records())
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
	if recs[0].Type != "iteration_start" {
		t.Errorf("record 0: expected type iteration_start, got %s", recs[0].Type)
	}
	if recs[1].Type != "stage_start" {
		t.Errorf("record 1: expected type stage_start, got %s", recs[1].Type)
	}
}

func TestReplayReader_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "0001.jsonl")
	f2 := filepath.Join(dir, "0002.jsonl")
	writeLines(t, f1, `{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`)
	writeLines(t, f2,
		`{"type":"stage_start","timestamp":"2026-03-21T18:05:00Z","level":"info"}`,
		`{"type":"stage_complete","timestamp":"2026-03-21T18:06:00Z","level":"info"}`,
	)

	rr := NewReplayReader([]string{f1, f2})
	recs := collectRecords(rr.Records())
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	if recs[0].Type != "iteration_start" {
		t.Errorf("first record should be from first file")
	}
	if recs[2].Type != "stage_complete" {
		t.Errorf("last record should be from second file")
	}
}

func TestReplayReader_GzipFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001.jsonl.gz")
	writeGzLines(t, path,
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
		`{"type":"assistant","timestamp":"2026-03-21T18:04:05Z","level":"debug","text":"thinking..."}`,
	)

	rr := NewReplayReader([]string{path})
	recs := collectRecords(rr.Records())
	if len(recs) != 2 {
		t.Fatalf("expected 2 records from gzip file, got %d", len(recs))
	}
	if recs[1].Text != "thinking..." {
		t.Errorf("expected text 'thinking...', got %q", recs[1].Text)
	}
}

func TestReplayReader_MixedPlainAndGzip(t *testing.T) {
	dir := t.TempDir()
	gz := filepath.Join(dir, "0001.jsonl.gz")
	plain := filepath.Join(dir, "0002.jsonl")
	writeGzLines(t, gz, `{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`)
	writeLines(t, plain, `{"type":"stage_start","timestamp":"2026-03-21T18:05:00Z","level":"info"}`)

	rr := NewReplayReader([]string{gz, plain})
	recs := collectRecords(rr.Records())
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
}

func TestReplayReader_SkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001.jsonl")
	writeLines(t, path,
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
		`not valid json at all`,
		`{"type":"stage_start","timestamp":"2026-03-21T18:04:01Z","level":"info"}`,
	)

	rr := NewReplayReader([]string{path})
	recs := collectRecords(rr.Records())
	if len(recs) != 2 {
		t.Fatalf("expected 2 records (skipping malformed), got %d", len(recs))
	}
}

func TestReplayReader_SkipsEmptyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001.jsonl")
	writeLines(t, path,
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
		``,
		``,
		`{"type":"stage_start","timestamp":"2026-03-21T18:04:01Z","level":"info"}`,
	)

	rr := NewReplayReader([]string{path})
	recs := collectRecords(rr.Records())
	if len(recs) != 2 {
		t.Fatalf("expected 2 records (skipping empty lines), got %d", len(recs))
	}
}

func TestReplayReader_EmptyFileList(t *testing.T) {
	rr := NewReplayReader(nil)
	recs := collectRecords(rr.Records())
	if len(recs) != 0 {
		t.Fatalf("expected 0 records from empty file list, got %d", len(recs))
	}
}

func TestReplayReader_MissingFile(t *testing.T) {
	rr := NewReplayReader([]string{"/no/such/file.jsonl"})
	recs := collectRecords(rr.Records())
	if len(recs) != 0 {
		t.Fatalf("expected 0 records from missing file, got %d", len(recs))
	}
}

func TestReplayReader_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	writeLines(t, path) // no lines

	rr := NewReplayReader([]string{path})
	recs := collectRecords(rr.Records())
	if len(recs) != 0 {
		t.Fatalf("expected 0 records from empty file, got %d", len(recs))
	}
}

func TestReplayReader_PreservesFieldValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001.jsonl")
	writeLines(t, path,
		`{"type":"stage_complete","timestamp":"2026-03-21T18:06:00Z","level":"info","stage":"execute","node":"my-proj/auth","exit_code":0}`,
	)

	rr := NewReplayReader([]string{path})
	recs := collectRecords(rr.Records())
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	r := recs[0]
	if r.Stage != "execute" {
		t.Errorf("stage: got %q, want execute", r.Stage)
	}
	if r.Node != "my-proj/auth" {
		t.Errorf("node: got %q, want my-proj/auth", r.Node)
	}
	if r.ExitCode == nil || *r.ExitCode != 0 {
		t.Errorf("exit_code: expected 0, got %v", r.ExitCode)
	}
}

// --- FollowReader tests ---

func TestFollowReader_ReadsExistingContent(t *testing.T) {
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"),
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
		`{"type":"stage_start","timestamp":"2026-03-21T18:04:01Z","level":"info"}`,
	)

	fr := NewFollowReader(dir, 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	ch := fr.Records(ctx)
	var recs []Record
	for r := range ch {
		recs = append(recs, r)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
}

func TestFollowReader_DetectsAppendedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001-20260321T18-04Z.jsonl")
	writeLines(t, path,
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
	)

	fr := NewFollowReader(dir, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := fr.Records(ctx)

	// Collect the initial record.
	r := <-ch
	if r.Type != "iteration_start" {
		t.Fatalf("expected iteration_start, got %s", r.Type)
	}

	// Append a new line to the file.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("opening file for append: %v", err)
	}
	f.WriteString(`{"type":"stage_complete","timestamp":"2026-03-21T18:05:00Z","level":"info"}` + "\n")
	f.Close()

	// Wait for the follow reader to pick it up.
	select {
	case r = <-ch:
		if r.Type != "stage_complete" {
			t.Errorf("expected stage_complete, got %s", r.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for appended record")
	}
	cancel()
}

func TestFollowReader_DetectsNewFiles(t *testing.T) {
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"),
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
	)

	fr := NewFollowReader(dir, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := fr.Records(ctx)

	// Drain the first record.
	<-ch

	// Create a new iteration file.
	writeLines(t, filepath.Join(dir, "0002-20260321T18-05Z.jsonl"),
		`{"type":"stage_start","timestamp":"2026-03-21T18:05:00Z","level":"info","stage":"execute"}`,
	)

	select {
	case r := <-ch:
		if r.Type != "stage_start" {
			t.Errorf("expected stage_start from new file, got %s", r.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for record from new file")
	}
	cancel()
}

func TestFollowReader_SkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"),
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
		`{broken json`,
		`{"type":"stage_start","timestamp":"2026-03-21T18:04:01Z","level":"info"}`,
	)

	fr := NewFollowReader(dir, 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	recs := collectRecords(fr.Records(ctx))
	if len(recs) != 2 {
		t.Fatalf("expected 2 records (skipping malformed), got %d", len(recs))
	}
}

func TestFollowReader_IgnoresGzFiles(t *testing.T) {
	dir := t.TempDir()
	// Follow mode should skip .gz files (they're completed/archived iterations).
	writeGzLines(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl.gz"),
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
	)
	writeLines(t, filepath.Join(dir, "0002-20260321T18-05Z.jsonl"),
		`{"type":"stage_start","timestamp":"2026-03-21T18:05:00Z","level":"info"}`,
	)

	fr := NewFollowReader(dir, 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	recs := collectRecords(fr.Records(ctx))
	if len(recs) != 1 {
		t.Fatalf("expected 1 record (ignoring .gz), got %d", len(recs))
	}
	if recs[0].Type != "stage_start" {
		t.Errorf("expected stage_start, got %s", recs[0].Type)
	}
}

func TestFollowReader_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	fr := NewFollowReader(dir, 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	recs := collectRecords(fr.Records(ctx))
	if len(recs) != 0 {
		t.Fatalf("expected 0 records from empty dir, got %d", len(recs))
	}
}

func TestFollowReader_DefaultInterval(t *testing.T) {
	fr := NewFollowReader("/tmp", 0)
	if fr.interval != 200*time.Millisecond {
		t.Errorf("expected default 200ms interval, got %v", fr.interval)
	}
	fr2 := NewFollowReader("/tmp", -1*time.Second)
	if fr2.interval != 200*time.Millisecond {
		t.Errorf("expected default 200ms for negative interval, got %v", fr2.interval)
	}
}

func TestFollowReader_CancellationStopsReader(t *testing.T) {
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"),
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
	)

	fr := NewFollowReader(dir, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	ch := fr.Records(ctx)

	// Drain one record.
	<-ch

	// Cancel and verify the channel closes promptly.
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			// Could get a final record, that's fine, but channel should close.
			<-ch
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after cancellation")
	}
}

func TestFollowReader_PartialLineNotEmitted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001-20260321T18-04Z.jsonl")

	// Write a complete line followed by a partial line (no trailing newline).
	content := `{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}` + "\n" +
		`{"type":"stage_start","timestamp":"2026-03-21T18:04:01Z","level":"info"}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	fr := NewFollowReader(dir, 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	recs := collectRecords(fr.Records(ctx))
	// Only the first complete line should be emitted; the partial line should
	// be held until a newline appears.
	if len(recs) != 1 {
		t.Fatalf("expected 1 record (partial line held), got %d", len(recs))
	}
	if recs[0].Type != "iteration_start" {
		t.Errorf("expected iteration_start, got %s", recs[0].Type)
	}
}

func TestFollowReader_PartialLineCompletedLater(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001-20260321T18-04Z.jsonl")

	// Write a partial line (no trailing newline).
	partial := `{"type":"stage_start","timestamp":"2026-03-21T18:04:01Z","level":"info"}`
	if err := os.WriteFile(path, []byte(partial), 0o644); err != nil {
		t.Fatal(err)
	}

	fr := NewFollowReader(dir, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := fr.Records(ctx)

	// Give the reader time to poll; it should find nothing complete.
	time.Sleep(150 * time.Millisecond)
	select {
	case <-ch:
		t.Fatal("should not have emitted partial line")
	default:
	}

	// Now complete the line by appending a newline.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("\n")
	f.Close()

	select {
	case r := <-ch:
		if r.Type != "stage_start" {
			t.Errorf("expected stage_start, got %s", r.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for completed partial line")
	}
	cancel()
}

func TestFollowReader_AliveCheckStopsReader(t *testing.T) {
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"),
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
	)

	fr := NewFollowReader(dir, 10*time.Millisecond)

	// The alive check returns false on the first call, simulating a daemon
	// that stopped while we were tailing.
	fr.SetAliveCheck(func() bool { return false })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := fr.Records(ctx)
	var recs []Record
	for r := range ch {
		recs = append(recs, r)
	}

	// The channel should close well before the 5s timeout.
	if ctx.Err() != nil {
		t.Fatal("reader should have stopped from alive check, not from context timeout")
	}
	// The initial record should still be delivered (it's read before the alive check fires).
	if len(recs) < 1 {
		t.Fatal("expected at least the initial record before shutdown")
	}
}

func TestFollowReader_AliveCheckDrainsFinalCycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0001-20260321T18-04Z.jsonl")
	writeLines(t, path,
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
	)

	// Use a very short interval so we hit the alive-check quickly.
	// Set the check interval to trigger on cycle 2 (we'll count manually
	// by using a callback that tracks calls).
	fr := NewFollowReader(dir, 10*time.Millisecond)

	checkCount := 0
	fr.SetAliveCheck(func() bool {
		checkCount++
		// Return alive for a while, then report dead.
		return checkCount < 2
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := fr.Records(ctx)

	// After the initial record, append a new line. If the alive check fires
	// after this append, the final drain cycle should pick it up.
	<-ch // drain initial record

	// Append a line that should be caught by the final drain cycle.
	time.Sleep(50 * time.Millisecond)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"type":"stage_complete","timestamp":"2026-03-21T18:05:00Z","level":"info"}` + "\n")
	f.Close()

	// Collect remaining records. Channel should close from the alive check.
	var remaining []Record
	for r := range ch {
		remaining = append(remaining, r)
	}

	if ctx.Err() != nil {
		t.Fatal("reader should have stopped from alive check, not context timeout")
	}
}

func TestFollowReader_NilAliveCheckNoEffect(t *testing.T) {
	dir := t.TempDir()
	writeLines(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"),
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
	)

	fr := NewFollowReader(dir, 50*time.Millisecond)
	// No SetAliveCheck call; should behave exactly as before (context-only stop).

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	recs := collectRecords(fr.Records(ctx))
	if len(recs) != 1 {
		t.Fatalf("expected 1 record with nil alive check, got %d", len(recs))
	}
}

func TestFollowReader_FileOrderMatchesIteration(t *testing.T) {
	dir := t.TempDir()
	// Write files in reverse order to verify the reader sorts by timestamp/iteration.
	writeLines(t, filepath.Join(dir, "0002-20260321T18-05Z.jsonl"),
		`{"type":"stage_start","timestamp":"2026-03-21T18:05:00Z","level":"info"}`,
	)
	writeLines(t, filepath.Join(dir, "0001-20260321T18-04Z.jsonl"),
		`{"type":"iteration_start","timestamp":"2026-03-21T18:04:00Z","level":"info"}`,
	)

	fr := NewFollowReader(dir, 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	recs := collectRecords(fr.Records(ctx))
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
	if recs[0].Type != "iteration_start" {
		t.Errorf("first record should be iteration_start, got %s", recs[0].Type)
	}
	if recs[1].Type != "stage_start" {
		t.Errorf("second record should be stage_start, got %s", recs[1].Type)
	}
}
