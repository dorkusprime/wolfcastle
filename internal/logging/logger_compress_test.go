package logging

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════
// compressFile: successful compression and read error paths
// ═══════════════════════════════════════════════════════════════════════════

func TestCompressFile_SuccessfulRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "roundtrip.jsonl")
	content := strings.Repeat(`{"level":"info","message":"test entry"}`+"\n", 50)
	_ = os.WriteFile(src, []byte(content), 0644)

	if err := compressFile(src); err != nil {
		t.Fatal(err)
	}

	// Verify original removed
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("original file should be removed after compression")
	}

	// Verify .gz exists and decompresses to original content
	f, err := os.Open(src + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = gz.Close() }()

	data, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("decompressed content mismatch: expected %d bytes, got %d", len(content), len(data))
	}
}

func TestCompressFile_ReadErrorUnreadableSource(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 has no effect on Windows")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "unreadable.jsonl")
	_ = os.WriteFile(src, []byte("secret data\n"), 0644)

	// Make file unreadable
	_ = os.Chmod(src, 0000)
	defer func() { _ = os.Chmod(src, 0644) }()

	err := compressFile(src)
	if err == nil {
		t.Error("expected error when source file is unreadable")
	}

	// No .gz should be created
	if _, statErr := os.Stat(src + ".gz"); !os.IsNotExist(statErr) {
		t.Error("no .gz file should exist when source is unreadable")
	}
}

func TestCompressFile_DestExistsAsDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "blocked.jsonl")
	_ = os.WriteFile(src, []byte("data\n"), 0644)

	// Pre-create destination as a directory to block os.Create
	_ = os.MkdirAll(src+".gz", 0755)

	err := compressFile(src)
	if err == nil {
		t.Error("expected error when .gz destination is a directory")
	}
}

func TestCompressFile_SingleByte(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "tiny.jsonl")
	_ = os.WriteFile(src, []byte("x"), 0644)

	if err := compressFile(src); err != nil {
		t.Fatal(err)
	}

	// Verify it decompresses correctly
	f, err := os.Open(src + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(gz)
	_ = gz.Close()
	if string(data) != "x" {
		t.Errorf("expected 'x', got %q", data)
	}
}
