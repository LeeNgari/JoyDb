# JoyDB Engineering Roadmap & Future Architectural Plans
This roadmap outlines high-priority performance optimizations, structural gaps, and architectural enhancements identified during the query engine and storage layer audit. These issues serve as future engineering tasks to transition JoyDB from a point-lookup optimized in-memory database to a highly concurrent, production-grade RDBMS.
---
## 1. Storage & Mutation Optimizations
### 1.1 High-Complexity Index Rebuilding on Row Mutation
* **Status / Classification:** Performance Bottleneck | Complexity: High
* **Current Behavior:** As implemented in [table.go](file:///C:/Users/lee.ngari/.gemini/antigravity/worktrees/JoyDb/analyze-project-implementation-plan/internal/domain/schema/table.go), deleting or updating a row causes a dense array shift (`table.Rows`) to maintain cache locality, immediately followed by `rebuildIndexesUnsafe()`. This completely clears and recreates both the B+Tree routing index and all secondary hash indexes from scratch in $O(n \log n)$ time.
* **Impact:** Modifying a single row triggers full index re-hydration, causing linear performance degradation as the record count increases.
* **Refactoring Plan:**
  - [ ] **Phase 1: Tombstone Implementation:** Introduce logical tombstones inside `data.Row` (e.g., a boolean `Deleted` flag). Skip tombstoned records during query evaluation and index lookups.
  - [ ] **Phase 2: Stable Row Identifiers:** De-couple the index pointers from physical array slots. Introduce stable, immutable Record IDs (RIDs) so that row array shifts do not require index updates for untouched rows.
  - [ ] **Phase 3: Background Compaction Thread:** Build an asynchronous worker thread that periodically locks small blocks of the array, physically purges tombstones, and adjusts matching index offsets incrementally.
### 1.2 Table Lock Granularity (Coarse Mutexes)
* **Status / Classification:** Concurrency Bottleneck | Complexity: High
* **Current Behavior:** JoyDB relies on a coarse table-level read-write lock (`Table.mu`). During checkpointing, the system acquires a database-wide read-lock (`db.RLock()`), blocking all concurrent DML writers.
* **Impact:** Thread safety is preserved, but concurrent transaction throughput is limited. High-frequency concurrent writes face significant lock contention.
* **Refactoring Plan:**
  - [ ] **Phase 1: Fine-Grained Locking:** Introduce latching/crabbing protocols on internal B+Tree nodes during search and insertion paths so that nodes can be locked independently.
  - [ ] **Phase 2: Row-Level Locks:** Transition from table-level locks to row-level exclusive locks using a centralized Lock Manager.
  - [ ] **Phase 3: MVCC / Snapshot Isolation:** Implement Multi-Version Concurrency Control (MVCC) utilizing the `TxID` transaction state. Read operations should fetch rows corresponding to their snapshot sequence, avoiding blocking active writes.
---
## 2. Durability & Recovery Improvements
### 2.1 Monolithic WAL File & Lack of Log Truncation
* **Status / Classification:** Resource Leak | Complexity: Medium
* **Current Behavior:** The database writes DML and DDL transactions to a single, monolithic `.wal` file that grows indefinitely.
* **Impact:** System execution will eventually deplete physical disk space. Startup recovery times also scale linearly with the size of the WAL file.
* **Refactoring Plan:**
  - [ ] **Phase 1: WAL Segmentation:** Modify `WALManager` to split logs into numbered WAL segments (e.g., `000001.wal`, `000002.wal`) capped at a physical limit (e.g., 64MB).
  - [ ] **Phase 2: Checkpoint Truncation:** Integrate checkpoint completion with log pruning. Once a checkpoint is written and verified, delete or compress all WAL segments that contain only LSNs older than the checkpointed LSN.
### 2.3 WAL Bloat from Aborted Transactions
* **Status / Classification:** Technical Debt | Complexity: Medium
* **Current Behavior:** If a transaction is aborted, a `RecordAbort` entry is logged. However, all intermediate DML inserts, updates, and deletes remain in the physical WAL file.
* **Impact:** Crash recovery correctly skips aborted transactions, but physical storage is wasted, and recovery parsing speeds are degraded by trailing garbage bytes.
* **Refactoring Plan:**
  - [ ] **Phase 1: In-Memory Transaction Buffering:** Modify `ExecutionContext` to buffer uncommitted transaction operations in a volatile memory ring-buffer.
  - [ ] **Phase 2: Atomic Group Commit:** Only serialize and write a transaction's operations to the physical WAL file during `Commit()`. Flush them in a single, contiguous sequential write operation.
### 2.4 Asynchronous Periodic Fsync (Group Commit)
* **Status / Classification:** Scalability | Complexity: Medium
* **Current Behavior:** JoyDB triggers a blocking `fsync` call on the log file during every commit operation. The `wal-sync-interval` configuration flag is scaffolded but not wired into the `WALManager`.
* **Impact:** Maximum transaction throughput is bounded by disk spindle speed or SSD write latency.
* **Refactoring Plan:**
  - [ ] **Phase 1: Background Syncer Thread:** Run a background syncer loop that wakes up at configured intervals (e.g., every 5ms or 10ms).
  - [ ] **Phase 2: Group Commit implementation:** Group multiple concurrently committing transactions into a single write buffer, flushing them with a single shared `fsync` system call.
---
## 3. Query Engine & Compilation Refactoring
### 3.1 Naive Nested Loop Joins
* **Status / Classification:** Algorithmic Bottleneck | Complexity: High
* **Current Behavior:** The physical planner and executor resolve relational joins using a simple nested loop join. Execution also builds temporary in-memory tables from intermediate child outputs, resulting in high memory allocations.
* **Impact:** Quadratic join execution complexity ($O(N \times M)$) causes performance degradation for table relations larger than a few thousand rows.
* **Refactoring Plan:**
  - [ ] **Phase 1: Hash Join Implementation:** Build an in-memory hash index over the join key of the smaller (build) relation, and probe it sequentially using the larger (probe) relation to execute in $O(N + M)$ time.
  - [ ] **Phase 2: Sort-Merge Join:** Leverage sorted keys or pre-existing B+Tree index order to perform merge-joins on pre-sorted arrays without sorting overhead.
### 3.2 Key Comparison Coercion in B+Tree
* **Status / Classification:** Code Quality & Safety | Complexity: Medium
* **Current Behavior:** The B+Tree comparator `compareKeys` in [bplustree.go](file:///C:/Users/lee.ngari/.gemini/antigravity/worktrees/JoyDb/analyze-project-implementation-plan/internal/index/btree/bplustree.go) formats key values as strings via `fmt.Sprintf` as a fallback strategy when types do not match or are not recognized.
* **Impact:** High memory overhead during formatting, along with semantic sorting bugs (string `"10"` sorting before `"2"`).
* **Refactoring Plan:**
  - [ ] **Phase 1: Strict Index Keys:** Enforce index constraints that prevent indexing columns with mismatched datatypes.
  - [ ] **Phase 2: Key Interface Definition:** Create a explicit `Key` interface containing a strict type-safe `Compare(other Key) int` contract to prevent dynamic runtime string-fallbacks.
### 3.3 Missing Index Range Scans in SQL Planner
* **Status / Classification:** Optimization | Complexity: High
* **Current Behavior:** The query planner defaults to a `sequential` scan type for query filtering. The primary key B+Tree is ignored during range scans, behaving as a sparse lookup index only.
* **Impact:** Queries such as `SELECT * FROM users WHERE id > 10` trigger full sequence scans followed by filtering in the executor.
* **Refactoring Plan:**
  - [ ] **Phase 1: Plan Representation:** Introduce an `IndexScanNode` to physical plan nodes.
  - [ ] **Phase 2: Range Propagation:** Teach the query planner to analyze predicates and extract index bounds. Map range filters directly to B+Tree leaf range traversals.
