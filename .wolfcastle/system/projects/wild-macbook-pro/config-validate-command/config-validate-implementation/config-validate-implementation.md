# Config Validate Implementation

Implement the 'wolfcastle config validate' CLI command. Wires existing three-tier config loading and ValidateStructure/Validate from internal/config into a cobra command under cmd/config/. Supports human-readable and JSON output, exits 0 on clean config, non-zero on errors. Includes tests and CLI documentation.
