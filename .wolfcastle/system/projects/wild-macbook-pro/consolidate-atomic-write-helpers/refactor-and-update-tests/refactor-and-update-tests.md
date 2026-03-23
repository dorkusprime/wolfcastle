# Refactor and Update Tests

Refactor atomicWriteJSON to delegate to AtomicWriteFile, then update tests whose error-message assertions break due to the changed error wrapping.
