# CLI Write Commands

Implement the four config write CLI commands (set, unset, append, remove) as cobra subcommands under 'wolfcastle config'. Each command uses the core write infrastructure (path utilities and ApplyMutation) to modify tier overlay files. Includes registration in the config command group and comprehensive tests.
