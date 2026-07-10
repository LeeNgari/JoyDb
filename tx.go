package joydb

import (
	"context"
	"fmt"
	"sync"

	"github.com/leengari/joydb/internal/domain/data"
	"github.com/leengari/joydb/internal/domain/schema"
	"github.com/leengari/joydb/internal/domain/transaction"
	"github.com/leengari/joydb/internal/index/btree"
	"github.com/leengari/joydb/internal/query/indexing"
)

// Tx is a serialized, multi-statement transaction.
type Tx struct {
	mu       sync.Mutex
	db       *DB
	internal *transaction.Transaction
	snapshot map[string]*schema.Table
	done     bool
}

// BeginTx starts a transaction and exclusively owns the DB until Commit or Rollback.
func (db *DB) BeginTx(ctx context.Context) (*Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db.mu.Lock()
	if err := ctx.Err(); err != nil {
		db.mu.Unlock()
		return nil, err
	}
	snapshot, err := cloneTables(db.engine.Database())
	if err != nil {
		db.mu.Unlock()
		return nil, err
	}
	internal := transaction.NewTransaction()
	if err := db.engine.BeginTransaction(internal); err != nil {
		internal.Close()
		db.mu.Unlock()
		return nil, mapError(err)
	}
	return &Tx{db: db, internal: internal, snapshot: snapshot}, nil
}

func (tx *Tx) Exec(query string, args ...interface{}) (ExecResult, error) {
	return tx.ExecContext(context.Background(), query, args...)
}

func (tx *Tx) ExecContext(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.done {
		return ExecResult{}, ErrTxDone
	}
	result, err := tx.db.engine.ExecuteTxContext(ctx, tx.internal, query, args...)
	if err != nil {
		return ExecResult{}, mapError(err)
	}
	return ExecResult{rowsAffected: int64(result.RowsAffected), lastInsertID: result.LastInsertID}, nil
}

func (tx *Tx) Query(query string, args ...interface{}) (*Rows, error) {
	return tx.QueryContext(context.Background(), query, args...)
}

func (tx *Tx) QueryContext(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.done {
		return nil, ErrTxDone
	}
	result, err := tx.db.engine.ExecuteTxContext(ctx, tx.internal, query, args...)
	if err != nil {
		return nil, mapError(err)
	}
	return newRows(result), nil
}

func (tx *Tx) Commit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.done {
		return ErrTxDone
	}
	if err := tx.db.engine.CommitTransaction(tx.internal); err != nil {
		tx.restore()
		_ = tx.db.engine.AbortTransaction(tx.internal)
		tx.finish()
		return mapError(err)
	}
	tx.finish()
	return nil
}

func (tx *Tx) Rollback() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.done {
		return ErrTxDone
	}
	tx.restore()
	err := tx.db.engine.AbortTransaction(tx.internal)
	tx.finish()
	return mapError(err)
}

func (tx *Tx) restore() {
	database := tx.db.engine.Database()
	database.Lock()
	database.Tables = tx.snapshot
	database.Unlock()
}

func (tx *Tx) finish() {
	tx.done = true
	tx.internal.Close()
	tx.snapshot = nil
	tx.db.mu.Unlock()
}

func cloneTables(database *schema.Database) (map[string]*schema.Table, error) {
	database.RLock()
	defer database.RUnlock()
	tables := make(map[string]*schema.Table, len(database.Tables))
	for name, table := range database.Tables {
		table.RLock()
		cloned := &schema.Table{
			Name: table.Name, Path: table.Path, NextRID: table.NextRID,
			TombstoneCount: table.TombstoneCount, LastInsertID: table.LastInsertID,
			Dirty: table.Dirty, RowsByRID: make(map[int64]data.Row, len(table.RowsByRID)),
			Schema: &schema.TableSchema{
				TableName:   table.Schema.TableName,
				Columns:     append([]schema.Column(nil), table.Schema.Columns...),
				ForeignKeys: append([]schema.ForeignKey(nil), table.Schema.ForeignKeys...),
			},
		}
		for rid, row := range table.RowsByRID {
			cloned.RowsByRID[rid] = row.Copy()
		}
		table.RUnlock()

		if err := indexing.BuildIndexes(cloned); err != nil {
			return nil, fmt.Errorf("clone table %s: %w", name, err)
		}
		if primaryKey := cloned.Schema.GetPrimaryKeyColumn(); primaryKey != nil {
			cloned.PKIndex = btree.New(btree.DefaultDegree)
			columnIndex := cloned.Schema.GetColumnIndex(primaryKey.Name)
			for rid, row := range cloned.RowsByRID {
				if row.Deleted || columnIndex < 0 || columnIndex >= len(row.Values) {
					continue
				}
				if err := cloned.PKIndex.Insert(row.Values[columnIndex], rid); err != nil {
					return nil, fmt.Errorf("clone primary index %s: %w", name, err)
				}
			}
		}
		tables[name] = cloned
	}
	return tables, nil
}
