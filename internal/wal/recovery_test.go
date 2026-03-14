package wal

import (
	"encoding/json"
	"fmt"
	"testing"
)

// =============================================================================
// RECOVERY LOGIC TESTS
// =============================================================================

func TestTxnTracker_CommittedTransactionsOnly(t *testing.T) {
	tracker := NewTxnTracker()

	// Tx1: Committed
	tracker.ProcessRecord(&BeginTxnRecord{TxID: 1, Header: WALRecordHeader{LSN: 10}})
	tracker.ProcessRecord(&InsertRecord{TxID: 1, Header: WALRecordHeader{LSN: 11}})
	tracker.ProcessRecord(&CommitRecord{TxID: 1, Header: WALRecordHeader{LSN: 12}})

	// Tx2: Active (Uncommitted)
	tracker.ProcessRecord(&BeginTxnRecord{TxID: 2, Header: WALRecordHeader{LSN: 20}})
	tracker.ProcessRecord(&InsertRecord{TxID: 2, Header: WALRecordHeader{LSN: 21}})

	committed := tracker.GetCommittedTransactions()
	if len(committed) != 1 {
		t.Errorf("Expected 1 committed txn, got %d", len(committed))
	}
	if committed[0].TxID != 1 {
		t.Errorf("Expected txn 1, got %d", committed[0].TxID)
	}
	if len(committed[0].Inserts) != 1 {
		t.Errorf("Expected 1 insert for txn 1, got %d", len(committed[0].Inserts))
	}
}

func TestTxnTracker_IgnoresAborted(t *testing.T) {
	tracker := NewTxnTracker()

	// Tx1: Aborted
	tracker.ProcessRecord(&BeginTxnRecord{TxID: 1, Header: WALRecordHeader{LSN: 10}})
	tracker.ProcessRecord(&InsertRecord{TxID: 1, Header: WALRecordHeader{LSN: 11}})
	tracker.ProcessRecord(&AbortRecord{TxID: 1, Header: WALRecordHeader{LSN: 12}})

	committed := tracker.GetCommittedTransactions()
	if len(committed) != 0 {
		t.Errorf("Expected 0 committed txns, got %d", len(committed))
	}

	aborted := tracker.GetAbortedTransactions()
	if len(aborted) != 1 {
		t.Errorf("Expected 1 aborted txn, got %d", len(aborted))
	}
}

func TestTxnTracker_HandlesMissingBeginTxn(t *testing.T) {
	tracker := NewTxnTracker()

	// Tx1 starts before log retention (no Begin), but we see ops and Commit
	tracker.ProcessRecord(&InsertRecord{TxID: 1, Header: WALRecordHeader{LSN: 100}})
	tracker.ProcessRecord(&CommitRecord{TxID: 1, Header: WALRecordHeader{LSN: 101}})

	committed := tracker.GetCommittedTransactions()
	if len(committed) != 1 {
		t.Errorf("Expected 1 committed txn, got %d", len(committed))
	}
	if len(committed[0].Inserts) != 1 {
		t.Errorf("Expected 1 insert, got %d", len(committed[0].Inserts))
	}
}

func TestGetAllOperations_SortsByLSN(t *testing.T) {
	// Setup recovery result with mixed ops
	res := &RecoveryResult{
		InsertOps: []*InsertRecord{
			{Header: WALRecordHeader{LSN: 10}},
			{Header: WALRecordHeader{LSN: 30}},
		},
		UpdateOps: []*UpdateRecord{
			{Header: WALRecordHeader{LSN: 20}},
		},
	}

	ops := res.GetAllOperations()
	if len(ops) != 3 {
		t.Fatalf("Expected 3 ops, got %d", len(ops))
	}

	if ops[0].GetHeader().LSN != 10 {
		t.Error("Op 0 should be LSN 10")
	}
	if ops[1].GetHeader().LSN != 20 {
		t.Error("Op 1 should be LSN 20")
	}
	if ops[2].GetHeader().LSN != 30 {
		t.Error("Op 2 should be LSN 30")
	}
}

// Mock Replay Target
type mockTarget struct {
	inserts []string
	ops     []string
}

func (m *mockTarget) ReplayInsert(table, key string, val json.RawMessage) error {
	m.inserts = append(m.inserts, key)
	return nil
}

func (m *mockTarget) ReplayUpdate(table, key string, val json.RawMessage) error { return nil }
func (m *mockTarget) ReplayDelete(table, key string) error { return nil }

func (m *mockTarget) ReplayCreateTable(name string, schemaBytes []byte) error {
	m.ops = append(m.ops, fmt.Sprintf("CREATE TABLE %s", name))
	return nil
}

func (m *mockTarget) ReplayDropTable(name string) error {
	m.ops = append(m.ops, fmt.Sprintf("DROP TABLE %s", name))
	return nil
}

func (m *mockTarget) ReplayAlterTable(name string, op uint8, colDesc []byte) error {
	m.ops = append(m.ops, fmt.Sprintf("ALTER TABLE %s", name))
	return nil
}

func TestReplayAll_UsesCorrectOrder(t *testing.T) {
	res := &RecoveryResult{
		InsertOps: []*InsertRecord{
			{Key: "K1", Header: WALRecordHeader{LSN: 10}},
			{Key: "K2", Header: WALRecordHeader{LSN: 20}},
		},
	}

	target := &mockTarget{}
	res.ReplayAll(target)

	if target.inserts[0] != "K1" || target.inserts[1] != "K2" {
		t.Error("Replay order incorrect")
	}
}
