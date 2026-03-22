# Documentation Update

Update all human-facing documentation to accurately describe the new daemon-driven git commit behavior. Every reference to agents committing, Phase H, or manual git operations during execution must be replaced with the new model: the daemon commits deterministically after every task iteration, controlled by git config fields.
