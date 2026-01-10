# Mini RDBMS

A lightweight, educational relational database management system built from scratch in Go. This project implements core database concepts including schema management, indexing, CRUD operations, and ACID-compliant persistence.

> **Built for:** Pesapal Challenge '26  
> **Status:** Active Development  
> **Language:** Go 1.25.4

---

## 🎯 Project Goals

- **Educational**: Learn database internals by building one
- **Practical**: Implement real-world database features
- **Scalable**: Design for future enhancements (SQL parser, transactions, etc.)
- **Professional**: Follow Go best practices and clean architecture

---

## ✨ Features

### Currently Implemented ✅

- **Schema Management**
  - Column types: INT, TEXT, BOOL, FLOAT, DATE, TIME, EMAIL
  - Constraints: PRIMARY KEY, UNIQUE, NOT NULL, AUTO_INCREMENT
  - Type validation with detailed error messages

- **CRUD Operations**
  - ✅ INSERT with auto-increment support
  - ✅ SELECT (all, where, by index)
  - 🚧 UPDATE (planned)
  - 🚧 DELETE (planned)

- **Indexing**
  - In-memory B-tree indexes for PRIMARY KEY and UNIQUE columns
  - Automatic index building on startup
  - Index updates on INSERT operations

- **Data Persistence**
  - JSON-based storage (human-readable)
  - Atomic writes with temp files
  - Database and table metadata tracking
  - Dirty flag for optimized saves

- **Thread Safety**
  - Read-write mutex protection on all table operations
  - Safe for concurrent access (ready for REPL/web server)

- **Logging**
  - Structured logging with `slog`
  - Dual output: console + Seq server
  - Automatic fallback with warnings

### Planned Features 🚧

- SQL Parser (lexer, AST, query execution)
- UPDATE and DELETE operations
- JOIN support (INNER, LEFT, RIGHT)
- Transactions with ACID guarantees
- Write-Ahead Log (WAL)
- Query optimization
- REPL (interactive shell)
- REST API server

---

## 📁 Project Structure

```
RDBMS/
├── cmd/
│   └── rdbms/              # Application entry point
│       └── main.go
│
├── internal/
│   ├── domain/             # Core business logic (no dependencies)
│   │   ├── schema/         # Database schema definitions
│   │   │   ├── column.go       # Column types and definitions
│   │   │   ├── table_schema.go # Table schema with helpers
│   │   │   ├── table.go        # Table structure with mutex
│   │   │   └── database.go     # Database structure
│   │   ├── data/           # Data structures
│   │   │   ├── row.go          # Row type with Copy method
│   │   │   └── index.go        # Index structure
│   │   └── errors/         # Domain errors
│   │       └── constraint.go   # Constraint violation errors
│   │
│   ├── storage/            # Persistence layer
│   │   ├── loader/         # Loading from disk
│   │   │   ├── database.go     # Database loader
│   │   │   └── table.go        # Table loader with validation
│   │   ├── writer/         # Writing to disk
│   │   │   └── writer.go       # Atomic save operations
│   │   └── metadata/       # JSON serialization
│   │       └── types.go        # Metadata structures
│   │
│   ├── query/              # Query execution
│   │   ├── operations/     # CRUD operations
│   │   │   ├── insert.go       # INSERT with auto-increment
│   │   │   ├── select.go       # SELECT operations
│   │   │   ├── update.go       # UPDATE (stub)
│   │   │   └── delete.go       # DELETE (stub)
│   │   ├── validation/     # Data validation
│   │   │   └── validator.go    # Row validation logic
│   │   └── indexing/       # Index management
│   │       └── builder.go      # Index building
│   │
│   ├── parser/             # SQL parsing (future)
│   │   ├── lexer/
│   │   ├── ast/
│   │   └── parser.go
│   │
│   ├── executor/           # Query execution (future)
│   │   └── executor.go
│   │
│   └── infrastructure/     # Cross-cutting concerns
│       └── logging/
│           └── logger.go       # Logging setup
│
├── databases/              # Data storage
│   └── testdb/
│       ├── meta.json           # Database metadata
│       └── users/
│           ├── meta.json       # Table schema
│           └── data.json       # Table data
│
├── go.mod
├── go.sum
└── README.md
```

**Architecture Principles:**
- **Layered**: Domain → Query → Storage → Infrastructure
- **Separation of Concerns**: Each package has single responsibility
- **Thread-Safe**: Mutex protection on all table operations
- **Testable**: Function-based operations, clear dependencies

---

## 🗄️ Database Layout

### Directory Structure

Each database is a directory containing:
- `meta.json` - Database-level metadata (name, version, table list)
- Table subdirectories (one per table)

Each table directory contains:
- `meta.json` - Schema definition and constraints
- `data.json` - Row data as JSON array

```
databases/testdb/
├── meta.json              # {"name": "testdb", "version": 1, "tables": ["users"]}
└── users/
    ├── meta.json          # Schema: columns, constraints, last_insert_id
    └── data.json          # Rows: [{"id": 1, "username": "alice", ...}, ...]
```

### Why This Structure?

| Aspect | Benefit |
|--------|---------|
| **Human-readable** | JSON format, easy to inspect and debug |
| **Self-contained** | Each table owns its schema and data |
| **Scalable** | Easy to add/remove tables |
| **Atomic writes** | Temp files + rename for crash safety |

---

## 🚀 Getting Started

### Prerequisites

- Go 1.25.4 or later
- (Optional) Seq server for centralized logging

### Installation

```bash
# Clone the repository
git clone https://github.com/leengari/mini-rdbms.git
cd mini-rdbms

# Install dependencies
go mod download

# Build the application
go build ./cmd/rdbms

# Run
./rdbms
```

### Quick Example

```go
package main

import (
    "github.com/leengari/mini-rdbms/internal/domain/data"
    "github.com/leengari/mini-rdbms/internal/query/operations"
    "github.com/leengari/mini-rdbms/internal/storage/loader"
)

func main() {
    // Load database
    db, _ := loader.LoadDatabase("databases/testdb")
    
    // Get table
    users := db.Tables["users"]
    
    // Insert row
    operations.Insert(users, data.Row{
        "username": "alice",
        "email":    "alice@example.com",
        "is_active": true,
    })
    
    // Select all
    rows := operations.SelectAll(users)
    
    // Select with predicate
    activeUsers := operations.SelectWhere(users, func(r data.Row) bool {
        return r["is_active"] == true
    })
    
    // Select by index
    user, found := operations.SelectByUniqueIndex(users, "id", 1)
}
```

---

## 🔧 Technical Details

### Schema Definition

Tables are defined with typed columns and constraints:

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
      "name": "email",
      "type": "TEXT",
      "unique": true,
      "not_null": true
    }
  ],
  "last_insert_id": 5
}
```

### Supported Column Types

| Type | Go Type | Validation |
|------|---------|------------|
| `INT` | `int64` | Integer values only |
| `TEXT` | `string` | Any string |
| `BOOL` | `bool` | true/false |
| `FLOAT` | `float64` | Decimal numbers |
| `DATE` | `string` | ISO 8601 format |
| `TIME` | `string` | RFC3339 format |
| `EMAIL` | `string` | Email regex validation |

### Constraints

- **PRIMARY KEY**: Unique identifier, auto-indexed
- **UNIQUE**: No duplicate values, auto-indexed
- **NOT NULL**: Value required
- **AUTO_INCREMENT**: Automatic ID generation

### Indexing

Indexes are built in-memory on startup for all PRIMARY KEY and UNIQUE columns:

```
Index Structure:
column_name → {
    value1 → [row_position_1, row_position_2, ...],
    value2 → [row_position_3],
    ...
}
```

**Why in-memory?**
- Fast lookups (O(1) for unique, O(log n) for non-unique)
- No index/data synchronization issues
- Simple to implement
- Rebuilding is cheap for typical dataset sizes

### Thread Safety

All table operations are protected by `sync.RWMutex`:

```go
// Write operations (INSERT, UPDATE, DELETE)
table.Lock()
defer table.Unlock()
// ... modify data ...

// Read operations (SELECT)
table.RLock()
defer table.RUnlock()
// ... read data ...
```

This makes the database safe for concurrent access from multiple goroutines.

### Persistence

**Atomic Writes:**
1. Write to temporary file (`data.json.tmp`)
2. Rename to actual file (`data.json`)
3. OS guarantees atomicity of rename

**Dirty Flag:**
- Tables track unsaved changes with `Dirty` flag
- Only dirty tables are saved on shutdown
- Optimizes performance for read-heavy workloads

---

## 📊 Current Stats

- **Lines of Code**: ~1,132 (Go)
- **Files**: 24 Go files
- **Packages**: 10 internal packages
- **Test Coverage**: TBD (tests planned)

---

## 🛠️ Development

### Code Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting
- Document exported functions
- Keep functions under 50 lines when possible

### Adding a New Operation

1. Define function in `internal/query/operations/`
2. Acquire appropriate lock (Lock/RLock)
3. Perform operation
4. Update indexes if needed
5. Mark table dirty if mutation
6. Release lock (defer)

Example:
```go
func Update(table *schema.Table, rowIndex int, updates data.Row) error {
    table.Lock()
    defer table.Unlock()
    
    // ... update logic ...
    
    table.MarkDirtyUnsafe()
    return nil
}
```

---

## 🎓 Learning Resources

This project implements concepts from:

- **Database Internals** by Alex Petrov
- **Designing Data-Intensive Applications** by Martin Kleppmann
- **CMU 15-445 Database Systems** course materials

---

## 📝 Design Decisions

### Why JSON instead of binary?

**Pros:**
- Human-readable for debugging
- Easy to inspect and modify manually
- Simple serialization/deserialization
- Version control friendly

**Cons:**
- Slower than binary formats
- Larger file sizes

**Decision:** JSON is perfect for an educational project. Performance can be optimized later if needed.

### Why in-memory indexes?

**Pros:**
- Simple implementation
- Fast lookups
- No synchronization issues
- Easy to rebuild

**Cons:**
- Lost on restart (must rebuild)
- Memory usage grows with data

**Decision:** For the scope of this project, rebuilding indexes on startup is acceptable. Real databases persist indexes for performance.

### Why directory-per-table?

**Pros:**
- Clear organization
- Easy to add/remove tables
- Self-contained schemas
- Mirrors real database structure

**Cons:**
- More files to manage

**Decision:** Clarity and organization outweigh the minor inconvenience of multiple files.

---

## 🚧 Roadmap

### Phase 1: Core Features ✅
- [x] Schema management
- [x] INSERT operations
- [x] SELECT operations
- [x] Indexing
- [x] Data persistence
- [x] Thread safety

### Phase 2: Advanced Operations 🚧
- [ ] UPDATE operations
- [ ] DELETE operations
- [ ] Batch operations
- [ ] Transaction support

### Phase 3: Query Language 🔮
- [ ] SQL lexer
- [ ] SQL parser
- [ ] Query optimizer
- [ ] Execution planner

### Phase 4: User Interfaces 🔮
- [ ] REPL (interactive shell)
- [ ] REST API server
- [ ] Web-based admin UI

### Phase 5: Advanced Features 🔮
- [ ] JOIN operations
- [ ] Aggregations (COUNT, SUM, AVG)
- [ ] Write-Ahead Log (WAL)
- [ ] Query caching
- [ ] Connection pooling

---

## 🤝 Contributing

This is a personal learning project, but suggestions and feedback are welcome!

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

---

## 📄 License

This project is for educational purposes. Feel free to use and modify as needed.

---

## 🙏 Acknowledgments

- **Pesapal** for the challenge opportunity
- **Go community** for excellent documentation
- **Database textbooks** for theoretical foundations

---

## 📧 Contact

**Author:** LeeNgari  
**Project:** Mini RDBMS  
**Year:** 2026

---

*Last Updated: January 10, 2026*
