# follow-no-op-when-stopped

Fix the log command entry point so that an explicit --follow flag is treated as a no-op when the daemon is not running at invocation time. Currently, the implicit-follow logic in follow.go correctly checks IsAlive() before activating follow mode, but an explicit --follow bypasses that entirely. The fix: when --follow is set (explicit or implicit) but daemon is not alive, fall through to replay mode instead of entering FollowReader. This matches the spec which says --follow is 'No-op when daemon is stopped.'
