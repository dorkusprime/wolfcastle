# SQL

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer uppercase for SQL keywords (`SELECT`, `FROM`, `WHERE`, `JOIN`). Lowercase keywords work, but uppercase creates a visual rhythm that separates structure from data, and most style guides converge on this convention.

Prefer explicit `JOIN` syntax over comma-separated implicit joins. `FROM orders o JOIN customers c ON o.customer_id = c.id` states the relationship at the point of joining; `FROM orders, customers WHERE ...` buries it among filter conditions.

Prefer table aliases in any query involving more than one table. Short, meaningful aliases (`o` for orders, `c` for customers) reduce line noise and make `ON` clauses readable. In single-table queries, aliases are unnecessary clutter.

Prefer CTEs (`WITH` clauses) over deeply nested subqueries. CTEs read top-to-bottom, can be referenced multiple times, and give intermediate results a name. Reserve inline subqueries for simple, single-use expressions where a CTE would add ceremony without clarity.

Prefer `COALESCE` over dialect-specific null-handling functions (`IFNULL`, `NVL`, `ISNULL`) when portability matters. `COALESCE` is standard SQL and works across every major engine.

Prefer `EXISTS` over `IN` for correlated subqueries, particularly when the subquery might return NULLs. `IN` with a NULL in the subquery result produces unexpected behavior due to three-valued logic.

## Dialect Awareness

Prefer noting dialect-specific behavior in comments when the query relies on it. PostgreSQL's `RETURNING` clause, MySQL's `ON DUPLICATE KEY UPDATE`, SQLite's type affinity system, and SQL Server's `OUTPUT` clause all lack direct equivalents elsewhere.

Prefer `LIMIT`/`OFFSET` on PostgreSQL, MySQL, and SQLite; use `TOP` or `FETCH FIRST` on SQL Server. When writing migrations or queries intended to be portable, isolate pagination logic so it can be swapped per dialect.

Prefer PostgreSQL's `jsonb` operators or MySQL's `JSON_EXTRACT` only when the project has committed to a single dialect. JSON column operations are among the least portable features across engines.

Prefer `GENERATED ALWAYS AS` for computed columns where supported. PostgreSQL and MySQL handle generated columns differently (stored vs. virtual defaults differ), so verify the engine's behavior before relying on them.

## Migrations

Prefer versioned migration files with a sequential or timestamp-based naming scheme (`001_create_users.sql`, `20260322_add_orders_index.sql`). Tools like Flyway, Alembic, golang-migrate, and dbmate all expect ordered, immutable migration files.

Prefer reversible migrations. Each `up` migration should have a corresponding `down` that cleanly undoes the schema change. Dropping a column is irreversible in production if it contained data; consider a staged approach (stop writing, deploy, then drop).

Prefer separating DDL from data manipulation in migrations. A migration that creates a table and backfills it conflates schema with data, making rollback unpredictable. Split into two migrations when the data operation is non-trivial.

Prefer avoiding data-dependent DDL. `ALTER TABLE ... ADD CONSTRAINT` that requires a full table scan on a large table will lock the table for the scan's duration. On PostgreSQL, prefer `CREATE INDEX CONCURRENTLY`; on MySQL, consider `pt-online-schema-change` or `gh-ost` for large tables.

## Testing

Prefer transaction-wrapped tests that roll back after each case. This keeps test data isolated without the cost of full database recreation between tests. Most testing frameworks support this pattern (`BEGIN` at setup, `ROLLBACK` at teardown).

Prefer explicit test data setup over shared fixtures. A test that inserts its own rows documents its preconditions; a test that relies on seed data hides them. When shared fixtures are unavoidable, keep them minimal and document what each test expects.

Prefer testing queries against a real database instance matching the production dialect. SQLite as a stand-in for PostgreSQL masks type coercion, constraint enforcement, and function availability differences. Container-based test databases (Testcontainers, Docker Compose) make dialect-matched testing practical.

## Common Pitfalls

String interpolation into SQL queries invites injection attacks. Prefer parameterized queries (`$1`, `?`, or named parameters depending on the driver) for all user-supplied values. Even internal tools deserve parameterization; the next developer might expose the endpoint.

`NULL` does not equal `NULL`. `WHERE status != 'active'` excludes rows where `status` is NULL, because `NULL != 'active'` evaluates to `NULL`, not `TRUE`. Prefer explicit `IS NULL` / `IS NOT NULL` checks when the column is nullable.

Implicit type casting silently coerces values in ways that disable index usage. `WHERE varchar_column = 123` forces a cast on every row in MySQL, turning an index scan into a full table scan. Prefer matching the parameter type to the column type.

N+1 queries surface when application code loops over a result set and issues a query per row. Prefer `JOIN` or batch `IN` queries to fetch related data in one round trip. ORMs often generate N+1 patterns silently; review the query log.

Missing indexes on foreign key columns cause full table scans on `JOIN` and `DELETE CASCADE` operations. Most databases do not auto-index foreign keys (PostgreSQL and MySQL included). Prefer adding an index on every foreign key column unless write-heavy benchmarks prove it harmful.

Large transactions that touch many rows hold locks for their entire duration, blocking concurrent writes and risking deadlocks. Prefer batching large updates or deletes into smaller transactions (1000-10000 rows per batch) with brief pauses between batches when running against a live system.
