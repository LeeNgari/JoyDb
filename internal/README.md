# Internal Architecture

This directory contains the internal implementation of JoyDB. The codebase is organized into **architectural layers**, each with a specific responsibility in the query execution pipeline.

## Architecture Overview

JoyDB follows a **layered architecture** where each layer depends only on layers below it:

```
Interface Layer (REPL, Network)
         ↓
    Engine Layer (Orchestration)
         ↓
    Parser Layer (SQL → AST)
         ↓
   Planning Layer (AST → Plan)
         ↓
  Execution Layer (Plan → Operations)
         ↓
Query Operations Layer (CRUD, JOIN)
         ↓
    Domain Layer (Tables, Rows)
         ↓
   Storage Layer (Persistence)
```

## Layer Documentation

### User Interfaces
- **[REPL & Network](interface/README.md)** - Interactive and server modes for user interaction

### Core Pipeline
- **[Engine](engine/README.md)** - Query execution orchestrator (coming soon)
- **[Parser](parser/README.md)** - SQL text → Abstract Syntax Tree conversion
- **[Planner](planner/README.md)** - AST → Execution Plan conversion
- **[Executor](executor/README.md)** - Execution plan → Database operations

### Data Operations
- **[Query Operations](query/README.md)** - Low-level CRUD, JOIN, and projection operations
- **[Domain](domain/README.md)** - Core business entities (Database, Table, Row, Errors)
- **[Storage](storage/README.md)** - Data persistence and retrieval

### Supporting Components
- **[Validation](validation/README.md)** - Data validation (coming soon)
- **[Indexing](index/README.md)** - Index management (coming soon)
- **[Infrastructure](infrastructure/README.md)** - Logging and configuration
- **[Utilities](util/README.md)** - Shared helper functions

## Quick Navigation

### I want to understand...

**...how SQL queries are executed**
1. Start with [ARCHITECTURE.md](../ARCHITECTURE.md) for the big picture
2. Read [Engine](engine/README.md) for orchestration (coming soon)
3. Follow the pipeline: [Parser](parser/README.md) → [Planner](planner/README.md) → [Executor](executor/README.md)

**...how data is stored**
- Read [Storage](storage/README.md) for persistence layer
- Read [Domain](domain/README.md) for in-memory data structures

**...how to add a new SQL statement**
1. [Parser](parser/README.md) - Add lexer tokens and parser logic
2. [Planner](planner/README.md) - Add plan node type
3. [Executor](executor/README.md) - Add executor implementation

**...how JOINs work**
- Read [Query Operations](query/README.md) for JOIN algorithms

**...how errors are handled**
- Read [Domain Errors](domain/README.md#errors) for custom error types

## Design Principles

### 1. Separation of Concerns
Each layer has a single, well-defined responsibility. Changes to one layer rarely affect others.

### 2. Dependency Direction
Dependencies flow downward. Higher layers depend on lower layers, never the reverse.

### 3. Testability
Each layer can be tested independently without requiring a full database setup.

### 4. Type Safety
Strong typing throughout the pipeline catches errors early.

### 5. Concurrency Safety
Thread-safe operations using `sync.RWMutex` where needed.

## Common Patterns

### Error Handling
All layers use custom error types from `domain/errors`:
```go
import "github.com/leengari/mini-rdbms/internal/domain/errors"

// Return specific error types
return errors.NewTableNotFoundError(tableName)
return errors.NewParseError("unexpected token", token)
```

### Predicate Functions
Many operations accept predicate functions for filtering:
```go
type PredicateFunc func(data.Row) bool

// Example usage
predicate := func(row data.Row) bool {
    age, ok := row["age"].(int)
    return ok && age > 18
}
```

### Locking Pattern
Tables use read/write locks for concurrency:
```go
// Read operations
table.RLock()
defer table.RUnlock()
// ... read data

// Write operations
table.Lock()
defer table.Unlock()
// ... modify data
```

## Development Guidelines

### Adding New Features
1. **Start with tests** - Write tests for the new feature first
2. **Update AST** - Add new node types if needed
3. **Update parser** - Add parsing logic
4. **Update planner** - Add planning logic
5. **Update executor** - Add execution logic
6. **Update documentation** - Update relevant READMEs

### Code Organization
- One file per statement type in parser (e.g., `statement_select.go`)
- One executor per operation type (e.g., `select_executor.go`)
- Group related operations in subdirectories (e.g., `query/operations/crud/`)

### Testing Strategy
- **Unit tests**: Test individual functions in isolation
- **Integration tests**: Test complete query execution in `integration_test/`
- **Table tests**: Use table-driven tests for multiple scenarios

## Related Documentation

- [Main README](../README.md) - User guide and getting started
- [ARCHITECTURE.md](../ARCHITECTURE.md) - High-level system architecture
- [SQL_REFERENCE.md](../SQL_REFERENCE.md) - SQL syntax reference
