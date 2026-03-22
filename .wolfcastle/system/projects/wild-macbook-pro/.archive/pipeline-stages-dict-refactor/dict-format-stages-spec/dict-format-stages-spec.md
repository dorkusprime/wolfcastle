# Dict-Format Stages Spec

Restore and register the dict-format stages specification. The spec defines the new JSON schema (map[string]PipelineStage + stage_order), PipelineStage struct changes (Name field removed), merge semantics for map-keyed stages, migration contract for existing array-format configs, updated validation rules, and consumer impact. This spec is the contract that all subsequent implementation and audit work verifies against.
