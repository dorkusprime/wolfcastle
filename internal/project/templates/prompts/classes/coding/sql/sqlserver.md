# SQL Server

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## T-SQL Style

Prefer `THROW` over `RAISERROR` for raising errors in SQL Server 2012+. `THROW` re-raises the current error inside a `CATCH` block without requiring parameters, reports accurate line numbers, and always terminates the batch. `RAISERROR` remains necessary only when you need to raise a message below severity 16 without terminating the batch.

Prefer `SET XACT_ABORT ON` at the top of stored procedures and scripts that contain transactions. When enabled, any runtime error automatically rolls back the entire transaction and terminates the batch. Without it, some errors leave the transaction open, requiring explicit `ROLLBACK` in the `CATCH` block and risking orphaned transactions.

Prefer semicolons as statement terminators. T-SQL does not require them everywhere, but `MERGE`, `WITH` (CTEs), and `THROW` require the preceding statement to end with a semicolon. Consistent semicolons prevent parsing surprises.

## TRY...CATCH

Prefer wrapping transactional logic in `BEGIN TRY ... END TRY BEGIN CATCH ... END CATCH`. Inside the `CATCH` block, check `XACT_STATE()`: a value of `-1` means the transaction is uncommittable and must be rolled back; `1` means it is still committable; `0` means no transaction is active.

Prefer keeping `TRY` blocks focused. Only the code that might fail belongs inside `TRY`. Variable declarations, temporary table creation, and result formatting should live outside when possible.

`TRY...CATCH` does not catch compile-time errors (misspelled column names resolved at parse time), statement-level recompilation errors, or errors with severity 20+ that terminate the connection. Deferred name resolution means some object-not-found errors surface at runtime and are caught; others surface at compile time and are not.

## OUTPUT Clause

Prefer `OUTPUT` to capture affected rows from `INSERT`, `UPDATE`, `DELETE`, and `MERGE` in a single statement. `OUTPUT inserted.*` returns the new values on `INSERT`; `OUTPUT deleted.*` returns the old values on `DELETE`; `UPDATE` can access both `inserted` and `deleted` pseudo-tables.

Prefer `OUTPUT INTO @table_variable` when the results must be stored for further processing within the same batch. `OUTPUT` without `INTO` streams rows to the client directly.

The `OUTPUT` clause cannot be used reliably when the target table has triggers (SQL Server may raise an error or produce unexpected results). In trigger-present scenarios, prefer a separate `SELECT` after the DML or use `SCOPE_IDENTITY()` for single-row inserts.

## MERGE Statement

Prefer `MERGE` for synchronized upserts where `INSERT`, `UPDATE`, and `DELETE` all depend on the same match condition. A single `MERGE` replaces the classic `IF EXISTS ... UPDATE ... ELSE ... INSERT` pattern with declarative `WHEN MATCHED`, `WHEN NOT MATCHED BY TARGET`, and `WHEN NOT MATCHED BY SOURCE` clauses.

Prefer terminating `MERGE` with a semicolon; the parser requires it. Combine with `OUTPUT $action, inserted.*, deleted.*` to capture which rows were inserted, updated, or deleted in one pass.

Prefer caution with `MERGE` on high-concurrency tables. It is susceptible to race conditions and deadlocks under concurrent execution. For critical upsert paths, `INSERT ... SELECT ... WHERE NOT EXISTS` inside a serializable transaction, or an explicit `UPDATE` then `INSERT` with proper locking hints, may be more predictable. Test under realistic concurrency.

## Temp Tables vs Table Variables

Prefer `#temp` tables for moderate-to-large result sets that participate in joins or need indexes. Temp tables have statistics, support indexes (both inline and after creation), and participate in parallel execution plans. They persist for the session or the scope of the creating stored procedure.

Prefer `@table` variables for small result sets (hundreds of rows) where the overhead of statistics and recompilation is unnecessary. Table variables do not have distribution statistics (the optimizer estimates one row), do not cause recompilation, and are scoped to the batch. On SQL Server 2019+, `OPTION (RECOMPILE)` or the `DEFERRED_COMPILATION_TV` database-scoped configuration give the optimizer accurate cardinality for table variables.

Prefer `SELECT INTO #temp FROM ...` for materializing a query result. It infers the schema from the source columns, avoiding manual column declarations.

## Window Functions

Prefer window functions (`ROW_NUMBER()`, `RANK()`, `DENSE_RANK()`, `NTILE()`, `LAG()`, `LEAD()`, `FIRST_VALUE()`, `LAST_VALUE()`, `SUM() OVER()`, `COUNT() OVER()`) over self-joins, correlated subqueries, and cursors for ranking, pagination, running totals, and gap detection.

Prefer `ROW_NUMBER() OVER (ORDER BY ...) BETWEEN @start AND @end` patterns for keyset-adjacent pagination. For true keyset pagination, prefer `WHERE id > @last_seen ORDER BY id FETCH FIRST N ROWS ONLY`, which avoids the cost of computing row numbers.

Prefer `OFFSET ... FETCH` (SQL Server 2012+) over `TOP` for paginated queries when the offset is small. For large offsets, both degrade; keyset pagination avoids the problem entirely.

## Execution Plans

Prefer `SET STATISTICS IO ON` and `SET STATISTICS TIME ON` for quick performance diagnostics. Logical reads (buffer pool hits) and physical reads (disk hits) reveal whether a query is touching more pages than expected.

Prefer `INCLUDE` columns in non-clustered indexes to cover queries without widening the index key. A covering index eliminates key lookups, which are the most common cause of "the query uses the index but is still slow."

Prefer examining the actual execution plan (`SET STATISTICS XML ON` or SSMS "Include Actual Execution Plan") over the estimated plan. The actual plan shows runtime row counts, memory grants, spills to tempdb, and parallel thread distribution.

## SQL Server 2022+ Features

Prefer `GENERATE_SERIES(start, stop [, step])` for producing number sequences without recursive CTEs or permanent tally tables.

Prefer `GREATEST(a, b, c, ...)` and `LEAST(a, b, c, ...)` for row-level min/max across columns. They replace verbose `CASE` expressions that compare values pairwise.

Prefer `DATETRUNC(part, date)` over `CAST`/`CONVERT` tricks for truncating dates to a specific precision (year, month, day, hour).

Prefer `JSON_OBJECT()` and `JSON_ARRAY()` (SQL Server 2022+) for constructing JSON inline. They complement the existing `FOR JSON PATH` for result-set serialization and `OPENJSON()` for parsing.

Prefer `IS [NOT] DISTINCT FROM` for null-safe equality comparisons. `a IS NOT DISTINCT FROM b` returns `TRUE` when both are `NULL`, unlike `a = b` which returns `UNKNOWN`.

Prefer `WINDOW` clause (SQL Server 2022+) to define a named window specification reused across multiple window functions in the same query, reducing duplication.

## SQL Server 2025 Features

SQL Server 2025 is generally available. It adds a native `VECTOR` data type with built-in vector search, native JSON document support, REST API endpoints, and RegEx functions directly in T-SQL.

Prefer Change Event Streaming (CES) over CDC or polling for real-time data streaming to Azure Event Hubs.

Prefer Optional Parameter Plan Optimization (SQL Server 2025) to reduce parameter sniffing issues without manual `OPTION (RECOMPILE)` hints.

SQL Server 2025 enforces TLS 1.3 by default on new installations. Resource Governor is now available in Standard edition. The Express edition maximum database size is now 50 GB, and Express with Advanced Services has been discontinued (its features are folded into the base Express edition).

## Testing

Prefer tSQLt for unit testing stored procedures, functions, and views. tSQLt runs inside the database, uses transactions for isolation (each test rolls back automatically), and provides `FakeTable`, `SpyProcedure`, and `AssertEqualsTable` for test setup and assertions.

Prefer isolating tests from shared state. `tSQLt.FakeTable` replaces a table with an empty copy inside the test transaction, removing dependencies on seed data and constraints. `tSQLt.ApplyConstraint` re-enables specific constraints when the test targets constraint behavior.

Prefer testing execution plans indirectly: assert on behavior and performance characteristics (row counts, elapsed time within a threshold) rather than on plan shape. Plan shapes change with statistics updates, parameter sniffing, and engine upgrades.

## Common Pitfalls

Parameter sniffing causes the optimizer to compile a plan based on the first parameter values it sees. Subsequent executions reuse that plan even if different parameters would benefit from a different strategy. Symptoms include queries that are fast for one filter value and slow for another. Prefer `OPTION (RECOMPILE)` on the specific statement (not the entire procedure) when parameter distributions vary widely. `OPTIMIZE FOR UNKNOWN` is a softer alternative that uses average statistics.

`NOLOCK` (`READ UNCOMMITTED`) reads dirty, uncommitted data. It does not make queries faster; it skips lock acquisition, which can also skip rows or read rows twice due to page splits during concurrent writes. Prefer `READ COMMITTED SNAPSHOT ISOLATION` (RCSI) at the database level for non-blocking reads with consistency guarantees.

`VARCHAR` vs `NVARCHAR` determines encoding. `VARCHAR` uses a single-byte code page; `NVARCHAR` uses UTF-16. Comparing an `NVARCHAR` column against a `VARCHAR` literal forces implicit conversion on every row, disabling index usage. Match the literal type to the column type: use `N'value'` for `NVARCHAR` columns.

Arithmetic overflow on `SUM()` over `INT` columns can fail silently or throw, depending on the `ANSI_WARNINGS` setting. Prefer `CAST(col AS BIGINT)` inside the aggregate when the sum of an `INT` column might exceed 2,147,483,647.

`IDENTITY` columns can have gaps after rolled-back inserts, server restarts, or deletes. Code that assumes sequential, gapless identity values will eventually break. Prefer treating identity values as opaque handles, not as counters or sequence positions.
