package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStructToMap_ValidStruct(t *testing.T) {
	t.Parallel()
	type sample struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	m, err := structToMap(sample{Name: "test", Count: 42})
	if err != nil {
		t.Fatal(err)
	}
	if m["name"] != "test" {
		t.Errorf("expected name=test, got %v", m["name"])
	}
}

func TestStructToMap_UnmarshalableInput(t *testing.T) {
	t.Parallel()
	// A channel cannot be marshaled
	_, err := structToMap(make(chan int))
	if err == nil {
		t.Error("expected error for unmarshable input")
	}
}

func TestLoad_InvalidConfigJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system", "base"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "config.json"), []byte("not json"), 0644)

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid base/config.json")
	}
}

func TestLoad_InvalidConfigLocalJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Valid base/config.json
	_ = os.MkdirAll(filepath.Join(dir, "system", "base"), 0755)
	cfg := Defaults()
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "config.json"), data, 0644)
	// Invalid local/config.json
	_ = os.MkdirAll(filepath.Join(dir, "system", "local"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "system", "local", "config.json"), []byte("bad json"), 0644)

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid local/config.json")
	}
}

func TestLoad_NoConfigFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Error("expected non-nil config from defaults")
	}
}
