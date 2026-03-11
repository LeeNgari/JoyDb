# Analysis Report: Migrating to In-Memory ART with WAL Persistence

## Executive Summary
The proposed change involves removing the current JSON-based file storage engine and replacing it with an in-memory Adaptive Radix Tree (ART) for data storage, persisted solely via a Write-Ahead Log (WAL). This architecture mimics high-performance in-memory databases (like Redis or Memcached with persistence) but retains relational capabilities. The primary challenge is ensuring durability and recovery without periodic full-state snapshots (unless implemented), meaning the entire dataset must be reconstructed from the WAL on every restart.

**Feasibility:** High. The current architecture separates `StorageEngine` and `Table` logic cleanly enough that this swap is viable without rewriting the entire query engine.
**Difficulty:** 7/10 (High Complexity in Storage/WAL, Medium elsewhere).

---

## Architectural Changes

### 1. Storage Layer (Removal of JSON)
*   **Current State:** `JSONEngine` loads `data.json` and `meta.json` into memory on startup. Writes are double-written to WAL and JSON files (eventually).
*   **Target State:** 
    *   **Schema Persistence:** We must retain `meta.json` (or equivalent) to define tables and columns, as the current SQL parser does not support `CREATE TABLE` statements.
    *   **Data Persistence:** `data.json` is removed. All row data exists *only* in the WAL files and in memory.
    *   **Engine:** A new `InMemoryWALEngine` will replace `JSONEngine`. Its `LoadDatabase` will only load schema definitions, and then trigger WAL Replay to populate data.

### 2. In-Memory Data Structure (Slice -> ART)
*   **Current State:** `schema.Table` uses `Rows []data.Row`.
    *   Insert: Append to slice (O(1)).
    *   Find by PK: Linear scan or map lookup (if indexed).
    *   Update/Delete: Linear scan or map lookup, then slice manipulation.
*   **Target State:** `schema.Table` will use an **Adaptive Radix Tree (ART)** (e.g., via `plar/go-adaptive-radix-tree`).
    *   **Key:** Primary Key (converted to byte slice).
    *   **Value:** `data.Row` (or pointer to it).
    *   **Impact:** O(k) lookups/inserts/deletes. `SelectAll` becomes a tree traversal.

### 3. WAL Enhancements (Rotation & Management)
*   **Current State:** Single `database.wal` file that grows indefinitely.
*   **Target State:** 
    *   **Rotation:** WAL split into segments (e.g., `db.wal.0001`, `db.wal.0002`) based on size (e.g., 64MB).
    *   **Retention:** All segments are kept (as requested).
    *   **Recovery:** The `WALManager` must identify the sequence of files and replay them in order.

---

## Detailed Implementation Plan

### Phase 1: Dependency & Core Structure
1.  **Add ART Library:** Import a robust Go ART implementation.
2.  **Refactor `schema.Table`:**
    *   Replace `Rows []data.Row` with `Data *art.Tree`.
    *   Update `Insert`, `Update`, `Delete`, `Select`, `SelectAll`, `SelectByIndex` to interact with the tree.
    *   *Constraint:* Ensure iteration order in `SelectAll` is deterministic (ART is sorted by key).

### Phase 2: WAL Improvements
3.  **Implement Log Rotation:**
    *   Modify `WAL` struct to handle file rolling.
    *   Implement `Rotate()` triggers (size check after write).
4.  **Multi-File Recovery:**
    *   Update `ScanWALState` and `RecoveryManager` to read a directory of WAL files, sort them by sequence number, and replay sequentially.

### Phase 3: Storage Engine Swap
5.  **Create `InMemoryWALEngine`:**
    *   Implement `LoadDatabase`: Read `meta.json` for schema. Initialize empty ARTs.
    *   Implement `SaveDatabase`: **No-op** for data. Only saves `meta.json` if schema changes.
6.  **Update `Bootstrap`:**
    *   Modify `EnsureDatabase` to create `meta.json` but **not** `data.json`.
    *   Initial data (users) must be injected via `LogInsert` to the WAL or `Table.Insert` (which logs to WAL).

### Phase 4: Cleanup & Testing
7.  **Remove Legacy Code:** Delete `JSONEngine`, `loader` (data part), `writer` (data part).
8.  **Fix Tests:** Rewrite integration tests that assert `data.json` existence.

---

## Refactoring Impact Analysis

| Component | Impact | Details |
| :--- | :--- | :--- |
| **`internal/domain/schema`** | **High** | `Table` struct and all data access methods rewritten for ART. |
| **`internal/storage/engine`** | **High** | `JSONEngine` replaced. New logic for schema-only loading. |
| **`internal/storage/loader`** | **Medium** | Remove data loading logic. Keep schema loading. |
| **`internal/wal`** | **High** | Add rotation, multi-file management, and robust replay. |
| **`internal/executor`** | **Low** | If `Table` interface is preserved (`Select`, `Update`), executor changes are minimal. |
| **`internal/planner`** | **None** | Planner works on abstract schema, unaware of storage format. |
| **Tests** | **High** | `testdb_helper.go` and many integration tests rely on JSON files. |

## Difficulty Assessment

**Overall Scale: 7/10**

*   **Complexity:** The hardest part is ensuring the WAL replay exactly matches the state of a JSON-loaded database, handling edge cases like partial writes during rotation.
*   **Risk:** 
    *   **Startup Time:** Replaying the entire history of a database from WAL on every boot is O(Total_Writes). For large datasets, startup will become very slow without snapshots.
    *   **Memory Usage:** The entire dataset must fit in RAM.
    *   **Schema Evolution:** Since we keep `meta.json`, schema changes (`ALTER TABLE`) must be carefully synchronized with WAL entries.

## Recommendation

Proceed with the plan but consider implementing a **Snapshot** mechanism (dumping the ART to a binary file) in the future to solve the startup time issue, as replaying "infinite" WAL logs is not scalable for production use.
