package joydb

import (
	"context"
	"sync"

	internalengine "github.com/leengari/joydb/internal/engine"
)

// DB is a concurrency-safe handle to one database.
type DB struct {
	name   string
	mu     sync.RWMutex
	engine *internalengine.Engine
}

func (db *DB) Name() string { return db.name }

func (db *DB) Exec(query string, args ...interface{}) (ExecResult, error) {
	return db.ExecContext(context.Background(), query, args...)
}

func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (ExecResult, error) {
	if err := ctx.Err(); err != nil {
		return ExecResult{}, err
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	result, err := db.engine.ExecuteContext(ctx, query, args...)
	if err != nil {
		return ExecResult{}, mapError(err)
	}
	return ExecResult{rowsAffected: int64(result.RowsAffected), lastInsertID: result.LastInsertID}, nil
}

func (db *DB) Query(query string, args ...interface{}) (*Rows, error) {
	return db.QueryContext(context.Background(), query, args...)
}

func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db.mu.RLock()
	defer db.mu.RUnlock()
	result, err := db.engine.ExecuteContext(ctx, query, args...)
	if err != nil {
		return nil, mapError(err)
	}
	return newRows(result), nil
}
