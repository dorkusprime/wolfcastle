# ADR: Interface for Validate-Daemon Boundary

File a new ADR superseding 2026-03-18T21-34Z (DaemonRepository uses concrete struct). Document why the validate package now defines a PIDChecker interface: the original decision assumed a single consumer where temp-dir testing was sufficient, but validate's role as a standalone structural checker means it should not depend on daemon's concrete type. The interface is narrow (five methods), follows Go's consumer-defines-interface convention, and enables validate to be tested and used without any daemon package dependency.
