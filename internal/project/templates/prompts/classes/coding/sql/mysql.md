# MySQL

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Storage Engines

Prefer InnoDB for all new tables. InnoDB provides row-level locking, ACID transactions, crash recovery, and foreign key enforcement. MyISAM lacks transactions and uses table-level locking; it persists in legacy schemas but should not be chosen for new work.

Prefer verifying the engine on existing tables before assuming InnoDB: `SHOW TABLE STATUS WHERE Name = 'tablename'`. Mixed-engine databases cannot wrap cross-engine operations in a single transaction.

Prefer MEMORY tables only for ephemeral session data or temporary lookup caches where loss on restart is acceptable. MEMORY uses hash indexes by default (no range queries) and has a fixed row format that wastes space on variable-length columns.

## Character Sets and Collations

Prefer `utf8mb4` over `utf8` (`utf8mb3`). MySQL's `utf8` encodes at most three bytes per character, silently truncating four-byte sequences (emoji, some CJK characters). `utf8mb4` is true UTF-8. MySQL 8.0+ defaults to `utf8mb4` with `utf8mb4_0900_ai_ci` collation.

Prefer setting character set at the database or table level rather than relying on the server default. Connection character set mismatches between client and server cause mojibake. Verify with `SHOW VARIABLES LIKE 'character_set%'`.

Prefer `utf8mb4_bin` collation when case-sensitive, accent-sensitive comparison is needed (tokens, hashes, encoded identifiers). The default `_ai_ci` collation treats `a` and `A` as equal, which may surprise code that stores case-significant values.

## Upsert Patterns

Prefer `INSERT ... ON DUPLICATE KEY UPDATE` for upsert operations. It fires on primary key or unique index collision and updates the existing row. Reference the incoming values with `VALUES(col)` in MySQL 8.0 or the row alias syntax (`AS new_row`) in MySQL 8.0.19+: `INSERT INTO t (...) VALUES (...) AS new ON DUPLICATE KEY UPDATE col = new.col`.

Prefer `INSERT IGNORE` only when the goal is to silently discard duplicates without updating the existing row. `IGNORE` suppresses all errors (not just duplicates), which can mask data truncation and constraint violations.

Prefer `REPLACE` sparingly. It deletes the conflicting row and inserts a new one, resetting auto-increment values and firing delete triggers. In most cases `ON DUPLICATE KEY UPDATE` is more predictable.

## Window Functions and CTEs

Prefer CTEs (`WITH` clauses) for readability in MySQL 8.0+. Both non-recursive and recursive CTEs are supported. Recursive CTEs handle tree traversal and hierarchical queries that previously required application-side recursion or stored procedures.

Prefer window functions (`ROW_NUMBER()`, `RANK()`, `DENSE_RANK()`, `LAG()`, `LEAD()`, `SUM() OVER()`) over self-joins or correlated subqueries for ranking, running totals, and gap detection. Window functions execute after `WHERE` and `GROUP BY`; filter on a window result by wrapping the windowed query in a subquery or CTE.

Prefer named windows (`WINDOW w AS (PARTITION BY ... ORDER BY ...)`) when multiple window functions share the same partitioning and ordering. It reduces repetition and ensures consistency.

## Generated Columns

Prefer `GENERATED ALWAYS AS (expr) STORED` for computed values that should be indexed or frequently queried. Stored generated columns are written to disk and can have secondary indexes. Virtual generated columns (`VIRTUAL`) are computed on read and cannot be indexed in all cases (InnoDB supports secondary indexes on virtual columns, but primary key and full-text indexes require stored columns).

Prefer generated columns over application-side computation when the derivation is deterministic and the column participates in queries. `ALTER TABLE ADD COLUMN ... GENERATED ALWAYS AS ... VIRTUAL` is an instant metadata operation in MySQL 8.0+ (no table rebuild).

## GROUP_CONCAT and Aggregation

Prefer `GROUP_CONCAT(col ORDER BY col SEPARATOR ',')` for assembling delimited lists within a group. The default separator is a comma. Watch the `group_concat_max_len` system variable (default 1024 bytes); results exceeding this limit are silently truncated. Increase it per-session when building long lists: `SET SESSION group_concat_max_len = 1000000`.

Prefer `JSON_ARRAYAGG(col)` and `JSON_OBJECTAGG(key, value)` (MySQL 8.0+) when the consumer expects structured JSON rather than a flat string. These functions produce valid JSON without manual escaping.

## Schema Changes on Large Tables

Prefer `ALTER TABLE ... ALGORITHM=INSTANT` for column additions at the end of the table and metadata-only changes (renaming, changing default values). Instant DDL avoids table copies and locks.

Prefer `ALTER TABLE ... ALGORITHM=INPLACE` for index creation, column reordering, and type changes that InnoDB can perform without a full copy. Inplace DDL may still require a metadata lock that blocks concurrent DDL but allows DML.

Prefer external schema migration tools (`gh-ost`, `pt-online-schema-change`, `spirit`) for large tables in production when the ALTER requires a table copy. These tools copy data in the background, apply changes incrementally, and perform an atomic cutover with minimal locking.

## Testing

Prefer testing against a real MySQL instance matching the production version. MariaDB diverges from MySQL on CTE optimization, JSON functions, window function edge cases, and system variable names. A test suite passing on MariaDB does not guarantee correctness on MySQL (and vice versa).

Prefer transaction-wrapped tests (`START TRANSACTION` / `ROLLBACK`) for isolation. In MySQL, DDL statements (`CREATE TABLE`, `ALTER TABLE`, `TRUNCATE`) cause an implicit commit, breaking the transaction boundary. Keep DDL in separate setup scripts, not inside test transactions.

## Common Pitfalls

`ONLY_FULL_GROUP_BY` is enabled by default in MySQL 8.0+. Queries that select non-aggregated columns not in the `GROUP BY` clause produce an error. This catches ambiguous grouping that older MySQL silently resolved by picking an arbitrary row. Prefer listing all non-aggregated columns in `GROUP BY` or wrapping them in `ANY_VALUE()` when the choice genuinely does not matter.

Implicit type casting on indexed columns disables index usage. `WHERE varchar_col = 123` forces per-row casting; use `WHERE varchar_col = '123'` instead. `EXPLAIN` reveals full table scans caused by this.

`TIMESTAMP` columns have a range of 1970-01-01 to 2038-01-19 and are stored in UTC. `DATETIME` covers 1000-01-01 to 9999-12-31 and stores the literal value without timezone conversion. Prefer `DATETIME` for dates outside the `TIMESTAMP` range or when timezone conversion at the storage layer is unwanted. Prefer `TIMESTAMP` when UTC normalization is desired and the range is sufficient.

Foreign keys are not supported on MyISAM or MEMORY tables. Creating a foreign key on a MyISAM table silently succeeds but is never enforced. Always verify the engine before relying on referential integrity.

`TRUNCATE TABLE` resets the auto-increment counter, while `DELETE FROM table` without a `WHERE` clause does not. This distinction matters for integration tests that rely on predictable ID sequences.
