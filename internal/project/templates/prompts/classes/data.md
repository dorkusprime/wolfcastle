# Data

When the project you're working in has established data conventions, schema patterns, or pipeline practices that differ from what's described here, follow the project.

## Schema Design

**Name things for the domain, not for the storage engine.** A column named `val1` communicates nothing. A column named `order_total_cents` communicates type, unit, and meaning. Use consistent naming conventions across tables: if some columns use snake_case and others use camelCase in the same schema, that inconsistency will propagate into every query and every application layer that touches the data.

**Store data in its most precise form.** Store timestamps with time zones. Store monetary values as integers in the smallest currency unit (cents, not dollars). Store durations in a fixed unit (milliseconds, not "sometimes seconds and sometimes minutes"). Precision lost at storage time cannot be recovered downstream. Formatting and rounding belong at the presentation layer.

**Design schemas for query patterns, not just storage.** A normalized schema that requires seven joins for the most common query is optimized for the wrong thing. Understand the read and write patterns before choosing a structure. Denormalization is a tradeoff, not a sin; document what it trades and why.

## Pipeline Design

**Make every pipeline step idempotent.** Running a pipeline step twice with the same input must produce the same output without duplication or corruption. Use upserts instead of inserts. Key on natural identifiers. Track watermarks to define processing boundaries. Idempotency transforms retry from a risk into a recovery mechanism.

**Validate at every boundary.** Data entering the system from an external source, data crossing between pipeline stages, and data leaving the system for downstream consumers all cross trust boundaries. Validate schema conformance, check for null or missing required fields, verify value ranges, and reject or quarantine records that fail. A validation gap at an ingestion boundary will surface as a mysterious failure three stages later.

**Prefer ELT over ETL when the target system is capable.** Loading raw data first and transforming inside the warehouse preserves the original signal for reprocessing and debugging. Transformations become SQL (or equivalent) that version controls cleanly and runs where the compute scales with the data. ETL remains appropriate when the target system cannot perform the transformation or when pre-load filtering is necessary for cost or compliance reasons.

## Data Quality

**Define and enforce data contracts.** A data contract specifies the schema, semantics, freshness guarantee, and quality thresholds for a dataset. Producers own the contract; consumers depend on it. Without a contract, every consumer independently discovers the data's quirks and builds private workarounds, and those workarounds diverge.

**Handle null and missing data explicitly.** Null means unknown, not zero, not empty string, not "not applicable." Coalesce nulls only when the replacement value has domain meaning. Silently treating null as zero in a sum changes the answer; filtering out null rows changes the denominator. Document the null semantics for every field in the schema.

**Monitor data freshness and volume.** A pipeline that ran successfully but produced zero rows is not healthy. A dataset that hasn't updated in 36 hours when it normally updates every 6 hours is not healthy. Track row counts, freshness timestamps, and distribution statistics. Alert when they deviate from historical norms.

## SQL Style

**Write SQL for the next reader.** Use uppercase for keywords (SELECT, FROM, WHERE). One clause per line. Indent subqueries. Alias every table and every computed column. A query that runs correctly but reads as a wall of text will be misunderstood, mismodified, and eventually produce wrong results because nobody could follow what it was doing.

**Qualify column names with table aliases.** In any query involving more than one table, unqualified column names force the reader to guess which table a column comes from. They also break silently when a new column with the same name is added to a different table. Qualify every column reference.

## Visualization

**Choose the chart type for the question, not the data.** A time series answers "what changed?" and calls for a line chart. A comparison answers "which is larger?" and calls for a bar chart. A composition answers "what fraction?" and calls for a stacked bar or pie. Choosing a chart type before articulating the question the audience needs answered produces charts that display data without communicating meaning.

**Label axes, include units, and show context.** A chart without axis labels is a shape. A chart without units is ambiguous. A y-axis labeled "latency" could mean microseconds or hours. Show reference lines for targets or SLAs. Show previous-period comparisons when trend matters. The reader should understand the chart without reading the surrounding text.
