# JoyDb Engineering Blog Notes

*This document is a living journal of architectural decisions, tradeoffs, and lessons learned while building JoyDb. It serves as raw material for future blog posts and will be appended to throughout the project.*

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
**The Tradeoff**: 
- **Win**: Slices copy in nanoseconds; maps take microseconds. Scan speeds increased exponentially. Cache locality drastically improved, and Go Garbage Collection pressure dropped by ~60%. WAL file sizes shrank by 50% because we stopped writing column names in JSON objects.
- **Loss**: The codebase became more complex. The query planner now had to be "schema-aware" to map `WHERE age > 25` into `row.Values[2] > 25` before execution.

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
