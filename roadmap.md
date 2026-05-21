# JoyDB roadmap

## Phase 1:

### 1. Transaction ID Consolidation
- **Context:** `Transaction.ID` (UUID string) is redundant alongside `Transaction.TxID` (uint64), which is required by the WAL.
- **Action:**
  - Remove `Transaction.ID` completely from `internal/domain/transaction/transaction.go`.
  - Update all observers and engine tracing (e.g. `Event.TxID`) to use `uint64`.

### 2. Strict B+Tree Key Comparison
- **Context:** `compareKeys` in `bplustree.go` currently falls back to `fmt.Sprintf` for unknown types, causing slow performance and semantic sorting bugs.
- **Action:**
  - Introduce a strict `Key` interface with a `Compare(other Key) int` method.
  - Implement the interface for all native database types (Int, Float, String).
  - Remove string-fallback coercion.

### 3. Foreign Key Constraints
- **Context:** Foreign key validation is missing.
- **Action:**
  - Extend `TableSchema` to track foreign key relationships.
  - Update the `insert` and `update` executors to validate referential integrity against parent tables.
  - Update the `delete` executor to validate/cascade deletions.

---

## Phase 2: Database Benchmarking

### 1. Performance Testing Suite
- **Context:** As JoyDB undergoes major architectural changes (e.g., locking, indexes, WAL group commit), we need quantitative metrics to ensure these changes improve throughput.
- **Action:**
  - Build a standalone benchmarking suite for JoyDB mimicking industry standards (like TPC-C or Sysbench).
  - Measure transactions per second (TPS), read/write latency, and concurrency throughput.
  - Establish a baseline before Phase 2-4 changes and continuously measure against it.


## Phase 3: WAL & Recovery Polish

### 1. In-Memory Transaction Buffering & Group Commit
- **Context:** Currently, aborted transactions bloat the WAL, and every operation writes individually.
- **Action:**
  - Buffer DML operations in memory within the `Transaction` or `ExecutionContext`.
  - Only write the buffered operations to the `wal.Writer` sequentially upon `Commit()`.
  - This inherently solves the aborted transaction bloat (they simply get discarded from memory).

### 2. WAL File Segmentation & Truncation 
- **Context:** The database uses a single, indefinitely growing `.wal` file.
- **Action:**
  - Modify `wal.Writer` to rotate to a new segment (e.g., `000002.wal`) when the current file exceeds a size limit (e.g., 64MB).
  - Modify `WALManager`'s checkpoint routine to safely delete or archive older WAL segments once a checkpoint is fully persisted.

### 3. WAL Recovery Improvements 
- **Context:** `RecoverFromScratch` needs to salvage data from corrupted logs instead of failing entirely.
- **Action:**
  - Enhance the WAL `Reader` to gracefully handle trailing garbage bytes and partial records, salvaging all valid transactions up to the corruption point.

---

## Phase 4: Query Execution & Planner Upgrades

### 1. Missing Index Range Scans
- **Context:** The planner ignores B+Trees for range queries (e.g., `id > 10`), defaulting to slow sequential scans.
- **Action:**
  - Introduce an `IndexScanNode` to the physical plan.
  - Enhance the query planner to map range predicates to B+Tree leaf traversal operations.

---

## Phase 5: Concurrency & Storage Overhaul

### 1. Tombstones & Stable Row IDs
- **Context:** Array shifts on row deletion trigger massive, $O(n \log n)$ full-index rebuilds.
- **Action:**
  - Decouple physical array indices from logical indexing. Introduce immutable Record IDs (RIDs).
  - Use logical tombstones (`Deleted: true`) instead of array shifts.
  - Build a background vacuum thread to periodically compact tombstones.

### 2. Fine-Grained Row-Level Locking
- **Context:** Coarse table-level locks (`Table.mu`) block concurrent writers.
- **Action:**
	- Replace `Table.mu` with a centralized Lock Manager handling fine-grained Row-Level Locks and Intent Locks.
	- (Optional future step) Move towards full MVCC for non-blocking reads.

---

