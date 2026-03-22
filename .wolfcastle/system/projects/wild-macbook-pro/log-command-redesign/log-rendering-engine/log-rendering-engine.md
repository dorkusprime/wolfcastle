# Log Rendering Engine

A new internal/logrender package containing session detection, duration formatting, and three output renderers (summary, thoughts, interleaved). This package is the shared rendering layer used by both the wolfcastle log command and non-daemon mode stdout. It reads NDJSON records and produces formatted terminal output. The renderers must handle both historical replay (reading completed log files) and live follow mode (tailing active files). The package has no dependency on the cobra command layer.
