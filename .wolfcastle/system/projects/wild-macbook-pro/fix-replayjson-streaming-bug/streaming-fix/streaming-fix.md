# streaming-fix

Fix replayJSON to stream file contents line-by-line and decompress .gz files through a gzip.Reader, matching the existing pattern in ReplayReader.readFile. Eliminates full-file memory loads and corrects raw-gzip output.
