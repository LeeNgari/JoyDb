package wal

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// WAL LIFECYCLE TESTS
// =============================================================================

func TestAllocateLSN_Monotonic(t *testing.T) {
	tmpDir := t.TempDir()
	w, _ := NewWAL(tmpDir, "db", "wal", 16*1024*1024)
	defer w.Close()

	lsn1 := w.allocateLSN()
	lsn2 := w.allocateLSN()

	if lsn2 <= lsn1 {
		t.Errorf("LSN not monotonic: %d -> %d", lsn1, lsn2)
	}
}

func TestWAL_CreateNewFile(t *testing.T) {
	dir, _ := os.MkdirTemp("", "wal_test")
	defer os.RemoveAll(dir)

	w, err := NewWAL(dir, "testdb", "wal", 16*1024*1024)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	w.Close()

	segmentPath := filepath.Join(dir, "wal_000001.wal")

	// Check file exists
	info, err := os.Stat(segmentPath)
	if err != nil {
		t.Fatal("WAL segment file not created")
	}
	if info.Size() != FileHeaderSize {
		t.Errorf("Expected header size %d, got %d", FileHeaderSize, info.Size())
	}
}

func TestWAL_SyncFlushesBuffer(t *testing.T) {
	tmpDir := createTempWAL(t)
	defer removeTempWAL(t, tmpDir)

	w, _ := NewWAL(tmpDir, "db", "wal", 16*1024*1024)

	// Write log but don't commit (so it's in buffer)
	// We force use of internal buffer writing by calling LogInsert
	w.BeginTransaction(1)
	w.LogInsert(1, "t", "k", nil)

	segmentPath := filepath.Join(tmpDir, "wal_000001.wal")

	// Check file size on disk (should be mostly header, might not have payload yet if buffered)
	info1, _ := os.Stat(segmentPath)

	w.Sync() // Flush

	info2, _ := os.Stat(segmentPath)

	if info2.Size() <= info1.Size() {
		t.Errorf("Sync did not increase file size on disk: %d -> %d", info1.Size(), info2.Size())
	}
	w.Close()
}
