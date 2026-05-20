package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/index/btree"
	"github.com/leengari/mini-rdbms/internal/wal"
)

var SnapshotMagic = []byte("JOYDBSNP")
const SnapshotVersion byte = 1

// CreateSnapshot serializes the entire database into a binary snapshot file.
func CreateSnapshot(db *schema.Database, snapshotDir string) (uint64, uint32, error) {
	db.RLock()
	defer db.RUnlock()

	snapshotLSN := uint64(time.Now().UnixNano())
	var buf bytes.Buffer

	// Header
	buf.Write(SnapshotMagic)
	buf.WriteByte(SnapshotVersion)

	lsnBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(lsnBuf, snapshotLSN)
	buf.Write(lsnBuf)

	tsBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(tsBuf, uint64(time.Now().Unix()))
	buf.Write(tsBuf)

	// TableCount
	tcBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(tcBuf, uint32(len(db.Tables)))
	buf.Write(tcBuf)

	for name, table := range db.Tables {
		if err := writeTable(&buf, name, table); err != nil {
			return 0, 0, err
		}
	}

	dataBytes := buf.Bytes()
	crc := crc32.ChecksumIEEE(dataBytes)

	crcBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(crcBuf, crc)
	buf.Write(crcBuf)

	finalBytes := buf.Bytes()

	snapPath := filepath.Join(snapshotDir, fmt.Sprintf("%d.snap", snapshotLSN))
	tmpPath := snapPath + ".tmp"

	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return 0, 0, err
	}
	if err := os.WriteFile(tmpPath, finalBytes, 0644); err != nil {
		return 0, 0, err
	}
	if err := os.Rename(tmpPath, snapPath); err != nil {
		return 0, 0, err
	}

	cleanupOldSnapshots(snapshotDir)

	return snapshotLSN, crc, nil
}

func writeTable(buf *bytes.Buffer, name string, table *schema.Table) error {
	table.RLock()
	defer table.RUnlock()

	// Name
	nameLenBuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(nameLenBuf, uint16(len(name)))
	buf.Write(nameLenBuf)
	buf.WriteString(name)

	// LastInsertID
	lidBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(lidBuf, uint64(table.LastInsertID))
	buf.Write(lidBuf)

	// Schema
	schemaBytes := wal.EncodeTableSchema(table.Schema)
	slenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(slenBuf, uint32(len(schemaBytes)))
	buf.Write(slenBuf)
	buf.Write(schemaBytes)

	// Rows
	rcBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(rcBuf, uint64(len(table.Rows)))
	buf.Write(rcBuf)

	for _, row := range table.Rows {
		rowBytes, err := row.Serialize()
		if err != nil {
			return err
		}
		rlBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(rlBuf, uint32(len(rowBytes)))
		buf.Write(rlBuf)
		buf.Write(rowBytes)
	}

	return nil
}

// LoadSnapshot reads a snapshot file and populates the database.
func LoadSnapshot(db *schema.Database, snapshotPath string) error {
	dataBytes, err := os.ReadFile(snapshotPath)
	if err != nil {
		return err
	}

	if len(dataBytes) < 8+1+8+8+4+4 {
		return fmt.Errorf("snapshot file too small")
	}

	if !bytes.Equal(dataBytes[:8], SnapshotMagic) {
		return fmt.Errorf("invalid snapshot magic")
	}

	if dataBytes[8] != SnapshotVersion {
		return fmt.Errorf("unsupported snapshot version: %d", dataBytes[8])
	}

	// Verify CRC
	payload := dataBytes[:len(dataBytes)-4]
	expectedCRC := binary.LittleEndian.Uint32(dataBytes[len(dataBytes)-4:])
	actualCRC := crc32.ChecksumIEEE(payload)
	if expectedCRC != actualCRC {
		return fmt.Errorf("snapshot CRC mismatch: expected %d, got %d", expectedCRC, actualCRC)
	}

	offset := 8 + 1 + 8 + 8

	tableCount := int(binary.LittleEndian.Uint32(dataBytes[offset:]))
	offset += 4

	db.Lock()
	defer db.Unlock()
	if db.Tables == nil {
		db.Tables = make(map[string]*schema.Table)
	}

	for i := 0; i < tableCount; i++ {
		table, newOffset, err := readTable(db.Path, dataBytes, offset)
		if err != nil {
			return err
		}
		db.Tables[table.Name] = table
		offset = newOffset
	}

	return nil
}

func readTable(dbPath string, dataBytes []byte, offset int) (*schema.Table, int, error) {
	if offset+2 > len(dataBytes) {
		return nil, 0, fmt.Errorf("corrupt snapshot: missing table name len")
	}
	nameLen := int(binary.LittleEndian.Uint16(dataBytes[offset:]))
	offset += 2

	if offset+nameLen > len(dataBytes) {
		return nil, 0, fmt.Errorf("corrupt snapshot: missing table name")
	}
	name := string(dataBytes[offset : offset+nameLen])
	offset += nameLen

	if offset+8 > len(dataBytes) {
		return nil, 0, fmt.Errorf("corrupt snapshot: missing last insert id")
	}
	lastInsertID := int64(binary.LittleEndian.Uint64(dataBytes[offset:]))
	offset += 8

	if offset+4 > len(dataBytes) {
		return nil, 0, fmt.Errorf("corrupt snapshot: missing schema len")
	}
	schemaLen := int(binary.LittleEndian.Uint32(dataBytes[offset:]))
	offset += 4

	if offset+schemaLen > len(dataBytes) {
		return nil, 0, fmt.Errorf("corrupt snapshot: missing schema bytes")
	}
	tableSchema, err := wal.DecodeTableSchema(dataBytes[offset : offset+schemaLen])
	if err != nil {
		return nil, 0, err
	}
	offset += schemaLen

	if offset+8 > len(dataBytes) {
		return nil, 0, fmt.Errorf("corrupt snapshot: missing row count")
	}
	rowCount := int(binary.LittleEndian.Uint64(dataBytes[offset:]))
	offset += 8

	tablePath := filepath.Join(dbPath, name)
	table := &schema.Table{
		Name:         name,
		Path:         tablePath,
		Schema:       tableSchema,
		Rows:         make([]data.Row, 0, rowCount),
		Indexes:      make(map[string]*data.Index),
		LastInsertID: lastInsertID,
	}

	// Initialize PKIndex if table has a primary key
	pkCol := tableSchema.GetPrimaryKeyColumn()
	if pkCol != nil {
		table.PKIndex = btree.New(btree.DefaultDegree)
	}

	// Setup Hash Indexes
	for _, col := range tableSchema.Columns {
		if col.Unique && !col.PrimaryKey {
			table.Indexes[col.Name] = &data.Index{
				Column: col.Name,
				Data:   make(map[interface{}][]int),
				Unique: true,
			}
		}
	}

	for i := 0; i < rowCount; i++ {
		if offset+4 > len(dataBytes) {
			return nil, 0, fmt.Errorf("corrupt snapshot: missing row len")
		}
		rowLen := int(binary.LittleEndian.Uint32(dataBytes[offset:]))
		offset += 4

		if offset+rowLen > len(dataBytes) {
			return nil, 0, fmt.Errorf("corrupt snapshot: missing row bytes")
		}
		row, err := data.Deserialize(dataBytes[offset : offset+rowLen])
		if err != nil {
			return nil, 0, err
		}
		
		pos := len(table.Rows)
		table.Rows = append(table.Rows, row)

		if table.PKIndex != nil && pkCol != nil {
			if val, exists := row.Data[pkCol.Name]; exists {
				table.PKIndex.Insert(val, pos)
			}
		}

		for colName, idx := range table.Indexes {
			if val, exists := row.Data[colName]; exists {
				idx.Data[val] = append(idx.Data[val], pos)
			}
		}
		
		offset += rowLen
	}

	return table, offset, nil
}

func cleanupOldSnapshots(snapshotDir string) {
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return
	}

	var snaps []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".snap") {
			snaps = append(snaps, entry.Name())
		}
	}

	if len(snaps) <= 2 {
		return
	}

	sort.Slice(snaps, func(i, j int) bool {
		lsn1, _ := strconv.ParseInt(strings.TrimSuffix(snaps[i], ".snap"), 10, 64)
		lsn2, _ := strconv.ParseInt(strings.TrimSuffix(snaps[j], ".snap"), 10, 64)
		return lsn1 > lsn2
	})

	// Keep the first two (newest), delete the rest
	for i := 2; i < len(snaps); i++ {
		os.Remove(filepath.Join(snapshotDir, snaps[i]))
	}
}
