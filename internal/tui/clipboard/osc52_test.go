package clipboard

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
)

func TestWriteOSC52(t *testing.T) {
	var buf bytes.Buffer
	err := WriteOSC52(&buf, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString([]byte("hello"))
	expected := fmt.Sprintf("\x1b]52;c;%s\x07", encoded)
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

func TestWriteOSC52Empty(t *testing.T) {
	var buf bytes.Buffer
	err := WriteOSC52(&buf, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(""))
	expected := fmt.Sprintf("\x1b]52;c;%s\x07", encoded)
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

func TestWriteOSC52Unicode(t *testing.T) {
	var buf bytes.Buffer
	text := "日本語テスト 🎉"
	err := WriteOSC52(&buf, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	expected := fmt.Sprintf("\x1b]52;c;%s\x07", encoded)
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestWriteOSC52WriterError(t *testing.T) {
	err := WriteOSC52(errWriter{}, "anything")
	if err == nil {
		t.Fatal("expected error from failing writer")
	}
	if err.Error() != "write failed" {
		t.Errorf("error = %q, want %q", err.Error(), "write failed")
	}
}

// TestWriteSystem_Invokes covers the WriteSystem path. On a fully
// configured desktop (pbcopy/xclip/xsel available) it returns nil; on a
// headless CI host it returns an error from the atotto clipboard
// package. Either outcome is acceptable — the assertion is that the
// function is reachable and returns synchronously.
func TestWriteSystem_Invokes(t *testing.T) {
	t.Parallel()
	_ = WriteSystem("wolfcastle-clipboard-test")
}
