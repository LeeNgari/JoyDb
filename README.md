# JoyDB

JoyDB is a lightweight, in-memory Relational Database Management System (RDBMS) written in Go. It supports a subset of standard SQL, including filtering, JOIN operations, and persistent storage via JSON files.

## Features

- **SQL Support**: SELECT, INSERT, UPDATE, DELETE, JOIN (INNER, LEFT, RIGHT, FULL).
- **In-Memory Execution**: Fast query processing with in-memory data structures.
- **Persistence**: Data is persisted to disk in JSON format, making it human-readable and easy to debug.
- **REPL**: Interactive Read-Eval-Print Loop for direct database interaction.
- **TCP Server**: Server mode for handling remote connections.

## Example Usage

This RDBMS is used as the backend database for the **Bliss Ecommerce Dashboard**.
Check out the project here: [https://github.com/LeeNgari/bliss-pesapal.git](https://github.com/LeeNgari/bliss-pesapal.git)

## Architecture Overview

JoyDB follows a layered architecture that separates concerns and provides a clear query execution pipeline:

```
User Interfaces (REPL / TCP Server)
         ↓
    Engine (Orchestration)
         ↓
    Parser (SQL → AST)
         ↓
   Planner (AST → Execution Plan)
         ↓
  Executor (Plan → Operations)
         ↓
Query Operations (CRUD, JOIN)
         ↓
    Domain (Tables, Rows)
         ↓
   Storage (JSON Persistence)
```

**Key Design Principles**:
- **Separation of Concerns**: Each layer has a single, well-defined responsibility
- **Dependency Direction**: Dependencies flow downward (higher layers depend on lower layers)
- **Rich Domain Model**: Tables contain both data and behavior
- **Concurrency Safety**: Thread-safe operations using read/write locks

For a detailed explanation of the architecture, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Documentation

### User Documentation
- **[SQL Syntax Reference](SQL_REFERENCE.md)** - Complete guide to supported SQL statements, operators, and syntax
- **[Architecture Overview](ARCHITECTURE.md)** - High-level system design and component interactions

### Developer Documentation
- **[Internal Architecture](internal/README.md)** - Overview of internal components and navigation guide
- **Layer-Specific Documentation**:
  - [Interface Layer](internal/interface/README.md) - REPL and Network server
  - [Parser Layer](internal/parser/README.md) - SQL parsing (Lexer, Parser, AST)
  - [Planner Layer](internal/planner/README.md) - Query planning and validation
  - [Executor Layer](internal/executor/README.md) - Query execution
  - [Query Operations](internal/query/README.md) - CRUD and JOIN operations
  - [Domain Layer](internal/domain/README.md) - Core entities (Database, Table, Row)
  - [Storage Layer](internal/storage/README.md) - Data persistence
  - [Infrastructure](internal/infrastructure/README.md) - Logging and configuration
  - [Utilities](internal/util/README.md) - Shared helper functions
  - [Errors](internal/domain/errors/README.md) - Custom error types

## Getting Started

### Prerequisites

- Go 1.25 or higher
- Make (optional, for build commands)

### Building

To build the project from source, run:

```bash
make build
```

This will create a `joydb` binary in the root directory.

## Binary Usage

If you have downloaded the `joydb` binary directly:

1.  **Permissions**: Ensure the binary is executable.
    ```bash
    chmod +x joydb
    ```
2.  **Running**: You can run it directly from the terminal.
    ```bash
    ./joydb          # Starts REPL mode
    ./joydb --server # Starts Server mode
    ```

## Usage

### 1. REPL Mode (Interactive)

The Read-Eval-Print Loop (REPL) allows you to interact with the database directly.

Start it with:
```bash
make repl
# OR
./joydb
```

**REPL Commands:**
- Type your SQL query and press Enter to execute.
- `ls`: List available databases.
- `ls tables`: List tables in the current database.
- `exit` or `\q`: Quit the REPL.

**Example Session:**
```sql
> ls;
> ls tables;
> USE main;
> SELECT * FROM users;
> SELECT * FROM orders;
> SELECT * FROM products;
```

### 2. Server Mode

Start the database server to accept TCP connections.

**Default Port**: `4444`

```bash
make server
# OR
./joydb --server
```

To specify a custom port (e.g., 54322):
```bash
./joydb --server --port 54322
```

### 3. Connecting from a Backend Server

JoyDB uses a simple TCP-based protocol for client-server communication.

#### Protocol Overview
- **Transport**: TCP
- **Format**: JSON (Newline Delimited)
- **Default Port**: `4444`

#### Connection Workflow
1.  **Establish Connection**: Open a TCP connection to the JoyDB server.
2.  **Select Database**: Send a `USE` command to select the active database.
3.  **Execute Queries**: Send SQL queries as JSON objects.

#### Request Format
```json
{"query": "SELECT * FROM users"}
```

#### Response Format
```json
{
  "Columns": ["id", "username"],
  "Rows": [{"id": 1, "username": "alice"}],
  "Error": ""
}
```

## Seed Data & Population

There are three ways to populate the database with data:

### 1. Automatic Seeding (Default)
When you run the `joydb` binary for the first time, it automatically creates and populates a `main` database if it doesn't exist. This database includes sample data (users, products, orders)

### 2. SQL INSERT Statements
You can use the REPL or a client connection to execute `INSERT` statements.
```sql
INSERT INTO users (id, name) VALUES (1, 'Alice');
```

### 3. Manual JSON Editing
Since JoyDB persists data as JSON, you can manually edit the files in the `databases/` directory.

1.  Create a directory for your database: `databases/mydb/`
2.  Create a table directory: `databases/mydb/users/`
3.  Create `meta.json` for the table definition.
4.  Create `data.json` for the rows.

**Example `meta.json`**:
```json
{
  "name": "users",
  "columns": [
    {"name": "id", "type": "INT", "primary_key": true},
    {"name": "name", "type": "TEXT"}
  ],
  "last_insert_id": 1,
  "row_count": 1
}
```

**Example `data.json`**:
```json
[
  {"id": 1, "name": "Alice"}
]
```

