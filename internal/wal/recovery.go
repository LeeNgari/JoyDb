package wal

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// ===========================================================================
// WAL RECOVERY SYSTEM
// ===========================================================================
//
// Recovery is responsible for:
// 1. Finding the last valid checkpoint
// 2. Verifying JSON file integrity using checkpoint checksums
// 3. Replaying committed transactions after the checkpoint
// 4. Rebuilding in-memory state from WAL if JSON is corrupted
//
// Recovery Strategy: REDO-only
// - We only replay committed transactions forward
// - Uncommitted transactions (no Commit record) are ignored
// - Aborted transactions are also skipped
//
// ===========================================================================

// RecoveryResult contains the outcome of WAL recovery
type RecoveryResult struct {
	// Checkpoint info
	LastCheckpoint  *CheckpointRecord // Last valid checkpoint found (nil if none)
	CheckpointValid bool              // Whether checkpoint JSON files are valid

	// Recovery stats
	RecordsScanned      int // Total records scanned
	TransactionsFound   int // Total transactions found
	TransactionsReplay  int // Transactions replayed (committed after checkpoint)
	TransactionsSkipped int // Transactions skipped (uncommitted or aborted)

	// Operations to replay
	InsertOps []*InsertRecord // Insert operations to replay
	UpdateOps []*UpdateRecord // Update operations to replay
	DeleteOps []*DeleteRecord // Delete operations to replay

	// State after recovery
	NextLSN        uint64 // Next LSN to use after recovery
	LastFlushedLSN uint64 // Last flushed LSN
}

// RecoveryManager handles WAL recovery operations
type RecoveryManager struct {
	walPath string     // Path to WAL file
	reader  *WALReader // WAL reader
	dbPath  string     // Path to database directory
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(walPath string, dbPath string) (*RecoveryManager, error) {
	reader, err := NewWALReader(walPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL reader: %w", err)
	}

	return &RecoveryManager{
		walPath: walPath,
		reader:  reader,
		dbPath:  dbPath,
	}, nil
}

// Close closes the recovery manager
func (rm *RecoveryManager) Close() error {
	if rm.reader != nil {
		return rm.reader.Close()
	}
	return nil
}

// ===========================================================================
// RECOVERY FLOW
// ===========================================================================

// Recover performs full WAL recovery
// Returns the operations that need to be replayed to restore state
func (rm *RecoveryManager) Recover() (*RecoveryResult, error) {
	// Find last checkpoint
	lastCheckpoint, err := rm.reader.FindLastCheckpoint()
	if err != nil {
		return nil, fmt.Errorf("failed to find last checkpoint: %w", err)
	}

	// Decide recovery strategy
	if lastCheckpoint != nil {
		// Verify checkpoint JSON files
		valid, verifyErr := rm.VerifyCheckpoint(lastCheckpoint)
		if verifyErr == nil && valid {
			// Checkpoint valid - recover from checkpoint
			return rm.RecoverFromCheckpoint(lastCheckpoint)
		}
		// Checkpoint invalid - fall through to scratch recovery
	}

	// No checkpoint or invalid checkpoint - recover from scratch
	return rm.RecoverFromScratch()
}

// RecoverFromCheckpoint recovers starting from a checkpoint
// Only replays transactions committed after the checkpoint
func (rm *RecoveryManager) RecoverFromCheckpoint(checkpoint *CheckpointRecord) (*RecoveryResult, error) {
	result := &RecoveryResult{
		LastCheckpoint:  checkpoint,
		CheckpointValid: true,
		InsertOps:       []*InsertRecord{},
		UpdateOps:       []*UpdateRecord{},
		DeleteOps:       []*DeleteRecord{},
		NextLSN:         checkpoint.CheckpointLSN + 1,
	}

	// Seek past the checkpoint record
	seekOffset := checkpoint.Header.FileOffset + uint64(checkpoint.Header.Length)
	if err := rm.reader.SeekToOffset(seekOffset); err != nil {
		return nil, fmt.Errorf("failed to seek past checkpoint: %w", err)
	}

	// Use transaction tracker for analysis and redo
	tracker := NewTxnTracker()

	// Scan all records after checkpoint
	for {
		record, err := rm.reader.ReadNextRecord()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading record during recovery: %w", err)
		}

		result.RecordsScanned++

		// Track highest LSN
		header := record.GetHeader()
		if header.LSN >= result.NextLSN {
			result.NextLSN = header.LSN + 1
		}

		// Process record through tracker
		if err := tracker.ProcessRecord(record); err != nil {
			return nil, fmt.Errorf("error processing record: %w", err)
		}
	}

	// Collect operations from committed transactions
	rm.collectCommittedOps(tracker, result)

	result.LastFlushedLSN = result.NextLSN - 1

	return result, nil
}

// RecoverFromScratch recovers from the beginning of the WAL
// Used when no checkpoint exists or JSON files are corrupted
func (rm *RecoveryManager) RecoverFromScratch() (*RecoveryResult, error) {
	result := &RecoveryResult{
		CheckpointValid: false,
		InsertOps:       []*InsertRecord{},
		UpdateOps:       []*UpdateRecord{},
		DeleteOps:       []*DeleteRecord{},
		NextLSN:         1,
	}

	// Read file header first
	_, err := rm.reader.ReadFileHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read WAL file header: %w", err)
	}

	// Use transaction tracker for analysis and redo
	tracker := NewTxnTracker()

	// Scan all records from beginning
	for {
		record, err := rm.reader.ReadNextRecord()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading record during recovery: %w", err)
		}

		result.RecordsScanned++

		// Track highest LSN
		header := record.GetHeader()
		if header.LSN >= result.NextLSN {
			result.NextLSN = header.LSN + 1
		}

		// Track checkpoints
		if cp, ok := record.(*CheckpointRecord); ok {
			result.LastCheckpoint = cp
		}

		// Process record through tracker
		if err := tracker.ProcessRecord(record); err != nil {
			return nil, fmt.Errorf("error processing record: %w", err)
		}
	}

	// Collect operations from committed transactions
	rm.collectCommittedOps(tracker, result)

	result.LastFlushedLSN = result.NextLSN - 1

	return result, nil
}

// collectCommittedOps extracts operations from committed transactions
func (rm *RecoveryManager) collectCommittedOps(tracker *TxnTracker, result *RecoveryResult) {
	committed := tracker.GetCommittedTransactions()
	uncommitted := tracker.GetUncommittedTransactions()

	result.TransactionsReplay = len(committed)
	result.TransactionsSkipped = len(uncommitted)
	result.TransactionsFound = len(committed) + len(uncommitted)

	for _, txn := range committed {
		result.InsertOps = append(result.InsertOps, txn.Inserts...)
		result.UpdateOps = append(result.UpdateOps, txn.Updates...)
		result.DeleteOps = append(result.DeleteOps, txn.Deletes...)
	}
}

// ===========================================================================
// CHECKPOINT VERIFICATION
// ===========================================================================

// VerifyCheckpoint verifies that JSON files match checkpoint checksums
func (rm *RecoveryManager) VerifyCheckpoint(checkpoint *CheckpointRecord) (bool, error) {
	// Verify database meta.json CRC
	dbMetaPath := filepath.Join(rm.dbPath, "meta.json")
	dbCRC, err := CalculateFileCRC32(dbMetaPath)
	if err != nil {
		// File doesn't exist or can't be read - checkpoint invalid
		return false, nil
	}
	if dbCRC != checkpoint.DatabaseCRC32 {
		return false, nil
	}

	// Verify each table's checksums
	for _, table := range checkpoint.Tables {
		tablePath := filepath.Join(rm.dbPath, table.TableName)

		// Check data.json CRC
		dataPath := filepath.Join(tablePath, "data.json")
		dataCRC, err := CalculateFileCRC32(dataPath)
		if err != nil {
			return false, nil
		}
		if dataCRC != table.DataCRC32 {
			return false, nil
		}

		// Check meta.json CRC
		metaPath := filepath.Join(tablePath, "meta.json")
		metaCRC, err := CalculateFileCRC32(metaPath)
		if err != nil {
			return false, nil
		}
		if metaCRC != table.MetaCRC32 {
			return false, nil
		}
	}

	return true, nil
}

// CalculateFileCRC32 calculates the CRC32 of a file
func CalculateFileCRC32(filePath string) (uint32, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}
	return crc32.ChecksumIEEE(data), nil
}

// ===========================================================================
// TRANSACTION TRACKING
// ===========================================================================

// TxnTracker tracks transaction states during recovery
type TxnTracker struct {
	transactions map[uint64]*TxnRecoveryState
}

// TxnRecoveryState tracks a single transaction during recovery
type TxnRecoveryState struct {
	TxID     uint64
	State    TxnStateType
	BeginLSN uint64
	EndLSN   uint64 // Commit or Abort LSN
	Inserts  []*InsertRecord
	Updates  []*UpdateRecord
	Deletes  []*DeleteRecord
}

// NewTxnTracker creates a new transaction tracker
func NewTxnTracker() *TxnTracker {
	return &TxnTracker{
		transactions: make(map[uint64]*TxnRecoveryState),
	}
}

// ProcessRecord processes a WAL record and updates transaction state
func (t *TxnTracker) ProcessRecord(record WALRecord) error {
	switch rec := record.(type) {
	case *BeginTxnRecord:
		t.BeginTransaction(rec)
	case *InsertRecord:
		return t.AddInsert(rec)
	case *UpdateRecord:
		return t.AddUpdate(rec)
	case *DeleteRecord:
		return t.AddDelete(rec)
	case *CommitRecord:
		return t.CommitTransaction(rec)
	case *AbortRecord:
		return t.AbortTransaction(rec)
	case *CheckpointRecord:
		// Checkpoints don't affect transaction tracking
	}
	return nil
}

// BeginTransaction records a transaction start
func (t *TxnTracker) BeginTransaction(record *BeginTxnRecord) {
	t.transactions[record.TxID] = &TxnRecoveryState{
		TxID:     record.TxID,
		State:    TxnActive,
		BeginLSN: record.Header.LSN,
		Inserts:  []*InsertRecord{},
		Updates:  []*UpdateRecord{},
		Deletes:  []*DeleteRecord{},
	}
}

// getOrCreateTxn gets an existing transaction or creates a new one
// This handles the case where we start recovery after a BeginTxn record
func (t *TxnTracker) getOrCreateTxn(txID uint64, lsn uint64) *TxnRecoveryState {
	txn, exists := t.transactions[txID]
	if !exists {
		// Transaction started before our recovery point
		txn = &TxnRecoveryState{
			TxID:     txID,
			State:    TxnActive,
			BeginLSN: lsn, // Best guess
			Inserts:  []*InsertRecord{},
			Updates:  []*UpdateRecord{},
			Deletes:  []*DeleteRecord{},
		}
		t.transactions[txID] = txn
	}
	return txn
}

// AddInsert adds an insert operation to a transaction
func (t *TxnTracker) AddInsert(record *InsertRecord) error {
	txn := t.getOrCreateTxn(record.TxID, record.Header.LSN)
	if txn.State != TxnActive {
		return fmt.Errorf("cannot add insert to non-active transaction %d", record.TxID)
	}
	txn.Inserts = append(txn.Inserts, record)
	return nil
}

// AddUpdate adds an update operation to a transaction
func (t *TxnTracker) AddUpdate(record *UpdateRecord) error {
	txn := t.getOrCreateTxn(record.TxID, record.Header.LSN)
	if txn.State != TxnActive {
		return fmt.Errorf("cannot add update to non-active transaction %d", record.TxID)
	}
	txn.Updates = append(txn.Updates, record)
	return nil
}

// AddDelete adds a delete operation to a transaction
func (t *TxnTracker) AddDelete(record *DeleteRecord) error {
	txn := t.getOrCreateTxn(record.TxID, record.Header.LSN)
	if txn.State != TxnActive {
		return fmt.Errorf("cannot add delete to non-active transaction %d", record.TxID)
	}
	txn.Deletes = append(txn.Deletes, record)
	return nil
}

// CommitTransaction marks a transaction as committed
func (t *TxnTracker) CommitTransaction(record *CommitRecord) error {
	txn := t.getOrCreateTxn(record.TxID, record.Header.LSN)
	txn.State = TxnCommitted
	txn.EndLSN = record.Header.LSN
	return nil
}

// AbortTransaction marks a transaction as aborted
func (t *TxnTracker) AbortTransaction(record *AbortRecord) error {
	txn := t.getOrCreateTxn(record.TxID, record.Header.LSN)
	txn.State = TxnAborted
	txn.EndLSN = record.Header.LSN
	// Clear operations - they won't be replayed
	txn.Inserts = nil
	txn.Updates = nil
	txn.Deletes = nil
	return nil
}

// GetCommittedTransactions returns all committed transactions
func (t *TxnTracker) GetCommittedTransactions() []*TxnRecoveryState {
	var committed []*TxnRecoveryState
	for _, txn := range t.transactions {
		if txn.State == TxnCommitted {
			committed = append(committed, txn)
		}
	}
	// Sort by EndLSN for consistent replay order
	sort.Slice(committed, func(i, j int) bool {
		return committed[i].EndLSN < committed[j].EndLSN
	})
	return committed
}

// GetUncommittedTransactions returns all uncommitted transactions (for logging)
func (t *TxnTracker) GetUncommittedTransactions() []*TxnRecoveryState {
	var uncommitted []*TxnRecoveryState
	for _, txn := range t.transactions {
		if txn.State == TxnActive {
			uncommitted = append(uncommitted, txn)
		}
	}
	return uncommitted
}

// GetAbortedTransactions returns all aborted transactions
func (t *TxnTracker) GetAbortedTransactions() []*TxnRecoveryState {
	var aborted []*TxnRecoveryState
	for _, txn := range t.transactions {
		if txn.State == TxnAborted {
			aborted = append(aborted, txn)
		}
	}
	return aborted
}

// ===========================================================================
// REPLAY OPERATIONS
// ===========================================================================

// ReplayTarget is an interface for replaying WAL operations
// This will be implemented by the storage layer (e.g., Engine)
type ReplayTarget interface {
	// ReplayInsert applies an insert operation
	ReplayInsert(tableName string, key string, value json.RawMessage) error

	// ReplayUpdate applies an update operation
	ReplayUpdate(tableName string, key string, newValue json.RawMessage) error

	// ReplayDelete applies a delete operation
	ReplayDelete(tableName string, key string) error
}

// ReplayAll replays all operations in the recovery result to the target
// Operations are replayed in LSN order for correctness
func (result *RecoveryResult) ReplayAll(target ReplayTarget) error {
	// Get all operations sorted by LSN
	ops := result.GetAllOperations()

	// Replay in order
	for _, op := range ops {
		switch rec := op.(type) {
		case *InsertRecord:
			if err := target.ReplayInsert(rec.TableName, rec.Key, rec.Value); err != nil {
				return fmt.Errorf("failed to replay insert at LSN %d: %w", rec.Header.LSN, err)
			}
		case *UpdateRecord:
			if err := target.ReplayUpdate(rec.TableName, rec.Key, rec.NewValue); err != nil {
				return fmt.Errorf("failed to replay update at LSN %d: %w", rec.Header.LSN, err)
			}
		case *DeleteRecord:
			if err := target.ReplayDelete(rec.TableName, rec.Key); err != nil {
				return fmt.Errorf("failed to replay delete at LSN %d: %w", rec.Header.LSN, err)
			}
		}
	}

	return nil
}

// GetAllOperations returns all operations sorted by LSN
func (result *RecoveryResult) GetAllOperations() []WALRecord {
	// Combine all operations
	ops := make([]WALRecord, 0, len(result.InsertOps)+len(result.UpdateOps)+len(result.DeleteOps))

	for _, op := range result.InsertOps {
		ops = append(ops, op)
	}
	for _, op := range result.UpdateOps {
		ops = append(ops, op)
	}
	for _, op := range result.DeleteOps {
		ops = append(ops, op)
	}

	// Sort by LSN for correct replay order
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].GetHeader().LSN < ops[j].GetHeader().LSN
	})

	return ops
}

// ===========================================================================
// HELPER FUNCTIONS
// ===========================================================================

// getTxIDFromRecord extracts the transaction ID from a WAL record
func getTxIDFromRecord(r WALRecord) uint64 {
	switch rec := r.(type) {
	case *BeginTxnRecord:
		return rec.TxID
	case *InsertRecord:
		return rec.TxID
	case *UpdateRecord:
		return rec.TxID
	case *DeleteRecord:
		return rec.TxID
	case *CommitRecord:
		return rec.TxID
	case *AbortRecord:
		return rec.TxID
	default:
		return 0
	}
}
