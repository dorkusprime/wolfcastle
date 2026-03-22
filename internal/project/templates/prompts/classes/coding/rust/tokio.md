# Tokio

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Runtime and Task Spawning

Prefer `#[tokio::main]` with the multi-threaded runtime for applications. Use `#[tokio::main(flavor = "current_thread")]` only for lightweight tools or when single-threaded semantics simplify the design. Prefer `tokio::spawn` to create concurrent tasks, and always store the returned `JoinHandle<T>` so panics propagate and results are retrievable. A dropped `JoinHandle` detaches the task silently: panics vanish, errors go unobserved, and cancellation becomes impossible. Prefer `.abort()` on the handle when early cancellation is needed rather than relying on drop behavior.

## Structured Concurrency

Prefer `tokio::task::JoinSet` when spawning a dynamic set of tasks that should be collected together. `JoinSet::join_next()` returns results in completion order and propagates panics. For fan-out/fan-in patterns, `JoinSet` replaces manual `Vec<JoinHandle<T>>` with `futures::future::join_all`, which swallows panics. Prefer `tokio_util::task::TaskTracker` when you need to wait for all spawned tasks during shutdown without collecting their results.

## Channels

Prefer `tokio::sync::mpsc` for multi-producer work queues; always use bounded channels unless you can prove the producer cannot outpace the consumer, because unbounded channels convert backpressure problems into memory exhaustion. Prefer `tokio::sync::oneshot` for single-response request/reply patterns. Prefer `tokio::sync::broadcast` when multiple consumers each need every message. Prefer `tokio::sync::watch` for configuration or state that consumers poll for the latest value, where missed intermediate values are acceptable.

## select! and Cancellation

Prefer `tokio::select!` to multiplex across futures (socket reads, channel receives, timers, shutdown signals). Every branch in a `select!` that is not chosen gets dropped, cancelling its future mid-execution. If a branch's future holds partial state that must not be lost (e.g., a half-filled buffer from `read_buf`), either pin the future outside the loop so it resumes on the next iteration or restructure to avoid holding transient state across the yield point. Prefer biased mode (`tokio::select! { biased; ... }`) when one branch must be checked first, such as a shutdown signal that should preempt work.

## Async I/O

Prefer `tokio::net::TcpListener` and `TcpStream` for network I/O. Wrap streams in `tokio::io::BufReader` and `BufWriter` to reduce syscall frequency; flush `BufWriter` explicitly before dropping or before the peer needs the data. Prefer `tokio::fs` for file operations, but be aware it delegates to a blocking threadpool internally, so it does not reduce thread usage the way network I/O does. For high-throughput file work, `spawn_blocking` with `std::fs` gives equivalent performance with more predictable backpressure.

## Synchronization

Prefer `tokio::sync::Mutex` when the lock must be held across an `.await` point; it yields the task instead of blocking the OS thread. Prefer `std::sync::Mutex` for short, synchronous critical sections that never span an `.await`, because it avoids the overhead of async lock tracking. Holding a `std::sync::Mutex` guard across an `.await` blocks the entire runtime thread for the duration of the await, starving other tasks on that thread. Prefer `tokio::sync::RwLock` for read-heavy shared state. Prefer `tokio::sync::Semaphore` to limit concurrency (connection pools, rate limiting). Prefer `tokio::sync::Notify` for signaling between tasks without transferring data.

## Graceful Shutdown

Prefer `tokio::signal::ctrl_c()` or a `broadcast`/`watch` channel to distribute a shutdown signal. The pattern: the main task receives the signal, drops or closes the channel sender, and all workers detect closure via their receiver's `.recv()` returning `None` or `.changed()` returning an error. Combine with `TaskTracker` or `JoinSet` to await in-flight work before exiting. Prefer `tokio::time::timeout` on the drain period so the process terminates even if a task hangs.

## Testing

Prefer `#[tokio::test]` for async test functions. Use `#[tokio::test(start_paused = true)]` to enable deterministic time: `tokio::time::sleep` and `tokio::time::timeout` advance instantly when all tasks are idle, making timing-dependent tests fast and non-flaky. Prefer `tokio::io::duplex` to create in-memory bidirectional byte streams for testing network code without binding a port. Prefer `tokio_util::sync::CancellationToken` in tests to trigger shutdown paths deterministically.

## Common Pitfalls

Calling blocking code (CPU-heavy computation, synchronous I/O, `std::thread::sleep`) inside an async task stalls the runtime thread, preventing all other tasks on that thread from making progress. Use `tokio::task::spawn_blocking` to move blocking work onto a dedicated threadpool, and `.await` the handle to get the result back into async context.

Every `tokio::select!` iteration drops the futures from unchosen branches. A loop that rebuilds an expensive future on each iteration (reconnecting a socket, re-preparing a query) pays the setup cost repeatedly. Pin long-lived futures outside the loop and reference them inside `select!` so they survive across iterations.

Unbounded channels (`mpsc::unbounded_channel`) never exert backpressure. A fast producer paired with a slow consumer grows the channel's internal buffer without limit until the process runs out of memory. Prefer bounded channels and handle the `SendError` when the channel is full.

`tokio::spawn` requires the future to be `Send + 'static` because the task may be scheduled on any runtime thread. Capturing a reference to a local variable fails to compile. Prefer cloning or wrapping shared data in `Arc` before the spawn, and move the clone into the async block.
