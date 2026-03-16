# User items as they come up. Don't process these until directed.

## Done

- ~~Terminal marker not detected~~ — misdiagnosis. `scanTerminalMarker` already handles the `result` envelope correctly. The actual problem was the deliverable unchanged check clearing the marker.
- ~~Deliverable globs don't recurse into subdirectories~~ — `globRecursive` now walks subdirs when the filename part contains wildcards. `cmd/*.go` matches `cmd/task/add.go`.
- ~~Deliverable unchanged false failures on retry~~ — baseline hashes are now re-snapshotted at the start of every retry, so prior attempt's writes become the new starting point.
- ~~CLI header cleanup~~ — [INFO] lines suppressed, version deduped, header reordered.
- ~~Status detail for completed tasks~~ — summary shown for completed tasks.
