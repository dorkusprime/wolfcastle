package config

import (
	"path/filepath"
	"testing"
)

func TestIdentityFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantNS  string
		wantErr bool
	}{
		{
			name:    "nil identity",
			cfg:     &Config{},
			wantErr: true,
		},
		{
			name: "empty user",
			cfg: &Config{
				Identity: &IdentityConfig{Machine: "laptop"},
			},
			wantErr: true,
		},
		{
			name: "empty machine",
			cfg: &Config{
				Identity: &IdentityConfig{User: "alice"},
			},
			wantErr: true,
		},
		{
			name: "valid identity",
			cfg: &Config{
				Identity: &IdentityConfig{User: "alice", Machine: "laptop"},
			},
			wantNS: "alice-laptop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := IdentityFromConfig(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id.User != tt.cfg.Identity.User {
				t.Errorf("User = %q, want %q", id.User, tt.cfg.Identity.User)
			}
			if id.Machine != tt.cfg.Identity.Machine {
				t.Errorf("Machine = %q, want %q", id.Machine, tt.cfg.Identity.Machine)
			}
			if id.Namespace != tt.wantNS {
				t.Errorf("Namespace = %q, want %q", id.Namespace, tt.wantNS)
			}
		})
	}
}

func TestDetectIdentity(t *testing.T) {
	id := DetectIdentity()

	if id == nil {
		t.Fatal("DetectIdentity returned nil")
	}
	if id.User == "" {
		t.Error("User should not be empty")
	}
	if id.Machine == "" {
		t.Error("Machine should not be empty")
	}
	if id.Namespace == "" {
		t.Error("Namespace should not be empty")
	}
	if id.Namespace != id.User+"-"+id.Machine {
		t.Errorf("Namespace = %q, want %q", id.Namespace, id.User+"-"+id.Machine)
	}
}

func TestIdentity_ProjectsDir(t *testing.T) {
	id := &Identity{
		User:      "alice",
		Machine:   "laptop",
		Namespace: "alice-laptop",
	}

	got := id.ProjectsDir("/root/.wolfcastle")
	want := filepath.Join("/root/.wolfcastle", "system", "projects", "alice-laptop")
	if got != want {
		t.Errorf("ProjectsDir = %q, want %q", got, want)
	}
}
