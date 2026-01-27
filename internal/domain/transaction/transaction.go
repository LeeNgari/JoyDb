package transaction

import (
	"time"

	"github.com/google/uuid"
)

// ChangeType represents the type of modification
type ChangeType string

const (
	ChangeTypeInsert ChangeType = "INSERT"
	ChangeTypeUpdate ChangeType = "UPDATE"
	ChangeTypeDelete ChangeType = "DELETE"
)

// Change represents a single modification within a transaction
type Change struct {
	Type    ChangeType
	Table   string
	RowID   int64
	Data    map[string]interface{} // New data for INSERT/UPDATE
	OldData map[string]interface{} // Old data for UPDATE/DELETE
}

// Transaction represents a database transaction context
type Transaction struct {
	ID        string    // Unique transaction identifier
	Active    bool      // Whether transaction is currently active
	StartTime time.Time // When the transaction began
	Changes   []Change  // Modifications made
}

// NewTransaction creates a new transaction with a unique ID
func NewTransaction() *Transaction {
	return &Transaction{
		ID:        uuid.New().String(),
		Active:    true,
		StartTime: time.Now(),
		Changes:   make([]Change, 0),
	}
}

// Close marks the transaction as inactive
func (tx *Transaction) Close() {
	tx.Active = false
}
