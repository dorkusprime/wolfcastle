package config

import (
	"testing"
)

func TestParsePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    []string
		wantErr bool
	}{
		{name: "single segment", path: "version", want: []string{"version"}},
		{name: "two segments", path: "models.fast", want: []string{"models", "fast"}},
		{name: "three segments", path: "models.fast.command", want: []string{"models", "fast", "command"}},
		{name: "empty path", path: "", wantErr: true},
		{name: "trailing dot", path: "models.", wantErr: true},
		{name: "leading dot", path: ".models", wantErr: true},
		{name: "double dot", path: "models..fast", wantErr: true},
		{name: "bracket open", path: "models[0]", wantErr: true},
		{name: "bracket in middle", path: "models.items[0].name", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for path %q, got nil", tt.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("segment %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetPath(t *testing.T) {
	m := map[string]any{
		"version": 1,
		"models": map[string]any{
			"fast": map[string]any{
				"command": "claude",
				"args":    []any{"--fast"},
			},
		},
	}

	tests := []struct {
		name   string
		path   string
		wantOK bool
		want   any
	}{
		{name: "top-level key", path: "version", wantOK: true, want: 1},
		{name: "nested map", path: "models.fast.command", wantOK: true, want: "claude"},
		{name: "intermediate map", path: "models.fast", wantOK: true},
		{name: "missing top-level", path: "nonexistent", wantOK: false},
		{name: "missing nested", path: "models.slow", wantOK: false},
		{name: "path through scalar", path: "version.sub", wantOK: false},
		{name: "invalid path", path: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := GetPath(m, tt.path)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.want != nil && got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetPath(t *testing.T) {
	t.Run("set top-level key", func(t *testing.T) {
		m := map[string]any{}
		if err := SetPath(m, "version", 2); err != nil {
			t.Fatal(err)
		}
		if m["version"] != 2 {
			t.Errorf("got %v, want 2", m["version"])
		}
	})

	t.Run("set nested key creating intermediates", func(t *testing.T) {
		m := map[string]any{}
		if err := SetPath(m, "models.fast.command", "claude"); err != nil {
			t.Fatal(err)
		}
		val, ok := GetPath(m, "models.fast.command")
		if !ok || val != "claude" {
			t.Errorf("got (%v, %v), want (claude, true)", val, ok)
		}
	})

	t.Run("overwrite existing key", func(t *testing.T) {
		m := map[string]any{"version": 1}
		if err := SetPath(m, "version", 2); err != nil {
			t.Fatal(err)
		}
		if m["version"] != 2 {
			t.Errorf("got %v, want 2", m["version"])
		}
	})

	t.Run("error on non-map intermediate", func(t *testing.T) {
		m := map[string]any{"version": 1}
		err := SetPath(m, "version.sub", "nope")
		if err == nil {
			t.Fatal("expected error when intermediate is not a map")
		}
	})

	t.Run("error on invalid path", func(t *testing.T) {
		m := map[string]any{}
		err := SetPath(m, "", "val")
		if err == nil {
			t.Fatal("expected error for empty path")
		}
	})
}

func TestDeletePath(t *testing.T) {
	t.Run("delete top-level key", func(t *testing.T) {
		m := map[string]any{"version": 1, "other": "keep"}
		if err := DeletePath(m, "version"); err != nil {
			t.Fatal(err)
		}
		if m["version"] != nil {
			t.Errorf("expected nil, got %v", m["version"])
		}
		if m["other"] != "keep" {
			t.Error("other key should be untouched")
		}
	})

	t.Run("delete nested key", func(t *testing.T) {
		m := map[string]any{
			"models": map[string]any{
				"fast": map[string]any{
					"command": "claude",
				},
			},
		}
		if err := DeletePath(m, "models.fast.command"); err != nil {
			t.Fatal(err)
		}
		inner := m["models"].(map[string]any)["fast"].(map[string]any)
		if inner["command"] != nil {
			t.Errorf("expected nil, got %v", inner["command"])
		}
	})

	t.Run("delete nonexistent intermediate is no-op", func(t *testing.T) {
		m := map[string]any{"version": 1}
		if err := DeletePath(m, "nonexistent.deep.path"); err != nil {
			t.Fatalf("expected no error for missing intermediate, got %v", err)
		}
	})

	t.Run("error on non-map intermediate", func(t *testing.T) {
		m := map[string]any{"version": 1}
		err := DeletePath(m, "version.sub")
		if err == nil {
			t.Fatal("expected error when intermediate is not a map")
		}
	})

	t.Run("error on invalid path", func(t *testing.T) {
		m := map[string]any{}
		err := DeletePath(m, "")
		if err == nil {
			t.Fatal("expected error for empty path")
		}
	})
}
