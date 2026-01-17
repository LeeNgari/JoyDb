# Domain Layer

## What

The Domain Layer contains the **core business entities** and rules of JoyDB. It defines the structure of databases, tables, and rows, along with custom error types for domain violations.

**Key Components**:
- **Schema** (`domain/schema/`): Database, Table, Column definitions and types
- **Data** (`domain/data/`): Row representation and data structures
- **Errors** (`domain/errors/`): Custom error types for domain violations

## Why

### Design Rationale

**Why a Rich Domain Model?**
- **Encapsulation**: Business logic lives with the data (Tables have CRUD methods)
- **Type safety**: Strong typing for column types, constraints, and operations
- **Clarity**: `table.Insert(row)` is clearer than `crud.Insert(table, row)`
- **Maintainability**: Changes to table behavior are localized

**Why custom error types?**
- **Type safety**: Can use `errors.As()` for specific error handling
- **Rich context**: Errors include table name, column name, values, etc.
- **Consistency**: Predictable error message format
- **Testability**: Easy to test for specific error conditions

**Why map[string]interface{} for rows?**
- **Flexibility**: Supports dynamic column types
- **Simplicity**: No code generation or reflection needed
- **JSON compatibility**: Easy serialization/deserialization

## How

### Schema Components

#### Database
```go
type Database struct {
    Name   string
    Path   string
    Tables map[string]*Table
}
```

**Responsibilities**:
- Container for tables
- Provides table lookup by name
- Manages database-level metadata

---

#### Table
```go
type Table struct {
    mu           sync.RWMutex
    Name         string
    Path         string
    Schema       *TableSchema
    Rows         []data.Row
    UniqueIndexes map[string]map[interface{}]int
    dirty        bool
}
```

**Responsibilities**:
- Stores table data (rows) in memory
- Enforces schema constraints
- Provides CRUD methods
- Manages indexes
- Thread-safe operations via `sync.RWMutex`

**Key Methods**:
- `Insert(row data.Row) error` - Add new row with validation
- `SelectAll() []data.Row` - Get all rows
- `Select(predicate func(data.Row) bool) []data.Row` - Filter rows
- `SelectByIndex(colName string, value interface{}) (data.Row, bool)` - Index lookup
- `Update(predicate func(data.Row) bool, updates data.Row) (int, error)` - Update rows
- `Delete(predicate func(data.Row) bool) (int, error)` - Delete rows

---

#### TableSchema
```go
type TableSchema struct {
    Columns       []Column
    LastInsertID  int
    RowCount      int
}
```

**Responsibilities**:
- Defines table structure (columns)
- Tracks auto-increment state
- Tracks row count for statistics

---

#### Column
```go
type Column struct {
    Name          string
    Type          ColumnType
    PrimaryKey    bool
    Unique        bool
    NotNull       bool
    AutoIncrement bool
}
```

**Responsibilities**:
- Defines column metadata
- Specifies constraints (primary key, unique, not null)
- Defines data type

**Column Types**:
```go
const (
    ColumnTypeInt
    ColumnTypeFloat
    ColumnTypeText
    ColumnTypeBool
    ColumnTypeDate
    ColumnTypeTime
    ColumnTypeEmail
)
```

### Data Components

#### Row
```go
type Row map[string]interface{}
```

**Representation**: Key-value map where:
- **Key**: Column name (string)
- **Value**: Column value (any type)

**Example**:
```go
row := data.Row{
    "id": 1,
    "username": "alice",
    "email": "alice@example.com",
    "is_active": true,
    "age": 25,
}
```

**Qualified Column Names** (for JOINs):
```go
joinedRow := data.Row{
    "users.id": 1,
    "users.username": "alice",
    "orders.id": 100,
    "orders.product": "laptop",
}
```

### Error Components

See [Errors README](errors/README.md) for detailed documentation.

**Error Types**:
- `ConstraintError` - Constraint violations (unique, primary key, not null)
- `ValidationError` - Data validation errors
- `TableNotFoundError` - Table doesn't exist
- `ColumnNotFoundError` - Column doesn't exist
- `ExecutionError` - Execution failures
- `ParseError` - Parsing errors
- `StorageError` - Storage/persistence errors

## Rich Domain Model Pattern

### Traditional Anemic Model (NOT used)
```go
// Anemic: Data and behavior separated
type Table struct {
    Name string
    Rows []Row
}

// Behavior in separate package
func Insert(table *Table, row Row) error { ... }
func Select(table *Table, predicate func(Row) bool) []Row { ... }
```



## Constraint Enforcement

### Primary Key
```go
// Enforced in Insert()
if col.PrimaryKey {
    if _, exists := row[col.Name]; !exists {
        return errors.NewNotNullViolation(t.Name, col.Name, len(t.Rows))
    }
    // Check uniqueness via index
    if _, found := t.UniqueIndexes[col.Name][row[col.Name]]; found {
        return errors.NewPrimaryKeyViolation(t.Name, col.Name, row[col.Name])
    }
}
```

### Unique Constraint
```go
// Enforced in Insert() and Update()
if col.Unique {
    if idx, exists := t.UniqueIndexes[col.Name]; exists {
        if _, found := idx[value]; found {
            return errors.NewUniqueViolation(t.Name, col.Name, value, existingRowIDs)
        }
    }
}
```

### Not Null Constraint
```go
// Enforced in Insert() and Update()
if col.NotNull {
    if value == nil {
        return errors.NewNotNullViolation(t.Name, col.Name, rowIndex)
    }
}
```

### Auto-Increment
```go
// Applied in Insert()
if col.AutoIncrement {
    if _, exists := row[col.Name]; !exists {
        t.Schema.LastInsertID++
        row[col.Name] = t.Schema.LastInsertID
    }
}
```

## Type Validation

### Type Checking
```go
func (t *Table) validateType(colName string, value interface{}, expectedType ColumnType) error {
    switch expectedType {
    case ColumnTypeInt:
        if _, ok := value.(int); !ok {
            return errors.NewTypeMismatch(t.Name, colName, value, "INT")
        }
    case ColumnTypeText:
        if _, ok := value.(string); !ok {
            return errors.NewTypeMismatch(t.Name, colName, value, "TEXT")
        }
    // ... other types
    }
    return nil
}
```

### Type Conversion
Type conversion happens in the Planner layer using `util/types` utilities.

## Indexing

### Unique Indexes
```go
type Table struct {
    // ...
    UniqueIndexes map[string]map[interface{}]int
    //             ↑         ↑                 ↑
    //          column    value            row index
}
```

**Purpose**:
- Fast lookup for unique columns
- Enforce uniqueness constraints
- Optimize JOIN operations

**Maintenance**:
- Built on table load by `query/indexing` package
- Updated on INSERT/UPDATE/DELETE
- Rebuilt when needed via `rebuildIndexesUnsafe()`

### Index Lookup
```go
func (t *Table) SelectByIndex(colName string, value interface{}) (data.Row, bool) {
    t.RLock()
    defer t.RUnlock()
    
    idx, exists := t.UniqueIndexes[colName]
    if !exists {
        return nil, false
    }
    
    rowIdx, found := idx[value]
    if !found {
        return nil, false
    }
    
    return t.Rows[rowIdx], true
}
```

## Dirty Tracking

### Purpose
Track whether table has unsaved changes for persistence.

### Implementation
```go
type Table struct {
    // ...
    dirty bool
}

func (t *Table) MarkDirty() {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.dirty = true
}

func (t *Table) MarkDirtyUnsafe() {
    // Called when already holding lock
    t.dirty = true
}
```

### Usage
- Set to `true` after INSERT/UPDATE/DELETE
- Checked by Storage layer to determine which tables to save
- Reset to `false` after successful save

## Interactions

### With Query Operations Layer
- Query operations call Table methods (Insert, Select, Update, Delete)
- Tables enforce constraints and validation
- Tables return data or errors

### With Storage Layer
- Storage layer loads tables from disk
- Storage layer saves dirty tables to disk
- Tables provide data and schema for serialization

### With Planner Layer
- Planner validates tables/columns exist
- Planner uses schema for type conversion
- Planner doesn't modify tables (read-only)



## Related Documentation

- [Query Operations Layer](../query/README.md) - Uses domain entities
- [Storage Layer](../storage/README.md) - Persists domain entities
- [Errors README](errors/README.md) - Detailed error type documentation
- [ARCHITECTURE.md](../../ARCHITECTURE.md) - System architecture overview
