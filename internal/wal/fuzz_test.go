package wal

import (
	"os"
	"testing"
)

func FuzzWALReader(f *testing.F) {
	// 1. Seed with a valid WAL file content
	// We construct one manually or use captured bytes.
	// Since creating a valid WAL involves multiple steps, we can just seed with empty or partial bytes.
	// But it's better to seed with "almost valid" data.
	f.Add([]byte{}) // Empty

	// Create a valid WAL in memory buffer?
	// NewWAL writes to file.
	// We can use a test helper to generate a small valid WAL file content.

	// Seed with valid file header
	header := make([]byte, FileHeaderSize)
	copy(header[0:8], WALMagic[:])
	ByteOrder.PutUint16(header[8:10], WALVersion)
	f.Add(header)

	f.Fuzz(func(t *testing.T, data []byte) {
		// Write data to temp file
		tmpFile := createTempWAL(t)
		defer removeTempWAL(t, tmpFile)

		if err := os.WriteFile(tmpFile, data, 0644); err != nil {
			return
		}

		// Try to read
		reader, err := NewWALReader(tmpFile)
		if err != nil {
			return // Invalid file header is expected
		}
		defer reader.Close()

		_, _ = reader.ReadFileHeader()

		// Read all records
		for {
			_, err := reader.ReadNextRecord()
			if err != nil {
				break
			}
		}
	})
}

func FuzzRecordDecoding(f *testing.F) {
	// Seed with valid payload
	f.Add([]byte{0, 0, 0, 0}) // Empty payload

	// Seed with encoded insert payload
	// We can't access encodeInsertPayload easily as it is a method on *WAL.
	// But we can construct similar bytes.
	// Format: TxID(8) + TableNameLen(2) + TableName...

	f.Fuzz(func(t *testing.T, payload []byte) {
		// Try to decode as every record type
		// Note: header is usually passed to decodeRecord.
		// We can construct a dummy header.

		// Accessing private method decodeRecord requires being in package wal.
		// We are in package wal.

		// Create a mock reader to call decodeRecord?
		// decodeRecord is a method on *WALReader.
		r := &WALReader{}

		// Iterate all types
		for rt := RecordBeginTxn; rt <= RecordCheckpoint; rt++ {
			header := WALRecordHeader{
				Type: rt,
				Length: uint32(len(payload) + RecordHeaderSize),
				PayloadLen: uint32(len(payload)),
			}

			// We only care about panic safety here
			_, _ = r.decodeRecord(header, payload)
		}
	})
}
