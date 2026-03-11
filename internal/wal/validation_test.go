package wal

import (
	"io"
	"os"
	"testing"
)

// =============================================================================
// VALIDATION TESTS
// =============================================================================

func TestValidateHeader_RejectsInvalidLength(t *testing.T) {
	// Create a file with a header claiming huge length
	tmpFile := createTempWAL(t)
	defer removeTempWAL(t, tmpFile)

	// Write file header
	f, _ := os.Create(tmpFile)
	writeFileHeader(f)

	// Write record header with Length > MaxRecordSize
	// MaxRecordSize = 4MB (4 * 1024 * 1024)
	invalidLen := uint32(5 * 1024 * 1024)
	buf := make([]byte, RecordHeaderSize)
	buf[0] = byte(RecordInsert)
	ByteOrder.PutUint32(buf[2:6], invalidLen)
	ByteOrder.PutUint64(buf[18:26], FileHeaderSize) // correct offset
	f.Write(buf)
	f.Close()

	r, _ := NewWALReader(tmpFile)
	defer r.Close()
	r.ReadFileHeader()

	_, err := r.ReadNextRecord()
	if err == nil {
		t.Error("Expected error for oversized record")
	} else if err.Error() == "EOF" {
		t.Error("Expected validation error, got EOF")
	}
}

func TestValidateHeader_RejectsInvalidType(t *testing.T) {
	tmpFile := createTempWAL(t)
	defer removeTempWAL(t, tmpFile)

	f, _ := os.Create(tmpFile)
	writeFileHeader(f)

	// Write record header with invalid type (e.g. 255)
	buf := make([]byte, RecordHeaderSize)
	buf[0] = 255
	ByteOrder.PutUint32(buf[2:6], MinRecordSize)
	ByteOrder.PutUint64(buf[18:26], FileHeaderSize)
	f.Write(buf)
	f.Close()

	r, _ := NewWALReader(tmpFile)
	defer r.Close()
	r.ReadFileHeader()

	_, err := r.ReadNextRecord()
	if err == nil {
		t.Error("Expected error for invalid type")
	}
}

func TestValidateHeader_RejectsOffsetMismatch(t *testing.T) {
	tmpFile := createTempWAL(t)
	defer removeTempWAL(t, tmpFile)

	f, _ := os.Create(tmpFile)
	writeFileHeader(f)

	// Write record header with Wrong Offset
	buf := make([]byte, RecordHeaderSize)
	buf[0] = byte(RecordInsert)
	ByteOrder.PutUint32(buf[2:6], MinRecordSize)
	ByteOrder.PutUint64(buf[18:26], 999999) // Wrong offset
	f.Write(buf)
	f.Close()

	r, _ := NewWALReader(tmpFile)
	defer r.Close()
	r.ReadFileHeader()

	_, err := r.ReadNextRecord()
	if err == nil {
		t.Error("Expected error for offset mismatch")
	}
}

func TestReadRecord_HandlesUnexpectedEOF(t *testing.T) {
	tmpFile := createTempWAL(t)
	defer removeTempWAL(t, tmpFile)

	f, _ := os.Create(tmpFile)
	writeFileHeader(f)

	// Write partial header (10 bytes)
	f.Write(make([]byte, 10))
	f.Close()

	r, _ := NewWALReader(tmpFile)
	defer r.Close()
	r.ReadFileHeader()

	_, err := r.ReadNextRecord()
	if err == nil {
		t.Error("Expected error for partial header, got nil")
	} else if err == io.EOF {
		t.Error("Expected incomplete header error, got clean EOF")
	}
}

// Helper to write a valid file header to a file
func writeFileHeader(f *os.File) {
	// We need to write 64 bytes
	buf := make([]byte, FileHeaderSize)
	copy(buf[0:8], WALMagic[:])
	ByteOrder.PutUint16(buf[8:10], WALVersion)
	// Rest is 0
	f.Write(buf)
}
