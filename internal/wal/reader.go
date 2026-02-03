package wal

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

// ===========================================================================
// WAL READER OPERATIONS
// ===========================================================================
//
// The reader is responsible for:
// 1. Scanning the WAL file from a given offset
// 2. Validating record headers (sanity checks, CRC verification)
// 3. Decoding payloads back into record structs
// 4. Iterating through records for recovery
//
// Safety checks performed before allocation:
// - Length <= MaxRecordSize (4MB)
// - Length >= MinRecordSize (32 bytes)
// - RecordType is valid (1-7)
// - FileOffset matches current read position
//
// ===========================================================================

// WALReader reads and decodes WAL records from a file
type WALReader struct {
	file       *os.File // File handle for reading
	walPath    string   // Path to WAL file
	currentPos uint64   // Current read position in file
}

// NewWALReader creates a new WAL reader for the given file
func NewWALReader(walPath string) (*WALReader, error) {
	file, err := os.Open(walPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	return &WALReader{
		file:       file,
		walPath:    walPath,
		currentPos: 0,
	}, nil
}

// Close closes the WAL reader
func (r *WALReader) Close() error {
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}

// ===========================================================================
// FILE HEADER READING
// ===========================================================================

// ReadFileHeader reads and validates the WAL file header
func (r *WALReader) ReadFileHeader() (*WALFileHeader, error) {
	// Seek to beginning
	if _, err := r.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to start: %w", err)
	}

	// Read header bytes
	buf := make([]byte, FileHeaderSize)
	n, err := io.ReadFull(r.file, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}
	if n != FileHeaderSize {
		return nil, fmt.Errorf("incomplete file header: read %d of %d bytes", n, FileHeaderSize)
	}

	// Validate magic bytes
	var magic [8]byte
	copy(magic[:], buf[0:8])
	if magic != WALMagic {
		return nil, fmt.Errorf("invalid WAL magic: expected %v, got %v", WALMagic, magic)
	}

	// Decode header
	header := &WALFileHeader{
		Magic:   magic,
		Version: ByteOrder.Uint16(buf[8:10]),
	}
	copy(header.DatabaseName[:], buf[10:42])
	header.InitialLSN = ByteOrder.Uint64(buf[42:50])
	header.CreatedAt = int64(ByteOrder.Uint64(buf[50:58]))

	// Validate version
	if header.Version != WALVersion {
		return nil, fmt.Errorf("unsupported WAL version: expected %d, got %d", WALVersion, header.Version)
	}

	// Update position
	r.currentPos = FileHeaderSize

	return header, nil
}

// ===========================================================================
// RECORD READING
// ===========================================================================

// ReadNextRecord reads the next WAL record from the current position
// Returns io.EOF when end of file is reached
func (r *WALReader) ReadNextRecord() (WALRecord, error) {
	// Read header bytes
	headerBuf := make([]byte, RecordHeaderSize)
	n, err := io.ReadFull(r.file, headerBuf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		if n == 0 {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("incomplete header at offset %d: read %d bytes", r.currentPos, n)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read header at offset %d: %w", r.currentPos, err)
	}

	// Decode header
	header := decodeHeader(headerBuf)

	// Validate header (safety checks)
	if err := r.validateHeader(header); err != nil {
		return nil, err
	}

	// Calculate payload size (total length - header size)
	payloadSize := int(header.Length) - RecordHeaderSize
	if payloadSize < 0 {
		return nil, fmt.Errorf("invalid payload size %d at offset %d", payloadSize, r.currentPos)
	}

	// Read payload
	payload := make([]byte, payloadSize)
	if payloadSize > 0 {
		n, err = io.ReadFull(r.file, payload)
		if err != nil {
			return nil, fmt.Errorf("failed to read payload at offset %d: %w", r.currentPos, err)
		}
		if n != payloadSize {
			return nil, fmt.Errorf("incomplete payload: read %d of %d bytes", n, payloadSize)
		}
	}

	// Calculate actual payload size (before padding)
	// The payload includes padding bytes at the end to reach alignment
	actualPayloadSize := payloadSize
	// Remove padding bytes from CRC calculation
	unalignedSize := RecordHeaderSize + payloadSize
	paddingSize := int(header.Length) - unalignedSize
	if paddingSize > 0 {
		actualPayloadSize = payloadSize - paddingSize
	}

	// Verify CRC32 of actual payload (excluding padding)
	if actualPayloadSize > 0 {
		if err := verifyCRC32(payload[:actualPayloadSize], header.CRC32); err != nil {
			return nil, fmt.Errorf("CRC mismatch at offset %d: %w", r.currentPos, err)
		}
	}

	// Update position
	r.currentPos += uint64(header.Length)

	// Decode payload based on record type
	return r.decodeRecord(header, payload[:actualPayloadSize])
}

// ReadRecordAt reads a WAL record at the specified file offset
func (r *WALReader) ReadRecordAt(offset uint64) (WALRecord, error) {
	if err := r.SeekToOffset(offset); err != nil {
		return nil, err
	}
	return r.ReadNextRecord()
}

// SeekToOffset moves the reader to the specified file offset
func (r *WALReader) SeekToOffset(offset uint64) error {
	_, err := r.file.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to offset %d: %w", offset, err)
	}
	r.currentPos = offset
	return nil
}

// CurrentPosition returns the current read position
func (r *WALReader) CurrentPosition() uint64 {
	return r.currentPos
}

// ===========================================================================
// HEADER DECODING & VALIDATION
// ===========================================================================

// decodeHeader decodes a 32-byte buffer into a WALRecordHeader
func decodeHeader(buf []byte) WALRecordHeader {
	return WALRecordHeader{
		Type:       RecordType(buf[0]),
		Length:     ByteOrder.Uint32(buf[2:6]),
		LSN:        ByteOrder.Uint64(buf[6:14]),
		CRC32:      ByteOrder.Uint32(buf[14:18]),
		FileOffset: ByteOrder.Uint64(buf[18:26]),
	}
}

// validateHeader performs sanity checks on a record header
func (r *WALReader) validateHeader(h WALRecordHeader) error {
	// Check Length <= MaxRecordSize
	if h.Length > MaxRecordSize {
		return fmt.Errorf("record length %d exceeds max %d at offset %d (possible corruption)",
			h.Length, MaxRecordSize, r.currentPos)
	}

	// Check Length >= MinRecordSize
	if h.Length < MinRecordSize {
		return fmt.Errorf("record length %d below min %d at offset %d (possible corruption)",
			h.Length, MinRecordSize, r.currentPos)
	}

	// Check Type is valid (1-7)
	if h.Type < RecordBeginTxn || h.Type > RecordCheckpoint {
		return fmt.Errorf("invalid record type %d at offset %d (possible corruption)",
			h.Type, r.currentPos)
	}

	// Check FileOffset matches current position
	if h.FileOffset != r.currentPos {
		return fmt.Errorf("file offset mismatch: header says %d, actual position %d (possible corruption)",
			h.FileOffset, r.currentPos)
	}

	return nil
}

// verifyCRC32 checks if payload CRC matches expected CRC
func verifyCRC32(payload []byte, expectedCRC uint32) error {
	actualCRC := crc32.ChecksumIEEE(payload)
	if actualCRC != expectedCRC {
		return fmt.Errorf("CRC mismatch: expected %08x, got %08x", expectedCRC, actualCRC)
	}
	return nil
}

// ===========================================================================
// RECORD DECODING DISPATCHER
// ===========================================================================

// decodeRecord dispatches to the appropriate payload decoder based on record type
func (r *WALReader) decodeRecord(header WALRecordHeader, payload []byte) (WALRecord, error) {
	switch header.Type {
	case RecordBeginTxn:
		return decodeBeginTxnPayload(header, payload)
	case RecordInsert:
		return decodeInsertPayload(header, payload)
	case RecordUpdate:
		return decodeUpdatePayload(header, payload)
	case RecordDelete:
		return decodeDeletePayload(header, payload)
	case RecordCommit:
		return decodeCommitPayload(header, payload)
	case RecordAbort:
		return decodeAbortPayload(header, payload)
	case RecordCheckpoint:
		return decodeCheckpointPayload(header, payload)
	default:
		return nil, fmt.Errorf("unknown record type: %d", header.Type)
	}
}

// ===========================================================================
// PAYLOAD DECODERS
// ===========================================================================

// decodeBeginTxnPayload decodes a BeginTxn record payload
// Format: TxID (8 bytes)
func decodeBeginTxnPayload(header WALRecordHeader, payload []byte) (*BeginTxnRecord, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("BeginTxn payload too short: %d bytes", len(payload))
	}

	return &BeginTxnRecord{
		Header: header,
		TxID:   ByteOrder.Uint64(payload[0:8]),
	}, nil
}

// decodeInsertPayload decodes an Insert record payload
// Format: TxID(8) + TableNameLen(2) + TableName + KeyLen(2) + Key + ValueLen(4) + Value
func decodeInsertPayload(header WALRecordHeader, payload []byte) (*InsertRecord, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("Insert payload too short: %d bytes", len(payload))
	}

	offset := 0

	// TxID (8 bytes)
	txID := ByteOrder.Uint64(payload[offset:])
	offset += 8

	// TableName
	tableName, newOffset, err := decodeString(payload, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode TableName: %w", err)
	}
	offset = newOffset

	// Key
	key, newOffset, err := decodeString(payload, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Key: %w", err)
	}
	offset = newOffset

	// Value
	value, _, err := decodeBytes(payload, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Value: %w", err)
	}

	return &InsertRecord{
		Header:    header,
		TxID:      txID,
		TableName: tableName,
		Key:       key,
		Value:     value,
	}, nil
}

// decodeUpdatePayload decodes an Update record payload
// Format: TxID(8) + TableNameLen(2) + TableName + KeyLen(2) + Key + OldValueLen(4) + OldValue + NewValueLen(4) + NewValue
func decodeUpdatePayload(header WALRecordHeader, payload []byte) (*UpdateRecord, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("Update payload too short: %d bytes", len(payload))
	}

	offset := 0

	// TxID (8 bytes)
	txID := ByteOrder.Uint64(payload[offset:])
	offset += 8

	// TableName
	tableName, newOffset, err := decodeString(payload, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode TableName: %w", err)
	}
	offset = newOffset

	// Key
	key, newOffset, err := decodeString(payload, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Key: %w", err)
	}
	offset = newOffset

	// OldValue
	oldValue, newOffset, err := decodeBytes(payload, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode OldValue: %w", err)
	}
	offset = newOffset

	// NewValue
	newValue, _, err := decodeBytes(payload, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NewValue: %w", err)
	}

	return &UpdateRecord{
		Header:    header,
		TxID:      txID,
		TableName: tableName,
		Key:       key,
		OldValue:  oldValue,
		NewValue:  newValue,
	}, nil
}

// decodeDeletePayload decodes a Delete record payload
// Format: TxID(8) + TableNameLen(2) + TableName + KeyLen(2) + Key + OldValueLen(4) + OldValue
func decodeDeletePayload(header WALRecordHeader, payload []byte) (*DeleteRecord, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("Delete payload too short: %d bytes", len(payload))
	}

	offset := 0

	// TxID (8 bytes)
	txID := ByteOrder.Uint64(payload[offset:])
	offset += 8

	// TableName
	tableName, newOffset, err := decodeString(payload, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode TableName: %w", err)
	}
	offset = newOffset

	// Key
	key, newOffset, err := decodeString(payload, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Key: %w", err)
	}
	offset = newOffset

	// OldValue
	oldValue, _, err := decodeBytes(payload, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to decode OldValue: %w", err)
	}

	return &DeleteRecord{
		Header:    header,
		TxID:      txID,
		TableName: tableName,
		Key:       key,
		OldValue:  oldValue,
	}, nil
}

// decodeCommitPayload decodes a Commit record payload
// Format: TxID (8 bytes)
func decodeCommitPayload(header WALRecordHeader, payload []byte) (*CommitRecord, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("Commit payload too short: %d bytes", len(payload))
	}

	return &CommitRecord{
		Header: header,
		TxID:   ByteOrder.Uint64(payload[0:8]),
	}, nil
}

// decodeAbortPayload decodes an Abort record payload
// Format: TxID (8 bytes)
func decodeAbortPayload(header WALRecordHeader, payload []byte) (*AbortRecord, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("Abort payload too short: %d bytes", len(payload))
	}

	return &AbortRecord{
		Header: header,
		TxID:   ByteOrder.Uint64(payload[0:8]),
	}, nil
}

// decodeCheckpointPayload decodes a Checkpoint record payload
// Format: CheckpointLSN(8) + CheckpointOffset(8) + LastFlushedLSN(8) + Timestamp(8) +
//
//	DatabaseCRC32(4) + TableCount(4) + [TableChecksums...]
func decodeCheckpointPayload(header WALRecordHeader, payload []byte) (*CheckpointRecord, error) {
	if len(payload) < 40 { // minimum fixed fields
		return nil, fmt.Errorf("Checkpoint payload too short: %d bytes", len(payload))
	}

	offset := 0

	// CheckpointLSN (8 bytes)
	checkpointLSN := ByteOrder.Uint64(payload[offset:])
	offset += 8

	// CheckpointOffset (8 bytes)
	checkpointOffset := ByteOrder.Uint64(payload[offset:])
	offset += 8

	// LastFlushedLSN (8 bytes)
	lastFlushedLSN := ByteOrder.Uint64(payload[offset:])
	offset += 8

	// Timestamp (8 bytes)
	timestamp := int64(ByteOrder.Uint64(payload[offset:]))
	offset += 8

	// DatabaseCRC32 (4 bytes)
	databaseCRC32 := ByteOrder.Uint32(payload[offset:])
	offset += 4

	// TableCount (4 bytes)
	tableCount := ByteOrder.Uint32(payload[offset:])
	offset += 4

	// Tables
	tables := make([]TableChecksum, tableCount)
	for i := uint32(0); i < tableCount; i++ {
		// TableName
		tableName, newOffset, err := decodeString(payload, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to decode table %d name: %w", i, err)
		}
		offset = newOffset

		// DataCRC32 (4 bytes)
		if offset+8 > len(payload) {
			return nil, fmt.Errorf("payload too short for table %d checksums", i)
		}
		dataCRC32 := ByteOrder.Uint32(payload[offset:])
		offset += 4

		// MetaCRC32 (4 bytes)
		metaCRC32 := ByteOrder.Uint32(payload[offset:])
		offset += 4

		tables[i] = TableChecksum{
			TableName: tableName,
			DataCRC32: dataCRC32,
			MetaCRC32: metaCRC32,
		}
	}

	return &CheckpointRecord{
		Header:           header,
		CheckpointLSN:    checkpointLSN,
		CheckpointOffset: checkpointOffset,
		LastFlushedLSN:   lastFlushedLSN,
		Timestamp:        timestamp,
		DatabaseCRC32:    databaseCRC32,
		TableCount:       tableCount,
		Tables:           tables,
	}, nil
}

// ===========================================================================
// SCANNING UTILITIES
// ===========================================================================

// ScanAll reads all records from the beginning of the WAL
// Returns slice of all records and any error encountered
func (r *WALReader) ScanAll() ([]WALRecord, error) {
	// Read file header first
	_, err := r.ReadFileHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}

	// Collect all records
	var records []WALRecord
	for {
		record, err := r.ReadNextRecord()
		if err == io.EOF {
			break
		}
		if err != nil {
			return records, err
		}
		records = append(records, record)
	}

	return records, nil
}

// ScanFrom reads all records with LSN greater than afterLSN
func (r *WALReader) ScanFrom(afterLSN uint64) ([]WALRecord, error) {
	// Read file header first
	_, err := r.ReadFileHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}

	// Collect records with LSN > afterLSN
	var records []WALRecord
	for {
		record, err := r.ReadNextRecord()
		if err == io.EOF {
			break
		}
		if err != nil {
			return records, err
		}
		if record.GetHeader().LSN > afterLSN {
			records = append(records, record)
		}
	}

	return records, nil
}

// FindLastCheckpoint scans the WAL to find the most recent checkpoint record
func (r *WALReader) FindLastCheckpoint() (*CheckpointRecord, error) {
	// Read file header first
	_, err := r.ReadFileHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}

	// Scan for last checkpoint
	var lastCheckpoint *CheckpointRecord
	for {
		record, err := r.ReadNextRecord()
		if err == io.EOF {
			break
		}
		if err != nil {
			return lastCheckpoint, err
		}
		if cp, ok := record.(*CheckpointRecord); ok {
			lastCheckpoint = cp
		}
	}

	return lastCheckpoint, nil
}

// ===========================================================================
// HELPER FUNCTIONS
// ===========================================================================

// decodeString reads a length-prefixed string (2-byte length prefix)
func decodeString(data []byte, offset int) (string, int, error) {
	if offset+2 > len(data) {
		return "", 0, fmt.Errorf("not enough data for string length at offset %d", offset)
	}

	length := int(ByteOrder.Uint16(data[offset:]))
	offset += 2

	if offset+length > len(data) {
		return "", 0, fmt.Errorf("not enough data for string of length %d at offset %d", length, offset)
	}

	s := string(data[offset : offset+length])
	return s, offset + length, nil
}

// decodeBytes reads a length-prefixed byte slice (4-byte length prefix)
func decodeBytes(data []byte, offset int) (json.RawMessage, int, error) {
	if offset+4 > len(data) {
		return nil, 0, fmt.Errorf("not enough data for bytes length at offset %d", offset)
	}

	length := int(ByteOrder.Uint32(data[offset:]))
	offset += 4

	if offset+length > len(data) {
		return nil, 0, fmt.Errorf("not enough data for bytes of length %d at offset %d", length, offset)
	}

	// Make a copy to avoid referencing the original buffer
	b := make([]byte, length)
	copy(b, data[offset:offset+length])

	return json.RawMessage(b), offset + length, nil
}
