# Log Record Types

Add missing NDJSON record types to the logging package. The spec requires iteration_start records for session boundary detection. Verify that stage_start, stage_complete, planning_start, planning_complete, and audit_report_written records already exist and contain the fields the log command needs (node address, stage type, timestamps). Add any missing fields.
