# WAL Design Document

## Overview
JoyDb uses a Write-Ahead Log (WAL) to ensure durability (ACID) and crash recovery. All modifications (INSERT, UPDATE, DELETE) are written to the WAL before being applied to the in-memory database or persisted to JSON storage files.

## Architecture

### Components
*   **WAL**: The core log structure that handles writing binary records to disk.
*   **WALManager**: Bridges the storage engine and the WAL. It manages the lifecycle of the WAL and coordinates checkpoints.
*   **AsyncCheckpointer**: A background worker that periodically triggers checkpoints to limit WAL growth and ensure data persistence.
*   **RecoveryManager**: Handles the replay of WAL records during database startup to restore consistency.

### Data Flow
1.  **Write**: `Executor` -> `WALManager` -> `WAL.LogX()` -> `WAL File`.
2.  **Commit**: `Executor` -> `WALManager.Commit()` -> `WAL.Commit()` -> `fsync`.
3.  **Checkpoint**: `AsyncCheckpointer` -> `WALManager.WriteCheckpoint()` -> `StorageEngine.SaveDatabase()` -> `WAL.WriteCheckpoint()`.
4.  **Recovery**: `Registry.Get()` -> `WALManager.Recover()` -> `RecoveryManager` -> `WAL File`.

## Binary Format

The WAL file consists of a **File Header** followed by a sequence of **Records**. All integers are Little Endian.

### File Header (64 bytes)
| Offset | Size | Field | Description |
| :--- | :--- | :--- | :--- |
| 0 | 8 | Magic | `JOYDBWAL` |
| 8 | 2 | Version | `1` |
| 10 | 32 | DatabaseName | Fixed-width string (padded with nulls) |
| 42 | 8 | InitialLSN | First LSN in this file |
| 50 | 8 | CreatedAt | Unix timestamp |
| 58 | 6 | Reserved | Padding (0) |

### Record Format
Each record is aligned to 8-byte boundaries.

#### Record Header (32 bytes)
| Offset | Size | Field | Description |
| :--- | :--- | :--- | :--- |
| 0 | 1 | Type | Record Type (Begin, Commit, Insert, etc.) |
| 1 | 1 | Padding | 0 |
| 2 | 4 | Length | Total record length (including header & padding) |
| 6 | 8 | LSN | Log Sequence Number (monotonic) |
| 14 | 4 | CRC32 | Checksum of payload |
| 18 | 8 | FileOffset | Offset in file (sanity check) |
| 26 | 4 | PayloadLen | Actual length of payload |
| 30 | 2 | Padding | 0 |

#### Record Types
1.  `BeginTxn`: TxID (8 bytes)
2.  `Commit`: TxID (8 bytes)
3.  `Abort`: TxID (8 bytes)
4.  `Insert`: TxID + TableName + Key + Value
5.  `Update`: TxID + TableName + Key + OldValue + NewValue
6.  `Delete`: TxID + TableName + Key + OldValue
7.  `Checkpoint`: Checkpoint Metadata (LSN, Offset, Table Checksums)

## Recovery Protocol

### Algorithm: REDO-only
JoyDB uses a simple REDO recovery mechanism. Because the storage engine is "snapshot-based" (overwriting JSON files atomically), the WAL is used to replay transactions that were committed but not yet persisted to JSON.

### Recovery Steps
1.  **Scan**: Read the WAL from the beginning (or last valid checkpoint).
2.  **Analyze**: Track transaction states (Active, Committed, Aborted).
3.  **Filter**: Identify operations belonging to **Committed** transactions that occurred *after* the last checkpoint.
4.  **Replay**: Apply these operations to the in-memory database.
5.  **Rebuild Indexes**: Rebuild in-memory indexes from the restored data.

### Checkpoints
Checkpoints allow truncating the WAL (conceptually) and speed up recovery.
*   **Trigger**: Periodic timer (default 5s) or manual request (`SaveAll`).
*   **Process**:
    1.  Flush in-memory database to JSON files (atomic replace).
    2.  Calculate CRC32 of persisted JSON files.
    3.  Write `CheckpointRecord` to WAL containing these CRCs.
    4.  Sync WAL.
*   **Verification**: On recovery, if a checkpoint is found, its CRCs are compared against on-disk files. If they match, recovery starts *after* the checkpoint. If not (e.g., crash during save), full recovery is performed.

## Concurrency
*   **WAL Writer**: Protected by a Mutex. Concurrent transactions serialize their writes to the log buffer.
*   **Async Checkpoint**: Runs in a separate goroutine. It acquires a read lock on the Database to snapshot the table list, ensuring `CREATE/DROP TABLE` safety.
*   **Group Commit**: (Planned) `fsync` can be batched for multiple concurrent commits.

## Future Improvements
*   **Log Truncation**: Physically delete or archive old WAL segments.
*   **Group Commit**: Optimize `fsync` calls.
*   **Binary Storage**: Replace JSON with a binary page-based format for better performance.
