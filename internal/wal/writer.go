package wal

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"time"
)

// ===========================================================================
// WAL WRITER OPERATIONS
// ===========================================================================
//
// All write operations follow this pattern:
// 1. Acquire mutex
// 2. Allocate LSN
// 3. Encode payload
// 4. Calculate CRC32
// 5. Build header with length and offset
// 6. Write header + payload + padding
// 7. Update currentOffset
// 8. Release mutex
//
// Sync (fsync) is NOT called on every write for performance.
// Call Sync() explicitly or use Commit() for durability guarantee.
//
// ===========================================================================

// BeginTransaction writes a BeginTxn record to the WAL
// Returns the LSN assigned to this record
func (w *WAL) BeginTransaction(txID uint64) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Encode payload: TxID (8 bytes)
	payload := make([]byte, 8)
	ByteOrder.PutUint64(payload, txID)

	// Write record
	lsn, err := w.writeRecord(RecordBeginTxn, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write BeginTxn record: %w", err)
	}

	// Track active transaction
	w.activeTxns[txID] = &TxnState{
		ID:    txID,
		State: TxnActive,
	}

	return lsn, nil
}

// LogInsert writes an Insert record to the WAL
// Returns the LSN assigned to this record
func (w *WAL) LogInsert(txID uint64, tableName string, key string, value json.RawMessage) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Verify transaction is active
	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	// Encode payload: TxID + TableName + Key + Value
	payload := w.encodeInsertPayload(txID, tableName, key, value)

	// Write record
	lsn, err := w.writeRecord(RecordInsert, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Insert record: %w", err)
	}

	return lsn, nil
}

// LogUpdate writes an Update record to the WAL
// Returns the LSN assigned to this record
func (w *WAL) LogUpdate(txID uint64, tableName string, key string, oldValue json.RawMessage, newValue json.RawMessage) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Verify transaction is active
	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	// Encode payload: TxID + TableName + Key + OldValue + NewValue
	payload := w.encodeUpdatePayload(txID, tableName, key, oldValue, newValue)

	// Write record
	lsn, err := w.writeRecord(RecordUpdate, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Update record: %w", err)
	}

	return lsn, nil
}

// LogDelete writes a Delete record to the WAL
// Returns the LSN assigned to this record
func (w *WAL) LogDelete(txID uint64, tableName string, key string, oldValue json.RawMessage) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Verify transaction is active
	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	// Encode payload: TxID + TableName + Key + OldValue
	payload := w.encodeDeletePayload(txID, tableName, key, oldValue)

	// Write record
	lsn, err := w.writeRecord(RecordDelete, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Delete record: %w", err)
	}

	return lsn, nil
}

// Commit writes a Commit record to the WAL and fsyncs
// This makes the transaction durable
// Returns the LSN assigned to this record
func (w *WAL) Commit(txID uint64) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Verify transaction is active
	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	// Encode payload: TxID (8 bytes)
	payload := make([]byte, 8)
	ByteOrder.PutUint64(payload, txID)

	// Write record
	lsn, err := w.writeRecord(RecordCommit, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Commit record: %w", err)
	}

	// Flush buffer and fsync to ensure durability
	if err := w.flushAndSync(); err != nil {
		return 0, fmt.Errorf("failed to sync after commit: %w", err)
	}

	// Update flushed LSN
	w.flushedLSN = lsn

	// Update transaction state and remove from active
	w.activeTxns[txID].State = TxnCommitted
	delete(w.activeTxns, txID)

	return lsn, nil
}

// Abort writes an Abort record to the WAL
// Returns the LSN assigned to this record
func (w *WAL) Abort(txID uint64) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Verify transaction is active
	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	// Encode payload: TxID (8 bytes)
	payload := make([]byte, 8)
	ByteOrder.PutUint64(payload, txID)

	// Write record
	lsn, err := w.writeRecord(RecordAbort, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Abort record: %w", err)
	}

	// Update transaction state and remove from active
	w.activeTxns[txID].State = TxnAborted
	delete(w.activeTxns, txID)

	return lsn, nil
}

// LogCreateTable writes a CreateTable record to the WAL
// schemaBytes is the binary-encoded TableSchema from EncodeTableSchema()
// Returns the LSN assigned to this record
func (w *WAL) LogCreateTable(txID uint64, tableName string, schemaBytes []byte) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	payload := w.encodeCreateTablePayload(txID, tableName, schemaBytes)

	lsn, err := w.writeRecord(RecordCreateTable, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write CreateTable record: %w", err)
	}

	return lsn, nil
}

// LogDropTable writes a DropTable record to the WAL
// Returns the LSN assigned to this record
func (w *WAL) LogDropTable(txID uint64, tableName string) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	payload := w.encodeDropTablePayload(txID, tableName)

	lsn, err := w.writeRecord(RecordDropTable, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write DropTable record: %w", err)
	}

	return lsn, nil
}

// LogAlterTable writes an AlterTable record to the WAL
// alterOp is one of AlterOpAddColumn, AlterOpDropColumn, AlterOpRenameColumn
// colDesc is the binary-encoded Column descriptor from EncodeColumn()
// Returns the LSN assigned to this record
func (w *WAL) LogAlterTable(txID uint64, tableName string, alterOp uint8, colDesc []byte) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.verifyActiveTxn(txID); err != nil {
		return 0, err
	}

	payload := w.encodeAlterTablePayload(txID, tableName, alterOp, colDesc)

	lsn, err := w.writeRecord(RecordAlterTable, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write AlterTable record: %w", err)
	}

	return lsn, nil
}

// WriteCheckpoint writes a Checkpoint record to the WAL
// This should be called after successfully persisting all dirty tables to JSON
// Returns the LSN assigned to this record
func (w *WAL) WriteCheckpoint(tables []TableChecksum, databaseCRC32 uint32) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Build checkpoint payload
	payload := w.encodeCheckpointPayload(tables, databaseCRC32)

	// Write record
	lsn, err := w.writeRecord(RecordCheckpoint, payload)
	if err != nil {
		return 0, fmt.Errorf("failed to write Checkpoint record: %w", err)
	}

	// Flush buffer and fsync to ensure checkpoint is durable
	if err := w.flushAndSync(); err != nil {
		return 0, fmt.Errorf("failed to sync after checkpoint: %w", err)
	}

	// Update flushed LSN and checkpoint LSN
	w.flushedLSN = lsn
	w.lastCheckpoint = lsn

	return lsn, nil
}

// ===========================================================================
// INTERNAL HELPERS
// ===========================================================================

// verifyActiveTxn checks if a transaction is active
// Must be called with mutex held
func (w *WAL) verifyActiveTxn(txID uint64) error {
	txn, exists := w.activeTxns[txID]
	if !exists {
		return fmt.Errorf("transaction %d not found", txID)
	}
	if txn.State != TxnActive {
		return fmt.Errorf("transaction %d is not active (state: %s)", txID, txn.State)
	}
	return nil
}

// writeRecord writes a complete WAL record (header + payload + padding)
// Must be called with mutex held
func (w *WAL) writeRecord(recordType RecordType, payload []byte) (uint64, error) {
	lsn := w.allocateLSN()
	header := w.buildRecordHeader(recordType, payload, lsn)
	return lsn, w.writeRecordData(header, payload)
}

// buildRecordHeader constructs the record header
func (w *WAL) buildRecordHeader(recordType RecordType, payload []byte, lsn uint64) WALRecordHeader {
	crc := crc32.ChecksumIEEE(payload)
	payloadLen := len(payload)
	totalLen := RecordHeaderSize + payloadLen
	alignedLen := AlignTo8(totalLen)

	return WALRecordHeader{
		Type:       recordType,
		Length:     uint32(alignedLen),
		LSN:        lsn,
		CRC32:      crc,
		FileOffset: w.currentOffset,
		PayloadLen: uint32(payloadLen),
	}
}

// writeRecordData writes the header and payload to the buffer
func (w *WAL) writeRecordData(header WALRecordHeader, payload []byte) error {
	// Encode header
	headerBytes := encodeHeader(header)

	// Write header
	if _, err := w.buf.Write(headerBytes); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write payload
	if _, err := w.buf.Write(payload); err != nil {
		return fmt.Errorf("failed to write payload: %w", err)
	}

	// Calculate padding
	paddingLen := int(header.Length) - RecordHeaderSize - int(header.PayloadLen)
	if paddingLen > 0 {
		padding := make([]byte, paddingLen)
		if _, err := w.buf.Write(padding); err != nil {
			return fmt.Errorf("failed to write padding: %w", err)
		}
	}

	// Update current offset
	w.currentOffset += uint64(header.Length)

	return nil
}

// encodeHeader encodes a WALRecordHeader to bytes (32 bytes)
func encodeHeader(h WALRecordHeader) []byte {
	buf := make([]byte, RecordHeaderSize)

	// Type (1 byte) at offset 0
	buf[0] = byte(h.Type)

	// Padding (1 byte) at offset 1 - already zero

	// Length (4 bytes) at offset 2
	ByteOrder.PutUint32(buf[2:6], h.Length)

	// LSN (8 bytes) at offset 6
	ByteOrder.PutUint64(buf[6:14], h.LSN)

	// CRC32 (4 bytes) at offset 14
	ByteOrder.PutUint32(buf[14:18], h.CRC32)

	// FileOffset (8 bytes) at offset 18
	ByteOrder.PutUint64(buf[18:26], h.FileOffset)

	// PayloadLen (4 bytes) at offset 26
	ByteOrder.PutUint32(buf[26:30], h.PayloadLen)

	// Remaining 2 bytes are padding at offset 30 (already zero)

	return buf
}

// ===========================================================================
// PAYLOAD ENCODERS
// ===========================================================================

// encodeInsertPayload encodes the payload for an Insert record
// Format: TxID(8) + TableNameLen(2) + TableName + KeyLen(2) + Key + ValueLen(4) + Value
func (w *WAL) encodeInsertPayload(txID uint64, tableName string, key string, value json.RawMessage) []byte {
	// Calculate total size
	size := 8 + 2 + len(tableName) + 2 + len(key) + 4 + len(value)
	buf := make([]byte, size)
	offset := 0

	// TxID (8 bytes)
	ByteOrder.PutUint64(buf[offset:], txID)
	offset += 8

	// TableName with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(tableName)))
	offset += 2
	copy(buf[offset:], tableName)
	offset += len(tableName)

	// Key with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(key)))
	offset += 2
	copy(buf[offset:], key)
	offset += len(key)

	// Value with length prefix (4 bytes)
	ByteOrder.PutUint32(buf[offset:], uint32(len(value)))
	offset += 4
	copy(buf[offset:], value)

	return buf
}

// encodeUpdatePayload encodes the payload for an Update record
// Format: TxID(8) + TableNameLen(2) + TableName + KeyLen(2) + Key + OldValueLen(4) + OldValue + NewValueLen(4) + NewValue
func (w *WAL) encodeUpdatePayload(txID uint64, tableName string, key string, oldValue json.RawMessage, newValue json.RawMessage) []byte {
	// Calculate total size
	size := 8 + 2 + len(tableName) + 2 + len(key) + 4 + len(oldValue) + 4 + len(newValue)
	buf := make([]byte, size)
	offset := 0

	// TxID (8 bytes)
	ByteOrder.PutUint64(buf[offset:], txID)
	offset += 8

	// TableName with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(tableName)))
	offset += 2
	copy(buf[offset:], tableName)
	offset += len(tableName)

	// Key with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(key)))
	offset += 2
	copy(buf[offset:], key)
	offset += len(key)

	// OldValue with length prefix (4 bytes)
	ByteOrder.PutUint32(buf[offset:], uint32(len(oldValue)))
	offset += 4
	copy(buf[offset:], oldValue)
	offset += len(oldValue)

	// NewValue with length prefix (4 bytes)
	ByteOrder.PutUint32(buf[offset:], uint32(len(newValue)))
	offset += 4
	copy(buf[offset:], newValue)

	return buf
}

// encodeDeletePayload encodes the payload for a Delete record
// Format: TxID(8) + TableNameLen(2) + TableName + KeyLen(2) + Key + OldValueLen(4) + OldValue
func (w *WAL) encodeDeletePayload(txID uint64, tableName string, key string, oldValue json.RawMessage) []byte {
	// Calculate total size
	size := 8 + 2 + len(tableName) + 2 + len(key) + 4 + len(oldValue)
	buf := make([]byte, size)
	offset := 0

	// TxID (8 bytes)
	ByteOrder.PutUint64(buf[offset:], txID)
	offset += 8

	// TableName with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(tableName)))
	offset += 2
	copy(buf[offset:], tableName)
	offset += len(tableName)

	// Key with length prefix (2 bytes)
	ByteOrder.PutUint16(buf[offset:], uint16(len(key)))
	offset += 2
	copy(buf[offset:], key)
	offset += len(key)

	// OldValue with length prefix (4 bytes)
	ByteOrder.PutUint32(buf[offset:], uint32(len(oldValue)))
	offset += 4
	copy(buf[offset:], oldValue)

	return buf
}

// encodeCheckpointPayload encodes the payload for a Checkpoint record
// Format: CheckpointLSN(8) + CheckpointOffset(8) + LastFlushedLSN(8) + Timestamp(8) +
//
//	DatabaseCRC32(4) + TableCount(4) + [TableChecksums...]
//
// Each TableChecksum: TableNameLen(2) + TableName + DataCRC32(4) + MetaCRC32(4)
func (w *WAL) encodeCheckpointPayload(tables []TableChecksum, databaseCRC32 uint32) []byte {
	// Calculate size for tables
	tablesSize := 0
	for _, t := range tables {
		tablesSize += 2 + len(t.TableName) + 4 + 4
	}

	// Total size: fixed fields + tables
	size := 8 + 8 + 8 + 8 + 4 + 4 + tablesSize
	buf := make([]byte, size)
	offset := 0

	// CheckpointLSN (8 bytes) - will be set to current LSN
	ByteOrder.PutUint64(buf[offset:], w.nextLSN)
	offset += 8

	// CheckpointOffset (8 bytes) - current file offset
	ByteOrder.PutUint64(buf[offset:], w.currentOffset)
	offset += 8

	// LastFlushedLSN (8 bytes)
	ByteOrder.PutUint64(buf[offset:], w.flushedLSN)
	offset += 8

	// Timestamp (8 bytes)
	ByteOrder.PutUint64(buf[offset:], uint64(time.Now().Unix()))
	offset += 8

	// DatabaseCRC32 (4 bytes)
	ByteOrder.PutUint32(buf[offset:], databaseCRC32)
	offset += 4

	// TableCount (4 bytes)
	ByteOrder.PutUint32(buf[offset:], uint32(len(tables)))
	offset += 4

	// Tables
	for _, t := range tables {
		// TableName with length prefix (2 bytes)
		ByteOrder.PutUint16(buf[offset:], uint16(len(t.TableName)))
		offset += 2
		copy(buf[offset:], t.TableName)
		offset += len(t.TableName)

		// DataCRC32 (4 bytes)
		ByteOrder.PutUint32(buf[offset:], t.DataCRC32)
		offset += 4

		// MetaCRC32 (4 bytes)
		ByteOrder.PutUint32(buf[offset:], t.MetaCRC32)
		offset += 4
	}

	return buf
}

// encodeCreateTablePayload encodes the payload for a CreateTable record
// Format: TxID(8) + TableNameLen(2) + TableName + SchemaLen(4) + Schema
func (w *WAL) encodeCreateTablePayload(txID uint64, tableName string, schemaBytes []byte) []byte {
	size := 8 + 2 + len(tableName) + 4 + len(schemaBytes)
	buf := make([]byte, size)
	offset := 0

	ByteOrder.PutUint64(buf[offset:], txID)
	offset += 8

	ByteOrder.PutUint16(buf[offset:], uint16(len(tableName)))
	offset += 2
	copy(buf[offset:], tableName)
	offset += len(tableName)

	ByteOrder.PutUint32(buf[offset:], uint32(len(schemaBytes)))
	offset += 4
	copy(buf[offset:], schemaBytes)

	return buf
}

// encodeDropTablePayload encodes the payload for a DropTable record
// Format: TxID(8) + TableNameLen(2) + TableName
func (w *WAL) encodeDropTablePayload(txID uint64, tableName string) []byte {
	size := 8 + 2 + len(tableName)
	buf := make([]byte, size)
	offset := 0

	ByteOrder.PutUint64(buf[offset:], txID)
	offset += 8

	ByteOrder.PutUint16(buf[offset:], uint16(len(tableName)))
	offset += 2
	copy(buf[offset:], tableName)

	return buf
}

// encodeAlterTablePayload encodes the payload for an AlterTable record
// Format: TxID(8) + TableNameLen(2) + TableName + AlterOp(1) + ColDescLen(4) + ColDesc
func (w *WAL) encodeAlterTablePayload(txID uint64, tableName string, alterOp uint8, colDesc []byte) []byte {
	size := 8 + 2 + len(tableName) + 1 + 4 + len(colDesc)
	buf := make([]byte, size)
	offset := 0

	ByteOrder.PutUint64(buf[offset:], txID)
	offset += 8

	ByteOrder.PutUint16(buf[offset:], uint16(len(tableName)))
	offset += 2
	copy(buf[offset:], tableName)
	offset += len(tableName)

	buf[offset] = alterOp
	offset++

	ByteOrder.PutUint32(buf[offset:], uint32(len(colDesc)))
	offset += 4
	copy(buf[offset:], colDesc)

	return buf
}
