# Archive Status Rendering

Modify the wolfcastle status command to be archive-aware. Active status (default) shows only non-archived nodes plus a footer count line 'N archived nodes' when archives exist. New --archived flag shows only archived nodes. Existing --all flag shows everything (active plus archived). JSON output includes archive metadata. Requires the archive service spec and core implementation to be complete first.
