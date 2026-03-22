# Foundation

Core types and utilities for the logrender package: NDJSON record struct and parser, compact duration formatter, session detection (grouping log files into sessions by iteration-1 boundaries), and a file-reading abstraction that supports both full-file replay and live tailing. Every renderer depends on this layer.
