# Daemon Commit Logic

Implement the daemon-side commit flow: commit on success (alongside existing failure path), staging area preservation, and respect for the new git config fields. This is the core behavior change from agent-driven to daemon-driven commits.
