# add-comment

Add inline comment to tryCleanStaleLock documenting the PID recycling edge case: a new process reusing the dead process's PID within the 50ms polling window causes stale detection to incorrectly conclude the lock is held, resulting in timeout (not corruption).
