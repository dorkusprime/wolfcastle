# Code Cleanups

Three targeted cleanups in internal/logrender: replace hand-rolled indexOf with bytes.IndexByte, remove the stageLabels identity map in record.go, and remove the reimplemented strings.Contains helper (expectContains) in interleaved_test.go.
