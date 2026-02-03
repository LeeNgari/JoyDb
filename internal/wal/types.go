package wal

import (
	"encoding/binary"
	"encoding/json"
)

// ===========================================================================
// WAL FILE FORMAT
// ===========================================================================
//
// WAL File Structure:
// ┌─────────────────────────────────────────────────────────────────────────┐
// │ WAL File Header (fixed 64 bytes, padded)                                │
// ├─────────────────────────────────────────────────────────────────────────┤
// │ Record 1: [Header (24 bytes)] [Payload (variable)] [Padding to 8-byte]  │
// ├─────────────────────────────────────────────────────────────────────────┤
// │ Record 2: [Header (24 bytes)] [Payload (variable)] [Padding to 8-byte]  │
// ├─────────────────────────────────────────────────────────────────────────┤
// │ ...                                                                     │
// └─────────────────────────────────────────────────────────────────────────┘
//
// All multi-byte integers are little-endian.
// All records are aligned to 8-byte boundaries for efficient I/O.
//
// ===========================================================================

// ByteOrder is the byte order used for encoding WAL data
var ByteOrder = binary.LittleEndian

// RecordAlignment is the byte alignment for all WAL records
const RecordAlignment = 8

// ===========================================================================
// SAFETY LIMITS
// ===========================================================================

// MaxRecordSize is the maximum allowed size for a single WAL record (4MB)
// This prevents OOM attacks from corrupted Length fields during recovery
const MaxRecordSize = 4 * 1024 * 1024

// MinRecordSize is the minimum valid record size (header only, no payload)
const MinRecordSize = RecordHeaderSize

// WriteBufferSize is the size of the bufio.Writer buffer (32KB)
// This reduces syscalls by batching small writes
const WriteBufferSize = 32 * 1024

// ===========================================================================
// WAL FILE HEADER
// ===========================================================================

// WALMagic identifies a valid WAL file (ASCII: "JOYDBWAL")
var WALMagic = [8]byte{'J', 'O', 'Y', 'D', 'B', 'W', 'A', 'L'}

// WALVersion is the current WAL format version
const WALVersion uint16 = 1

// WALFileHeader is written at the beginning of every WAL file
// Fixed size: 64 bytes (padded for alignment)
type WALFileHeader struct {
	Magic        [8]byte  // Magic bytes to identify WAL file
	Version      uint16   // WAL format version
	DatabaseName [32]byte // Database name (null-padded)
	InitialLSN   uint64   // First LSN in this WAL file
	CreatedAt    int64    // Unix timestamp when WAL was created
	_            [6]byte  // Reserved padding to reach 64 bytes
}

// FileHeaderSize is the fixed size of the WAL file header
const FileHeaderSize = 64

// ===========================================================================
// RECORD TYPES
// ===========================================================================

// RecordType represents the type of WAL record
type RecordType uint8

const (
	RecordBeginTxn RecordType = iota + 1
	RecordInsert
	RecordUpdate
	RecordDelete
	RecordCommit
	RecordAbort
	RecordCheckpoint
)

// String returns a human-readable name for the record type
func (rt RecordType) String() string {
	switch rt {
	case RecordBeginTxn:
		return "BeginTxn"
	case RecordInsert:
		return "Insert"
	case RecordUpdate:
		return "Update"
	case RecordDelete:
		return "Delete"
	case RecordCommit:
		return "Commit"
	case RecordAbort:
		return "Abort"
	case RecordCheckpoint:
		return "Checkpoint"
	default:
		return "Unknown"
	}
}

// ===========================================================================
// WAL RECORD HEADER
// ===========================================================================

// WALRecordHeader is the common header for all WAL records
// Fixed size: 32 bytes (aligned to 8-byte boundary)
//
// Binary layout:
// ┌─────────┬─────────┬──────────┬─────────┬──────────┬────────────┬─────────┐
// │ Type(1) │ Pad(1)  │ Length(4)│ LSN(8)  │ CRC32(4) │ FileOff(8) │ Pad(6)  │
// │  uint8  │ reserved│  uint32  │ uint64  │  uint32  │   uint64   │ reserved│
// └─────────┴─────────┴──────────┴─────────┴──────────┴────────────┴─────────┘
// Offsets: 0        1         2          6         14         18          26
type WALRecordHeader struct {
	Type       RecordType // Type of record (1 byte) - offset 0
	_          uint8      // Padding for alignment (1 byte) - offset 1
	Length     uint32     // Total record length including header and padding - offset 2
	LSN        uint64     // Log Sequence Number - monotonically increasing - offset 6
	CRC32      uint32     // CRC32 checksum of payload (after header, before padding) - offset 14
	FileOffset uint64     // Byte offset in WAL file where this record starts - offset 18
	_          [6]byte    // Padding to reach 32 bytes - offset 26
}

// RecordHeaderSize is the fixed size of the WAL record header in bytes
// Computed: 1 + 1 + 4 + 8 + 4 + 8 + 6 = 32 bytes (aligned to 8-byte boundary)
const RecordHeaderSize = 32

// AlignTo8 rounds up a size to the next 8-byte boundary
func AlignTo8(size int) int {
	return (size + 7) &^ 7
}

// ===========================================================================
// TRANSACTION RECORDS
// ===========================================================================

// BeginTxnRecord marks the start of a transaction
// Payload: TxID (8 bytes)
type BeginTxnRecord struct {
	Header WALRecordHeader
	TxID   uint64
}

// CommitRecord marks a transaction as committed
// Payload: TxID (8 bytes)
type CommitRecord struct {
	Header WALRecordHeader
	TxID   uint64
}

// AbortRecord marks a transaction as aborted/rolled back
// Payload: TxID (8 bytes)
type AbortRecord struct {
	Header WALRecordHeader
	TxID   uint64
}

// ===========================================================================
// DML RECORDS (Data Manipulation)
// ===========================================================================

// InsertRecord logs an insert operation (REDO only)
// Payload: TxID (8) + TableNameLen (2) + TableName + KeyLen (2) + Key + ValueLen (4) + Value
type InsertRecord struct {
	Header    WALRecordHeader
	TxID      uint64
	TableName string
	Key       string          // Primary key value serialized as string
	Value     json.RawMessage // Row data as JSON (for REDO)
}

// UpdateRecord logs an update operation (REDO + UNDO)
// Payload: TxID (8) + TableNameLen (2) + TableName + KeyLen (2) + Key +
//
//	OldValueLen (4) + OldValue + NewValueLen (4) + NewValue
type UpdateRecord struct {
	Header    WALRecordHeader
	TxID      uint64
	TableName string
	Key       string          // Primary key value serialized as string
	OldValue  json.RawMessage // Previous row data (for UNDO during abort)
	NewValue  json.RawMessage // New row data (for REDO during recovery)
}

// DeleteRecord logs a delete operation (REDO + UNDO)
// Payload: TxID (8) + TableNameLen (2) + TableName + KeyLen (2) + Key + OldValueLen (4) + OldValue
type DeleteRecord struct {
	Header    WALRecordHeader
	TxID      uint64
	TableName string
	Key       string          // Primary key value serialized as string
	OldValue  json.RawMessage // Deleted row data (for UNDO during abort)
}

// ===========================================================================
// CHECKPOINT RECORD
// ===========================================================================

// CheckpointRecord marks a point where the database state was persisted to disk
// It includes checksums of all JSON files to detect external corruption
//
// Payload binary layout:
// ┌──────────────────┬──────────────────┬────────────────┬─────────────┬───────────────┬────────────┬─────────────────┐
// │ CheckpointLSN(8) │ CheckpointOff(8) │ FlushedLSN(8)  │ Timestamp(8)│ DatabaseCRC(4)│ TableCnt(4)│ Tables (var)    │
// └──────────────────┴──────────────────┴────────────────┴─────────────┴───────────────┴────────────┴─────────────────┘
type CheckpointRecord struct {
	Header           WALRecordHeader
	CheckpointLSN    uint64          // LSN at which checkpoint was taken (offset 0)
	CheckpointOffset uint64          // Byte offset in WAL file of this checkpoint (offset 8)
	LastFlushedLSN   uint64          // Last LSN guaranteed to be fsynced (offset 16)
	Timestamp        int64           // Unix timestamp of checkpoint (offset 24)
	DatabaseCRC32    uint32          // Checksum of database meta.json (offset 32)
	TableCount       uint32          // Number of tables (offset 36)
	Tables           []TableChecksum // Checksums of each table's JSON files (offset 40+)
}

// TableChecksum stores the checksum of a table's JSON files at checkpoint time
// Used to detect if JSON files were modified externally (user tampering)
type TableChecksum struct {
	TableName string // Name of the table
	DataCRC32 uint32 // CRC32 of data.json
	MetaCRC32 uint32 // CRC32 of meta.json
}

// ===========================================================================
// TRANSACTION STATE TRACKING
// ===========================================================================

// TxnStateType represents the state of a transaction
type TxnStateType uint8

const (
	TxnActive TxnStateType = iota + 1
	TxnCommitted
	TxnAborted
)

// String returns a human-readable name for the transaction state
func (ts TxnStateType) String() string {
	switch ts {
	case TxnActive:
		return "Active"
	case TxnCommitted:
		return "Committed"
	case TxnAborted:
		return "Aborted"
	default:
		return "Unknown"
	}
}

// TxnState tracks the state of an in-flight transaction
type TxnState struct {
	ID    uint64
	State TxnStateType
}

// ===========================================================================
// INTERFACES
// ===========================================================================

// WALRecord is an interface for all WAL record types
type WALRecord interface {
	GetHeader() WALRecordHeader
}

// Implement WALRecord interface for all record types
func (r BeginTxnRecord) GetHeader() WALRecordHeader   { return r.Header }
func (r InsertRecord) GetHeader() WALRecordHeader     { return r.Header }
func (r UpdateRecord) GetHeader() WALRecordHeader     { return r.Header }
func (r DeleteRecord) GetHeader() WALRecordHeader     { return r.Header }
func (r CommitRecord) GetHeader() WALRecordHeader     { return r.Header }
func (r AbortRecord) GetHeader() WALRecordHeader      { return r.Header }
func (r CheckpointRecord) GetHeader() WALRecordHeader { return r.Header }
