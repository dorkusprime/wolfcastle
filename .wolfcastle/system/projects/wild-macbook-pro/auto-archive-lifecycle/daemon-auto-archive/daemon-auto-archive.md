# Daemon Auto-Archive

Integrate the auto-archive timer into the daemon loop. The daemon periodically scans for completed top-level orchestrators whose last activity timestamp exceeds the configured delay threshold (default 24h). Eligible nodes are automatically archived using the archive service. The timer runs as part of the daemon's main loop or as a separate goroutine (following the pattern established by the inbox goroutine in ADR-064). Must not interfere with active task execution.
