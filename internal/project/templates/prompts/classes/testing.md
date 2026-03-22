# Testing

When the project you're working in has established testing conventions, frameworks, or coverage expectations that differ from what's described here, follow the project.

## Coverage Strategy

**Test behavior, not implementation.** Assert on observable outputs given known inputs. Do not assert on which internal methods were called, in what order, or how many times. Tests coupled to implementation break on every refactor without catching real regressions. A passing test should mean the system does the right thing, not that it does it in a particular way.

**Cover the contract, then the edge cases.** Start with the documented or intended behavior: the function's stated purpose, the API's response contract, the module's public interface. Once the expected path is covered, move to boundaries, error cases, and degenerate inputs. A test suite that covers only the happy path is a brochure, not a safety net.

## Boundaries and Edge Cases

**Test at the edges of input domains.** For each input range, test at the minimum, maximum, and the values immediately adjacent to both boundaries. Defects cluster at boundaries: off-by-one errors, overflow conditions, empty collections, single-element collections, zero-length strings. Equivalence partitioning identifies the ranges; boundary value analysis targets their fault-prone edges.

**Test error paths with the same rigor as success paths.** Every documented error condition should have a test that triggers it and verifies the response: correct error type, correct message content, correct side effects (or absence of them). An error path exercised only in production is a defect waiting to surface.

## Test Isolation

**Each test is self-sufficient.** A test creates the state it needs, exercises the behavior, verifies the result, and cleans up after itself. No test depends on another test's execution, ordering, or side effects. When tests share state, a failure in one test can cascade into false failures elsewhere, and the resulting investigation wastes more time than the shared setup saved.

**Control external dependencies.** Use test doubles (stubs, fakes, or mocks) for resources outside your control: network services, clocks, random number generators, filesystems with variable state. The goal is determinism: the same test with the same code produces the same result on every run. Reserve real dependencies for integration tests that explicitly validate the integration.

## Naming and Structure

**Name tests for the scenario, not the method.** A test named `TestCalculateTotal` says nothing about what it verifies. `TestCalculateTotal_AppliesDiscountWhenQuantityExceedsTen` tells the reader exactly what breaks when it fails. Good test names read as a specification of the system's behavior.

**One assertion per logical concept.** A test that verifies five unrelated properties fails opaquely: which property broke? Separate tests for separate concepts produce clear signals. Multiple assertions that verify facets of a single result (checking both the status code and the response body of one API call) are fine; they describe one behavior from different angles.

## Flaky Test Prevention

**Never use fixed delays for synchronization.** Sleeping for a hardcoded duration is a race condition with a longer fuse. Wait for the condition you need: poll for state changes, use synchronization primitives, or subscribe to events. If a test cannot observe the condition it needs, the system under test may need a testability hook, and that is a design conversation, not a workaround.

**Eliminate shared mutable state between tests.** Shared databases, global variables, singleton caches, and filesystem paths that multiple tests read and write concurrently are the primary source of test-order-dependent failures. Fresh fixtures per test, or hermetic environments that are created and destroyed for each run, prevent the entire category.

## Property-Based Testing

**Declare invariants, not examples.** Property-based testing generates hundreds of random inputs and checks that a stated property holds for all of them. Use it when the property is expressible as a general rule: encode-then-decode returns the original, sorting is idempotent, output length never exceeds input length. The framework's shrinking capability automatically minimizes failing inputs to the simplest reproduction case. Keep example-based tests alongside for readability and regression anchoring.

## Performance Testing

**Establish baselines before optimizing.** Measure first, then change. A benchmark without a baseline is a number without meaning. Report percentile distributions (p50, p95, p99) rather than averages; averages conceal the tail latency that users actually experience.

**Treat microbenchmarks with skepticism.** Short-running benchmarks are susceptible to noise from code alignment, branch prediction state, OS scheduling, and dead-code elimination by the compiler. Control for these by using the benchmarking harness your language provides, running sufficient iterations, and comparing relative changes rather than absolute numbers.
