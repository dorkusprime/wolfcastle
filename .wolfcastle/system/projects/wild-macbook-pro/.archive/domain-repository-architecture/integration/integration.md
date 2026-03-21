# Integration

Build composite services that depend on multiple repositories, refactor the App struct, and remove the legacy Resolver. This covers: ClassRepository (depends on PromptRepository), ContextBuilder (depends on PromptRepo + ClassRepo + render methods), ScaffoldService + MigrationService (depends on ConfigRepo + PromptRepo + DaemonRepo), App struct refactor (replaces WolfcastleDir/Resolver/raw Cfg with repository references, adds RequireIdentity), and tree.Resolver removal + final test migration. Steps 8, 10-13 from the migration path.
