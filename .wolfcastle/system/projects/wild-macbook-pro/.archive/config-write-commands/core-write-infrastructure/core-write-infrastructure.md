# Core Write Infrastructure

Build the shared infrastructure that all four config write commands depend on: dot-notation path utilities for navigating and mutating nested map[string]any structures, and a read-modify-write-validate flow that reads a tier overlay, applies a mutation, writes it back, validates the merged result, and rolls back on validation failure. Lives in internal/config/.
