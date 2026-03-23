# Schema Documentation Update

Update the log command design spec (.wolfcastle/docs/specs/2026-03-21T18-00Z-log-command-design.md) to document the duration_ms field in the NDJSON Records Used table and note that the field is emitted by the daemon at stage completion time. Revise the Data Source section to reflect that the daemon now pre-computes duration for structured consumers while renderers still own display formatting.
