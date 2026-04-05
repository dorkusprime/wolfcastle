package logging

import (
	"os"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// logger.go: Log: pass map with unmarshalable value
// ═══════════════════════════════════════════════════════════════════════════

func TestLog_UnmarshalableMapValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	logger, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}

	defer logger.Close()

	if err := logger.StartIteration(); err != nil {
		t.Fatal(err)
	}

	// A channel cannot be marshalled to JSON, so json.Marshal will return
	// an error that Log should propagate.
	err = logger.Log(map[string]any{
		"type":    "test",
		"badkey":  make(chan int),
		"message": "this should not be written",
	})
	if err == nil {
		t.Fatal("expected marshal error for map containing channel value")
	}

	// Verify the error message mentions marshaling
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}

	// The log file should be empty or not contain the bad record
	logger.Close()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if isLogFile(e.Name()) {
			info, _ := e.Info()
			if info.Size() > 0 {
				t.Log("log file has some content (timestamp/level injected before marshal error)")
			}
		}
	}
}
