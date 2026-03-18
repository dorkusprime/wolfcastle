package tierfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a test helper that creates a file with the given content,
// creating parent directories as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(root string) // populate tier dirs
		rel     string
		want    string
		wantErr bool
		notExist bool
	}{
		{
			name: "local wins over custom and base",
			setup: func(root string) {
				writeFile(t, filepath.Join(root, "base", "f.md"), "base")
				writeFile(t, filepath.Join(root, "custom", "f.md"), "custom")
				writeFile(t, filepath.Join(root, "local", "f.md"), "local")
			},
			rel:  "f.md",
			want: "local",
		},
		{
			name: "custom wins when local is absent",
			setup: func(root string) {
				writeFile(t, filepath.Join(root, "base", "f.md"), "base")
				writeFile(t, filepath.Join(root, "custom", "f.md"), "custom")
			},
			rel:  "f.md",
			want: "custom",
		},
		{
			name: "falls through to base",
			setup: func(root string) {
				writeFile(t, filepath.Join(root, "base", "f.md"), "base")
			},
			rel:  "f.md",
			want: "base",
		},
		{
			name: "not exist when no tier has file",
			setup: func(root string) {
				// create tier dirs but no file
				os.MkdirAll(filepath.Join(root, "base"), 0o755)
			},
			rel:      "missing.md",
			wantErr:  true,
			notExist: true,
		},
		{
			name: "subdirectory path",
			setup: func(root string) {
				writeFile(t, filepath.Join(root, "base", "prompts", "execute.md"), "base-exec")
				writeFile(t, filepath.Join(root, "local", "prompts", "execute.md"), "local-exec")
			},
			rel:  "prompts/execute.md",
			want: "local-exec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(root)
			fs := New(root)

			got, err := fs.Resolve(tt.rel)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.notExist && !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("expected os.ErrNotExist, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveAll(t *testing.T) {
	tests := []struct {
		name  string
		setup func(root string)
		sub   string
		want  map[string]string // filename -> content
	}{
		{
			name: "collects across tiers",
			setup: func(root string) {
				writeFile(t, filepath.Join(root, "base", "prompts", "a.md"), "base-a")
				writeFile(t, filepath.Join(root, "custom", "prompts", "b.md"), "custom-b")
				writeFile(t, filepath.Join(root, "local", "prompts", "c.md"), "local-c")
			},
			sub: "prompts",
			want: map[string]string{
				"a.md": "base-a",
				"b.md": "custom-b",
				"c.md": "local-c",
			},
		},
		{
			name: "higher tier overrides lower",
			setup: func(root string) {
				writeFile(t, filepath.Join(root, "base", "prompts", "a.md"), "base-a")
				writeFile(t, filepath.Join(root, "local", "prompts", "a.md"), "local-a")
			},
			sub: "prompts",
			want: map[string]string{
				"a.md": "local-a",
			},
		},
		{
			name: "skips directories and non-md files",
			setup: func(root string) {
				writeFile(t, filepath.Join(root, "base", "prompts", "good.md"), "content")
				writeFile(t, filepath.Join(root, "base", "prompts", "skip.txt"), "nope")
				writeFile(t, filepath.Join(root, "base", "prompts", "subdir", "nested.md"), "nested")
			},
			sub: "prompts",
			want: map[string]string{
				"good.md": "content",
			},
		},
		{
			name: "empty map when subdir missing everywhere",
			setup: func(root string) {
				os.MkdirAll(filepath.Join(root, "base"), 0o755)
			},
			sub:  "nonexistent",
			want: map[string]string{},
		},
		{
			name: "handles partial tier presence",
			setup: func(root string) {
				// only base has the subdir
				writeFile(t, filepath.Join(root, "base", "frags", "one.md"), "one")
			},
			sub: "frags",
			want: map[string]string{
				"one.md": "one",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(root)
			fs := New(root)

			got, err := fs.ResolveAll(tt.sub)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.want))
			}
			for k, wantV := range tt.want {
				gotV, ok := got[k]
				if !ok {
					t.Errorf("missing key %q", k)
					continue
				}
				if string(gotV) != wantV {
					t.Errorf("key %q: got %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

func TestWriteBase(t *testing.T) {
	tests := []struct {
		name    string
		rel     string
		content string
	}{
		{
			name:    "writes file to base tier",
			rel:     "config.md",
			content: "hello",
		},
		{
			name:    "creates parent directories",
			rel:     "deep/nested/dir/file.md",
			content: "nested content",
		},
		{
			name:    "overwrites existing file",
			rel:     "existing.md",
			content: "new content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			fs := New(root)

			// For the overwrite test, create the file first.
			if tt.name == "overwrites existing file" {
				writeFile(t, filepath.Join(root, "base", tt.rel), "old content")
			}

			if err := fs.WriteBase(tt.rel, []byte(tt.content)); err != nil {
				t.Fatalf("WriteBase: %v", err)
			}

			got, err := os.ReadFile(filepath.Join(root, "base", tt.rel))
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			if string(got) != tt.content {
				t.Errorf("got %q, want %q", got, tt.content)
			}
		})
	}
}

func TestBasePath(t *testing.T) {
	root := t.TempDir()
	fs := New(root)

	got := fs.BasePath("prompts")
	want := filepath.Join(root, "base", "prompts")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTierDirs(t *testing.T) {
	root := t.TempDir()
	fs := New(root)

	dirs := fs.TierDirs()
	if len(dirs) != 3 {
		t.Fatalf("got %d dirs, want 3", len(dirs))
	}

	want := []string{
		filepath.Join(root, "base"),
		filepath.Join(root, "custom"),
		filepath.Join(root, "local"),
	}
	for i, w := range want {
		if dirs[i] != w {
			t.Errorf("dirs[%d] = %q, want %q", i, dirs[i], w)
		}
	}
}
