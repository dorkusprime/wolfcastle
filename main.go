// Wolfcastle is a model-agnostic autonomous project orchestrator.
// It breaks complex work into a persistent tree of projects and tasks,
// then executes them through configurable multi-model pipelines.
//
// See https://github.com/dorkusprime/wolfcastle for documentation.
package main

import "github.com/dorkusprime/wolfcastle/cmd"

func main() {
	cmd.Execute()
}
