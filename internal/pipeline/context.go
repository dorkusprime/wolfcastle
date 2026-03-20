package pipeline

// FailureHeaderContext holds template variables for context-headers.md.
type FailureHeaderContext struct {
	FailureCount    int
	DecompThreshold int
	MaxDecompDepth  int
	CurrentDepth    int
	HardCap         int
}

// DecompositionContext holds template variables for decomposition.md.
type DecompositionContext struct {
	NodeAddr string
}
