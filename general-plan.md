# JoyDB: Migration from JSON Storage → In-Memory B+Tree + WAL Persistence

## Confirmed Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| WAL backward compatibility | ❌ Break it (bump to `WALVersion = 2`) | Dev/test data only; simplifies reader significantly |
| DDL scope | CREATE TABLE + DROP TABLE + ALTER TABLE | Plan for all three now — WAL format bump needed anyway |
| B+Tree node degree | **32** (63 keys per internal node, linked leaves) | See rationale below |
| Snapshot retention | **2 files** (current + 1 previous) | Safety net during crash at checkpoint moment |
| Snapshot serialization | **Custom binary** (`encoding/binary.LittleEndian`) | Consistent with WAL format; faster than gob; no reflection |

## Codebase Analysis: What Was Found

After reading every critical file, here is the ground-truth picture. The original plan was mostly correct but missed several important details.

### Confirmed Coupling Points (Plan Was Right)

| Location | Coupling |
|---|---|
| [wal/types.go](file:///home/leengari/projects/joydb/internal/wal/types.go) | `InsertRecord.Value`, `UpdateRecord.OldValue/NewValue`, `DeleteRecord.OldValue` are all `json.RawMessage` |
| [wal/recovery.go](file:///home/leengari/projects/joydb/internal/wal/recovery.go) | [ReplayTarget](file:///home/leengari/projects/joydb/internal/wal/recovery.go#532-542) interface methods accept `json.RawMessage` |
| [wal/recovery.go](file:///home/leengari/projects/joydb/internal/wal/recovery.go) → [VerifyCheckpoint()](file:///home/leengari/projects/joydb/internal/wal/recovery.go#310-350) | Reads `data.json` / `meta.json` CRCs from disk to validate checkpoint |
| [storage/manager/wal_manager.go](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go) → [calculateChecksums()](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go#242-299) | Explicitly constructs `data.json` and `meta.json` paths and checksums them |
| [storage/manager/wal_manager.go](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go) → `DatabaseReplayTarget.ReplayInsert/Update/Delete()` | Calls `data.FromJSON(value json.RawMessage)` |
| [domain/data/row.go](file:///home/leengari/projects/joydb/internal/domain/data/row.go) | [ToJSON() json.RawMessage](file:///home/leengari/projects/joydb/internal/domain/data/row.go#54-58) and [FromJSON(json.RawMessage)](file:///home/leengari/projects/joydb/internal/domain/data/row.go#59-67) |

### Critical Gaps in the Original Plan

**Gap 1: `Table.Path` Is Hardwired to a Filesystem Layout**
The [Table](file:///home/leengari/projects/joydb/internal/domain/schema/table.go#14-24) struct in [schema/table.go](file:///home/leengari/projects/joydb/internal/domain/schema/table.go) has a `Path string` field that maps to a physical directory (`databases/mydb/users/`). The `WALManager.calculateChecksums()` method (line 260-261 in [wal_manager.go](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go)) constructs `data.json`/`meta.json` paths directly from `table.Path`. If you build a `MemoryEngine` but don't change the [Table](file:///home/leengari/projects/joydb/internal/domain/schema/table.go#14-24) struct, the table's [Path](file:///home/leengari/projects/joydb/internal/wal/wal.go#221-225) will be empty/meaningless and [calculateChecksums()](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go#242-299) will silently skip every table (see `continue` on line 270). **The `Table.Path` concept must be rethought or replaced with a snapshot directory.**

**Gap 2: [DatabaseReplayTarget](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go#356-359) Appends to `[]data.Row` Slice, Not a B+Tree**
The replay target in [wal_manager.go](file:///home/leengari/projects/joydb/internal/storage/manager/wal_manager.go) (lines 366–457) does direct slice operations: `table.Rows = append(table.Rows, row)` and `table.Rows = append(table.Rows[:i], table.Rows[i+1:]...)`. If you change the internal data structure to a B+Tree, every replay path must use the B+Tree's ordered insert/delete API, not the raw slice. This is actually **the biggest refactor surface area**.

**Gap 3: [ReplayTarget](file:///home/leengari/projects/joydb/internal/wal/recovery.go#532-542) Interface in [wal/recovery.go](file:///home/leengari/projects/joydb/internal/wal/recovery.go) Knows About `json.RawMessage`**
This is a WAL-layer interface (line 534 in [recovery.go](file:///home/leengari/projects/joydb/internal/wal/recovery.go)). It must be changed to `[]byte` as part of Phase 1 — the plan mentions this but attributes it to the wrong level. [ReplayTarget](file:///home/leengari/projects/joydb/internal/wal/recovery.go#532-542) lives in `internal/wal`, not `internal/storage/manager`. Both must change.

**Gap 4: `RecoverFromScratch` Cannot Rebuild Schema Without DDL in WAL**
The original plan correctly identifies the DDL problem, but look at what `RecoverFromScratch()` actually does (line 214 in `recovery.go`): it just replays `InsertRecord`, `UpdateRecord`, `DeleteRecord`. It has no knowledge of tables. The `DatabaseReplayTarget.ReplayInsert` (line 367) does `t.db.Tables[tableName]` — and if the table doesn't exist it silently logs a warning and returns `nil`. **Schema is loaded from JSON first, then WAL is replayed on top of it.** If you remove JSON, the schema load step goes away and WAL replay has no table to insert into. This is a showstopper if not addressed.

**Gap 5: `SaveDatabase` Writes `meta.json` for Table List**
`writer.SaveDatabase()` (line 133 onwards) writes a `meta.json` at the database level listing all table names. This is how `ListDatabases()` in `JSONEngine` discovers valid databases (it checks for `meta.json`). With the new engine, `ListDatabases()` detection logic must change.

**Gap 6: WAL File Per Database, Named `{dbName}.wal`**
The WAL file is created at `filepath.Join(dbPath, dbName+".wal")` (registry.go line 100, wal_manager.go line 45). This means there's one WAL per database directory. With the `MemoryEngine`, all data lives in RAM and the "database directory" becomes a directory only containing the `.wal` file and the snapshot file. The directory concept is still needed but its role changes dramatically.

---

## Corrections to Original Plan

### Phase 2 Correction: `wal_manager.go` Is the Real Problem
The plan says "Modify `WALManager.calculateChecksums()` to stop looking for `.json` files." This is correct but undersells the scope. The `VerifyCheckpoint()` in `wal/recovery.go` (a completely separate file) also reads JSON files. Both must change atomically.

### Phase 3 Correction: The "No-Op `SaveTable`" Misses the Checkpoint Flow
The plan says `MemoryEngine.SaveTable()` should "do practically nothing." But look at the checkpoint flow in `wal_manager.go` (line 61-78): the `checkpointAction` closure calls `saveFunc()` (which calls `storageEngine.SaveDatabase()`) **before** writing the WAL checkpoint record. If `SaveDatabase` is a no-op, the checkpoint has no snapshot behind it. Recovery on restart would have to replay from the start of the WAL. The snapshot must be written by `SaveDatabase()` — just in binary format instead of JSON.

### Phase 4 Correction: Bootstrap Package Is Also JSON-Only
`internal/storage/bootstrap/` (not covered in the plan) is used to create database and table directories. It likely also creates JSON files. This needs to be removed/replaced.

### The `metadata` Package
`internal/storage/metadata/` defines `TableMeta` and `DatabaseMeta` structs with JSON struct tags. These are only used by loader and writer. They can be safely deleted when loader/writer are gone.

---

## Revised Migration Plan: 5 Phases

### Phase 0: Pre-work — Add DDL WAL Records (Schema Persistence)

> **This must come before everything else.** It is the riskiest and most fundamental change.

**The Core Problem:** Currently the schema (CREATE TABLE) is written to `meta.json` and never logged to the WAL. Recovery assumes the JSON schema files exist. Without them, the B-Tree engine has no tables to insert recovered rows into.

**Solution: Add DDL Records to the WAL**

#### New WAL Record Types in `wal/types.go`
```
RecordCreateTable  // stores: TxID, TableName, full column schema (binary serialized)
RecordDropTable    // stores: TxID, TableName
RecordAlterTable   // stores: TxID, TableName, AlterOp type (ADD/DROP/RENAME col), column descriptor
```

**`RecordAlterTable` binary payload:**
```
[TxIDLen(8)][TableNameLen(2)][TableName][AlterOp(1)][ColumnDescriptor (variable)]
```
`AlterOp` values: `0x01=ADD_COLUMN`, `0x02=DROP_COLUMN`, `0x03=RENAME_COLUMN`. The column descriptor reuses the same compact schema encoding as `RecordCreateTable`. The `ReplayAlterTable()` on `DatabaseReplayTarget` applies the structural change to the in-memory `TableSchema` and re-validates the rows (or simply leaves existing rows as-is for `ADD_COLUMN` with a default). Leave `RENAME_COLUMN` as a stub for now — it just renames the column in the schema without touching row data.

#### Schema Serialization Format
Encode `TableSchema` (name + `[]Column`) as a compact binary: `[NameLen(2)][Name][ColCount(2)][[NameLen(2)][Name][Type(1)][Flags(1)]...]`. This is pure binary, no JSON. Version it with a single byte for forward compatibility.

#### WAL Writer Changes (`wal/writer.go`)
Add `LogCreateTable(txID uint64, tableName string, schema []byte) (uint64, error)` and `LogDropTable(txID uint64, tableName string) (uint64, error)`.

#### Executor Changes
In the CREATE TABLE executor (which currently calls `bootstrap.CreateTable()` and then `storageEngine.CreateDatabase()`), add a `WALManager.LogCreateTable()` call **before** the in-memory table is created.

#### Recovery Changes (`wal/recovery.go`)
- Add `CreateTableOps []*CreateTableRecord`, `DropTableOps []*DropTableRecord`, `AlterTableOps []*AlterTableRecord` to `RecoveryResult`
- DDL ops are ordered by LSN and applied **before** any DML replays
- Extend `ReplayTarget` interface:
```go
ReplayCreateTable(name string, schemaBytes []byte) error
ReplayDropTable(name string) error
ReplayAlterTable(name string, op AlterOp, colDescriptor []byte) error
```

#### Replay Implementation
`DatabaseReplayTarget.ReplayCreateTable()` creates an in-memory table from the decoded schema and registers it in `db.Tables`. Once this works, schema is 100% reconstructable from WAL.

---

### Phase 1: Abstract Serialization (De-JSON the Domain & WAL)

**Goal:** Remove `json.RawMessage` from all WAL types and the `ReplayTarget` interface. This is a pure refactoring phase with zero behavior change.

#### `domain/data/row.go`
- Rename `ToJSON() json.RawMessage` → `Serialize() []byte`. Keep the underlying `json.Marshal` for now.
- Rename `FromJSON(json.RawMessage) Row` → `Deserialize([]byte) Row`. Keep underlying `json.Unmarshal` for now.
- Keep `MarshalJSON()`/`UnmarshalJSON()` for any JSON serialization still needed (loader, writer — which Phase 4 will delete anyway).

#### `wal/types.go`
- `InsertRecord.Value json.RawMessage` → `Value []byte`
- `UpdateRecord.OldValue/NewValue json.RawMessage` → `OldValue/NewValue []byte`
- `DeleteRecord.OldValue json.RawMessage` → `OldValue []byte`

#### `wal/recovery.go` — `ReplayTarget` Interface
```go
// Before
ReplayInsert(tableName, key string, value json.RawMessage) error
// After
ReplayInsert(tableName, key string, value []byte) error
```

#### `wal/writer.go` and `wal/reader.go`
The WAL already encodes payloads as raw bytes. The `json.RawMessage` is just a named type alias for `[]byte`. This is a find-replace across the codebase for the type name — the actual binary format of the WAL file does **not** change.

#### `storage/manager/wal_manager.go`
- `LogInsert/Update/Delete`: call `row.Serialize()` instead of `row.ToJSON()`
- `ReplayInsert/Update/Delete`: call `data.Deserialize(value)` instead of `data.FromJSON(value)`

**Test:** All existing WAL integration tests in `storage/manager/wal_integration_test.go` and `storage/manager/wal_crash_test.go` must pass unchanged. Run with `go test ./internal/storage/manager/... -v`.

---

### Phase 2: Decouple WAL Checkpointing from JSON Files

**Goal:** WAL checkpoint records must reference a snapshot identifier, not CRCs of `data.json`/`meta.json`.

#### New Snapshot Concept
A snapshot is a binary file written atomically by the storage engine. Its identifier is a 64-bit LSN (the flushed LSN at the time of snapshot). File name: `{snapshotLSN}.snap`.

#### `StorageEngine` Interface Addition (`storage/engine/engine.go`)
```go
// CreateSnapshot atomically persists the current in-memory state and returns the snapshot LSN.
// The returned LSN is embedded in the WAL checkpoint record.
CreateSnapshot(db *schema.Database, snapshotDir string) (snapshotLSN uint64, snapshotCRC uint32, err error)

// LoadSnapshot loads a snapshot file into the given (possibly empty) database.
LoadSnapshot(db *schema.Database, snapshotPath string) error
```

#### `wal/types.go` — Simplify `CheckpointRecord`
```go
// Before: DatabaseCRC32, TableCount, []TableChecksum
// After:
type CheckpointRecord struct {
    Header           WALRecordHeader
    CheckpointLSN    uint64
    CheckpointOffset uint64
    LastFlushedLSN   uint64
    Timestamp        int64
    SnapshotLSN      uint64  // LSN of the snapshot this checkpoint references
    SnapshotCRC32    uint32  // CRC of the snapshot file for tamper detection
}
```
> [!IMPORTANT]
> This changes the WAL binary format. Bump `WALVersion` to `2` and **do not support v1 WAL reading**. The new reader can simply return an error on a v1 magic to force a clean start. Since data is dev/test-only this is safe. Delete any v1 decoding branches from `wal/reader.go` — they add dead code and complexity.

#### `wal/recovery.go` — `VerifyCheckpoint()`
- Remove all `data.json`/`meta.json` file reads
- Replace with: read the snapshot file at `snapshotDir/{snapshotLSN}.snap`, calculate CRC32, compare to `checkpoint.SnapshotCRC32`

#### `storage/manager/wal_manager.go` — `calculateChecksums()` → `createAndRecordSnapshot()`
- Remove all `data.json`/`meta.json` path construction
- Call `storageEngine.CreateSnapshot(db, dbPath)` → returns `(snapshotLSN, snapshotCRC, err)`
- Pass these to `wal.WriteCheckpoint(snapshotLSN, snapshotCRC)` (simplified signature)

#### `JSONEngine` Updates (Temporary)
Implement `CreateSnapshot()` on `JSONEngine` to just return `(currentWALFlushedLSN, crc32OfAllFiles, nil)` so the JSON engine still works while `MemoryEngine` is built.

---

### Phase 3: Implement the In-Memory B+Tree Engine + Binary Snapshot

**Goal:** Build the new data structure and storage engine.

#### 3a: The In-Memory Data Structure

**B+Tree + Dense Rows — Why This Combination Is Right:**

The B+Tree was chosen over a plain B-Tree after analysis of JoyDB's access patterns:

- **B+Tree (ordered by primary key):** Internal nodes store only routing keys (no values), while leaf nodes store key→position mappings and are linked in a doubly-linked list. This gives O(log n) point lookups and O(log n + k) range scans.
- **Why B+Tree over B-Tree:** (1) Leaf linked-list enables efficient range scans — find the start leaf, then walk the list instead of traversing the tree. (2) Internal nodes hold only keys → higher fan-out → shallower tree → fewer comparisons. (3) Every production RDBMS (PostgreSQL, MySQL/InnoDB, SQLite) uses B+Trees for indexes. (4) Deletion is simpler since values only exist in leaves.
- **Dense Row Array (`[]data.Row`):** The same `[]data.Row` that exists today. Rows are stored compactly. The B+Tree's leaf values are indices (`int`) into this array.
- **Why not store rows in the B+Tree directly:** Storing full `Row` structs in B+Tree nodes ruins cache locality. The node would become huge, and pointer chasing across a wide tree is slow. The indirection (B+Tree leaf → dense array index) is the standard approach used by DuckDB and similar systems.

**New Package: `internal/index/btree/`**
```go
// BPlusTree is an ordered map from primary key → row position in dense array.
// Internal nodes store only routing keys. Leaf nodes store key→pos pairs
// and are linked for efficient range scans.
type BPlusTree struct { ... }

func (b *BPlusTree) Insert(key interface{}, pos int) error
func (b *BPlusTree) Search(key interface{}) (pos int, found bool)
func (b *BPlusTree) Delete(key interface{}) error
func (b *BPlusTree) RangeScan(lo, hi interface{}) []int   // For WHERE key BETWEEN x AND y
func (b *BPlusTree) All() []int                           // Full scan in key order via leaf list
```

> [!NOTE]
> **Use degree 32 (internal nodes hold up to 63 routing keys + 64 child pointers; leaf nodes hold up to 63 key→pos pairs and are linked).** Rationale: at degree 32, an internal node holds ~63 `interface{}` keys ≈ ~0.5KB. That's 8 cache lines — large enough to reduce tree depth to 3–4 levels for millions of rows, small enough to be easy to test, debug, and reason about. Degree 128 would give 2–3 levels for the same dataset but a node that evicts L1 cache on every read. Degree 32 matches the order of magnitude used by SQLite's in-memory B+Trees. You can tune it up later by changing a single constant if benchmarks warrant it.

**Update `schema.Table` to Support Both Structures (Transitionally)**

```go
type Table struct {
    mu           sync.RWMutex
    Name         string
    Path         string              // kept for WAL path context; now points to db dir, not data files
    Schema       *TableSchema
    Rows         []data.Row          // dense row store
    PKIndex      *btree.BPlusTree    // primary key → row position (new, B+Tree with linked leaves)
    Indexes      map[string]*data.Index  // existing secondary hash indexes
    LastInsertID int64
    Dirty        bool
}
```

**Update `Table.Insert()`**
After appending to `t.Rows`, insert the new row position into `t.PKIndex`.

**Update `Table.Delete()`**
After removing from `t.Rows`, call `t.PKIndex.Delete(key)` and rebuild PKIndex (or use a tombstone approach). Note: deleting from the middle of `[]data.Row` shifts all subsequent indices — PKIndex must be rebuilt after any delete, just like the existing `rebuildIndexesUnsafe()`. PKIndex can be added to that rebuild function.

**Update `Table.SelectByIndex()` for PK lookup**
If the column is the PK, use `t.PKIndex.Search(value)` → O(log n) instead of linear scan.

**Update `DatabaseReplayTarget`**
Replace `table.Rows = append(table.Rows, row)` with `table.InsertReplay(row)` which uses the B+Tree insert path (a new, lock-free variant of `Insert` for use during recovery before the table is live).

#### 3b: The `MemoryEngine`

**New file: `internal/storage/engine/memory_engine.go`**

```go
type MemoryEngine struct {
    encoder RowEncoder  // swappable serialization (for snapshot writing)
}

func (e *MemoryEngine) LoadDatabase(dbPath string) (*schema.Database, error) {
    // 1. Find latest .snap file in dbPath
    // 2. If found, call LoadSnapshot to hydrate in-memory db
    // 3. If not found, return empty db (WAL phase 0 DDL records will recreate tables)
}

func (e *MemoryEngine) SaveDatabase(db *schema.Database, tx *transaction.Transaction) error {
    // Delegates to CreateSnapshot. This is called by the checkpoint action.
    _, _, err := e.CreateSnapshot(db, db.Path)
    return err
}

func (e *MemoryEngine) CreateDatabase(name, basePath string) error {
    // mkdir basePath/name only (no meta.json)
    return os.MkdirAll(filepath.Join(basePath, name), 0755)
}

func (e *MemoryEngine) ListDatabases(basePath string) ([]string, error) {
    // Find directories containing any *.wal OR *.snap file
    // (no longer requires meta.json)
}

func (e *MemoryEngine) CreateSnapshot(db *schema.Database, snapshotDir string) (uint64, uint32, error) {
    // See snapshot format below
}

func (e *MemoryEngine) LoadSnapshot(db *schema.Database, snapshotPath string) error {
    // Inverse of CreateSnapshot
}
```

`SaveTable()` is a near-no-op for `MemoryEngine` since table data lives in the in-memory `Database` struct. The caller (`SaveDatabase`) handles it.

#### 3c: The Binary Snapshot Format

The snapshot must store: the schema for every table AND all rows. WAL recovery then replays only changes since `snapshotLSN`. **All encoding uses `encoding/binary.LittleEndian` — identical to the WAL format.**

**Snapshot File Layout:**
```
[Magic: 8 bytes "JOYDBSNP"]
[Version: 1 byte]               // bumped independently of WAL version
[SnapshotLSN: 8 bytes]          // flushed WAL LSN at snapshot time
[Timestamp: 8 bytes]
[TableCount: 4 bytes]
[Table 1]:
  [NameLen: 2][Name: N bytes]
  [LastInsertID: 8 bytes]
  [ColCount: 2]
  [Col 1]: [NameLen:2][Name:N][Type:1][Flags:1]    // Flags: bit0=PK, bit1=Unique, bit2=NotNull, bit3=AutoIncrement
  ...
  [RowCount: 8 bytes]
  [Row 1]: [ColValCount:2][ [ColNameLen:2][ColName:N][TypeTag:1][ValLen:4][Val:N] ...]
  ...
[Table 2]: ...
[Trailer CRC32: 4 bytes]        // CRC of entire file content before this field
```

**TypeTag values for row cell encoding (1 byte):**
```
0x01 = INT64    (val = 8 bytes little-endian)
0x02 = FLOAT64  (val = 8 bytes IEEE 754)
0x03 = TEXT     (val = UTF-8 bytes, len = ValLen)
0x04 = BOOL     (val = 1 byte, 0x00=false, 0x01=true)
0x05 = NULL     (val absent, ValLen = 0)
```

> [!TIP]
> Keep a `RowEncoder` interface with a `Version() byte` method so a future columnar or compressed encoder can be swapped in without touching the engine. The first implementation is the custom binary above.

#### 3d: Snapshot Retention Policy (2 files)

After `CreateSnapshot()` successfully writes and fsyncs the new `.snap` file:
1. Verify its CRC32 (re-read and check)
2. List all `.snap` files in the database directory sorted by LSN ascending
3. If there are more than 2, delete the oldest ones (keep `[count-2:]`)

This means: during a crash at exactly the checkpoint moment, if the new snapshot is corrupt, the previous snapshot + newer WAL records can still achieve full recovery.

#### 3d: Startup Sequence with `MemoryEngine`

```
1. MemoryEngine.LoadDatabase(dbPath)
   └─ Find latest {lsn}.snap in dbPath
   └─ If found: LoadSnapshot → hydrate db.Tables (schema + rows + PKIndex)
   └─ If not found: return empty db{} (no tables yet)

2. Registry.GetWithWAL()
   └─ WAL file exists? → RecoveryManager.Recover()
       └─ Find last checkpoint → verify using snapshot CRC
       └─ Seek past checkpoint
       └─ Replay DDL records FIRST (RecordCreateTable → creates tables)
       └─ Replay DML records (Insert/Update/Delete into B-Tree)

3. Database is now live
```

---

### Phase 4: The Table `Path` Field Problem — Resolution

`Table.Path` currently stores a filesystem path for JSON files. With the memory engine it's meaningless. Two options:

**Option A (Simple):** Keep `Table.Path` but change its meaning to be the **parent database path** (e.g., `databases/mydb/`). Remove all places that construct `data.json`/`meta.json` from it. The WAL manager uses `db.Path` anyway and already has it directly.

**Option B (Clean):** Remove `Table.Path` entirely and have the engine pass paths via method arguments. This requires finding every read of `Table.Path`.

**Recommendation: Option A** — it's a 1-line semantic change per table and avoids a large refactor during an already complex migration.

---

### Phase 5: Deprecate & Remove JSON

**Goal:** Clean up after the new system is proven stable.

1. Delete `internal/storage/loader/`
2. Delete `internal/storage/writer/`
3. Delete `internal/storage/metadata/`
4. Delete `internal/storage/bootstrap/` (replaced by `MemoryEngine.CreateDatabase()`)
5. Delete `internal/storage/engine/json_engine.go`
6. In `cmd/joydb/main.go`: switch `engine.NewJSONEngine()` → `engine.NewMemoryEngine()`
7. Update `ARCHITECTURE.md` to reflect the new storage diagram
8. Remove `json.RawMessage` import from `wal/recovery.go` — verify with `go build ./...`

---

## B+Tree Design: Detailed Considerations

### Why B+Tree Over Other Structures

| Structure | Range Scan | Point Lookup | Insert | Cache Friendly |
|---|---|---|---|---|
| `[]data.Row` (current) | O(n) | O(n) | O(1) amortized | ✅ best |
| `map[pk]int` (hash) | ❌ impossible | O(1) avg | O(1) | ❌ |
| B-Tree | O(k·log n) | O(log n) | O(log n) | ✅ good |
| **B+Tree** | **O(log n + k)** | **O(log n)** | **O(log n)** | **✅ best (linked leaves)** |
| Skip List | O(log n + k) | O(log n) | O(log n) | ❌ poor |

The B+Tree wins because: (1) its leaf linked-list gives truly O(log n + k) range scans (B-Tree range scans require tree traversal = O(k·log n)), (2) internal nodes hold only routing keys → higher fan-out → shallower tree, (3) it is the universal standard for database indexes (PostgreSQL, MySQL/InnoDB, SQLite all use B+Trees).

### B+Tree + Dense Row Array: The Interplay

```
Dense Row Array (t.Rows):
[ Row0 | Row1 | Row2 | Row3 | ... ]
    0      1      2      3

B+Tree (PK index) — leaf nodes linked:
  Internal:  [PK=3 | PK=7]
              /     |     \
  Leaf[0]:  PK=1→2  Leaf[1]: PK=3→0, PK=7→1  Leaf[2]: PK=9→3
    ↔ linked list ↔
```

**Critical Issue: Delete Invalidates Positions**
When you delete `Row1` from the dense array:
- Rows shift: what was `Row2` is now at position 1, `Row3` at position 2
- The B+Tree's positions for pk=1 and pk=9 are now **wrong**

**Solutions (choose one):**

**Option A: Rebuild PK Index on Every Delete (Simple, Current Approach)**
The existing `rebuildIndexesUnsafe()` already does this for hash indexes. Add PK B+Tree rebuild there. Cost: O(n log n) per delete. Acceptable for now.

**Option B: Tombstone Approach (Advanced)**
Mark deleted rows with a tombstone bit in the dense array. Never shift rows. B+Tree lookups skip tombstoned positions. Compact (defragment) during checkpoint. Cost: more complex, wastes memory until checkpoint. Enables O(log n) deletes.

**Option C: Gap List (Intermediate)**
Maintain a `freeList []int` of available positions from deleted rows. On insert, reuse from `freeList` before appending. B+Tree update is targeted (only the reused slot). Cost: O(log n) delete (just B+Tree remove) + O(1) freeList append.

**Recommendation: Start with Option A (rebuild).** With WAL and snapshotting in place, correctness matters more than optimal delete performance at this stage. Option C can be added in a follow-up.

### Handling Multi-Key Tables (No PK)
Currently JoyDB requires a PK for WAL records (see `GetPrimaryKeyValue()`). If no PK column exists, the B+Tree is not applicable. This is fine — the existing behavior (linear scan) remains for tables without PKs. The B+Tree is only built when `table.Schema.GetPrimaryKeyColumn() != nil`.

---

## Locked-In Design Decisions

> [!IMPORTANT]
> All decisions are finalized. No open questions remain before implementation can begin.

---

## Summary of What Changes vs. What Doesn't

| Component | Changes? | Notes |
|---|---|---|
| `domain/data/row.go` | ✅ Yes | Rename `ToJSON/FromJSON` to `Serialize/Deserialize` |
| `domain/schema/table.go` | ✅ Yes | Add `PKIndex *btree.BPlusTree`; update `Insert/Delete/rebuildIndexes` |
| `domain/schema/database.go` | ❌ No | |
| `wal/types.go` | ✅ Yes | `json.RawMessage` → `[]byte`; new `RecordCreateTable/Drop`; simplified `CheckpointRecord` |
| `wal/writer.go` | ✅ Yes | Add `LogCreateTable()`, update `LogInsert/Update/Delete` signatures |
| `wal/reader.go` | ✅ Yes | Update deserialization for new types |
| `wal/recovery.go` | ✅ Yes | Update `ReplayTarget`; add `ReplayCreateTable`; update `VerifyCheckpoint` |
| `wal/wal.go` | ❌ Minimal | Nothing structural |
| `storage/engine/engine.go` | ✅ Yes | Add `CreateSnapshot` / `LoadSnapshot` to interface |
| `storage/engine/json_engine.go` | ✅ Temporary | Stub `CreateSnapshot`; delete in Phase 5 |
| `storage/engine/memory_engine.go` | ✅ New | Core of the migration |
| `storage/manager/wal_manager.go` | ✅ Yes | Replace `calculateChecksums` with `createAndRecordSnapshot`; DDL logging |
| `storage/manager/registry.go` | ✅ Minor | Startup sequence update for DDL replay ordering |
| `storage/bootstrap/` | ✅ Delete (Phase 5) | |
| `storage/loader/` | ✅ Delete (Phase 5) | |
| `storage/writer/` | ✅ Delete (Phase 5) | |
| `storage/metadata/` | ✅ Delete (Phase 5) | |
| `executor/` (CREATE TABLE) | ✅ Yes | Add WAL DDL logging |
| `parser/`, `planner/`, `query/` | ❌ No | Untouched |
| `engine/` (orchestrator) | ❌ No | |
| `internal/index/btree/` | ✅ New | B+Tree implementation (with linked leaf nodes for range scans) |

---

## Effort Estimation (Revised)

| Phase | Effort | Risk | Blocking Dependency |
|---|---|---|---|
| 0: DDL in WAL | High | High | None (start here) |
| 1: De-JSON Domain/WAL types | Low | Low | Phase 0 |
| 2: Decouple Checkpointing | Medium | Medium | Phase 1 |
| 3a: B+Tree data structure | Medium | Medium | None (can parallelise) |
| 3b-d: MemoryEngine + Snapshot | High | High | Phases 1, 2, 3a |
| 4: Table.Path cleanup | Low | Low | Phase 3 |
| 5: Delete JSON code | Low | Low | Phase 3 |

**Total: ~3–6 weeks of focused engineering depending on B-Tree implementation complexity.**

The hardest single task is Phase 0 (DDL in WAL) combined with Phase 3d (startup sequence correctness under crash scenarios). Get those right and everything else follows.

> [!NOTE]
> **Design decision log (2026-05-06):** Changed from B-Tree to B+Tree after analysis. B+Tree's linked leaf list provides O(log n + k) range scans (vs B-Tree's O(k·log n)), higher fan-out from value-free internal nodes, and is the industry standard for database indexes.
