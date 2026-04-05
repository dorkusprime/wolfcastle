# ADR-073: Follow-to-Log Rename

## Status
Accepted

## Date
2026-03-16

## Context

The `wolfcastle follow` command streamed daemon logs in real time. The name was descriptive enough for that single behavior, but it had no room to grow. There was no way to view recent logs without tailing them live. If the daemon had already finished a run and you wanted to see what happened, `follow` could not help you. You had to go find the NDJSON files yourself.

The name also broke convention. `git log` shows history. `docker logs` shows container output. `journalctl` shows service logs. Every tool in the ecosystem uses "log" or "logs" for this purpose. "follow" is what you call the flag, not the command.

## Decision

`wolfcastle follow` is renamed to `wolfcastle log`.

Without flags, `wolfcastle log` displays recent log entries and exits. The output is formatted for human reading, same as the daemon's console output.

With `--follow` or `-f`, `wolfcastle log` streams logs in real time, reproducing the original `follow` behavior.

The old `wolfcastle follow` name is kept as a hidden alias. Existing scripts and muscle memory continue to work. The alias is not advertised in help text.

## Consequences

- `wolfcastle log` handles both historical review and live streaming in one command. The flag determines the mode.
- The CLI surface aligns with conventions that operators already know. `wolfcastle log -f` reads the same as `docker logs -f` or `tail -f`.
- The hidden alias means no breaking change. Anyone using `wolfcastle follow` in scripts does not need to update immediately.
- Help text and documentation reference `wolfcastle log` exclusively. The alias exists for backward compatibility, not as a supported alternative.

## Amendment: v0.5.0

The hidden `follow` alias was removed. Scripts using `wolfcastle follow` must update to `wolfcastle log`. The alias served its purpose during the transition period and is no longer needed.
