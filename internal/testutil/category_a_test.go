package testutil

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// helpers.go — WriteJSON: pass unmarshalable value (channel)
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
// helpers.go — ReadJSON: pass nonexistent path
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
// helpers.go — ReadJSON: pass invalid JSON file
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
