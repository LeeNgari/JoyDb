# JoyDB Architecture

## Overview

JoyDB implements a **layered architecture** that separates concerns and provides a clear query execution pipeline from SQL text to results.

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Application Entry                        │
│                      (cmd/joydb)                             │
│              Startup, Mode Selection, Shutdown               │
└────────────────────┬────────────────────────────────────────┘
                     │
        ┌────────────┴────────────┐
        │                         │
┌───────▼────────┐      ┌────────▼────────┐
│  REPL Mode     │      │  Server Mode    │
│  (Interactive) │      │  (TCP Network)  │
└───────┬────────┘      └────────┬────────┘
        │                        │
        └────────────┬───────────┘
                     │
        ┌────────────▼────────────┐
        │      Engine Layer       │
        │   (Query Orchestrator)  │
        └────────────┬────────────┘
                     │
        ┌────────────▼────────────┐
        │     Parser Layer        │
        │  Lexer → Parser → AST   │
        └────────────┬────────────┘
                     │
        ┌────────────▼────────────┐
        │    Planning Layer       │
        │  AST → Execution Plan   │
        └────────────┬────────────┘
                     │
        ┌────────────▼────────────┐
        │   Execution Layer       │
        │  Plan → Operations      │
        └────────────┬────────────┘
                     │
        ┌────────────▼────────────┐
        │  Query Operations       │
        │  CRUD, JOIN, Projection │
        └────────────┬────────────┘
                     │
        ┌────────────▼────────────┐
        │     Domain Layer        │
        │  Database, Table, Row   │
        └────────────┬────────────┘
                     │
        ┌────────────▼────────────┐
        │    Storage Layer        │
        │  Persistence (JSON)     │
        └─────────────────────────┘
```

## Architectural Layers

### 1. Application Entry Layer
**Location**: `cmd/joydb`

**Responsibility**: Application lifecycle management

**What it does**:
- Parses command-line flags (`--server`, `--port`)
- Initializes logging infrastructure
- Creates database registry
- Selects execution mode (REPL or Server)
- Handles graceful shutdown and data persistence

**Why it exists**: Provides a clean entry point and separates application concerns from business logic.

---

### 2. Interface Layer
**Location**: `internal/repl`, `internal/network`

**Responsibility**: User interaction interfaces

**What it does**:
- **REPL**: Interactive command-line interface for SQL queries
- **Network**: TCP server accepting JSON-formatted SQL queries

**Why it exists**: Supports both interactive development (REPL) and programmatic access (TCP server) without duplicating query execution logic.

**Interaction**: Both interfaces delegate SQL execution to the Engine layer.

---

### 3. Engine Layer
**Location**: `internal/engine`

**Responsibility**: Query execution orchestration

**What it does**:
- Receives SQL strings from interfaces
- Coordinates the execution pipeline: Lexer → Parser → Planner → Executor
- Handles database management commands (CREATE DATABASE, USE, DROP DATABASE)
- Manages database context (currently active database)
- Returns formatted results

**Why it exists**: Provides a single entry point for SQL execution, hiding the complexity of the multi-stage pipeline.

**Key component**: `Engine.Execute(sql string) (*Result, error)`

---

### 4. Parser Layer
**Location**: `internal/parser` (including `lexer/` and `ast/`)

**Responsibility**: SQL text → Abstract Syntax Tree (AST) conversion

**What it does**:
- **Lexer**: Tokenizes SQL text into tokens (keywords, identifiers, operators, literals)
- **Parser**: Builds AST from tokens using recursive descent parsing
- **AST**: Defines node types representing SQL constructs (SelectStatement, BinaryExpression, etc.)

**Why it exists**: Separates syntax analysis from execution, enabling easier testing

**Design pattern**: Recursive descent parser with operator precedence climbing for expressions.

---

### 5. Planning Layer
**Location**: `internal/planner`, `internal/plan`

**Responsibility**: AST → Execution Plan conversion

**What it does**:
- Validates table and column existence
- Converts AST expressions to predicate functions
- Performs type conversion and validation
- Builds typed plan nodes (SelectNode, InsertNode, UpdateNode, DeleteNode)
- Resolves column references and builds projections

**Why it exists**: Separates validation and optimization from parsing and execution. Converts declarative AST into executable plan nodes.

**Key transformation**: AST (syntax tree) → Plan Nodes (execution instructions)

---

### 6. Execution Layer
**Location**: `internal/executor`

**Responsibility**: Execute query plans against the database

**What it does**:
- Dispatches plan nodes to appropriate executors
- Executes SELECT, INSERT, UPDATE, DELETE operations
- Handles JOIN operations
- Formats results for return to user

**Why it exists**: Separates execution logic from planning. Each executor focuses on one operation type.

**Design pattern**: Strategy pattern - different executors for different plan node types.

---

### 7. Query Operations Layer
**Location**: `internal/query`

**Responsibility**: Low-level database operations

**What it does**:
- **CRUD**: Insert, Select, Update, Delete operations on tables
- **JOIN**: Implements INNER, LEFT, RIGHT, FULL OUTER join algorithms
- **Projection**: Column selection and filtering
- **Indexing**: Index building and management for performance

**Why it exists**: Provides reusable operations independent of SQL syntax. Can be used by executors or directly by other components.

**Design principle**: Pure functions operating on domain entities (Tables, Rows).

---

### 8. Domain Layer
**Location**: `internal/domain`

**Responsibility**: Core business entities and rules

**What it does**:
- **Schema**: Defines Database, Table, Column structures and types
- **Data**: Represents rows as `map[string]interface{}`
- **Errors**: Custom error types for domain violations (ConstraintError, ValidationError, etc.)

**Why it exists**: Rich domain model - Tables have CRUD methods

**Design pattern**: Rich Domain Model - entities contain both data and behavior.

---

### 9. Storage Layer
**Location**: `internal/storage`

**Responsibility**: Data persistence and retrieval

**What it does**:
- **Loader**: Reads database/table metadata and data from JSON files
- **Writer**: Persists in-memory data to disk
- **Manager/Registry**: Manages loaded databases with lazy loading and caching
- **Metadata**: Handles schema serialization/deserialization
- **Bootstrap**: Creates new databases and tables

**Why it exists**: Separates persistence concerns from business logic. Enables easy swapping of storage backends (currently JSON, could be binary etc.).

**File structure**:
```
databases/
├── mydb/
│   ├── users/
│   │   ├── meta.json    (table schema)
│   │   └── data.json    (table rows)
│   └── orders/
│       ├── meta.json
│       └── data.json
```

---

### 10. Supporting Layers

#### Validation (`internal/validation`)
- Row-level validation against schema constraints
- Type checking and conversion

#### Indexing (`internal/index`)
- Index data structures (hash maps for unique indexes)
- Index building and maintenance

#### Infrastructure (`internal/infrastructure`)
- Structured logging with `slog`
- Configuration management

#### Utilities (`internal/util`)
- Type conversion utilities
- Value comparison functions
- Shared helper functions

**Why they exist**: Cross-cutting concerns that don't belong to a specific layer. Prevents code duplication and circular dependencies.

---

## Query Execution Flow

### Example: `SELECT * FROM users WHERE age > 18`

```
1. Interface Layer (REPL/Network)
   ↓ Receives SQL string
   
2. Engine Layer
   ↓ Orchestrates pipeline
   
3. Lexer
   ↓ Tokenizes: [SELECT, *, FROM, IDENTIFIER("users"), WHERE, ...]
   
4. Parser
   ↓ Builds AST: SelectStatement { Fields: [*], TableName: "users", Where: BinaryExpression {...} }
   
5. Planner
   ↓ Validates table exists, builds predicate function
   ↓ Creates: SelectNode { TableName: "users", Predicate: func(row) { return row["age"] > 18 }, ... }
   
6. Executor
   ↓ Dispatches to SELECT executor
   
7. Query Operations
   ↓ Calls: table.Select(predicate)
   
8. Domain Layer (Table)
   ↓ Acquires read lock
   ↓ Filters rows using predicate
   ↓ Returns matching rows
   
9. Executor
   ↓ Formats result
   
10. Engine
    ↓ Returns result to interface
    
11. Interface Layer
    ↓ Displays result to user
```

## Design Principles

### 1. Separation of Concerns
Each layer has a single, well-defined responsibility. Changes to one layer rarely affect others.

### 2. Dependency Direction
Dependencies flow downward: Interface → Engine → Parser → Planner → Executor → Query Operations → Domain → Storage

Higher layers depend on lower layers, never the reverse.

### 3. Immutability
AST nodes and Plan nodes are immutable once created. This simplifies reasoning and enables potential parallelization.

### 4. Rich Domain Model
Tables are not just data containers - they have methods for CRUD operations, validation, and constraint enforcement.

### 5. Type Safety
Strong typing throughout: AST types, Plan node types, Column types. Errors are caught early in the pipeline.

### 6. Testability
Each layer can be tested independently. Parser tests don't need a database. Executor tests use in-memory tables.

### 7. Concurrency Safety
Tables use `sync.RWMutex` for thread-safe operations. Multiple readers or single writer pattern.

## Key Design Decisions

### Why JSON for Storage?
- **Human-readable**: Easy to inspect and debug
- **Simple**: No binary format complexity
- **Portable**: Works across platforms
- **Trade-off**: Performance vs. simplicity (chose simplicity for this project)

### Why In-Memory Execution?
- **Speed**: No disk I/O during queries
- **Simplicity**: No buffer pool or page management
- **Trade-off**: Limited by RAM

### Why Separate Parser and Planner?
- **Flexibility**: Can parse SQL without a database (syntax checking)
- **Optimization**: Planner can optimize without re-parsing
- **Clarity**: Syntax analysis vs. semantic analysis are distinct concerns

### Why Rich Domain Model?
- **Encapsulation**: Business rules live with the data
- **Simplicity**: `table.Insert(row)` is clear
- **Maintainability**: Changes to table behavior are localized

## Performance Characteristics

### Strengths
- **Fast reads**: In-memory with optional indexing
- **Simple queries**: Minimal overhead for basic CRUD
- **Concurrent reads**: Multiple readers supported

### Limitations
- **Memory-bound**: All data must fit in RAM
- **Write performance**: Single writer per table
- **No query optimization**: Executes queries as written
- **No aggregate functions**: SUM, COUNT, etc. not yet implemented



## Related Documentation

- [README.md](README.md) - Getting started and usage guide
- [SQL_REFERENCE.md](SQL_REFERENCE.md) - Complete SQL syntax reference
- [internal/README.md](internal/README.md) - Internal architecture overview
- Layer-specific READMEs in `internal/` subdirectories
