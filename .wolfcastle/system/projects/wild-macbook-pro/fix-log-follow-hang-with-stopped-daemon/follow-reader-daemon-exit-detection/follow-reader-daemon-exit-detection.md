# follow-reader-daemon-exit-detection

Add daemon-exit detection to FollowReader so that an active follow session stops polling when the daemon process dies. Currently, FollowReader.poll() cycles forever until the context is cancelled (Ctrl+C). If the daemon crashes or stops cleanly while a follow session is active, the reader should detect the process is gone and close the records channel. This prevents the secondary hang scenario where the daemon was alive at invocation but dies afterward.
