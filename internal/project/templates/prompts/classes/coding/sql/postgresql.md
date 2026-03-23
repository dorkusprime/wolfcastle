# PostgreSQL

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Data Types

Prefer `text` over `varchar(n)` unless the length constraint carries business meaning (a two-letter country code, a fixed-width external ID). PostgreSQL stores both identically; the length check on `varchar(n)` adds overhead without benefit when the limit is arbitrary.

Prefer `timestamptz` over `timestamp`. A `timestamp without time zone` discards timezone information at insert and reconstructs ambiguously at read. `timestamptz` stores UTC internally and converts to the session timezone on display. Use bare `timestamp` only when the value represents a wall-clock time disconnected from any timezone (recurring schedules, fictional dates).

Prefer `jsonb` over `json`. `jsonb` is stored in a decomposed binary format that supports indexing, containment operators (`@>`, `<@`), and existence checks (`?`, `?|`, `?&`). `json` preserves formatting and key order but requires re-parsing on every access. Use `json` only when exact input preservation matters.

Prefer `uuid` as a column type for identifiers when the application generates UUIDs. PostgreSQL validates format on insert. For ordered inserts with minimal index fragmentation, prefer UUIDv7 (time-sorted) over UUIDv4 (random). The `gen_random_uuid()` function produces v4; use `uuidv7()` from the `pg_uuidv7` extension or generate v7 values in application code.

Prefer `bigint` or `bigserial` over `integer`/`serial` for primary keys on tables expected to grow. Migrating from `int4` to `int8` on a large table requires a full rewrite and exclusive lock.

## JSONB Operations

Prefer the subscript syntax (`col['key']`) introduced in PostgreSQL 14 for simple key access. It returns `jsonb` and supports assignment in `UPDATE` statements: `UPDATE t SET data['status'] = '"active"'`. The arrow operators (`->`, `->>`, `#>`, `#>>`) remain necessary for path traversal, text extraction, and older PostgreSQL versions.

Prefer `jsonb_path_query` and `jsonb_path_exists` (SQL/JSON path language) for conditional filtering inside JSONB documents. Path expressions (`$.items[*] ? (@.price > 10)`) push filtering into the engine rather than extracting and filtering in application code.

Prefer GIN indexes on JSONB columns used in containment or existence queries. `CREATE INDEX ON t USING gin (data)` supports `@>`, `?`, `?|`, and `?&`. For queries that only access specific keys, a path-specific GIN index (`CREATE INDEX ON t USING gin ((data -> 'status'))`) is smaller and faster.

## Index Types

Prefer B-tree (the default) for equality and range queries on scalar columns. It covers `=`, `<`, `>`, `BETWEEN`, `IN`, `IS NULL`, and `ORDER BY`.

Prefer GIN for multi-valued columns: arrays, JSONB, full-text search (`tsvector`), and `hstore`. GIN indexes are larger and slower to update than B-tree but fast for containment and overlap queries.

Prefer GiST for geometric data, range types, nearest-neighbor queries (`ORDER BY ... <->` for `pg_trgm` similarity or PostGIS distance), and exclusion constraints (`EXCLUDE USING gist`). GiST is lossy for some operators; the engine rechecks the heap, so index-only scans are not always possible.

Prefer BRIN for very large, append-mostly tables where the indexed column correlates with physical row order (timestamps on log tables, monotonic IDs). BRIN indexes are tiny (orders of magnitude smaller than B-tree) but only effective when the correlation between value and physical position is high. Check `pg_stats.correlation`; values near 1.0 or -1.0 indicate a good BRIN candidate.

Prefer partial indexes (`CREATE INDEX ... WHERE condition`) to index only the rows that queries actually filter. A partial index on `WHERE status = 'pending'` is far smaller than a full index when most rows are `'completed'`.

## RETURNING Clause

Prefer `RETURNING` on `INSERT`, `UPDATE`, and `DELETE` to retrieve affected rows in a single round trip. `INSERT INTO orders (...) VALUES (...) RETURNING id, created_at` eliminates a follow-up `SELECT`. Combine with CTEs to chain operations: `WITH ins AS (INSERT ... RETURNING *) SELECT ... FROM ins JOIN ...`.

## Advisory Locks

Prefer `pg_advisory_xact_lock(key)` over session-level `pg_advisory_lock(key)` for most use cases. Transaction-level locks release automatically at `COMMIT` or `ROLLBACK`, removing the risk of leaked locks from forgotten `pg_advisory_unlock` calls or crashed sessions.

Prefer `pg_try_advisory_xact_lock(key)` when the caller should skip work rather than wait. This returns `false` immediately if the lock is held, enabling skip-locked job-claiming patterns without blocking.

Use advisory locks for application-level coordination that does not map to row locks: singleton cron jobs, schema migration serialization, rate limiting per tenant. They are lightweight (no table bloat, no vacuum overhead) but invisible to standard lock monitoring unless you query `pg_locks` explicitly.

## LISTEN/NOTIFY

Prefer `LISTEN`/`NOTIFY` for lightweight, real-time event signaling between database sessions. A `NOTIFY channel, 'payload'` inside a transaction delivers the payload to all sessions that have executed `LISTEN channel`, but only after the transaction commits. Payloads are limited to 8000 bytes.

Prefer LISTEN/NOTIFY over polling loops for cache invalidation, job dispatch, and UI refresh triggers. The notification is delivered in-band on the existing connection; no separate message broker is required. Keep payloads small (an ID or a JSON summary) and fetch full data with a follow-up query.

Handle connection drops in the listening client. Notifications are not queued durably; if the listener disconnects and reconnects, it misses everything sent in the interim. For guaranteed delivery, pair NOTIFY with a work queue table and use the notification as a wake-up signal rather than the sole transport.

## Partitioning

Prefer declarative partitioning (`PARTITION BY RANGE`, `PARTITION BY LIST`, `PARTITION BY HASH`) over inheritance-based partitioning. Declarative partitioning is simpler, supports partition pruning at plan time, and integrates with `ATTACH`/`DETACH PARTITION` for maintenance operations.

Prefer range partitioning on time columns for time-series and event data. Each partition covers a fixed interval (day, week, month). Create future partitions ahead of schedule; an insert into a nonexistent partition fails rather than falling back to the parent.

Prefer keeping partition counts in the low hundreds. PostgreSQL 17 improved partition pruning performance, but thousands of partitions still slow planning. If each partition holds fewer than 10,000 rows, the overhead of partition management likely exceeds the benefit.

Prefer `DETACH PARTITION ... CONCURRENTLY` (PostgreSQL 14+) for dropping old data. It avoids an `ACCESS EXCLUSIVE` lock on the parent, allowing concurrent queries to continue.

## PL/pgSQL

Prefer SQL functions (`CREATE FUNCTION ... LANGUAGE sql`) for simple expressions and single-statement operations. SQL functions can be inlined by the planner, which PL/pgSQL functions cannot.

Prefer PL/pgSQL when the logic requires control flow (conditionals, loops, exception handling) or must perform multiple statements with intermediate variables. Use `RETURNS TABLE` or `RETURNS SETOF` for functions that produce row sets.

Prefer `RAISE NOTICE` for debugging and `RAISE EXCEPTION` for error signaling. Custom error codes (`RAISE EXCEPTION USING ERRCODE = 'P0001'`) let callers handle specific failures without parsing message text.

Prefer `PERFORM` instead of `SELECT` when calling a function for its side effects and discarding the result. A bare `SELECT` in PL/pgSQL without an `INTO` target raises an error.

## Extensions

Prefer verifying extension availability with `SELECT * FROM pg_available_extensions WHERE name = '...'` before `CREATE EXTENSION`. On managed services (RDS, Cloud SQL, Supabase), the available extension set is a subset of what self-hosted PostgreSQL offers.

Commonly relied-upon extensions: `pg_trgm` for trigram similarity and `LIKE`/`ILIKE` index support, `pgcrypto` for `gen_random_uuid()` (pre-PostgreSQL 13) and hashing, `pg_stat_statements` for query performance monitoring, `btree_gin` and `btree_gist` for using B-tree-indexable types in GIN/GiST indexes, `citext` for case-insensitive text without `LOWER()` wrappers, and `postgis` for geospatial data.

Prefer pinning extension versions in migration files (`CREATE EXTENSION IF NOT EXISTS pg_trgm VERSION '1.6'`) so that upgrades are explicit and reviewable.
