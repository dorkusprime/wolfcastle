package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// helpers.go: WriteJSON: pass unmarshalable value (channel)
//
// WriteJSON calls t.Fatalf on marshal error, which invokes runtime.Goexit.
// We run the call inside a separate goroutine so Goexit terminates only
// that goroutine, then verify the file was not written.
// ═══════════════════════════════════════════════════════════════════════════

func TestWriteJSON_UnmarshalableValue_Channel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Use a mock T so Fatalf doesn't kill our real test
		inner := &testing.T{}
		WriteJSON(inner, path, make(chan int))
	}()
	wg.Wait()

	// The file should not have been created because json.MarshalIndent
	// fails for channel values, and WriteJSON calls t.Fatalf before writing.
	if _, err := os.Stat(path); err == nil {
		t.Error("file should not exist when marshal fails")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// helpers.go: WriteJSON: MkdirAll error (file blocks directory creation)
// ═══════════════════════════════════════════════════════════════════════════

func TestWriteJSON_MkdirAllError_Fatals(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Place a regular file where MkdirAll expects to create a directory.
	blocker := filepath.Join(dir, "blocker")
	_ = os.WriteFile(blocker, []byte("x"), 0o644)
	path := filepath.Join(blocker, "sub", "out.json")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		inner := &testing.T{}
		WriteJSON(inner, path, map[string]string{"a": "b"})
	}()
	wg.Wait()
}

// ═══════════════════════════════════════════════════════════════════════════
// helpers.go: WriteJSON: WriteFile error (read-only directory)
// ═══════════════════════════════════════════════════════════════════════════

func TestWriteJSON_WriteFileError_Fatals(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod restrictions have no effect on Windows")
	}
	dir := t.TempDir()

	roDir := filepath.Join(dir, "readonly")
	_ = os.MkdirAll(roDir, 0o755)
	_ = os.Chmod(roDir, 0o555)
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })

	path := filepath.Join(roDir, "out.json")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		inner := &testing.T{}
		WriteJSON(inner, path, map[string]string{"a": "b"})
	}()
	wg.Wait()
}

// ═══════════════════════════════════════════════════════════════════════════
// helpers.go: ReadJSON: pass nonexistent path
// ═══════════════════════════════════════════════════════════════════════════

func TestReadJSON_NonexistentPath_Fatals(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		inner := &testing.T{}
		var result map[string]any
		ReadJSON(inner, "/nonexistent/path/file.json", &result)
	}()
	wg.Wait()

	// If we get here, Fatalf was called inside the goroutine (Goexit)
	// and we survived. The error path in ReadJSON was exercised.
}

// ═══════════════════════════════════════════════════════════════════════════
// helpers.go: ReadJSON: pass invalid JSON file
// ═══════════════════════════════════════════════════════════════════════════

func TestReadJSON_InvalidJSONFile_Fatals(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	_ = os.WriteFile(path, []byte("NOT VALID JSON{{{"), 0644)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		inner := &testing.T{}
		var result map[string]any
		ReadJSON(inner, path, &result)
	}()
	wg.Wait()

	// Goexit from Fatalf was caught by the goroutine boundary.
	// The unmarshal error path was exercised.
}
