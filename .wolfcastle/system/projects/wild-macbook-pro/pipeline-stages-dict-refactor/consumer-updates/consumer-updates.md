# Consumer Updates

Update all code that consumes Config.Pipeline.Stages to work with the new map[string]PipelineStage + StageOrder format. Covers daemon iteration logic (iterating StageOrder with map lookup), daemon stage lookup (intake detection), pipeline prompt assembly (receives stage name separately since Name field is removed), and all associated tests across daemon/ and pipeline/ packages. Must maintain >90% test coverage in all modified packages.
