package wal

import (
	"encoding/binary"
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
const WALVersion uint16 = 2

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
	RecordCreateTable // = 8
	RecordDropTable   // = 9
	RecordAlterTable  // = 10
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
	case RecordCreateTable:
		return "CreateTable"
	case RecordDropTable:
		return "DropTable"
	case RecordAlterTable:
		return "AlterTable"
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
	PayloadLen uint32     // Length of payload (before padding) - offset 26
	_          [2]byte    // Padding to reach 32 bytes - offset 30
}

// RecordHeaderSize is the fixed size of the WAL record header in bytes
// Computed: 1 + 1 + 4 + 8 + 4 + 8 + 4 + 2 = 32 bytes (aligned to 8-byte boundary)
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
	Key       string // Primary key value serialized as string
	Value     []byte // Row data serialized (for REDO)
}

// UpdateRecord logs an update operation (REDO + UNDO)
// Payload: TxID (8) + TableNameLen (2) + TableName + KeyLen (2) + Key +
//
//	OldValueLen (4) + OldValue + NewValueLen (4) + NewValue
type UpdateRecord struct {
	Header    WALRecordHeader
	TxID      uint64
	TableName string
	Key       string // Primary key value serialized as string
	OldValue  []byte // Previous row data (for UNDO during abort)
	NewValue  []byte // New row data (for REDO during recovery)
}

// DeleteRecord logs a delete operation (REDO + UNDO)
// Payload: TxID (8) + TableNameLen (2) + TableName + KeyLen (2) + Key + OldValueLen (4) + OldValue
type DeleteRecord struct {
	Header    WALRecordHeader
	TxID      uint64
	TableName string
	Key       string // Primary key value serialized as string
	OldValue  []byte // Deleted row data (for UNDO during abort)
}

// ===========================================================================
// CHECKPOINT RECORD
// ===========================================================================

// CheckpointRecord marks a point where the database state was persisted to disk
// It includes checksums of all JSON files to detect external corruption
//
// Payload binary layout:
// ┌──────────────────┬──────────────────┬────────────────┬─────────────┬────────────────┬─────────────────┐
// │ CheckpointLSN(8) │ CheckpointOff(8) │ FlushedLSN(8)  │ Timestamp(8)│ SnapshotLSN(8) │ SnapshotCRC32(4)│
// └──────────────────┴──────────────────┴────────────────┴─────────────┴────────────────┴─────────────────┘
type CheckpointRecord struct {
	Header           WALRecordHeader
	CheckpointLSN    uint64 // LSN at which checkpoint was taken (offset 0)
	CheckpointOffset uint64 // Byte offset in WAL file of this checkpoint (offset 8)
	LastFlushedLSN   uint64 // Last LSN guaranteed to be fsynced (offset 16)
	Timestamp        int64  // Unix timestamp of checkpoint (offset 24)
	SnapshotLSN      uint64 // LSN of the snapshot this checkpoint references (offset 32)
	SnapshotCRC32    uint32 // CRC of the snapshot file for tamper detection (offset 40)
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
func (r BeginTxnRecord) GetHeader() WALRecordHeader    { return r.Header }
func (r InsertRecord) GetHeader() WALRecordHeader      { return r.Header }
func (r UpdateRecord) GetHeader() WALRecordHeader      { return r.Header }
func (r DeleteRecord) GetHeader() WALRecordHeader      { return r.Header }
func (r CommitRecord) GetHeader() WALRecordHeader      { return r.Header }
func (r AbortRecord) GetHeader() WALRecordHeader       { return r.Header }
func (r CheckpointRecord) GetHeader() WALRecordHeader  { return r.Header }
func (r CreateTableRecord) GetHeader() WALRecordHeader { return r.Header }
func (r DropTableRecord) GetHeader() WALRecordHeader   { return r.Header }
func (r AlterTableRecord) GetHeader() WALRecordHeader  { return r.Header }

// ===========================================================================
// DDL RECORDS (Data Definition Language)
// ===========================================================================

// CreateTableRecord logs a CREATE TABLE DDL operation
// Payload: TxID(8) + TableNameLen(2) + TableName + SchemaLen(4) + Schema
type CreateTableRecord struct {
	Header    WALRecordHeader
	TxID      uint64
	TableName string
	Schema    []byte // binary-encoded TableSchema (see schema_encoder.go)
}

// DropTableRecord logs a DROP TABLE DDL operation
// Payload: TxID(8) + TableNameLen(2) + TableName
type DropTableRecord struct {
	Header    WALRecordHeader
	TxID      uint64
	TableName string
}

// AlterTableRecord logs an ALTER TABLE DDL operation
// Payload: TxID(8) + TableNameLen(2) + TableName + AlterOp(1) + ColDescLen(4) + ColDesc
type AlterTableRecord struct {
	Header    WALRecordHeader
	TxID      uint64
	TableName string
	AlterOp   uint8  // 0x01=ADD_COLUMN, 0x02=DROP_COLUMN, 0x03=RENAME_COLUMN
	ColDesc   []byte // binary-encoded Column descriptor
}

// AlterOp constants for AlterTableRecord
const (
	AlterOpAddColumn    uint8 = 0x01
	AlterOpDropColumn   uint8 = 0x02
	AlterOpRenameColumn uint8 = 0x03
)
