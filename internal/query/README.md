# Query Operations Package

The query operations package contains the core database operations that manipulate data. These operations are called by executors and work directly with the database schema and storage.

## Package Structure

```
query/
├── operations/
│   ├── crud/          # Create, Read, Update, Delete operations
│   ├── join/          # JOIN operations
│   ├── projection/    # Column projection and selection
│   └── testutil/      # Test utilities
├── indexing/          # Index building and management
└── validation/        # Row validation
```

## CRUD Operations

Located in `operations/crud/`:

| File | Responsibility | LOC |
|------|---------------|-----|
| `select.go` | SELECT operations (SelectAll, SelectWhere) | ~70 |
| `insert.go` | INSERT operations with validation | ~133 |
| `update.go` | UPDATE operations with validation | ~157 |
| `delete.go` | DELETE operations | ~86 |

### Usage

```go
import "github.com/leengari/mini-rdbms/internal/query/operations/crud"

// SELECT all rows
rows := crud.SelectAll(table, projection)

// SELECT with WHERE clause
rows := crud.SelectWhere(table, predicate, projection)

// INSERT row
err := crud.Insert(table, row)

// UPDATE rows
count, err := crud.Update(table, predicate, updates)

// DELETE rows
count, err := crud.Delete(table, predicate)
```

## JOIN Operations

Located in `operations/join/`:

| File | Responsibility | LOC |
|------|---------------|-----|
| `executor.go` | Main JOIN execution logic | ~324 |
| `types.go` | JOIN types and predicates | ~76 |
| `helpers.go` | Helper functions | ~115 |

### Supported JOIN Types

- **INNER JOIN**: Returns rows with matches in both tables
- **LEFT JOIN**: Returns all left table rows + matches from right
- **RIGHT JOIN**: Returns all right table rows + matches from left
- **FULL OUTER JOIN**: Returns all rows from both tables

### Usage

```go
import "github.com/leengari/mini-rdbms/internal/query/operations/join"

// Execute JOIN
joinedRows, err := join.ExecuteJoin(
    leftTable,
    rightTable,
    leftJoinCol,
    rightJoinCol,
    join.JoinTypeInner,
    predicate,      // Optional WHERE clause
    projection,
)
```

## Projection Operations

Located in `operations/projection/`:

| File | Responsibility | LOC |
|------|---------------|-----|
| `projector.go` | Column projection logic | ~94 |
| `projection_test.go` | Tests | ~109 |

### Usage

```go
import "github.com/leengari/mini-rdbms/internal/query/operations/projection"

// SELECT * (all columns)
proj := projection.NewProjection()

// SELECT specific columns
proj := &projection.Projection{
    SelectAll: false,
    Columns: []projection.ColumnRef{
        {Column: "id"},
        {Table: "users", Column: "name"},
    },
}

// Apply projection to row
projectedRow := projection.Project(row, proj)
```

## Design Principles

1. **Separation of Concerns**: Operations don't know about parsing or execution
2. **Reusability**: Operations can be composed and reused
3. **Validation**: Operations validate constraints and types
4. **Concurrency Safety**: Operations use table locks appropriately

## Predicate Functions

Many operations accept predicate functions for filtering:

```go
type PredicateFunc func(data.Row) bool

// Example: Filter rows where age > 18
pred := func(row data.Row) bool {
    age, ok := row["age"].(int)
    return ok && age > 18
}

rows := crud.SelectWhere(table, pred, projection)
```

## Validation

Operations validate:
- **Constraints**: Primary key, unique, not null, auto-increment
- **Types**: Column types match schema
- **References**: Foreign key constraints (if implemented)

## Indexing

Located in `indexing/`:

Builds and manages indexes for faster lookups:

```go
import "github.com/leengari/mini-rdbms/internal/query/indexing"

// Build indexes for all tables
err := indexing.BuildDatabaseIndexes(database)
```

## Testing

Test utilities in `operations/testutil/`:

```go
import "github.com/leengari/mini-rdbms/internal/query/operations/testutil"

// Create test table
table := testutil.CreateTestTable()

// Assert row count
testutil.AssertRowCount(t, table, expectedCount)
```

## Related Packages

- `executor/` - Calls these operations
- `domain/schema/` - Defines table structures
- `domain/data/` - Defines row types
- `util/types/` - Type conversion and comparison
