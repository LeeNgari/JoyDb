package manager

import (
	"github.com/leengari/joydb/internal/wal"
)

// BufferedEntry represents a single WAL operation buffered in memory
type BufferedEntry struct {
	Type      wal.RecordType
	TableName string
	Key       string
	Value     []byte // For INSERT: row data; For UPDATE: new row data
	OldValue  []byte // For UPDATE/DELETE: old row data
	Schema    []byte // For CREATE TABLE: schema bytes
	AlterOp   uint8  // For ALTER TABLE
	ColDesc   []byte // For ALTER TABLE
}

// TxBuffer holds buffered WAL entries for a single transaction
type TxBuffer struct {
	TxID    uint64
	Entries []BufferedEntry
}

// NewTxBuffer creates a new transaction buffer
func NewTxBuffer(txID uint64) *TxBuffer {
	return &TxBuffer{
		TxID:    txID,
		Entries: make([]BufferedEntry, 0),
	}
}
