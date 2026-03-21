# Archive Service Spec

Write the specification for the auto-archive service contract. Defines the archive state model (completion timestamps, archived flag in RootIndex), file layout for archived node state, operations (move, restore, delete), config schema (ArchiveConfig with delay threshold), and the daemon timer interface. This spec is the single source of truth that all implementation children reference.
