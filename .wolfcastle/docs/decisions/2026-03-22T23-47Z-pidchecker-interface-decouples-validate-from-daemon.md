# PIDChecker interface decouples validate from daemon

## Status
Accepted (supersedes 2026-03-18T21-34Z)

## Date
2026-03-22

## Context
The validate package is a standalone structural checker for Wolfcastle project trees. It needs to query daemon lifecycle state (is the process alive, does the PID file exist, does the stop file exist) and, in fix mode, clean up stale files. Before this change, validate imported the daemon package directly to use DaemonRepository.

That import created an unnecessary compile-time coupling. validate has no business depending on daemon internals; it only needs five methods: IsAlive, PIDFileExists, StopFileExists, RemovePID, and RemoveStopFile. Those five methods form a natural interface boundary.

The original ADR (2026-03-18T21-34Z, "DaemonRepository uses concrete struct with explicit parameters") decided against defining an interface because there was exactly one implementation and all callers lived inside or close to the daemon package. That reasoning remains sound for daemon-internal consumers. validate, however, is an external consumer with a different concern: it should compile and test independently of daemon.

## Options Considered
1. Keep the direct import. Accept the coupling as a cost of simplicity.
2. Define a shared interface in a third package (e.g., internal/daemonapi). Both daemon and validate import the shared package.
3. Define a consumer-side interface in validate. Let DaemonRepository satisfy it implicitly through Go's structural typing.

## Decision
Option 3. A PIDChecker interface is defined in internal/validate/pid_checker.go with five methods. DaemonRepository satisfies this interface without modification; a compile-time assertion in repository_test.go confirms the contract holds.

The interface lives in the consumer (validate), not the provider (daemon), following Go's convention that interfaces belong to the code that depends on them. No shared package is needed because Go interfaces are satisfied implicitly.

## Consequences
validate no longer imports daemon. It can be compiled, tested, and reasoned about in isolation.

Future PID checkers (remote daemon proxies, test fakes) can satisfy the same interface without touching daemon or validate.

The original ADR's guidance still applies within the daemon package itself: DaemonRepository remains a concrete struct with no provider-side interface, tested against real temp directories. This decision narrows the scope of the supersession to the validate-daemon boundary only.
