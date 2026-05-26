package wal

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WAL represents a Write-Ahead Log for crash recovery
type WAL struct {
	file *os.File      // WAL file handle
	buf  *bufio.Writer // Buffered writer to reduce syscalls
	mu   sync.Mutex    // Protects concurrent access

	segmentDir     string // Directory containing WAL segments
	segmentPrefix  string // Prefix for segment files
	currentSegment uint64 // Current segment number
	maxSegmentSize uint64 // Max size before rotation

	walPath string // Path to CURRENT WAL file
	dbName  string // Database name this WAL belongs to

	// LSN tracking
	nextLSN        uint64 // Next LSN to assign
	flushedLSN     uint64 // Last LSN guaranteed to be fsynced to disk
	lastCheckpoint uint64 // LSN of last checkpoint

	// File position tracking
	currentOffset uint64 // Current write position in file

	// Transaction tracking
	activeTxns map[uint64]*TxnState // Currently active transactions

	// Background sync
	syncInterval time.Duration
	stopSync     chan struct{}
	syncWg       sync.WaitGroup
}

// NewWAL creates or opens a WAL in the specified directory
func NewWAL(segmentDir string, dbName string, segmentPrefix string, maxSegmentSize uint64) (*WAL, error) {
	// Ensure directory exists
	if err := os.MkdirAll(segmentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create segment dir: %w", err)
	}

	// Scan state across all existing segments
	state, err := ScanWALState(segmentDir, segmentPrefix)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to scan WAL state: %w", err)
	}

	highestSegment := uint64(1)
	if state != nil && state.LastSegment > 0 {
		highestSegment = state.LastSegment
	}

	wal := &WAL{
		segmentDir:     segmentDir,
		segmentPrefix:  segmentPrefix,
		currentSegment: highestSegment,
		maxSegmentSize: maxSegmentSize,
		dbName:         dbName,
		activeTxns:     make(map[uint64]*TxnState),
		nextLSN:        1,
		flushedLSN:     0,
	}

	if wal.maxSegmentSize == 0 {
		wal.maxSegmentSize = 16 * 1024 * 1024 // 16MB default
	}

	if state != nil {
		wal.nextLSN = state.MaxLSN + 1
		wal.flushedLSN = state.MaxLSN
		wal.lastCheckpoint = state.LastCheckpointLSN
		wal.activeTxns = state.ActiveTxns

		slog.Info("WAL state recovered",
			"nextLSN", wal.nextLSN,
			"activeTxns", len(wal.activeTxns),
			"lastCheckpoint", wal.lastCheckpoint,
			"segment", highestSegment,
			"offset", state.CurrentOffset)
	}

	// Open the current segment
	if err := wal.openSegment(highestSegment, state != nil); err != nil {
		return nil, err
	}

	// If recovering, we need to seek and truncate
	if state != nil {
		if _, err := wal.file.Seek(int64(state.CurrentOffset), io.SeekStart); err != nil {
			wal.file.Close()
			return nil, fmt.Errorf("failed to seek to recovered offset: %w", err)
		}
		if err := wal.file.Truncate(int64(state.CurrentOffset)); err != nil {
			wal.file.Close()
			return nil, fmt.Errorf("failed to truncate trailing junk: %w", err)
		}
		wal.currentOffset = state.CurrentOffset
	}

	return wal, nil
}

// openSegment opens or creates a segment file
func (w *WAL) openSegment(segmentNum uint64, existing bool) error {
	w.currentSegment = segmentNum
	w.walPath = filepath.Join(w.segmentDir, fmt.Sprintf("%s_%06d.wal", w.segmentPrefix, segmentNum))

	file, err := os.OpenFile(w.walPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open segment %d: %w", segmentNum, err)
	}

	w.file = file
	w.buf = bufio.NewWriterSize(file, WriteBufferSize)

	if !existing {
		// New file, write header
		if err := w.writeFileHeader(); err != nil {
			w.file.Close()
			return fmt.Errorf("failed to write WAL header: %w", err)
		}
	}

	return nil
}

// rotateSegment closes the current segment and creates a new one
func (w *WAL) rotateSegment() error {
	// Flush and sync current segment
	if err := w.flushAndSync(); err != nil {
		return fmt.Errorf("failed to sync before rotation: %w", err)
	}

	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close current segment: %w", err)
	}

	// Open new segment
	if err := w.openSegment(w.currentSegment+1, false); err != nil {
		return fmt.Errorf("failed to open new segment: %w", err)
	}

	slog.Info("WAL rotated segment", "newSegment", w.currentSegment, "path", w.walPath)
	return nil
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

	// Write to buffered writer
	n, err := w.buf.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if n != FileHeaderSize {
		return fmt.Errorf("incomplete header write: wrote %d of %d bytes", n, FileHeaderSize)
	}

	// Flush buffer and sync header to disk (critical for file header)
	if err := w.buf.Flush(); err != nil {
		return fmt.Errorf("failed to flush header: %w", err)
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync header: %w", err)
	}

	w.currentOffset = FileHeaderSize
	return nil
}

// Close flushes, syncs and closes the WAL file
func (w *WAL) Close() error {
	w.StopPeriodicSync()

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	// Flush buffered data
	if w.buf != nil {
		if err := w.buf.Flush(); err != nil {
			return fmt.Errorf("failed to flush buffer on close: %w", err)
		}
	}

	// Sync before closing to ensure durability
	if err := w.file.Sync(); err != nil {
		return err
	}

	err := w.file.Close()
	w.file = nil
	w.buf = nil
	return err
}

// Sync flushes the buffer and forces an fsync on the WAL file
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	// Flush buffered data first
	if w.buf != nil {
		if err := w.buf.Flush(); err != nil {
			return fmt.Errorf("failed to flush buffer: %w", err)
		}
	}

	// Fsync to disk
	if err := w.file.Sync(); err != nil {
		return err
	}

	// Update flushed LSN to current LSN - 1 (last written)
	if w.nextLSN > 1 {
		w.flushedLSN = w.nextLSN - 1
	}

	return nil
}

// flushAndSync flushes buffer and syncs to disk, updating flushedLSN
// Must be called with mutex held
func (w *WAL) flushAndSync() error {
	if w.buf != nil {
		if err := w.buf.Flush(); err != nil {
			return fmt.Errorf("failed to flush buffer: %w", err)
		}
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
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

// StartPeriodicSync starts a background goroutine that fsyncs the WAL periodically
func (w *WAL) StartPeriodicSync(interval time.Duration) {
	if interval <= 0 {
		return
	}

	w.mu.Lock()
	if w.stopSync != nil {
		w.mu.Unlock()
		return // already running
	}
	w.syncInterval = interval
	w.stopSync = make(chan struct{})
	w.syncWg.Add(1)
	w.mu.Unlock()

	go func() {
		defer w.syncWg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.stopSync:
				return
			case <-ticker.C:
				if err := w.Sync(); err != nil {
					slog.Error("WAL: periodic sync failed", "database", w.dbName, "error", err)
				}
			}
		}
	}()
}

// StopPeriodicSync stops the background sync goroutine and performs a final sync
func (w *WAL) StopPeriodicSync() {
	w.mu.Lock()
	if w.stopSync == nil {
		w.mu.Unlock()
		return
	}
	close(w.stopSync)
	w.stopSync = nil
	w.mu.Unlock()

	w.syncWg.Wait()

	// Final sync to ensure everything is flushed
	if err := w.Sync(); err != nil {
		slog.Error("WAL: final sync failed", "database", w.dbName, "error", err)
	}
}
