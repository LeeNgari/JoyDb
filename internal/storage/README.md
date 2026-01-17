# Storage Layer

## What

The Storage Layer handles **data persistence and retrieval** for JoyDB. It provides durability by saving in-memory data to disk in JSON format and loading it back on startup.

**Key Components**:
- **Loader** (`storage/loader/`): Reads databases and tables from disk
- **Writer** (`storage/writer/`): Saves databases and tables to disk
- **Manager/Registry** (`storage/manager/`): Manages loaded databases with caching
- **Metadata** (`storage/metadata/`): Handles schema serialization
- **Bootstrap** (`storage/bootstrap/`): Creates new databases and tables

## Why

### Design Rationale

**Why persist to disk?**
- **Durability**: Data survives application restarts
- **Recovery**: Can restore state after crashes
- **Portability**: Can move databases between systems

**Why JSON format?**
- **Human-readable**: Easy to inspect and debug
- **Simple**: No binary format complexity
- **Portable**: Works across platforms and languages
- **Editable**: Can manually edit data files if needed

**Why separate loader and writer?**
- **Single Responsibility**: Loading and saving have different concerns
- **Testability**: Can test loading and saving independently
- **Flexibility**: Could add different storage backends (binary, SQL, etc.)

**Why lazy loading with registry?**
- **Performance**: Don't load all databases on startup
- **Memory**: Only load databases that are actually used
- **Caching**: Keep loaded databases in memory for fast access

## How

### File Structure

```
databases/
├── mydb/                    # Database directory
│   ├── users/               # Table directory
│   │   ├── meta.json        # Table schema
│   │   └── data.json        # Table rows
│   ├── orders/
│   │   ├── meta.json
│   │   └── data.json
│   └── products/
│       ├── meta.json
│       └── data.json
└── testdb/
    └── customers/
        ├── meta.json
        └── data.json
```

### File Formats

#### meta.json (Table Schema)
```json
{
  "name": "users",
  "columns": [
    {
      "name": "id",
      "type": "INT",
      "primary_key": true,
      "unique": true,
      "not_null": true,
      "auto_increment": true
    },
    {
      "name": "username",
      "type": "TEXT",
      "unique": true,
      "not_null": true
    },
    {
      "name": "email",
      "type": "EMAIL",
      "unique": true
    },
    {
      "name": "is_active",
      "type": "BOOL"
    }
  ],
  "last_insert_id": 5,
  "row_count": 3
}
```

#### data.json (Table Rows)
```json
[
  {
    "id": 1,
    "username": "alice",
    "email": "alice@example.com",
    "is_active": true
  },
  {
    "id": 2,
    "username": "bob",
    "email": "bob@example.com",
    "is_active": true
  },
  {
    "id": 5,
    "username": "eve",
    "email": "eve@example.com",
    "is_active": false
  }
]
```

## Components

### Loader

**Location**: `storage/loader/loader.go`

**Responsibilities**:
- Load database from directory
- Load all tables in database
- Parse meta.json and data.json
- Build in-memory Table structures

**Key Functions**:
```go
// Load entire database
func LoadDatabase(dbPath string) (*schema.Database, error)

// Load single table
func LoadTable(tablePath string) (*schema.Table, error)
```

**Loading Process**:
1. Read database directory
2. For each table subdirectory:
   - Read `meta.json` → TableSchema
   - Read `data.json` → []Row
   - Create Table with schema and rows
3. Return Database with all tables

**Error Handling**:
- Missing directory → Error
- Invalid JSON → Error
- Type mismatch → Error
- Corrupted data → Error (with details)

---

### Writer

**Location**: `storage/writer/writer.go`

**Responsibilities**:
- Save database to disk
- Save only dirty tables (optimization)
- Write meta.json and data.json
- Ensure atomic writes (write to temp, then rename)

**Key Functions**:
```go
// Save entire database
func SaveDatabase(db *schema.Database) error

// Save single table
func SaveTable(table *schema.Table) error
```

**Saving Process**:
1. For each dirty table:
   - Acquire read lock
   - Marshal schema to JSON → meta.json
   - Marshal rows to JSON → data.json
   - Write to temp files
   - Rename temp files to final names (atomic)
   - Mark table as clean

**Atomic Writes**:
```go
// Write to temp file
tmpFile := filepath.Join(dir, "meta.json.tmp")
ioutil.WriteFile(tmpFile, jsonData, 0644)

// Atomic rename
os.Rename(tmpFile, filepath.Join(dir, "meta.json"))
```

**Error Handling**:
- Write failure → Error (data not lost, still in memory)
- Partial write → Rollback via temp files
- Disk full → Error with clear message

---

### Manager/Registry

**Location**: `storage/manager/registry.go`

**Responsibilities**:
- Manage loaded databases
- Lazy loading (load on first access)
- Caching (keep in memory)
- Database lifecycle (create, drop, rename)

**Key Functions**:
```go
// Get or load database
func (r *Registry) Get(name string) (*schema.Database, error)

// Create new database
func (r *Registry) Create(name string) error

// Drop database
func (r *Registry) Drop(name string) error

// Rename database
func (r *Registry) Rename(oldName, newName string) error

// Save all loaded databases
func (r *Registry) SaveAll()
```

**Registry Structure**:
```go
type Registry struct {
    mu       sync.RWMutex
    loaded   map[string]*schema.Database  // Cache
    basePath string                       // databases/
}
```

**Lazy Loading**:
```go
func (r *Registry) Get(name string) (*schema.Database, error) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // Check cache
    if db, ok := r.loaded[name]; ok {
        return db, nil
    }
    
    // Load from disk
    dbPath := filepath.Join(r.basePath, name)
    db, err := loader.LoadDatabase(dbPath)
    if err != nil {
        return nil, err
    }
    
    // Build indexes
    indexing.BuildDatabaseIndexes(db)
    
    // Cache
    r.loaded[name] = db
    return db, nil
}
```

**Shutdown**:
```go
// Called on application shutdown
defer func() {
    registry.SaveAll()  // Save all dirty tables
}()
```

---

### Metadata

**Location**: `storage/metadata/metadata.go`

**Responsibilities**:
- Serialize/deserialize table schema
- Handle column type conversion
- Validate metadata structure

**Key Functions**:
```go
// Serialize schema to JSON
func SerializeSchema(schema *schema.TableSchema) ([]byte, error)

// Deserialize JSON to schema
func DeserializeSchema(data []byte) (*schema.TableSchema, error)
```

**Type Mapping**:
```go
// String → ColumnType
"INT"   → ColumnTypeInt
"TEXT"  → ColumnTypeText
"BOOL"  → ColumnTypeBool
"DATE"  → ColumnTypeDate
"TIME"  → ColumnTypeTime
"EMAIL" → ColumnTypeEmail
```

---

### Bootstrap

**Location**: `storage/bootstrap/bootstrap.go`

**Responsibilities**:
- Create new database directories
- Create new table directories
- Initialize empty data files

**Key Functions**:
```go
// Create new database
func CreateDatabase(name string, basePath string) error

// Create new table
func CreateTable(db *schema.Database, table *schema.Table) error
```

**Database Creation**:
```go
func CreateDatabase(name string, basePath string) error {
    dbPath := filepath.Join(basePath, name)
    return os.MkdirAll(dbPath, 0755)
}
```

**Table Creation**:
```go
func CreateTable(db *schema.Database, table *schema.Table) error {
    // Create table directory
    tablePath := filepath.Join(db.Path, table.Name)
    os.MkdirAll(tablePath, 0755)
    
    // Write meta.json
    metadata := SerializeSchema(table.Schema)
    ioutil.WriteFile(filepath.Join(tablePath, "meta.json"), metadata, 0644)
    
    // Write empty data.json
    ioutil.WriteFile(filepath.Join(tablePath, "data.json"), []byte("[]"), 0644)
    
    return nil
}
```

## Interactions

### With Domain Layer
- Loads data into Table structures
- Saves Table data to disk
- Uses Table.dirty flag to optimize saves

### With Engine Layer
- Engine uses Registry to get/create databases
- Engine triggers SaveAll() on shutdown

### With Query/Indexing Layer
- Registry builds indexes after loading
- Ensures indexes are ready before returning database

## Design Decisions

### Why JSON Instead of Binary?
**Trade-off**: Performance vs. debuggability
- **Current**: JSON (human-readable)
- **Alternative**: Binary format (faster, smaller)
- **Reason**: Simplicity and debuggability more important for this project

### Why Save Only Dirty Tables?
**Trade-off**: Complexity vs. performance
- **Current**: Track dirty flag, save only changed tables
- **Alternative**: Save all tables every time
- **Reason**: Significant performance improvement for large databases

### Why Atomic Writes (temp + rename)?
**Trade-off**: Complexity vs. safety
- **Current**: Write to temp, rename atomically
- **Alternative**: Write directly to file
- **Reason**: Prevents data corruption on crash during write

### Why Lazy Loading?
**Trade-off**: Startup time vs. memory usage
- **Current**: Load databases on first access
- **Alternative**: Load all databases on startup
- **Reason**: Faster startup, lower memory usage

## Performance Characteristics

### Strengths
- **Fast reads**: Data already in memory after first load
- **Efficient saves**: Only save dirty tables
- **Lazy loading**: Fast startup time

### Limitations
- **Write amplification**: Entire table written on any change
- **No incremental saves**: Can't save just changed rows
- **Memory-bound**: All data must fit in RAM
- **No compression**: JSON is verbose

## Error Handling

### Load Errors
```go
// Database not found
return nil, fmt.Errorf("database not found: %s", name)

// Invalid JSON
return nil, fmt.Errorf("failed to parse meta.json: %w", err)

// Missing file
return nil, fmt.Errorf("data.json not found for table: %s", tableName)
```

### Save Errors
```go
// Write failure
return fmt.Errorf("failed to write meta.json: %w", err)

// Permission denied
return fmt.Errorf("permission denied writing to: %s", path)
```

### Recovery
- **Load failure**: Database not loaded, error returned to user
- **Save failure**: Data remains in memory, can retry save
- **Partial write**: Temp files prevent corruption

## Limitations

### Current Limitations
1. **No write-ahead log (WAL)**: Crash during write may lose data
2. **No incremental saves**: Entire table written on change
3. **No compression**: Large tables use lots of disk space
4. **No encryption**: Data stored in plain text
5. **No backup/restore**: Must manually copy directories
6. **No versioning**: Can't rollback to previous state

### Future Enhancements
- **Write-ahead log**: Durability and crash recovery
- **Incremental saves**: Save only changed rows
- **Compression**: Reduce disk usage
- **Encryption**: Secure sensitive data
- **Backup/restore**: Built-in backup functionality
- **Versioning**: Snapshot and rollback support
- **Binary format**: Faster serialization/deserialization

## Testing

### Unit Tests
Test loading and saving:
```go
func TestLoadTable(t *testing.T) {
    table, err := loader.LoadTable("testdata/users")
    
    assert.NoError(t, err)
    assert.Equal(t, "users", table.Name)
    assert.Equal(t, 3, len(table.Rows))
}

func TestSaveTable(t *testing.T) {
    table := createTestTable()
    table.Insert(data.Row{"id": 1, "name": "Alice"})
    
    err := writer.SaveTable(table)
    
    assert.NoError(t, err)
    // Verify files exist
    assert.FileExists(t, filepath.Join(table.Path, "meta.json"))
    assert.FileExists(t, filepath.Join(table.Path, "data.json"))
}
```

### Integration Tests
Test full load/save cycle:
```go
func TestLoadSaveCycle(t *testing.T) {
    // Create and save database
    db := createTestDatabase()
    writer.SaveDatabase(db)
    
    // Load database
    loaded, err := loader.LoadDatabase(db.Path)
    
    // Verify data matches
    assert.NoError(t, err)
    assert.Equal(t, db.Name, loaded.Name)
    assert.Equal(t, len(db.Tables), len(loaded.Tables))
}
```

## Related Documentation

- [Domain Layer](../domain/README.md) - Entities that are persisted
- [Engine Layer](../engine/README.md) - Uses Registry for database management (coming soon)
- [ARCHITECTURE.md](../../ARCHITECTURE.md) - System architecture overview
