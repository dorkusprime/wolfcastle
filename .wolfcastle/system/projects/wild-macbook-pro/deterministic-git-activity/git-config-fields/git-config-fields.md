# Git Config Fields

Add new git config fields (commit_on_success, commit_on_failure, commit_state) to the config struct, defaults, and validation. These fields gate the daemon's commit behavior and must exist before the daemon logic can reference them.
