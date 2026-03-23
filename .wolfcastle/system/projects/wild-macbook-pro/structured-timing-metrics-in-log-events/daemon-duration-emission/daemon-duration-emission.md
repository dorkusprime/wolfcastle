# Daemon Duration Emission

Track stage start times and emit a duration_ms field in stage_complete and planning_complete NDJSON records. Each emission point (iteration.go execute stages, stages.go intake, planning.go planning) must record time.Now() at stage_start, compute elapsed milliseconds at completion, and include duration_ms as an integer in the log map. This is the foundational change that all downstream consumers depend on.
