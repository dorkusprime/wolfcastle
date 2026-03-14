package project

import "embed"

// Templates holds the embedded base prompt and rule templates that are
// extracted into .wolfcastle/base/ during scaffolding (ADR-033).
//
//go:embed all:templates
var Templates embed.FS
