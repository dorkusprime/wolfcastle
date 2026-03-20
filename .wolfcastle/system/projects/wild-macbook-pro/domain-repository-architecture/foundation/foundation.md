# Foundation

Build the shared tier resolution primitive (internal/tierfs) and the test environment (internal/testutil/Environment). These are the foundational pieces everything else depends on: tierfs provides three-tier file resolution used by ConfigRepository, PromptRepository, and ClassRepository; Environment provides the test infrastructure all subsequent steps use instead of manual path construction. Steps 1-2 from the migration path.
