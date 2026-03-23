# Add Commit Prefix Config

Add a CommitPrefix field (json: commit_prefix) to GitConfig in internal/config/types.go, with a default value of 'wolfcastle' in Defaults(). This field controls the prefix used in daemon-generated commit messages, allowing users to customize or remove it. Update validation to accept the new field.
