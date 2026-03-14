package output

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPrint_WritesJSONToStdout(t *testing.T) {
	// Capture stdout
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	resp := Ok("test-print", map[string]string{"key": "value"})
	Print(resp)

	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	output := string(buf[:n])

	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput was: %s", err, output)
	}
	if decoded["ok"] != true {
		t.Errorf("expected ok=true, got %v", decoded["ok"])
	}
	if decoded["action"] != "test-print" {
		t.Errorf("expected action='test-print', got %v", decoded["action"])
	}
	data, ok := decoded["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data to be an object")
	}
	if data["key"] != "value" {
		t.Errorf("expected data.key='value', got %v", data["key"])
	}
}

func TestPrint_ErrorResponse(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	resp := Err("fail", 1, "broken")
	Print(resp)

	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	output := string(buf[:n])

	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput was: %s", err, output)
	}
	if decoded["ok"] != false {
		t.Errorf("expected ok=false, got %v", decoded["ok"])
	}
	if decoded["error"] != "broken" {
		t.Errorf("expected error='broken', got %v", decoded["error"])
	}
	if decoded["code"] != float64(1) {
		t.Errorf("expected code=1, got %v", decoded["code"])
	}
}

func TestPrint_IndentedOutput(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	resp := Ok("indent-test", nil)
	Print(resp)

	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	output := string(buf[:n])

	// Verify the output is indented (contains newlines and spaces)
	if !strings.Contains(output, "\n") {
		t.Error("expected indented JSON output with newlines")
	}
	if !strings.Contains(output, "  ") {
		t.Error("expected indented JSON output with spaces")
	}
}

func TestPrintHuman_WritesFormattedOutput(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	PrintHuman("Hello, %s! Count: %d", "world", 42)

	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	output := string(buf[:n])

	expected := "Hello, world! Count: 42\n"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestPrintHuman_NoArgs(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	PrintHuman("simple message")

	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	output := string(buf[:n])

	expected := "simple message\n"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestPrintError_WritesToStderr(t *testing.T) {
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	PrintError("something failed: %s", "reason")

	w.Close()
	os.Stderr = origStderr

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	output := string(buf[:n])

	expected := "Error: something failed: reason\n"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestPrintError_NoArgs(t *testing.T) {
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	PrintError("simple error")

	w.Close()
	os.Stderr = origStderr

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	output := string(buf[:n])

	expected := "Error: simple error\n"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestPrint_NilData(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	resp := Ok("nil-data", nil)
	Print(resp)

	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	r.Close()
	output := string(buf[:n])

	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded["ok"] != true {
		t.Errorf("expected ok=true, got %v", decoded["ok"])
	}
	// data should not be present when nil
	if _, exists := decoded["data"]; exists {
		t.Error("expected data field to be omitted when nil")
	}
}
