package tree

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// ResolveNamespace returns the engineer namespace directory name.
func ResolveNamespace(cfg *config.Config) (string, error) {
	if cfg.Identity == nil {
		return "", fmt.Errorf("identity not configured")
	}
	if cfg.Identity.User == "" || cfg.Identity.Machine == "" {
		return "", fmt.Errorf("identity.user and identity.machine must both be set")
	}
	return cfg.Identity.User + "-" + cfg.Identity.Machine, nil
}
