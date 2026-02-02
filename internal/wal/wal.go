package wal

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// WAL represents a Write-Ahead Log for crash recovery
type WAL struct {
	file    *os.File   // WAL file handle
	mu      sync.Mutex // Protects concurrent access
	walPath string     // Path to WAL file
	dbName  string     // Database name this WAL belongs to

	// LSN tracking
	nextLSN        uint64 // Next LSN to assign
	flushedLSN     uint64 // Last LSN guaranteed to be fsynced to disk
	lastCheckpoint uint64 // LSN of last checkpoint

	// File position tracking
	currentOffset uint64 // Current write position in file

	// Transaction tracking
	activeTxns map[uint64]*TxnState // Currently active transactions
}

// NewWAL creates or opens a WAL at the specified path
func NewWAL(walPath string, dbName string) (*WAL, error) {
	// Check if WAL file exists
	fileExists := false
	if _, err := os.Stat(walPath); err == nil {
		fileExists = true
	}

	// Open file with read-write mode, create if not exists
	file, err := os.OpenFile(walPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	wal := &WAL{
		file:       file,
		walPath:    walPath,
		dbName:     dbName,
		activeTxns: make(map[uint64]*TxnState),
		nextLSN:    1, // LSN starts at 1
		flushedLSN: 0, // Nothing flushed yet
	}

	if fileExists {
		// TODO: Scan existing WAL to recover state (nextLSN, flushedLSN, activeTxns, currentOffset)
		// For now, seek to end
		offset, err := file.Seek(0, os.SEEK_END)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to seek to end of WAL: %w", err)
		}
		wal.currentOffset = uint64(offset)
	} else {
		// Write file header for new WAL
		if err := wal.writeFileHeader(); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to write WAL header: %w", err)
		}
	}

	return wal, nil
}

// writeFileHeader writes the WAL file header
func (w *WAL) writeFileHeader() error {
	header := WALFileHeader{
		Magic:      WALMagic,
		Version:    WALVersion,
		InitialLSN: w.nextLSN,
		CreatedAt:  time.Now().Unix(),
	}

	// Copy database name (truncate if too long)
	copy(header.DatabaseName[:], w.dbName)

	// Encode header
	buf := make([]byte, FileHeaderSize)

	// Magic (8 bytes)
	copy(buf[0:8], header.Magic[:])

	// Version (2 bytes)
	ByteOrder.PutUint16(buf[8:10], header.Version)

	// DatabaseName (32 bytes)
	copy(buf[10:42], header.DatabaseName[:])

	// InitialLSN (8 bytes)
	ByteOrder.PutUint64(buf[42:50], header.InitialLSN)

	// CreatedAt (8 bytes)
	ByteOrder.PutUint64(buf[50:58], uint64(header.CreatedAt))

	// Reserved padding (6 bytes) - already zeroed

	// Write to file
	n, err := w.file.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if n != FileHeaderSize {
		return fmt.Errorf("incomplete header write: wrote %d of %d bytes", n, FileHeaderSize)
	}

	// Sync header to disk
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync header: %w", err)
	}

	w.currentOffset = FileHeaderSize
	return nil
}

// Close syncs and closes the WAL file
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	// Sync before closing to ensure durability
	if err := w.file.Sync(); err != nil {
		return err
	}

	err := w.file.Close()
	w.file = nil
	return err
}

// Sync forces an fsync on the WAL file and updates flushedLSN
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	if err := w.file.Sync(); err != nil {
		return err
	}

	// Update flushed LSN to current LSN - 1 (last written)
	if w.nextLSN > 1 {
		w.flushedLSN = w.nextLSN - 1
	}

	return nil
}

// Path returns the WAL file path
func (w *WAL) Path() string {
	return w.walPath
}

// DatabaseName returns the database name this WAL belongs to
func (w *WAL) DatabaseName() string {
	return w.dbName
}

// NextLSN returns the next LSN that will be assigned (thread-safe)
func (w *WAL) NextLSN() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.nextLSN
}

// FlushedLSN returns the last LSN guaranteed to be fsynced (thread-safe)
func (w *WAL) FlushedLSN() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushedLSN
}

// LastCheckpointLSN returns the LSN of the last checkpoint (thread-safe)
func (w *WAL) LastCheckpointLSN() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastCheckpoint
}

// CurrentOffset returns the current write position in the WAL file (thread-safe)
func (w *WAL) CurrentOffset() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentOffset
}

// allocateLSN allocates and returns the next LSN
// Must be called with mutex held
func (w *WAL) allocateLSN() uint64 {
	lsn := w.nextLSN
	w.nextLSN++
	return lsn
}
