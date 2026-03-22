# Git Config Fields

Add commit_on_success, commit_on_failure, and commit_state fields to GitConfig. Wire defaults in config.Defaults(), add validation in config.Validate(), and handle migration for existing configs that lack the new fields. The spec field skip_hooks maps to the existing SkipHooksOnAutoCommit but may need renaming for consistency.
