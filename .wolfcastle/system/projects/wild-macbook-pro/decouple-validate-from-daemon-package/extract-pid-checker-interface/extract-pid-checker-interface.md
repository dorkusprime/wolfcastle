# Extract PID Checker Interface

Define a PIDChecker interface in the validate package covering the five methods it actually calls (IsAlive, PIDFileExists, StopFileExists, RemovePID, RemoveStopFile). Replace all concrete *daemon.DaemonRepository references in engine.go and fix.go with this interface. Update callers in cmd/ that pass DaemonRepository to validate functions. Verify daemon.DaemonRepository implicitly satisfies the interface. Add test confirming the decoupling compiles without importing daemon.
