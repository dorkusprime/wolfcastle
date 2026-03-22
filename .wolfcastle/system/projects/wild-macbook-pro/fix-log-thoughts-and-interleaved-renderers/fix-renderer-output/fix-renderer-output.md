# Fix Renderer Output

Investigate and fix why wolfcastle log --thoughts and --interleaved output raw NDJSON instead of formatted text. The renderer source code appears correct (thoughts.go filters for assistant records, interleaved.go formats stage headers with timestamps/glyphs), so the bug likely lives in the pipeline wiring: how records flow from the reader through the channel to the renderer, or in a runtime condition that causes the renderer to be bypassed. The spec at docs/specs/2026-03-21T18-00Z-log-command-design.md defines expected output for both modes.
