package manager

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/index/btree"
	"github.com/leengari/mini-rdbms/internal/storage/engine"
	"github.com/leengari/mini-rdbms/internal/wal"
)

// WALManager bridges the WAL package with the storage layer
// It handles transaction lifecycle and logging operations
type WALManager struct {
	wal          *wal.WAL
	checkpointer *wal.AsyncCheckpointer
	db           *schema.Database
	dbPath       string
	dbName       string
	enabled      bool
	engine       engine.StorageEngine
}

// NewWALManager creates a new WAL manager for a database
// If enabled is false, all operations become no-ops
func NewWALManager(
	db *schema.Database,
	dbPath, dbName string,
	enabled bool,
	checkpointInterval time.Duration,
	saveFunc func() error,
	storageEngine engine.StorageEngine,
) (*WALManager, error) {
	if !enabled {
		return &WALManager{
			db:      db,
			dbPath:  dbPath,
			dbName:  dbName,
			enabled: false,
			engine:  storageEngine,
		}, nil
	}

	walPath := filepath.Join(dbPath, dbName+".wal")
	w, err := wal.NewWAL(walPath, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL: %w", err)
	}

	wm := &WALManager{
		wal:     w,
		db:      db,
		dbPath:  dbPath,
		dbName:  dbName,
		enabled: true,
		engine:  storageEngine,
	}

	// Setup async checkpointer
	// The closure captures 'wm' and 'db' (via wm.db or directly)
	checkpointAction := func() error {
		// 1. Save database (flush to disk) if saveFunc provided
		if saveFunc != nil {
			if err := saveFunc(); err != nil {
				slog.Error("AsyncCheckpoint: failed to save database", "db", dbName, "error", err)
				return err
			}
		}

		// 2. Write checkpoint to WAL
		// We use the db instance stored in WALManager
		if err := wm.WriteCheckpoint(wm.db); err != nil {
			slog.Error("AsyncCheckpoint: failed to write checkpoint", "db", dbName, "error", err)
			return err
		}

		return nil
	}

	wm.checkpointer = wal.NewAsyncCheckpointer(checkpointInterval, checkpointAction)
	wm.checkpointer.Start()

	slog.Info("WAL initialized", "database", dbName, "path", walPath, "checkpoint_interval", checkpointInterval)

	return wm, nil
}

// IsEnabled returns whether WAL is enabled
func (m *WALManager) IsEnabled() bool {
	return m.enabled && m.wal != nil
}

// AsyncCheckpoint triggers an asynchronous checkpoint and returns a channel that will receive any error
func (m *WALManager) AsyncCheckpoint() <-chan error {
	if !m.IsEnabled() || m.checkpointer == nil {
		ch := make(chan error, 1)
		ch <- nil
		close(ch)
		return ch
	}
	return m.checkpointer.RequestCheckpoint()
}

// BeginTransaction logs a transaction begin to WAL
func (m *WALManager) BeginTransaction(tx *transaction.Transaction) error {
	if !m.IsEnabled() {
		return nil
	}

	lsn, err := m.wal.BeginTransaction(tx.TxID)
	if err != nil {
		return fmt.Errorf("WAL BeginTransaction failed: %w", err)
	}

	slog.Debug("WAL: BeginTransaction", "txID", tx.TxID, "lsn", lsn)
	return nil
}

// LogInsert logs an insert operation to WAL
func (m *WALManager) LogInsert(tx *transaction.Transaction, table *schema.Table, row data.Row) error {
	if !m.IsEnabled() {
		return nil
	}

	// Extract primary key
	key, err := table.GetPrimaryKeyValue(row)
	if err != nil {
		return fmt.Errorf("failed to get primary key for WAL: %w", err)
	}

	// Serialize row to []byte
	value, err := row.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize row for WAL: %w", err)
	}

	lsn, err := m.wal.LogInsert(tx.TxID, table.Name, key, value)
	if err != nil {
		return fmt.Errorf("WAL LogInsert failed: %w", err)
	}

	slog.Debug("WAL: LogInsert", "txID", tx.TxID, "table", table.Name, "key", key, "lsn", lsn)
	return nil
}

// LogUpdate logs an update operation to WAL
func (m *WALManager) LogUpdate(tx *transaction.Transaction, table *schema.Table, key string, oldRow, newRow data.Row) error {
	if !m.IsEnabled() {
		return nil
	}

	oldValue, err := oldRow.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize old row for WAL: %w", err)
	}

	newValue, err := newRow.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize new row for WAL: %w", err)
	}

	lsn, err := m.wal.LogUpdate(tx.TxID, table.Name, key, oldValue, newValue)
	if err != nil {
		return fmt.Errorf("WAL LogUpdate failed: %w", err)
	}

	slog.Debug("WAL: LogUpdate", "txID", tx.TxID, "table", table.Name, "key", key, "lsn", lsn)
	return nil
}

// LogDelete logs a delete operation to WAL
func (m *WALManager) LogDelete(tx *transaction.Transaction, table *schema.Table, key string, oldRow data.Row) error {
	if !m.IsEnabled() {
		return nil
	}

	oldValue, err := oldRow.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize old row for WAL: %w", err)
	}

	lsn, err := m.wal.LogDelete(tx.TxID, table.Name, key, oldValue)
	if err != nil {
		return fmt.Errorf("WAL LogDelete failed: %w", err)
	}

	slog.Debug("WAL: LogDelete", "txID", tx.TxID, "table", table.Name, "key", key, "lsn", lsn)
	return nil
}

// LogCreateTable logs a CREATE TABLE operation to WAL
func (m *WALManager) LogCreateTable(tx *transaction.Transaction, tableName string, tableSchema *schema.TableSchema) error {
	if !m.IsEnabled() {
		return nil
	}

	schemaBytes := wal.EncodeTableSchema(tableSchema)
	lsn, err := m.wal.LogCreateTable(tx.TxID, tableName, schemaBytes)
	if err != nil {
		return fmt.Errorf("WAL LogCreateTable failed: %w", err)
	}

	slog.Debug("WAL: LogCreateTable", "txID", tx.TxID, "table", tableName, "lsn", lsn)
	return nil
}

// LogDropTable logs a DROP TABLE operation to WAL
func (m *WALManager) LogDropTable(tx *transaction.Transaction, tableName string) error {
	if !m.IsEnabled() {
		return nil
	}

	lsn, err := m.wal.LogDropTable(tx.TxID, tableName)
	if err != nil {
		return fmt.Errorf("WAL LogDropTable failed: %w", err)
	}

	slog.Debug("WAL: LogDropTable", "txID", tx.TxID, "table", tableName, "lsn", lsn)
	return nil
}

// LogAlterTable logs an ALTER TABLE operation to WAL
func (m *WALManager) LogAlterTable(tx *transaction.Transaction, tableName string, alterOp uint8, col schema.Column) error {
	if !m.IsEnabled() {
		return nil
	}

	colDesc := wal.EncodeColumn(col)
	lsn, err := m.wal.LogAlterTable(tx.TxID, tableName, alterOp, colDesc)
	if err != nil {
		return fmt.Errorf("WAL LogAlterTable failed: %w", err)
	}

	slog.Debug("WAL: LogAlterTable", "txID", tx.TxID, "table", tableName, "op", alterOp, "lsn", lsn)
	return nil
}

// Commit commits a transaction and fsyncs the WAL
func (m *WALManager) Commit(tx *transaction.Transaction) error {
	if !m.IsEnabled() {
		return nil
	}

	lsn, err := m.wal.Commit(tx.TxID)
	if err != nil {
		return fmt.Errorf("WAL Commit failed: %w", err)
	}

	slog.Debug("WAL: Commit", "txID", tx.TxID, "lsn", lsn)
	return nil
}

// Abort aborts a transaction
func (m *WALManager) Abort(tx *transaction.Transaction) error {
	if !m.IsEnabled() {
		return nil
	}

	lsn, err := m.wal.Abort(tx.TxID)
	if err != nil {
		return fmt.Errorf("WAL Abort failed: %w", err)
	}

	slog.Debug("WAL: Abort", "txID", tx.TxID, "lsn", lsn)
	return nil
}

// WriteCheckpoint writes a checkpoint record to WAL
// This should be called after successfully persisting all tables to JSON
func (m *WALManager) WriteCheckpoint(db *schema.Database) error {
	if !m.IsEnabled() {
		return nil
	}

	snapshotLSN, snapshotCRC, err := m.createAndRecordSnapshot(db)
	if err != nil {
		return err
	}

	lsn, err := m.wal.WriteCheckpoint(snapshotLSN, snapshotCRC)
	if err != nil {
		return fmt.Errorf("WAL WriteCheckpoint failed: %w", err)
	}

	slog.Info("WAL: Checkpoint written", "database", m.dbName, "snapshot_lsn", snapshotLSN, "lsn", lsn)
	return nil
}

// createAndRecordSnapshot creates a snapshot using the storage engine
func (m *WALManager) createAndRecordSnapshot(db *schema.Database) (uint64, uint32, error) {
	if m.engine == nil {
		return 0, 0, fmt.Errorf("StorageEngine is not configured in WALManager")
	}

	return m.engine.CreateSnapshot(db, m.dbPath)
}

// Recover performs WAL recovery and returns operations to replay
func (m *WALManager) Recover() (*wal.RecoveryResult, error) {
	return m.RecoverWithProgress(nil)
}

// RecoverWithProgress performs WAL recovery with a progress callback
func (m *WALManager) RecoverWithProgress(callback wal.ProgressCallback) (*wal.RecoveryResult, error) {
	if !m.IsEnabled() {
		return nil, nil
	}

	walPath := filepath.Join(m.dbPath, m.dbName+".wal")
	recoveryMgr, err := wal.NewRecoveryManager(walPath, m.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create recovery manager: %w", err)
	}
	defer recoveryMgr.Close()

	result, err := recoveryMgr.RecoverWithProgress(callback)
	if err != nil {
		return nil, fmt.Errorf("WAL recovery failed: %w", err)
	}

	slog.Info("WAL: Recovery complete",
		"database", m.dbName,
		"records_scanned", result.RecordsScanned,
		"txns_replayed", result.TransactionsReplay,
		"txns_skipped", result.TransactionsSkipped,
	)

	return result, nil
}

// Sync forces an fsync on the WAL file
func (m *WALManager) Sync() error {
	if !m.IsEnabled() {
		return nil
	}
	return m.wal.Sync()
}

// Close closes the WAL file and stops the checkpointer
func (m *WALManager) Close() error {
	if !m.IsEnabled() {
		return nil
	}

	if m.checkpointer != nil {
		m.checkpointer.Stop()
	}

	slog.Info("WAL: Closing", "database", m.dbName)
	return m.wal.Close()
}

// DatabaseReplayTarget implements wal.ReplayTarget for replaying operations
type DatabaseReplayTarget struct {
	db *schema.Database
}

// NewDatabaseReplayTarget creates a replay target for a database
func NewDatabaseReplayTarget(db *schema.Database) *DatabaseReplayTarget {
	return &DatabaseReplayTarget{db: db}
}

// ReplayInsert applies an insert operation during recovery
func (t *DatabaseReplayTarget) ReplayInsert(tableName, key string, value []byte) error {
	table, ok := t.db.Tables[tableName]
	if !ok {
		slog.Warn("Replay: table not found, skipping insert", "table", tableName)
		return nil
	}

	row, err := data.Deserialize(value)
	if err != nil {
		return fmt.Errorf("failed to deserialize row: %w", err)
	}

	// Use a nil transaction since we're replaying
	// Insert directly to rows without full validation (data already validated when originally inserted)
	table.Lock()
	defer table.Unlock()

	// Use InsertReplay for lock-free insert that also maintains B+Tree PKIndex
	table.InsertReplay(row)
	table.MarkDirtyUnsafe()

	slog.Debug("Replay: Insert", "table", tableName, "key", key)
	return nil
}

// ReplayUpdate applies an update operation during recovery
func (t *DatabaseReplayTarget) ReplayUpdate(tableName, key string, newValue []byte) error {
	table, ok := t.db.Tables[tableName]
	if !ok {
		slog.Warn("Replay: table not found, skipping update", "table", tableName)
		return nil
	}

	newRow, err := data.Deserialize(newValue)
	if err != nil {
		return fmt.Errorf("failed to deserialize row: %w", err)
	}

	table.Lock()
	defer table.Unlock()

	// Find the row by primary key and update it
	pkCol := table.Schema.GetPrimaryKeyColumn()
	if pkCol == nil {
		return fmt.Errorf("table %s has no primary key", tableName)
	}

	for i, row := range table.Rows {
		if pkVal, exists := row.Data[pkCol.Name]; exists {
			pkStr := fmt.Sprintf("%v", pkVal)
			if pkStr == key {
				table.Rows[i] = newRow
				table.MarkDirtyUnsafe()
				slog.Debug("Replay: Update", "table", tableName, "key", key)
				return nil
			}
		}
	}

	slog.Warn("Replay: row not found for update", "table", tableName, "key", key)
	return nil
}

// ReplayDelete applies a delete operation during recovery
func (t *DatabaseReplayTarget) ReplayDelete(tableName, key string) error {
	table, ok := t.db.Tables[tableName]
	if !ok {
		slog.Warn("Replay: table not found, skipping delete", "table", tableName)
		return nil
	}

	table.Lock()
	defer table.Unlock()

	pkCol := table.Schema.GetPrimaryKeyColumn()
	if pkCol == nil {
		return fmt.Errorf("table %s has no primary key", tableName)
	}

	// Find and remove the row
	for i, row := range table.Rows {
		if pkVal, exists := row.Data[pkCol.Name]; exists {
			pkStr := fmt.Sprintf("%v", pkVal)
			if pkStr == key {
				table.Rows = append(table.Rows[:i], table.Rows[i+1:]...)
				table.MarkDirtyUnsafe()
				slog.Debug("Replay: Delete", "table", tableName, "key", key)
				return nil
			}
		}
	}

	slog.Warn("Replay: row not found for delete", "table", tableName, "key", key)
	return nil
}

// ReplayCreateTable applies a CREATE TABLE operation during recovery
func (t *DatabaseReplayTarget) ReplayCreateTable(name string, schemaBytes []byte) error {
	s, err := wal.DecodeTableSchema(schemaBytes)
	if err != nil {
		return fmt.Errorf("failed to decode table schema during replay: %w", err)
	}

	// Create table directory (needed for JSON engine compatibility)
	tablePath := filepath.Join(t.db.Path, name)
	if err := os.MkdirAll(tablePath, 0755); err != nil {
		return fmt.Errorf("failed to create table directory during replay: %w", err)
	}

	// Create new table instance
	table := &schema.Table{
		Name:    name,
		Path:    tablePath,
		Schema:  s,
		Rows:    []data.Row{},
		Indexes: make(map[string]*data.Index),
	}

	// Initialize PKIndex if table has a primary key
	if pkCol := s.GetPrimaryKeyColumn(); pkCol != nil {
		table.PKIndex = btree.New(btree.DefaultDegree)
	}

	t.db.Lock()
	defer t.db.Unlock()

	// Add to database
	if t.db.Tables == nil {
		t.db.Tables = make(map[string]*schema.Table)
	}
	t.db.Tables[name] = table

	slog.Debug("Replay: CreateTable", "table", name)
	return nil
}

// ReplayDropTable applies a DROP TABLE operation during recovery
func (t *DatabaseReplayTarget) ReplayDropTable(name string) error {
	t.db.Lock()
	defer t.db.Unlock()

	if t.db.Tables != nil {
		delete(t.db.Tables, name)
	}

	slog.Debug("Replay: DropTable", "table", name)
	return nil
}

// ReplayAlterTable applies an ALTER TABLE operation during recovery
func (t *DatabaseReplayTarget) ReplayAlterTable(name string, op uint8, colDesc []byte) error {
	t.db.Lock()
	defer t.db.Unlock()

	table, ok := t.db.Tables[name]
	if !ok {
		slog.Warn("Replay: table not found for ALTER TABLE", "table", name)
		return nil
	}

	table.Lock()
	defer table.Unlock()

	if op == wal.AlterOpAddColumn {
		col, _, err := wal.DecodeColumn(colDesc, 0)
		if err != nil {
			return fmt.Errorf("failed to decode column for ADD_COLUMN: %w", err)
		}
		table.Schema.Columns = append(table.Schema.Columns, col)
		table.MarkDirtyUnsafe()
		slog.Debug("Replay: AlterTable ADD_COLUMN", "table", name, "column", col.Name)
	} else if op == wal.AlterOpDropColumn {
		col, _, err := wal.DecodeColumn(colDesc, 0)
		if err != nil {
			return fmt.Errorf("failed to decode column for DROP_COLUMN: %w", err)
		}
		
		newCols := make([]schema.Column, 0, len(table.Schema.Columns))
		for _, c := range table.Schema.Columns {
			if c.Name != col.Name {
				newCols = append(newCols, c)
			}
		}
		table.Schema.Columns = newCols
		
		for i := range table.Rows {
			delete(table.Rows[i].Data, col.Name)
		}
		
		table.MarkDirtyUnsafe()
		slog.Debug("Replay: AlterTable DROP_COLUMN", "table", name, "column", col.Name)
	} else {
		slog.Warn("Replay: unsupported AlterOp", "table", name, "op", op)
	}

	return nil
}
