# JoyDb Engineering Blog Notes

*This document is a living journal of architectural decisions, tradeoffs, and lessons learned while building JoyDb. It serves as raw material for future blog posts and will be appended to throughout the project.*

## Pre-Phase 1: Early Architectural Milestones

Before the formal phases began, several critical architectural decisions were made that transformed JoyDb from a simple REPL toy into a robust engine.

### 1. Decoupling the Engine (The Planner/Executor Shift)
**What we did**: Initially, the REPL executed AST (Abstract Syntax Tree) nodes directly. We refactored the system into a three-tier architecture: Parser -> Planner -> Executor. We also introduced a TCP Server mode.
**The Tradeoff**: The codebase became significantly more complex and required intermediate representations (Plan Nodes). However, this decoupling was mandatory to support optimizations. A database cannot optimize queries if it executes them while parsing them.

### 2. De-JSONing the Storage Engine
**What we did**: Removed `json.RawMessage` fields from WAL payload records and transitioned to raw `[]byte` binary serialization. `ToJSON()` became `Serialize()`.
**The Lesson**: JSON is fantastic for debugging and terrible for database storage. It bloats file sizes and burns CPU cycles on reflection. Moving to binary serialization was the first major step toward a high-performance storage layer.

### 3. WAL Concurrency and Write Ordering
**What we did**: Discovered a critical durability bug and fixed the execution order: we now log to the WAL *before* mutating the in-memory table. We also added read locks during WAL checksum calculations, and truncated garbage bytes to handle mid-write crashes.
**The Lesson**: Write-Ahead Logging means *Ahead*. If you mutate memory first and the WAL write fails, your database is instantly in an inconsistent state. Concurrency also breaks checksums—you cannot calculate a CRC on a file while another thread is simultaneously appending to it.

## Phase 1: Foundational Integrity

### 1. Transaction ID Consolidation
**What we did**: Removed redundant UUID strings for transactions and standardized on `uint64` `TxID`s across the board (engine, WAL, and observers).
**The Tradeoff**: UUIDs are universally unique but expensive to generate, store, and compare. Moving to `uint64` dramatically shrinks memory footprint and WAL size, but requires a central monotonic counter (which could become a bottleneck if not managed carefully).

### 2. Strict B+Tree Key Comparison
**What we did**: Eliminated `fmt.Sprintf` fallback comparisons in the B+Tree and introduced a strict `Key` interface with typed `Compare(other Key) int` methods for native types.
**The Lesson**: Reflection and string coercion are the silent killers of database performance. Forcing strict type comparisons caught hidden sorting bugs and vastly improved tree traversal speed.

### 3. Foreign Key Constraints
**What we did**: Built referential integrity checks directly into the `insert`, `update`, and `delete` executors.
**The Tradeoff**: Enforcing FK constraints slows down writes because every insertion into a child table requires an index lookup in the parent table. It's a classic tradeoff of performance vs. data integrity.

---

## Phase 2 & 3: Benchmarking, WAL, and Recovery

### 1. The Benchmarking Suite as a Compass
**What we did**: Before embarking on massive architectural shifts (Phases 3-5), we built a standalone performance suite measuring TPS and latency.
**The Lesson**: Without a baseline, optimization is just guessing. The benchmark suite became the single source of truth that guided (and sometimes rejected) later "optimizations."

### 2. In-Memory Transaction Buffering (Group Commit)
**What we did**: Stopped writing every single DML operation directly to disk. Buffered them in the `ExecutionContext` and wrote them sequentially to the WAL only upon `Commit()`.
**The Tradeoff**: 
- **Win**: Aborted transactions silently disappear from memory without bloating the WAL. I/O throughput skyrocketed.
- **Loss**: If the database crashes right before a commit, those buffered changes are lost (which is actually correct ACID behavior, but it means holding uncommitted state in RAM).

### 3. WAL Segmentation & Graceful Recovery
**What we did**: Implemented WAL file rotation (e.g., 64MB segments) and improved the crash-recovery parser to salvage valid transactions up to the exact point of byte-level corruption.
**The Lesson**: Disks fail, and processes crash mid-write. A database engine cannot just panic on a trailing garbage byte; it must salvage everything it can.

---

## Phase 4: Query Execution & Planner Polish

### 1. Multi-Way JOINs & Schema Lineage
**What we did**: Overhauled the Query Planner to enforce explicit output schema propagation. Stopped relying on dynamic `join_0x...` prefixes and implemented standard column identifier resolution.
**The Lesson**: Ad-hoc map merging works for a single join, but fundamentally breaks down for N-way joins. The planner *must* know the exact shape of the data before execution begins.

### 2. AST Upgrades: Range Scans & Aliases
**What we did**: Added `IndexScanNode` to map `WHERE id > 10` to B+Tree leaf traversals, and implemented `AS` syntax for table and column aliases.

---

## Phase 5: The Road to Real Database Storage (Concurrency & Scaling)

Phase 5 marked the transition of JoyDb from a simple slice-of-structs toy to a B+Tree backed, concurrently safe, real-world architecture. The journey was fraught with performance regressions and hard lessons about Go's memory model.

### Phase 5.1: Tombstones, B+Trees, and the Cost of Maps
**What we did**: 
Decoupled physical array indices from logical indexing by introducing immutable Record IDs (RIDs). Replaced the monolithic `Rows []data.Row` slice with a `map[int64]data.Row` (for storage) and a `BPlusTree` (for primary key indexing). We also introduced strict 2-tier locking to prevent deadlocks and logical tombstones (`Deleted: true`) to avoid $O(n \log n)$ array shifts on deletion.
**The Tradeoff**: 
- **Win**: OLTP workloads (point lookups, inserts, updates) became incredibly fast and concurrent. We gained proper O(log N) indexing.
- **Loss**: Full table scans plummeted by 50x (e.g., 6,700 TPS down to 132 TPS for 10k rows).
**The Lesson**: 
Iterating a map and calling `sort.Slice` to guarantee deterministic ordering is devastatingly slow compared to iterating a contiguous slice.

### Phase 5.2: The Iterator Trap
**What we did**: 
Attempted to fix the scan regression by bypassing the `sort.Slice`. Instead of iterating the map and sorting, we walked the B+Tree leaf nodes to get pre-sorted RIDs (`PKIndex.All()`), and then fetched each row from the map.
**The Tradeoff**: 
- **Win**: Single-row operations and selective `WHERE` clauses doubled in speed because we stopped deep-copying rows we didn't need.
- **Loss**: Small table scans (1k rows) actually got *17% slower*. 
**The Lesson**: 
*Constant factors matter.* N individual map lookups (`t.RowsByRID[rid]`) have so much hashing and bucket-traversal overhead that they are actually slower than doing 1 raw map iteration and 1 quick `sort.Slice`. 

Furthermore, we discovered the **Ultimate Bottleneck**: Deep-copying `map[string]interface{}`. Even with perfect iteration, allocating 10,000 maps per query took 5-6 milliseconds. It physically hard-capped throughput at ~160 TPS regardless of iteration strategy.

### Phase 5.3: Array-Based Rows (Escaping the Map Bottleneck)
**What we did**: 
Recognized that real Go databases *never* use maps for row storage. We refactored `Row` to use `[]interface{}` instead of `map[string]interface{}`, relying on `TableSchema` to resolve column names to array indexes at planning time.
* **Phase 1 (Core)**: Transitioned the base `Row` struct and binary/JSON serialization. We observed that dropping the column name strings from the binary format inherently shrinks the row size on disk.
* **Phase 2 (Operations)**: Refactored base table operations (`Insert`, `Update`, `Delete`) and Executors to perform map-to-array translation using `TableSchema.GetColumnIndex`. Data enters the executor as a map (from the parser) but is immediately packed into contiguous arrays for storage.
**The Tradeoff**: 
- **Win**: Slices copy in nanoseconds; maps take microseconds. Scan speeds increased exponentially. Cache locality drastically improved, and Go Garbage Collection pressure dropped by ~60%. WAL file sizes shrank by 50% because we stopped writing column names in JSON objects.
- **Loss**: The codebase became more complex. The query planner now had to be "schema-aware" to map `WHERE age > 25` into `row.Values[2] > 25` before execution.

---

### Phase 5.4: Zero-Allocation Reads & True Hash Joins
**What we did**: 
Identified that `Select` and `SelectAll` were doing a deep `row.Copy()` for every row, allocating 10,000 slices per query. We realized that JoyDB's `UpdateRow` actually deep-copies *before* mutating the storage map, meaning the map values are strictly immutable! We stripped out the final `sync.Mutex` from the row struct and removed the `row.Copy()` during scans.
For Joins, we deleted the obsolete `createTempTable` materialization logic and rewrote the Executor to perform a **True Hash Join** directly on the raw tuple streams ($O(N+M)$).
**The Tradeoff**:
- **Win**: Zero-allocation scans. Scan speeds jumped by +64% (hitting ~5.4 million rows/sec). Join speeds skyrocketed by over 350% (3.6x faster). The codebase actually became simpler as we deleted the entire intermediate `join` package.
- **Loss**: None. We achieved pure speed by utilizing the native Map properties.

### Phase 5.5: Benchmarking Reality Check (Visibility over Ops/sec)
**What we did**: 
Received critical feedback that our benchmark harness was measuring raw throughput (`ops/sec`) while completely ignoring database behavior (tail latencies, garbage collection, memory overhead, and concurrency). We discovered a massive blind spot: we were explicitly turning off the GC (`debug.SetGCPercent(-1)`) during benchmarks and throwing away raw latency data to save JSON size!
We overhauled the harness to:
1. Capture `runtime.MemStats` (Heap Allocations and GC Pause times).
2. Persist a statistically significant down-sampled slice of raw Latencies (900 uniform + 100 worst) so we could observe P99 tail spikes.
3. Add true goroutine concurrency for lock contention testing.
4. Add a `baseline_map_mutex` workload (raw Go map with RWMutex) to compare JoyDb's engine overhead against pure language bounds.
**The Tradeoff**: 
- **Win**: We discovered that JoyDb is actually incredibly efficient—only about ~10x slower than a raw native Go map for inserts (`~61k TPS` vs `~618k TPS`). However, we also proved the GC pauses theory: P99 tail latencies spike by 10x (0.01ms -> 0.10ms) due to garbage collection cycles. We also discovered that synchronous WAL (`fsync` per commit) plummets throughput to just 111 TPS.
- **Loss**: JSON history files got slightly larger due to the latency arrays, but it was worth it.
**The Lesson**: 
A benchmark suite that throws away tail latencies and disables the garbage collector is a harness lying to itself. "Fast on average" doesn't matter for databases; tail latency under load is the only thing that matters.

---

### Phase M4.3: Aggregations (`COUNT`, `SUM`, `MIN`, `MAX`, `AVG`)
**What we did**: 
- **Lexer & AST:** Introduced tokens for aggregate functions and added `AggregateFunctionCall` struct in AST. Updated `SelectStatement.Fields` from `[]*Identifier` to `[]Expression` to support function calls.
- **Parsing:** Implemented `parseSelectFields()` to handle both identifiers and aggregate functions, supporting optional `AS` aliases for aggregates.
- **Planner:** Updated planner to validate aggregate arguments and extract `plan.AggregateSpec` structures into the `SelectNode`.
- **Execution:** Added a blocking loop in `select_executor.go`. When aggregates are present, the executor streams all rows, computes running accumulators, and yields a single synthetic row with a synthetic `TableSchema`. Added type coercion (`toFloat64`) to handle numbers gracefully.
**The Tradeoff**: 
- **Win**: We now have basic analytics capabilities.
- **Loss**: Aggregations are pipeline breakers. They require iterating all rows in the intermediate result before emitting a single synthetic row, unlike normal projection which streams nicely.

---

## What is Missing (The Roadmap)

While the engine is blazing fast, there are several key architectural milestones left to tackle:

### 1. Concurrency & Compaction
* **Central Lock Manager:** We need to implement a Lock Manager to track fine-grained row-level locks by their logical `RID` to unblock concurrent writers (replacing the table-level RWMutex).
* **Background Vacuum Thread:** A background worker is needed to periodically scan for and compact tombstones (deleted rows) to prevent memory leaks over time.

### 2. Analytical SQL (Milestone 4)
* **HAVING:** Add post-aggregation predicates once aggregate expressions can be represented in general expression trees.
* **Grouping Expressions:** Extend identifier-only grouping to expressions, ordinals, and aggregate expressions in `ORDER BY`.

### 3. The Embedded SDK (Phase 6)
* **Module Rename:** Rename the project from `mini-rdbms` to `joydb`.
* **Public API:** Expose a clean, two-tier public API (`Store` -> `DB`) so developers can easily `import "github.com/leengari/joydb/joydb"` and use it as an embedded database in their own applications.

---

## Core Engineering Themes for the Blog

1. **Memory Allocation is the Enemy in Go**: 
   A database in Go will live or die by the Garbage Collector. Escaping `map[string]interface{}` is mandatory for performance.
2. **The "Execution Layer" Middle Ground**: 
   While hardcore databases serialize rows into contiguous `[]byte` arrays (Tuple format) to completely bypass the GC, JoyDb opted for the "Typed Slice" approach (`[]interface{}`). This is a pragmatic middle ground used by execution engines like TiDB, offering massive speedups over maps without writing a custom binary serialization engine.
3. **Benchmarking as a Compass**: 
   We wouldn't have found the map-lookup penalty in Phase 5.2 without rigorous, workload-specific benchmarks. When a "zero-copy optimization" makes things slower, profiling and benchmarking are the only ways to uncover the hidden overheads of the runtime.
4. **Data Structures dictate Architecture**:
   The move from a slice to a map enabled concurrent B+Tree indexing but destroyed table scan performance. Every data structure solves one problem by creating another.

---

## Phase 6: From Database Engine to Embedded Go Library (2026-07-10)

### 1. A Public API at the Module Root
**What we did**: Renamed the module from `github.com/leengari/mini-rdbms` to `github.com/leengari/joydb` and exposed the SDK directly at the module root. Applications now call `joydb.Open`, access isolated databases through `Store.DB`, and use `Exec`/`Query` without importing internal packages.
**The lesson**: Package ergonomics are part of product design. Avoiding `joydb/joydb` import stutter made the first interaction with the library simpler, while keeping engine, planner, executor, WAL, and storage implementation details behind Go's `internal` boundary.

### 2. Create-on-Open and Two Storage Modes
**What we did**: Added create-on-open database handles, per-store handle caching, lifecycle methods, functional WAL/checkpoint options, and a genuine RAM-only `InMemoryEngine` for `Open(":memory:")`.
**The tradeoff**: The existing `MemoryEngine` was actually disk-backed snapshot storage. Adding a separate in-memory backend duplicated a small lifecycle surface, but it kept filesystem concerns out of RAM mode and made the no-disk guarantee testable.

### 3. Typed Results Without Exposing Internal Rows
**What we did**: Wrapped executor results in a `Rows` iterator with `Scan`, `StructScan`, column metadata, null checks, and named typed accessors. `ExecResult` now exposes affected rows and auto-increment IDs.
**The lesson**: A user-friendly embedded API should make the common path direct without hiding errors. Struct tags and ordered scans cover most application code, while raw values remain available for dynamic consumers.

### 4. Parameters Belong in the AST
**What we did**: Added `?` tokens, ordinal `ParameterExpression` AST nodes, typed parameter binding, and engine argument propagation across SELECT, INSERT, UPDATE, DELETE, and JOIN expressions.
**The lesson**: String replacement is not parameterization. Binding values after parsing prevents question marks and quotes inside data from changing SQL structure, and lets schema conversion operate on typed values rather than reconstructed SQL text.

### 5. Transactions Before Fine-Grained Locks
**What we did**: Split engine execution into auto-commit and caller-owned transaction paths. A public transaction buffers one WAL transaction, holds exclusive ownership of its `DB`, and keeps a deep catalog snapshot so DDL and DML can both be rolled back. Context-aware APIs stop work at execution-stage boundaries.
**The tradeoff**: Transactions are correct but serialized. Holding the DB lock sacrifices concurrent transaction throughput, yet provides clear atomic behavior today. The future row-level lock manager can improve concurrency without changing the public transaction contract.

### Phase 6 Verification
- Full repository test suite passes with `go test ./...`.
- Every package and command builds with `go build ./...`.
- Black-box SDK tests cover persistent recovery, in-memory isolation, typed scanning, parameter edge cases, public errors, transaction commit/rollback, DDL rollback, and cancelled contexts.

---

## Phase M4.4: GROUP BY and the Shape of Aggregate Results (2026-07-10)

### 1. Grouping Is More Than a Hash Map
**What we did**: Added `GROUP BY` across the lexer, AST, parser, planner, and executor. The first release supports one or more identifier columns, qualified columns across joins, every existing aggregate function, NULL groups, grouped distinct results, and post-aggregation ordering and pagination.
**The lesson**: Partitioning rows was the easy part. The difficult contract was preserving the SELECT-list order while carrying enough column lineage to distinguish `employees.id` from `departments.id`. The plan now keeps ordered select items instead of treating projections and aggregates as unrelated lists.

### 2. Aggregate Queries Need a Result Pipeline
**What we did**: Replaced the aggregate executor's early return with a unified aggregate-result path. Both grouped and ungrouped aggregates now produce a typed intermediate schema before `ORDER BY`, `OFFSET`, and `LIMIT` run.
**The lesson**: Aggregation changes the meaning of every downstream operator. Sorting input rows before `COUNT` is wasted work, and returning immediately after aggregation silently skips pagination. A pipeline breaker still needs to rejoin the execution pipeline as a normal relation.

### 3. Stable Groups and Typed Keys
**What we did**: Used a typed binary encoding for multi-column group keys and retained groups in first-seen order. NULL, integer, float, text, and boolean values cannot collide through delimiter tricks, while explicit `ORDER BY` remains the authority for requested output order.
**The tradeoff**: The initial implementation stores rows per group and reuses the existing aggregate semantics. Incremental accumulator state would reduce memory for large analytical workloads, but the current design keeps behavior centralized and testable while the SQL surface is still growing.

### 4. Strict SQL Validation
**What we did**: The planner rejects non-aggregate SELECT columns missing from `GROUP BY`, mixed grouped and ungrouped projections, `SELECT *` with grouping, missing columns, and ambiguous unqualified join columns. `HAVING`, grouping expressions, ordinals, and `ORDER BY COUNT(*)` remain explicit follow-up work.
**The lesson**: Rejecting an unsupported query is safer than returning a plausible but incorrect row. Planner errors turn silent data bugs into actionable feedback before execution starts.

### GROUP BY Verification
- Parser tests cover single, multiple, and qualified grouping columns with following clauses.
- Planner tests cover ordered output items and invalid non-grouped columns.
- Integration tests cover all aggregates, NULL handling, WHERE, multi-column grouping, grouped distinct results, ORDER BY aliases, LIMIT/OFFSET, empty input, validation failures, and grouping across a join.
