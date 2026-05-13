# JoyDB Migration: Phases 3–5 Implementation Plan

## Audit of Completed Phases (0–2)

### Phase 0: DDL in WAL — ✅ Complete

**What was planned:**
- Add `RecordCreateTable`, `RecordDropTable`, `RecordAlterTable` to `wal/types.go`
- Add `LogCreateTable`, `LogDropTable`, `LogAlterTable` to `wal/writer.go`
- Extend `ReplayTarget` with DDL methods
- Wire `LogCreateTable` into the CREATE TABLE executor
- Order DDL replay before DML in recovery

**What was implemented:**
- All three record types added with correct binary layouts ([types.go:296-326](file:///home/leengari/projects/joydb/internal/wal/types.go#L296-L326))
- Writer methods implemented with proper encode/decode ([writer.go:190-248](file:///home/leengari/projects/joydb/internal/wal/writer.go#L190-L248))
- Reader decoders added for all three types ([reader.go:645-736](file:///home/leengari/projects/joydb/internal/wal/reader.go#L645-L736))
- Schema binary encoder created in a dedicated file ([schema_encoder.go](file:///home/leengari/projects/joydb/internal/wal/schema_encoder.go)) — clean, well-tested
- `TxnTracker` extended with DDL tracking ([recovery.go:366-369](file:///home/leengari/projects/joydb/internal/wal/recovery.go#L366-L369))
- `ReplayAll` correctly orders DDL before DML ([recovery.go:594-634](file:///home/leengari/projects/joydb/internal/wal/recovery.go#L594-L634))
- `executeCreateTable` calls `WALManager.LogCreateTable` before mutation ([ddl_executor.go:31-35](file:///home/leengari/projects/joydb/internal/executor/ddl_executor.go#L31-L35))
- `executeDropTable` calls `WALManager.LogDropTable` before mutation ([ddl_executor.go:87-91](file:///home/leengari/projects/joydb/internal/executor/ddl_executor.go#L87-L91))
- `DatabaseReplayTarget` implements all DDL replay methods ([wal_manager.go:464-510](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go#L464-L510))

> [!WARNING]
> **Issue #1: `ReplayAlterTable` is a stub.** It logs a warning and returns `nil` ([wal_manager.go:505-509](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go#L505-L509)). This is acceptable for now since ALTER TABLE isn't frequently used yet, but the migration plan expected it to at least apply `ADD_COLUMN` operations. This should be completed in Phase 3 alongside table struct changes.

> [!WARNING]
> **Issue #2: Registry replay condition silently drops DDL-only recoveries.** The condition at [registry.go:119](file:///home/leengari/projects/joydb/internal/storage/manager/registry.go#L119) only triggers replay when there are DML ops (`InsertOps`, `UpdateOps`, `DeleteOps`). If a WAL contains only `CreateTable` records and no DML, `ReplayAll` is never called and the table is lost on restart. This is a **real bug** that will become critical once you move to the MemoryEngine.

---

### Phase 1: Abstract Serialization — ✅ Complete

**What was planned:**
- Rename `ToJSON()` → `Serialize()`, `FromJSON()` → `Deserialize()` in `row.go`
- Change `json.RawMessage` → `[]byte` in WAL types
- Update `ReplayTarget` interface methods
- Update `WALManager` to use new names

**What was implemented:**
- `row.go` correctly has `Serialize() ([]byte, error)` and `Deserialize([]byte) (Row, error)` ([row.go:54-66](file:///home/leengari/projects/joydb/internal/domain/data/row.go#L54-L66))
- All WAL types use `[]byte` instead of `json.RawMessage` ([types.go:189, 201-202, 212](file:///home/leengari/projects/joydb/internal/wal/types.go#L189))
- `ReplayTarget` interface correctly uses `[]byte` ([recovery.go:578-588](file:///home/leengari/projects/joydb/internal/wal/recovery.go#L578-L588))
- `WALManager` calls `row.Serialize()` and `data.Deserialize()` correctly ([wal_manager.go:136, 375](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go#L136))

> [!NOTE]
> **Observation:** `Serialize()` and `Deserialize()` still use `json.Marshal`/`json.Unmarshal` internally. This is exactly what was planned — the internal format doesn't change until Phase 3's binary snapshot format. The abstraction is correctly in place for a future swap to binary row encoding.

**No issues found. Phase 1 is clean.**

---

### Phase 2: Decouple WAL Checkpointing from JSON Files — ✅ Complete

**What was planned:**
- Bump `WALVersion` to `2` and reject v1
- Simplify `CheckpointRecord` to reference `SnapshotLSN` + `SnapshotCRC32`
- Add `CreateSnapshot`/`LoadSnapshot` to `StorageEngine` interface
- Stub these on `JSONEngine`
- Update `VerifyCheckpoint` to check `.snap` file CRC
- Replace `calculateChecksums()` with `createAndRecordSnapshot()`

**What was implemented:**
- `WALVersion = 2` ([types.go:56](file:///home/leengari/projects/joydb/internal/wal/types.go#L56))
- v1 rejection added to reader ([reader.go:93-95](file:///home/leengari/projects/joydb/internal/wal/reader.go#L93-L95))
- `CheckpointRecord` now has `SnapshotLSN` + `SnapshotCRC32` ([types.go:226-234](file:///home/leengari/projects/joydb/internal/wal/types.go#L226-L234))
- `StorageEngine` interface has both methods ([engine.go:39-44](file:///home/leengari/projects/joydb/internal/storage/engine/engine.go#L39-L44))
- `JSONEngine.CreateSnapshot` writes an empty `0.snap` file ([json_engine.go:165-173](file:///home/leengari/projects/joydb/internal/storage/engine/json_engine.go#L165-L173))
- `VerifyCheckpoint` reads `.snap` file and validates CRC ([recovery.go:324-337](file:///home/leengari/projects/joydb/internal/wal/recovery.go#L324-L337))
- `createAndRecordSnapshot` delegates to `engine.CreateSnapshot` ([wal_manager.go:294-300](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go#L294-L300))

> [!WARNING]
> **Issue #3: `JSONEngine.CreateSnapshot()` is non-functional.** It always writes an empty file at `0.snap` with LSN=0 and CRC=0 ([json_engine.go:167-172](file:///home/leengari/projects/joydb/internal/storage/engine/json_engine.go#L165-L173)). This means checkpoint verification (`VerifyCheckpoint`) will always fail for this engine after any real data operations, since the snapshot CRC won't match the checkpoint CRC. This causes **all recovery to fall through to `RecoverFromScratch`**, which replays the entire WAL from the beginning every restart. Not a correctness issue, but a performance issue for large databases — all committed transactions are replayed every time.

> [!NOTE]
> **This is expected and acceptable** since the JSONEngine is transitional. The MemoryEngine will implement real snapshots.

---

## Summary of Pre-existing Issues to Fix

| # | Issue | Severity | Fix Phase |
|---|-------|----------|-----------|
| 1 | `ReplayAlterTable` is a stub — does nothing | Low | Phase 3 |
| 2 | Registry skips replay when only DDL ops exist (no DML) | **High** | Phase 3 (or fix immediately) |
| 3 | `JSONEngine.CreateSnapshot()` always returns LSN=0/CRC=0 | Low | Resolved by Phase 3 (MemoryEngine replaces it) |
| 4 | `Table.Path` empty for WAL-created tables → checkpoint crash | **High** | Phase 4 |

---

## Phase 3: Implement the In-Memory B+Tree Engine + Binary Snapshot

This is the largest phase. I recommend breaking it into 4 discrete sub-phases that can be individually committed and tested.

---

### Phase 3a: B+Tree Data Structure

> [!IMPORTANT]
> This sub-phase has **zero dependencies** on other work. It can be implemented and tested in complete isolation.

#### [NEW] `internal/index/btree/bplustree.go`

A B+Tree implementation with degree 32. Internal nodes store only routing keys (no values). Leaf nodes store key→pos pairs and are linked in a doubly-linked list for efficient range scans.

**Why B+Tree over B-Tree:** (1) Leaf linked-list gives O(log n + k) range scans vs B-Tree's O(k·log n). (2) Internal nodes hold only keys → higher fan-out → shallower tree. (3) Every production RDBMS (PostgreSQL, MySQL/InnoDB, SQLite) uses B+Trees. (4) Deletion is simpler since values only exist in leaves.

**Core API:**
```go
package btree

// BPlusTree is an ordered map from a comparable key to an int (row position).
// Internal nodes store only routing keys. Leaf nodes store key→pos pairs
// and are linked for efficient range scans.
type BPlusTree struct {
    root   *node
    degree int    // half-order (min keys = degree-1, max keys = 2*degree-1)
    size   int    // total number of entries
    first  *node  // pointer to first (leftmost) leaf for full scans
}

func New(degree int) *BPlusTree

// Insert adds a key → pos mapping. Returns error if key already exists.
func (t *BPlusTree) Insert(key interface{}, pos int) error

// Search returns the position for a key, or (0, false) if not found.
func (t *BPlusTree) Search(key interface{}) (pos int, found bool)

// Delete removes a key. Returns error if key not found.
func (t *BPlusTree) Delete(key interface{}) error

// RangeScan returns all positions where lo <= key <= hi, in key order.
// Uses the leaf linked-list for O(log n + k) performance.
func (t *BPlusTree) RangeScan(lo, hi interface{}) []int

// All returns all positions in key order by walking the leaf linked-list.
func (t *BPlusTree) All() []int

// Size returns the number of entries.
func (t *BPlusTree) Size() int

// Clear removes all entries.
func (t *BPlusTree) Clear()
```

**Key comparison:** Use a `compareKeys(a, b interface{}) int` function that handles `int64`, `float64`, `string` (the three PK types JoyDB supports). Return `-1`, `0`, `1`.

**Design notes:**
- Internal node: `type node struct { keys []interface{}; children []*node; leaf bool }`
- Leaf node: `type node struct { keys []interface{}; values []int; next *node; prev *node; leaf bool }`
  (Can use a single node struct with optional fields)
- Use proactive splitting on insert (split on the way down)
- For deletion, merge/redistribute at the leaf level only

#### [NEW] `internal/index/btree/bplustree_test.go`

Comprehensive tests:
- Insert/Search/Delete correctness with int64, string, float64 keys
- RangeScan with various boundaries (verify O(log n + k) via leaf list traversal)
- Full scan via `All()` using linked leaf list
- Large-scale insert (10k+ keys) to verify tree depth stays at 3-4 levels
- Concurrent read safety (B+Tree is read-locked at table level, but test anyway)
- Edge cases: empty tree, single element, duplicate key rejection

#### [DELETE] `internal/index/index.go`
The existing placeholder file (14 bytes) should be replaced by the `btree/` subdirectory.

---

### Phase 3b: Integrate B+Tree into Table

#### [MODIFY] [table.go](file:///home/leengari/projects/joydb/internal/domain/schema/table.go)

**Changes:**
1. Add `PKIndex *btree.BPlusTree` field to `Table` struct (line 14-23):
   ```go
   type Table struct {
       mu           sync.RWMutex
       Name         string
       Path         string
       Schema       *TableSchema
       Rows         []data.Row
       PKIndex      *btree.BPlusTree      // NEW: primary key B+Tree index (linked leaves)
       Indexes      map[string]*data.Index
       LastInsertID int64
       Dirty        bool
   }
   ```

2. Update `Insert()` (line 153-164) — after appending to `t.Rows`, insert into PKIndex:
   ```go
   // After: t.Rows = append(t.Rows, row)
   if t.PKIndex != nil {
       pkVal, _ := row.Data[t.Schema.GetPrimaryKeyColumn().Name]
       t.PKIndex.Insert(pkVal, newRowPos)
   }
   ```

3. Update `rebuildIndexesUnsafe()` (line 362-378) — also rebuild PKIndex:
   ```go
   if t.PKIndex != nil {
       t.PKIndex.Clear()
       pkCol := t.Schema.GetPrimaryKeyColumn()
       if pkCol != nil {
           for pos, row := range t.Rows {
               if val, exists := row.Data[pkCol.Name]; exists {
                   t.PKIndex.Insert(val, pos)
               }
           }
       }
   }
   ```

4. Update `SelectByIndex()` (line 204-230) — use PKIndex for PK lookups:
   ```go
   // At the start: if the column is the PK and PKIndex exists, use it
   if t.PKIndex != nil {
       pkCol := t.Schema.GetPrimaryKeyColumn()
       if pkCol != nil && pkCol.Name == colName {
           pos, found := t.PKIndex.Search(value)
           if !found { return data.Row{}, false }
           return t.Rows[pos], true
       }
   }
   ```

5. Add `InsertReplay(row data.Row)` — a lock-free variant for recovery:
   ```go
   // InsertReplay appends a row during WAL recovery (no validation, no locking).
   func (t *Table) InsertReplay(row data.Row) {
       pos := len(t.Rows)
       t.Rows = append(t.Rows, row)
       if t.PKIndex != nil {
           pkCol := t.Schema.GetPrimaryKeyColumn()
           if pkCol != nil {
               if val, exists := row.Data[pkCol.Name]; exists {
                   t.PKIndex.Insert(val, pos)
               }
           }
       }
   }
   ```

#### [MODIFY] [ddl_executor.go](file:///home/leengari/projects/joydb/internal/executor/ddl_executor.go)

When creating a table (line 38-43), initialize the PKIndex:
```go
table := &schema.Table{
    Name:    n.TableName,
    Schema:  tableSchema,
    Rows:    []data.Row{},
    PKIndex: nil, // set below if PK exists
    Indexes: make(map[string]*data.Index),
}
if pkColumn != nil {
    table.PKIndex = btree.New(32)
}
```

#### [MODIFY] [wal_manager.go](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go)

Update `ReplayInsert` (line 382-386) to use `InsertReplay`:
```go
// Before:
// table.Rows = append(table.Rows, row)
// After:
table.InsertReplay(row)
```

Update `ReplayCreateTable` (line 470-476) to initialize PKIndex:
```go
table := &schema.Table{
    Name:    name,
    Schema:  s,
    Rows:    []data.Row{},
    PKIndex: nil, // set below
    Indexes: make(map[string]*data.Index),
}
if pkCol := s.GetPrimaryKeyColumn(); pkCol != nil {
    table.PKIndex = btree.New(32)
}
```

---

### Phase 3c: MemoryEngine + Binary Snapshot

#### [NEW] `internal/storage/engine/memory_engine.go`

```go
type MemoryEngine struct{}

func NewMemoryEngine() *MemoryEngine

// LoadDatabase finds the latest .snap file, deserializes it.
// If no snapshot exists, returns an empty database.
func (e *MemoryEngine) LoadDatabase(dbPath string) (*schema.Database, error)

// SaveDatabase delegates to CreateSnapshot (called by checkpoint action).
func (e *MemoryEngine) SaveDatabase(db *schema.Database, tx *transaction.Transaction) error

// CreateDatabase just creates the directory (no meta.json).
func (e *MemoryEngine) CreateDatabase(name, basePath string) error

// DropDatabase removes the directory.
func (e *MemoryEngine) DropDatabase(name, basePath string) error

// RenameDatabase renames the directory.
func (e *MemoryEngine) RenameDatabase(oldName, newName, basePath string) error

// ListDatabases finds directories containing *.wal or *.snap files.
func (e *MemoryEngine) ListDatabases(basePath string) ([]string, error)

// LoadTable returns an error — individual table loading is not supported.
func (e *MemoryEngine) LoadTable(tablePath string) (*schema.Table, error)

// SaveTable is a no-op — all persistence is via snapshots.
func (e *MemoryEngine) SaveTable(table *schema.Table, tx *transaction.Transaction) error

// CreateSnapshot writes a binary snapshot file at {snapshotDir}/{lsn}.snap
func (e *MemoryEngine) CreateSnapshot(db *schema.Database, snapshotDir string) (uint64, uint32, error)

// LoadSnapshot reads a binary snapshot file and populates the database.
func (e *MemoryEngine) LoadSnapshot(db *schema.Database, snapshotPath string) error
```

#### [NEW] `internal/storage/engine/snapshot.go`

Binary snapshot encoder/decoder — the snapshot file format as specified in the migration plan:

```
[Magic: 8 bytes "JOYDBSNP"]
[Version: 1 byte]
[SnapshotLSN: 8 bytes]
[Timestamp: 8 bytes]
[TableCount: 4 bytes]
[Table 1]:
  [NameLen: 2][Name: N bytes]
  [LastInsertID: 8 bytes]
  [ColCount: 2]
  [Col 1]: [NameLen:2][Name:N][Type:1][Flags:1]  // Reuse wal.EncodeColumn!
  ...
  [RowCount: 8 bytes]
  [Row 1]: [ColValCount:2][ [ColNameLen:2][ColName:N][TypeTag:1][ValLen:4][Val:N] ...]
  ...
[Trailer CRC32: 4 bytes]
```

> [!TIP]
> **Reuse `wal.EncodeColumn` / `wal.DecodeColumn`** for the column schema encoding inside snapshots. The format is identical. Import the `wal` package for this — it avoids duplication and keeps the schema representation consistent.

**Snapshot retention:** After writing a new `.snap` file, list all `.snap` files, sort by LSN, keep only the 2 most recent.

#### [NEW] `internal/storage/engine/snapshot_test.go`

- Round-trip test: create database with tables + rows → `CreateSnapshot` → `LoadSnapshot` → verify identical
- CRC verification test: corrupt a byte → `LoadSnapshot` fails
- Empty database snapshot
- Multiple tables with different column types
- Snapshot retention: create 3 snapshots → verify only 2 remain

---

### Phase 3d: Fix Registry Replay Bug + Wire MemoryEngine

#### [MODIFY] [registry.go](file:///home/leengari/projects/joydb/internal/storage/manager/registry.go)

**Fix Issue #2** — the replay condition at line 119 must also check for DDL ops:

```go
// Before:
if result != nil && (len(result.InsertOps) > 0 || len(result.UpdateOps) > 0 || len(result.DeleteOps) > 0) {

// After:
hasOps := len(result.InsertOps) > 0 || len(result.UpdateOps) > 0 || len(result.DeleteOps) > 0 ||
    len(result.CreateTableOps) > 0 || len(result.DropTableOps) > 0 || len(result.AlterTableOps) > 0
if result != nil && hasOps {
```

Also update the log message to include DDL counts:
```go
slog.Info("WAL: Replaying operations",
    "database", name,
    "inserts", len(result.InsertOps),
    "updates", len(result.UpdateOps),
    "deletes", len(result.DeleteOps),
    "create_tables", len(result.CreateTableOps),
    "drop_tables", len(result.DropTableOps),
    "alter_tables", len(result.AlterTableOps),
)
```

#### [MODIFY] [main.go](file:///home/leengari/projects/joydb/cmd/joydb/main.go)

Add a `--engine` flag to switch between `json` and `memory`:
```go
engineType := flag.String("engine", "json", "Storage engine: 'json' or 'memory'")
// ...
var storageEngine engine.StorageEngine
switch *engineType {
case "memory":
    storageEngine = engine.NewMemoryEngine()
default:
    storageEngine = engine.NewJSONEngine()
}
```

This allows testing the MemoryEngine without breaking existing JSON behavior.

---

## Phase 4: The Table `Path` Field Problem

#### [MODIFY] [table.go](file:///home/leengari/projects/joydb/internal/domain/schema/table.go)

As per migration plan Option A: keep `Table.Path` but change its semantic meaning from "filesystem path to table directory" to "parent database path". Update the comment:

```go
Path string // database directory path (used for WAL/snapshot context)
```

#### [MODIFY] [ddl_executor.go](file:///home/leengari/projects/joydb/internal/executor/ddl_executor.go)

When creating a table, set `Path` to the database path:
```go
table := &schema.Table{
    Name:   n.TableName,
    Path:   ctx.Database.Path, // Set to database dir, not table subdir
    // ...
}
```

#### [MODIFY] [wal_manager.go](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go)

In `ReplayCreateTable`, set the table's Path:
```go
table := &schema.Table{
    Name:   name,
    Path:   t.db.Path, // Set to database path
    // ...
}
```

> [!IMPORTANT]
> This also fixes **Issue #4** — the checkpoint crash. With `Path` set to the database directory, `SaveTable` in `writer.go` won't get an empty path anymore. For the JSONEngine, the path still needs to point to a valid table directory (subdirectory). But since we're moving to MemoryEngine where `SaveTable` is a no-op, this is acceptable. If you want JSONEngine to still work during the transition, the fix is to have `executeCreateTable` also `mkdir` the table subdirectory and set `Path` to it. But this is unnecessary if you switch to `--engine memory` immediately.

---

## Phase 5: Deprecate & Remove JSON

This phase should only begin after the MemoryEngine is proven stable.

#### [DELETE] `internal/storage/loader/` — entire directory
#### [DELETE] `internal/storage/writer/` — entire directory
#### [DELETE] `internal/storage/metadata/` — entire directory
#### [DELETE] `internal/storage/bootstrap/` — entire directory (if it exists)
#### [DELETE] `internal/storage/engine/json_engine.go`

#### [MODIFY] [main.go](file:///home/leengari/projects/joydb/cmd/joydb/main.go)
- Remove the `--engine` flag
- Hard-wire `engine.NewMemoryEngine()`
- Remove the `databases.Content` embedded FS seeding (or convert it to a WAL-based seed)
- Remove `import "encoding/json"` from any WAL/recovery files

#### [MODIFY] [engine.go](file:///home/leengari/projects/joydb/internal/storage/engine/engine.go)
- Consider removing `LoadTable` and `SaveTable` from the interface since MemoryEngine doesn't use them

#### Verification
```bash
go build ./...  # Must compile with zero JSON imports in storage/wal layers
go test ./...   # All tests must pass
```

---

## Open Questions

> [!IMPORTANT]
> **Q1: Should the registry replay bug (Issue #2) be fixed immediately before Phase 3?**
> It's a 2-line fix and is a real correctness bug. I recommend fixing it first as a standalone commit.

> [!IMPORTANT]
> **Q2: Do you want to keep JSONEngine working during Phase 3, or are you comfortable switching to `--engine memory` immediately?**
> This affects how we handle `Table.Path` in Phase 4. If JSONEngine must still work, we need to create table subdirectories in the executor. If not, we can skip that.

> [!IMPORTANT]
> **Q3: Should `ReplayAlterTable` be implemented now, or deferred?**
> The stub currently silently ignores ALTER TABLE operations during recovery. If ALTER TABLE is used in production queries, this is a data loss bug. If it's never used yet, deferring is fine.

---

## Verification Plan

### Automated Tests

Each phase has its own test suite as described above. The full command after each phase:
```bash
go test ./... -v -count=1
go build ./cmd/joydb
```

### Integration Testing (Phase 3d)

After wiring the MemoryEngine, run JoyDB in memory mode and verify end-to-end:
```bash
./joydb --engine memory
```

Then execute:
```sql
CREATE DATABASE test;
USE test;
CREATE TABLE users (id INT PRIMARY KEY, name TEXT, age INT);
INSERT INTO users (id, name, age) VALUES (1, 'Alice', 30);
INSERT INTO users (id, name, age) VALUES (2, 'Bob', 25);
SELECT * FROM users;
SELECT * FROM users WHERE id = 1;
UPDATE users SET age = 31 WHERE id = 1;
DELETE FROM users WHERE id = 2;
SELECT * FROM users;
```

Then **kill the process** (Ctrl+C) and restart — verify that WAL recovery restores the state.

### Crash Recovery Testing (Phase 3d)

1. Start with `--engine memory`
2. Insert several rows
3. `kill -9` the process (no graceful shutdown)
4. Restart — verify WAL replay reconstructs the tables and data

---

## Effort Estimation

| Sub-phase | Files Changed | Effort | Risk |
|-----------|--------------|--------|------|
| 3a: B+Tree implementation | 2 new | Medium | Medium (algorithmic) |
| 3b: Integrate B+Tree into Table | 3 modified | Low | Low |
| 3c: MemoryEngine + Snapshot | 3 new | High | High (binary format) |
| 3d: Registry fix + wiring | 2 modified | Low | Low |
| 4: Table.Path fix | 3 modified | Low | Low |
| 5: Delete JSON code | 5 deleted, 2 modified | Low | Low |

**Recommended commit order:**
1. Fix registry replay bug (Issue #2) — standalone 1-line commit
2. Phase 3a (B+Tree) — standalone, independently testable
3. Phase 3b (integrate into Table)
4. Phase 3c (MemoryEngine + snapshot)
5. Phase 3d (wire it up + main.go flag)
6. Phase 4 (Table.Path)
7. Phase 5 (cleanup) — only after MemoryEngine is validated

> [!NOTE]
> **Design decision log (2026-05-06):** Changed from B-Tree to B+Tree after analysis. B+Tree's linked leaf list provides O(log n + k) range scans (vs B-Tree's O(k·log n)), higher fan-out from value-free internal nodes, and is the industry standard for database indexes.
