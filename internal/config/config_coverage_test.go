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
	_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte("not json"), 0644)

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid config.json")
	}
}

func TestLoad_InvalidConfigLocalJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Valid config.json
	cfg := Defaults()
	data, _ := json.Marshal(cfg)
	_ = os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)
	// Invalid config.local.json
	_ = os.WriteFile(filepath.Join(dir, "config.local.json"), []byte("bad json"), 0644)

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid config.local.json")
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
