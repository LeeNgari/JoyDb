package joydb

// ExecResult describes the outcome of a mutating statement.
type ExecResult struct {
	rowsAffected int64
	lastInsertID int64
}

func (r ExecResult) RowsAffected() int64 { return r.rowsAffected }
func (r ExecResult) LastInsertID() int64 { return r.lastInsertID }
