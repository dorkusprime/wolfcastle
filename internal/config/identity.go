package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// Identity is the domain type representing a resolved user+machine pair.
// IdentityConfig in types.go remains the config-tier representation;
// Identity is what the rest of the system works with.
type Identity struct {
	User      string
	Machine   string
	Namespace string
}

// IdentityFromConfig extracts an Identity from a loaded Config.
func IdentityFromConfig(cfg *Config) (*Identity, error) {
	if cfg.Identity == nil {
		return nil, fmt.Errorf("identity not configured")
	}
	if cfg.Identity.User == "" {
		return nil, fmt.Errorf("identity.user must be set")
	}
	if cfg.Identity.Machine == "" {
		return nil, fmt.Errorf("identity.machine must be set")
	}
	return &Identity{
		User:      cfg.Identity.User,
		Machine:   cfg.Identity.Machine,
		Namespace: cfg.Identity.User + "-" + cfg.Identity.Machine,
	}, nil
}

// DetectIdentity reads the current username and hostname from the OS,
// falling back to "unknown" for either if the system call fails.
func DetectIdentity() *Identity {
	username := "unknown"
	machine := "unknown"

	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	if h, err := os.Hostname(); err == nil {
		if idx := strings.IndexByte(h, '.'); idx > 0 {
			h = h[:idx]
		}
		machine = strings.ToLower(h)
	}

	return &Identity{
		User:      username,
		Machine:   machine,
		Namespace: username + "-" + machine,
	}
}

// ProjectsDir returns the projects directory for this identity's namespace.
func (id *Identity) ProjectsDir(wolfcastleRoot string) string {
	return filepath.Join(wolfcastleRoot, "system", "projects", id.Namespace)
}
