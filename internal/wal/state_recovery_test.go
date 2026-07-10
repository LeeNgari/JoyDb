package wal

import (
	"testing"
)

func TestScanWALState_EmptyWAL(t *testing.T) {
	tmpDir := t.TempDir()

	// Create new empty WAL (just header)
	w, err := NewWAL(tmpDir, "db", "wal", 16*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	state, err := ScanWALState(tmpDir, "wal")
	if err != nil {
		t.Fatalf("ScanWALState failed: %v", err)
	}

	if state.MaxLSN != 0 {
		t.Errorf("Expected MaxLSN 0, got %d", state.MaxLSN)
	}
	if len(state.ActiveTxns) != 0 {
		t.Errorf("Expected 0 active txns, got %d", len(state.ActiveTxns))
	}
	if state.CurrentOffset != FileHeaderSize {
		t.Errorf("Expected offset %d, got %d", FileHeaderSize, state.CurrentOffset)
	}
}

func TestScanWALState_WithRecords(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := NewWAL(tmpDir, "db", "wal", 16*1024*1024)
	if err != nil {
		t.Fatalf("failed to create WAL: %v", err)
	}
	w.BeginTransaction(1) // LSN 1
	w.Commit(1)           // LSN 2
	w.BeginTransaction(2) // LSN 3
	w.LogInsert(2, "t", "k", []byte("{}")) // LSN 4
	w.Close()

	state, err := ScanWALState(tmpDir, "wal")
	if err != nil {
		t.Fatalf("ScanWALState failed: %v", err)
	}

	if state.MaxLSN != 4 {
		t.Errorf("Expected MaxLSN 4, got %d", state.MaxLSN)
	}
	if len(state.ActiveTxns) != 1 {
		t.Errorf("Expected 1 active txn, got %d", len(state.ActiveTxns))
	}
	if _, ok := state.ActiveTxns[2]; !ok {
		t.Error("Txn 2 should be active")
	}
	if _, ok := state.ActiveTxns[1]; ok {
		t.Error("Txn 1 should be gone")
	}
}

func TestScanWALState_RecoverLSN(t *testing.T) {
	tmpDir := t.TempDir()

	w, _ := NewWAL(tmpDir, "db", "wal", 16*1024*1024)
	w.BeginTransaction(1) // LSN 1
	w.Close()

	// Reopen should pick up LSN
	// But ScanWALState is tested here independently
	state, err := ScanWALState(tmpDir, "wal")
	if err != nil {
		t.Fatal(err)
	}
	if state.MaxLSN != 1 {
		t.Errorf("Expected MaxLSN 1, got %d", state.MaxLSN)
	}
}
