package config

import (
	"encoding/json"
	"testing"
)

func TestValidation_ParallelMaxWorkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		maxWorkers int
		wantErr    bool
	}{
		{name: "zero fails", maxWorkers: 0, wantErr: true},
		{name: "one passes", maxWorkers: 1, wantErr: false},
		{name: "five passes", maxWorkers: 5, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := Defaults()
			cfg.Daemon.Parallel.MaxWorkers = tt.maxWorkers

			err := ValidateStructure(cfg)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for max_workers=%d", tt.maxWorkers)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error for max_workers=%d, got: %v", tt.maxWorkers, err)
			}
		})
	}
}

func TestValidation_AuditRequireTests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "block passes", value: "block", wantErr: false},
		{name: "warn passes", value: "warn", wantErr: false},
		{name: "skip passes", value: "skip", wantErr: false},
		{name: "empty passes", value: "", wantErr: false},
		{name: "invalid fails", value: "ignore", wantErr: true},
		{name: "typo fails", value: "Block", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := Defaults()
			cfg.Audit.RequireTests = tt.value

			err := ValidateStructure(cfg)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for require_tests=%q", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error for require_tests=%q, got: %v", tt.value, err)
			}
		})
	}
}

func TestDeepMerge_ParallelConfig(t *testing.T) {
	t.Parallel()

	base, err := structToMap(Defaults())
	if err != nil {
		t.Fatal(err)
	}

	overlay := map[string]any{
		"daemon": map[string]any{
			"parallel": map[string]any{
				"max_workers": float64(5),
			},
		},
	}

	merged := DeepMerge(base, overlay)

	data, err := json.Marshal(merged)
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Daemon.Parallel.MaxWorkers != 5 {
		t.Errorf("expected max_workers=5 after merge, got %d", cfg.Daemon.Parallel.MaxWorkers)
	}
	if cfg.Daemon.Parallel.Enabled != false {
		t.Error("expected enabled to remain false (default) after merging only max_workers")
	}
}
