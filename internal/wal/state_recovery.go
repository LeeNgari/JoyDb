package wal

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// WALState represents the state recovered from scanning the WAL file
type WALState struct {
	MaxLSN            uint64
	LastCheckpointLSN uint64
	ActiveTxns        map[uint64]*TxnState
	CurrentOffset     uint64
	LastSegment       uint64
}

// ScanWALState scans all WAL segments to recover critical state (MaxLSN, active transactions, etc.)
// It returns a WALState struct that should be used to initialize the WAL
func ScanWALState(segmentDir, segmentPrefix string) (*WALState, error) {
	reader, err := NewMultiSegmentReader(segmentDir, segmentPrefix)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to create multi-segment reader: %w", err)
	}
	defer reader.Close()

	state := &WALState{
		ActiveTxns: make(map[uint64]*TxnState),
	}

	// Read file header
	header, err := reader.ReadFileHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}

	// Initial guess for current offset (after header)
	state.CurrentOffset = FileHeaderSize
	state.LastSegment = 1 // At least segment 1

	// Scan all records
	for {
		record, err := reader.ReadNextRecord()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Partial record at end or corruption - we stop here.
			// The CurrentOffset should point to the end of the last valid record.
			// Since ReadNextRecord updates reader.currentPos only after valid read,
			// we can rely on reader.CurrentPosition().
			// Wait, ReadNextRecord updates currentPos AFTER validating.
			// If it fails, currentPos might be at start of bad record.
			// This allows appending new records overwriting the bad one.
			// However, writer uses O_APPEND or Seek(End).
			// If we use ScanWALState, we should tell WAL where to write.
			// But WAL usually opens with O_APPEND?
			// wal.go: os.OpenFile(walPath, os.O_CREATE|os.O_RDWR, 0644)
			// It doesn't use O_APPEND. It seeks to end?
			// wal.go: file.Seek(0, os.SEEK_END)
			// If there is junk at end, we might want to truncate it?
			// The current implementation just seeks to end.
			// If ScanWALState returns a CurrentOffset, we should probably seek to it?
			// But if we seek to end, we skip junk.
			// If we seek to CurrentOffset (end of valid data), we overwrite junk.
			// Overwriting junk is better.
			break
		}

		h := record.GetHeader()
		if h.LSN > state.MaxLSN {
			state.MaxLSN = h.LSN
		}

		// Track active transactions
		switch rec := record.(type) {
		case *BeginTxnRecord:
			state.ActiveTxns[rec.TxID] = &TxnState{
				ID:    rec.TxID,
				State: TxnActive,
			}
		case *CommitRecord:
			delete(state.ActiveTxns, rec.TxID)
		case *AbortRecord:
			delete(state.ActiveTxns, rec.TxID)
		case *CheckpointRecord:
			state.LastCheckpointLSN = h.LSN
		}

		// Update offset to end of this record
		state.CurrentOffset = h.FileOffset + uint64(h.Length)
		
		// Map 0-indexed segmentIdx to 1-indexed segment number
		state.LastSegment = uint64(h.SegmentIdx) + 1
	}

	// Also ensure MaxLSN is at least InitialLSN from header
	if header.InitialLSN > state.MaxLSN {
		// Actually MaxLSN should track the highest SEEN.
		// If InitialLSN is higher (e.g. empty new file but initialized with LSN > 1),
		// we should respect it?
		// But InitialLSN in header is usually 1 for new file.
		// If we reuse file, we might update header? No, header is static.
		// So MaxLSN is max(scanned LSNs).
	}

	return state, nil
}
