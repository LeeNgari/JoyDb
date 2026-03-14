package wal

import (
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
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

	// DML operations to replay
	InsertOps []*InsertRecord // Insert operations to replay
	UpdateOps []*UpdateRecord // Update operations to replay
	DeleteOps []*DeleteRecord // Delete operations to replay

	// DDL operations to replay (applied before DML)
	CreateTableOps []*CreateTableRecord // CREATE TABLE operations to replay
	DropTableOps   []*DropTableRecord   // DROP TABLE operations to replay
	AlterTableOps  []*AlterTableRecord  // ALTER TABLE operations to replay

	// State after recovery
	NextLSN        uint64 // Next LSN to use after recovery
	LastFlushedLSN uint64 // Last flushed LSN
}

// RecoveryProgress represents the progress of WAL recovery
type RecoveryProgress struct {
	Phase            string        // "Scanning" / "Replaying"
	TotalRecords     int64         // Total records to process (estimated)
	ProcessedRecords int64         // Records processed so far
	EstimatedTime    time.Duration // Time elapsed
	StartTime        time.Time
}

// Percentage returns the completion percentage
func (p RecoveryProgress) Percentage() int {
	if p.TotalRecords == 0 {
		return 0
	}
	return int(float64(p.ProcessedRecords) / float64(p.TotalRecords) * 100)
}

// ProgressCallback is a function called to report progress
type ProgressCallback func(progress RecoveryProgress)

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
	return rm.RecoverWithProgress(nil)
}

// RecoverWithProgress performs recovery with progress reporting
func (rm *RecoveryManager) RecoverWithProgress(callback ProgressCallback) (*RecoveryResult, error) {
	// Find last checkpoint
	if callback != nil {
		callback(RecoveryProgress{Phase: "Finding Checkpoint", StartTime: time.Now()})
	}

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
			return rm.RecoverFromCheckpoint(lastCheckpoint, callback)
		}
		// Checkpoint invalid - fall through to scratch recovery
	}

	// No checkpoint or invalid checkpoint - recover from scratch
	return rm.RecoverFromScratch(callback)
}

// RecoverFromCheckpoint recovers starting from a checkpoint
// Only replays transactions committed after the checkpoint
func (rm *RecoveryManager) RecoverFromCheckpoint(checkpoint *CheckpointRecord, callback ProgressCallback) (*RecoveryResult, error) {
	result := &RecoveryResult{
		LastCheckpoint:  checkpoint,
		CheckpointValid: true,
		InsertOps:       []*InsertRecord{},
		UpdateOps:       []*UpdateRecord{},
		DeleteOps:       []*DeleteRecord{},
		CreateTableOps:  []*CreateTableRecord{},
		DropTableOps:    []*DropTableRecord{},
		AlterTableOps:   []*AlterTableRecord{},
		NextLSN:         checkpoint.CheckpointLSN + 1,
	}

	// Seek past the checkpoint record
	seekOffset := checkpoint.Header.FileOffset + uint64(checkpoint.Header.Length)
	if err := rm.reader.SeekToOffset(seekOffset); err != nil {
		return nil, fmt.Errorf("failed to seek past checkpoint: %w", err)
	}

	startTime := time.Now()
	if callback != nil {
		callback(RecoveryProgress{
			Phase:     "Scanning WAL (from checkpoint)",
			StartTime: startTime,
		})
	}

	// Use transaction tracker for analysis and redo
	tracker := NewTxnTracker()

	// Scan all records after checkpoint
	processed := int64(0)
	for {
		record, err := rm.reader.ReadNextRecord()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading record during recovery: %w", err)
		}

		result.RecordsScanned++
		processed++

		// Report progress periodically
		if callback != nil && processed%1000 == 0 {
			callback(RecoveryProgress{
				Phase:            "Scanning WAL (from checkpoint)",
				ProcessedRecords: processed,
				StartTime:        startTime,
				EstimatedTime:    time.Since(startTime),
			})
		}

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
func (rm *RecoveryManager) RecoverFromScratch(callback ProgressCallback) (*RecoveryResult, error) {
	result := &RecoveryResult{
		CheckpointValid: false,
		InsertOps:       []*InsertRecord{},
		UpdateOps:       []*UpdateRecord{},
		DeleteOps:       []*DeleteRecord{},
		CreateTableOps:  []*CreateTableRecord{},
		DropTableOps:    []*DropTableRecord{},
		AlterTableOps:   []*AlterTableRecord{},
		NextLSN:         1,
	}

	// Read file header first
	_, err := rm.reader.ReadFileHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read WAL file header: %w", err)
	}

	startTime := time.Now()
	if callback != nil {
		callback(RecoveryProgress{
			Phase:     "Scanning WAL (full)",
			StartTime: startTime,
		})
	}

	// Use transaction tracker for analysis and redo
	tracker := NewTxnTracker()

	// Scan all records from beginning
	processed := int64(0)
	for {
		record, err := rm.reader.ReadNextRecord()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading record during recovery: %w", err)
		}

		result.RecordsScanned++
		processed++

		// Report progress periodically
		if callback != nil && processed%1000 == 0 {
			callback(RecoveryProgress{
				Phase:            "Scanning WAL (full)",
				ProcessedRecords: processed,
				StartTime:        startTime,
				EstimatedTime:    time.Since(startTime),
			})
		}

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
	aborted := tracker.GetAbortedTransactions()

	result.TransactionsReplay = len(committed)
	result.TransactionsSkipped = len(uncommitted) + len(aborted)
	result.TransactionsFound = len(committed) + len(uncommitted) + len(aborted)

	for _, txn := range committed {
		result.InsertOps = append(result.InsertOps, txn.Inserts...)
		result.UpdateOps = append(result.UpdateOps, txn.Updates...)
		result.DeleteOps = append(result.DeleteOps, txn.Deletes...)
		result.CreateTableOps = append(result.CreateTableOps, txn.CreateTables...)
		result.DropTableOps = append(result.DropTableOps, txn.DropTables...)
		result.AlterTableOps = append(result.AlterTableOps, txn.AlterTables...)
	}
}

// ===========================================================================
// CHECKPOINT VERIFICATION
// ===========================================================================

// VerifyCheckpoint verifies that the snapshot file matches the checkpoint checksum
func (rm *RecoveryManager) VerifyCheckpoint(checkpoint *CheckpointRecord) (bool, error) {
	// Verify snapshot file CRC
	snapshotPath := filepath.Join(rm.dbPath, fmt.Sprintf("%d.snap", checkpoint.SnapshotLSN))
	snapshotCRC, err := CalculateFileCRC32(snapshotPath)
	if err != nil {
		// File doesn't exist or can't be read - checkpoint invalid
		return false, nil
	}
	if snapshotCRC != checkpoint.SnapshotCRC32 {
		return false, nil
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
	// DDL operations within this transaction
	CreateTables []*CreateTableRecord
	DropTables   []*DropTableRecord
	AlterTables  []*AlterTableRecord
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
	case *CreateTableRecord:
		return t.AddCreateTable(rec)
	case *DropTableRecord:
		return t.AddDropTable(rec)
	case *AlterTableRecord:
		return t.AddAlterTable(rec)
	case *CheckpointRecord:
		// Checkpoints don't affect transaction tracking
	}
	return nil
}

// BeginTransaction records a transaction start
func (t *TxnTracker) BeginTransaction(record *BeginTxnRecord) {
	t.transactions[record.TxID] = &TxnRecoveryState{
		TxID:         record.TxID,
		State:        TxnActive,
		BeginLSN:     record.Header.LSN,
		Inserts:      []*InsertRecord{},
		Updates:      []*UpdateRecord{},
		Deletes:      []*DeleteRecord{},
		CreateTables: []*CreateTableRecord{},
		DropTables:   []*DropTableRecord{},
		AlterTables:  []*AlterTableRecord{},
	}
}

// getOrCreateTxn gets an existing transaction or creates a new one
// This handles the case where we start recovery after a BeginTxn record
func (t *TxnTracker) getOrCreateTxn(txID uint64, lsn uint64) *TxnRecoveryState {
	txn, exists := t.transactions[txID]
	if !exists {
		// Transaction started before our recovery point
		txn = &TxnRecoveryState{
			TxID:         txID,
			State:        TxnActive,
			BeginLSN:     lsn, // Best guess
			Inserts:      []*InsertRecord{},
			Updates:      []*UpdateRecord{},
			Deletes:      []*DeleteRecord{},
			CreateTables: []*CreateTableRecord{},
			DropTables:   []*DropTableRecord{},
			AlterTables:  []*AlterTableRecord{},
		}
		t.transactions[txID] = txn
	}
	return txn
}

// checkActiveTxn gets or creates a transaction and ensures it is active
func (t *TxnTracker) checkActiveTxn(txID uint64, lsn uint64, opName string) (*TxnRecoveryState, error) {
	txn := t.getOrCreateTxn(txID, lsn)
	if txn.State != TxnActive {
		return nil, fmt.Errorf("cannot add %s to non-active transaction %d", opName, txID)
	}
	return txn, nil
}

// AddInsert adds an insert operation to a transaction
func (t *TxnTracker) AddInsert(record *InsertRecord) error {
	txn, err := t.checkActiveTxn(record.TxID, record.Header.LSN, "insert")
	if err != nil {
		return err
	}
	txn.Inserts = append(txn.Inserts, record)
	return nil
}

// AddUpdate adds an update operation to a transaction
func (t *TxnTracker) AddUpdate(record *UpdateRecord) error {
	txn, err := t.checkActiveTxn(record.TxID, record.Header.LSN, "update")
	if err != nil {
		return err
	}
	txn.Updates = append(txn.Updates, record)
	return nil
}

// AddDelete adds a delete operation to a transaction
func (t *TxnTracker) AddDelete(record *DeleteRecord) error {
	txn, err := t.checkActiveTxn(record.TxID, record.Header.LSN, "delete")
	if err != nil {
		return err
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
	// Clear all operations - they won't be replayed
	txn.Inserts = nil
	txn.Updates = nil
	txn.Deletes = nil
	txn.CreateTables = nil
	txn.DropTables = nil
	txn.AlterTables = nil
	return nil
}

// AddCreateTable adds a CREATE TABLE operation to a transaction
func (t *TxnTracker) AddCreateTable(record *CreateTableRecord) error {
	txn, err := t.checkActiveTxn(record.TxID, record.Header.LSN, "CreateTable")
	if err != nil {
		return err
	}
	txn.CreateTables = append(txn.CreateTables, record)
	return nil
}

// AddDropTable adds a DROP TABLE operation to a transaction
func (t *TxnTracker) AddDropTable(record *DropTableRecord) error {
	txn, err := t.checkActiveTxn(record.TxID, record.Header.LSN, "DropTable")
	if err != nil {
		return err
	}
	txn.DropTables = append(txn.DropTables, record)
	return nil
}

// AddAlterTable adds an ALTER TABLE operation to a transaction
func (t *TxnTracker) AddAlterTable(record *AlterTableRecord) error {
	txn, err := t.checkActiveTxn(record.TxID, record.Header.LSN, "AlterTable")
	if err != nil {
		return err
	}
	txn.AlterTables = append(txn.AlterTables, record)
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
	// DML replay methods
	ReplayInsert(tableName string, key string, value []byte) error
	ReplayUpdate(tableName string, key string, newValue []byte) error
	ReplayDelete(tableName string, key string) error

	// DDL replay methods
	ReplayCreateTable(name string, schemaBytes []byte) error
	ReplayDropTable(name string) error
	ReplayAlterTable(name string, op uint8, colDesc []byte) error
}

// ReplayAll replays all operations in the recovery result to the target.
// DDL operations (CREATE/DROP/ALTER TABLE) are applied first in LSN order,
// then DML operations (INSERT/UPDATE/DELETE) in LSN order.
// This ordering is critical: rows can only be inserted into tables that exist.
func (result *RecoveryResult) ReplayAll(target ReplayTarget) error {
	// Step 1: Replay DDL in LSN order (schema must exist before row data)
	ddlOps := result.GetAllDDLOperations()
	for _, op := range ddlOps {
		switch rec := op.(type) {
		case *CreateTableRecord:
			if err := target.ReplayCreateTable(rec.TableName, rec.Schema); err != nil {
				return fmt.Errorf("failed to replay CreateTable at LSN %d: %w", rec.Header.LSN, err)
			}
		case *DropTableRecord:
			if err := target.ReplayDropTable(rec.TableName); err != nil {
				return fmt.Errorf("failed to replay DropTable at LSN %d: %w", rec.Header.LSN, err)
			}
		case *AlterTableRecord:
			if err := target.ReplayAlterTable(rec.TableName, rec.AlterOp, rec.ColDesc); err != nil {
				return fmt.Errorf("failed to replay AlterTable at LSN %d: %w", rec.Header.LSN, err)
			}
		}
	}

	// Step 2: Replay DML in LSN order
	dmlOps := result.GetAllDMLOperations()
	for _, op := range dmlOps {
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

// GetAllDDLOperations returns all DDL operations sorted by LSN
func (result *RecoveryResult) GetAllDDLOperations() []WALRecord {
	ops := make([]WALRecord, 0, len(result.CreateTableOps)+len(result.DropTableOps)+len(result.AlterTableOps))
	for _, op := range result.CreateTableOps {
		ops = append(ops, op)
	}
	for _, op := range result.DropTableOps {
		ops = append(ops, op)
	}
	for _, op := range result.AlterTableOps {
		ops = append(ops, op)
	}
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].GetHeader().LSN < ops[j].GetHeader().LSN
	})
	return ops
}

// GetAllDMLOperations returns all DML operations sorted by LSN
func (result *RecoveryResult) GetAllDMLOperations() []WALRecord {
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
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].GetHeader().LSN < ops[j].GetHeader().LSN
	})
	return ops
}

// GetAllOperations returns all DML+DDL operations sorted by LSN (for backward compat/logging)
func (result *RecoveryResult) GetAllOperations() []WALRecord {
	ops := make([]WALRecord, 0,
		len(result.InsertOps)+len(result.UpdateOps)+len(result.DeleteOps)+
			len(result.CreateTableOps)+len(result.DropTableOps)+len(result.AlterTableOps))

	for _, op := range result.InsertOps {
		ops = append(ops, op)
	}
	for _, op := range result.UpdateOps {
		ops = append(ops, op)
	}
	for _, op := range result.DeleteOps {
		ops = append(ops, op)
	}
	for _, op := range result.CreateTableOps {
		ops = append(ops, op)
	}
	for _, op := range result.DropTableOps {
		ops = append(ops, op)
	}
	for _, op := range result.AlterTableOps {
		ops = append(ops, op)
	}

	sort.Slice(ops, func(i, j int) bool {
		return ops[i].GetHeader().LSN < ops[j].GetHeader().LSN
	})
	return ops
}

