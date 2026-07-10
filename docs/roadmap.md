# JoyDB roadmap

## Phase 1:

### 1. Transaction ID Consolidation
- **Context:** `Transaction.ID` (UUID string) is redundant alongside `Transaction.TxID` (uint64), which is required by the WAL.
- **Action:**
  - [x] Remove `Transaction.ID` completely from `internal/domain/transaction/transaction.go`.
  - [x] Update all observers and engine tracing (e.g. `Event.TxID`) to use `uint64`.

### 2. Strict B+Tree Key Comparison
- **Context:** `compareKeys` in `bplustree.go` currently falls back to `fmt.Sprintf` for unknown types, causing slow performance and semantic sorting bugs.
- **Action:**
  - [x] Introduce a strict `Key` interface with a `Compare(other Key) int` method.
  - [x] Implement the interface for all native database types (Int, Float, String).
  - [x] Remove string-fallback coercion.

### 3. Foreign Key Constraints
- **Context:** Foreign key validation is missing.
- **Action:**
  - [x] Extend `TableSchema` to track foreign key relationships.
  - [x] Update the `insert` and `update` executors to validate referential integrity against parent tables.
  - [x] Update the `delete` executor to validate/cascade deletions.

---

## Phase 2: Database Benchmarking

### 1. Performance Testing Suite

- **Action:**
  - [x] Build a standalone benchmarking suite.
  - [x] Measure transactions per second (TPS), read/write latency, and concurrency throughput.
  - [x] Establish a baseline before Phase 3-5 changes and continuously measure against it.


## Phase 3: WAL & Recovery Polish

### 1. In-Memory Transaction Buffering & Group Commit
- **Context:** Currently, aborted transactions bloat the WAL, and every operation writes individually.
- **Action:**
  - [x] Buffer DML operations in memory within the `Transaction` or `ExecutionContext`.
  - [x] Only write the buffered operations to the `wal.Writer` sequentially upon `Commit()`.
  - [x] This inherently solves the aborted transaction bloat (they simply get discarded from memory).

### 2. WAL File Segmentation & Truncation 
- **Context:** The database uses a single, indefinitely growing `.wal` file.
- **Action:**
  - [x] Modify `wal.Writer` to rotate to a new segment (e.g., `000002.wal`) when the current file exceeds a size limit (e.g., 64MB).
  - [x] Modify `WALManager`'s checkpoint routine to safely delete or archive older WAL segments once a checkpoint is fully persisted.

### 3. WAL Recovery Improvements 
- **Context:** `RecoverFromScratch` needs to salvage data from corrupted logs instead of failing entirely.
- **Action:**
  - [x] Enhance the WAL `Reader` to gracefully handle trailing garbage bytes and partial records, salvaging all valid transactions up to the corruption point.

---

## Phase 4: Query Execution & Planner Upgrades

### 1. Missing Index Range Scans
- **Context:** The planner ignores B+Trees for range queries (e.g., `id > 10`), defaulting to slow sequential scans.
- **Action:**
  - Introduce an `IndexScanNode` to the physical plan.
  - Enhance the query planner to map range predicates to B+Tree leaf traversal operations.

### 2. Multi-Way JOIN Support & Schema Lineage
- **Context:** Currently, JoyDB only supports a single `JOIN` because intermediate joined table schemas dynamically prefix columns with intermediate pointer strings (e.g., `join_0x...`), corrupting query projections and ON conditions for subsequent joins.
- **Action:**
  - Introduce **explicit output schema propagation** in the Query Planner so each plan node pre-calculates its resulting column names.
  - Standardize column identifier resolution to resolve both fully qualified and unqualified names cleanly.
  - Update `join_executor.go` to safely preserve already-qualified columns (e.g., `users.id`) instead of blindly prepending intermediate pointer-based prefixes.

### 3. Table & Column Aliases (AS Syntax)
- **Context:** JoyDB does not support table aliases (e.g., `FROM requests r` or `FROM requests AS r`) or column aliases (e.g., `SELECT r.id AS request_id`), which are standard in mainstream SQL databases like Postgres or SQL Server for qualifying joined columns and custom output renaming.
- **Action:**
  - Extend the lexer and parser to recognize the optional `AS` keyword and alias identifiers for both fields and tables.
  - Upgrade the Query Planner's identifier resolver to register table aliases in the planning scope, mapping alias references (e.g., `r.id`) back to base columns (`requests.id`).
  - Upgrade the projection node to rename output columns to their specified column aliases.

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

