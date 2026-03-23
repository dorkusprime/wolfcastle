# SQLite

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Type Affinity

SQLite uses type affinity, not strict types. A column declared as `INTEGER` will happily store a string. The declared type influences how values are coerced on comparison and sorting, but it does not reject mismatched inserts. This means a `CHECK` constraint or application-side validation is the only barrier between a text value and a numeric column.

Prefer `STRICT` tables (SQLite 3.37+) when type enforcement matters. A `CREATE TABLE t (...) STRICT` declaration rejects values that do not match the declared column type. Strict tables support `INT`, `INTEGER`, `REAL`, `TEXT`, `BLOB`, and `ANY`. Use `ANY` for columns that intentionally accept mixed types.

Prefer JSONB storage (SQLite 3.45+) for JSON data that will be queried or manipulated frequently. JSONB stores JSON in a decomposed binary format that avoids re-parsing on every access. All JSON functions were rewritten in 3.45 to use JSONB internally. Use `jsonb()` to convert text JSON to JSONB on insert, and `json()` to convert back to text for display. SQLite 3.51 added `jsonb_each()` and `jsonb_tree()` for iterating JSONB directly.

Prefer `INTEGER PRIMARY KEY` for rowid aliases. This makes the column an alias for SQLite's internal rowid, giving it auto-increment behavior without the `AUTOINCREMENT` keyword. `AUTOINCREMENT` adds a monotonically increasing guarantee (never reuses deleted IDs) at the cost of a lookup in the `sqlite_sequence` table on every insert. Use `AUTOINCREMENT` only when reuse would be harmful.

## WAL Mode

Prefer WAL (Write-Ahead Logging) mode for any database with concurrent readers: `PRAGMA journal_mode=WAL`. WAL allows reads to proceed while a write is in progress. The default rollback journal mode locks the entire database during writes, serializing all access.

Prefer `PRAGMA synchronous=NORMAL` in WAL mode. The default `FULL` syncs the WAL on every commit; `NORMAL` syncs only at checkpoints. `NORMAL` is still safe against corruption from crashes in WAL mode (the WAL file is append-only), and the throughput improvement is significant.

Prefer setting `PRAGMA busy_timeout` to a non-zero value (e.g., 5000 milliseconds) on every connection. Without it, a write attempt that encounters a lock returns `SQLITE_BUSY` immediately rather than retrying. A short busy timeout absorbs brief contention windows without requiring application-level retry loops.

Prefer `PRAGMA wal_checkpoint(TRUNCATE)` during maintenance windows to reset the WAL file. The WAL grows as writes accumulate between automatic checkpoints; a large WAL degrades read performance because readers must scan it for recent changes.

## Essential PRAGMAs

Set these at connection open, not once per database lifetime (most are per-connection settings):

`PRAGMA journal_mode=WAL` enables concurrent reads. `PRAGMA synchronous=NORMAL` balances safety and speed in WAL mode. `PRAGMA busy_timeout=5000` avoids immediate `SQLITE_BUSY` errors. `PRAGMA foreign_keys=ON` enables foreign key enforcement (off by default in SQLite). `PRAGMA cache_size=-64000` sets the page cache to 64 MB (negative values are in kibibytes). `PRAGMA mmap_size=268435456` memory-maps up to 256 MB of the database file for faster reads on platforms that support it.

Prefer `PRAGMA optimize` before closing long-lived connections. It runs `ANALYZE` on tables where the query planner's statistics are stale, improving plan quality for future connections.

## ALTER TABLE Limitations

`ALTER TABLE ... DROP COLUMN` is supported since SQLite 3.35.0 (March 2021). On older versions, dropping a column requires the 12-step table rebuild: create a new table, copy data, drop the old table, rename the new one. Verify your embedded SQLite version before relying on `DROP COLUMN`.

`ALTER TABLE ... RENAME COLUMN` is supported since SQLite 3.25.0. `ALTER TABLE ... ADD COLUMN` has been available much longer but cannot add columns with `PRIMARY KEY`, `UNIQUE`, or non-constant default values.

Prefer applying multiple schema changes inside a single transaction. SQLite supports transactional DDL; a failed migration rolls back cleanly without leaving the schema in a half-altered state.

## Concurrent Access

SQLite allows multiple readers but only one writer at a time, even in WAL mode. The write lock is database-wide, not table-wide or row-wide. High write concurrency from multiple processes or threads will serialize at the lock, creating contention.

Prefer a single writer process with internal queuing when write throughput matters. Application architectures that funnel writes through one connection (or a small pool with serialized access) avoid `SQLITE_BUSY` errors entirely. WAL2 mode and `BEGIN CONCURRENT` exist as experimental branches and in SQLite forks like LibSQL; they allow multiple concurrent writers to proceed as long as they don't modify the same pages. These features are not in mainline SQLite as of 3.51.

Prefer connection pooling with `SQLITE_OPEN_FULLMUTEX` (serialized threading mode) or `SQLITE_OPEN_NOMUTEX` (multi-thread mode with the application guaranteeing single-thread-per-connection). The wrong threading mode produces silent corruption.

Prefer keeping transactions short. A long-running read transaction in WAL mode prevents the WAL from checkpointing past that transaction's snapshot, causing the WAL to grow unboundedly.

## Application File Format

SQLite databases serve as application file formats (document files, configuration stores, local caches). The file is a single cross-platform binary, portable across architectures and operating systems without conversion.

Prefer setting `PRAGMA application_id` and `PRAGMA user_version` to identify the file format and schema version. This lets applications detect whether a file belongs to them and whether it needs migration.

Prefer `VACUUM` after bulk deletes to reclaim disk space. SQLite does not shrink the file when rows are deleted; freed pages remain in the file for future reuse. `VACUUM` rebuilds the database into a minimal file. `PRAGMA auto_vacuum=INCREMENTAL` reclaims pages lazily without the full rebuild cost.

## Testing

Prefer in-memory databases (`:memory:` or `file::memory:?cache=shared`) for fast, isolated test runs. Each `:memory:` connection gets its own database; the `cache=shared` URI parameter allows multiple connections to the same in-memory database for testing concurrent access patterns.

Prefer replicating production PRAGMAs in test setup. Tests that omit `PRAGMA foreign_keys=ON` will miss referential integrity violations that production encounters. Tests that skip WAL mode will not catch concurrency bugs.

Prefer checking the embedded SQLite version in tests (`SELECT sqlite_version()`) when the test relies on features added in recent releases. The SQLite version bundled with a language runtime (Python, Go, Ruby) often lags the latest release.

## Common Pitfalls

`PRAGMA foreign_keys` defaults to `OFF`. Every connection must enable it explicitly. A schema with foreign key constraints that never enforces them silently accumulates orphaned rows.

`NULL` handling in `UNIQUE` constraints differs from most databases. SQLite considers each `NULL` distinct, so a `UNIQUE` column can hold multiple `NULL` values. PostgreSQL and MySQL follow the same convention, but SQL Server does not (by default).

`REAL` is an 8-byte IEEE 754 float. There is no `DECIMAL` or `NUMERIC` type with fixed precision. Monetary values stored as `REAL` accumulate floating-point drift. Prefer storing monetary amounts as integer cents or using a text representation with application-side parsing.

`GROUP BY` without `ORDER BY` does not guarantee output order. SQLite may return groups in any order, and the order can change between versions or even between runs. Always add an explicit `ORDER BY` when order matters.

`ATTACH DATABASE` lets a single connection query across multiple database files. Transactions span all attached databases atomically. This is powerful for splitting data across files (archival, sharding by tenant) while maintaining referential integrity at query time.
