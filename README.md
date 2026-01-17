# JoyDB

JoyDB is a lightweight, in-memory Relational Database Management System (RDBMS) written in Go. It supports a subset of standard SQL, including complex filtering, JOIN operations, and persistent storage via JSON files.

## Features

- **SQL Support**: SELECT, INSERT, UPDATE, DELETE, JOIN (INNER, LEFT, RIGHT, FULL).
- **In-Memory Execution**: Fast query processing with in-memory data structures.
- **Persistence**: Data is persisted to disk in JSON format, making it human-readable and easy to debug.
- **ACID-compliant**: (Planned) Currently supports basic transaction isolation via table-level locking.
- **REPL**: Interactive Read-Eval-Print Loop for direct database interaction.
- **TCP Server**: Server mode for handling remote connections.

## Documentation

- [SQL Syntax Reference](SQL_REFERENCE.md) - Detailed guide on supported SQL statements.
- **Internal Architecture**:
  - [Parser](internal/parser/README.md)
  - [AST](internal/parser/ast/README.md)
  - [Lexer](internal/parser/lexer/README.md)
  - [Executor](internal/executor/README.md)
  - [Query Engine](internal/query/README.md)
  - [Errors](internal/domain/errors/README.md)
  - [Utilities](internal/util/README.md)

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
- `exit` or `\q`: Quit the REPL.

**Example Session:**
```sql
> CREATE DATABASE mydb;
> USE mydb;
> CREATE TABLE users (id INT, name TEXT);
> INSERT INTO users VALUES (1, 'Alice');
> SELECT * FROM users;
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

There are two ways to populate the database with data:

### 1. SQL INSERT Statements
You can use the REPL or a client connection to execute `INSERT` statements.
```sql
INSERT INTO users (id, name) VALUES (1, 'Alice');
```

### 2. Manual JSON Editing
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

## Project Structure

- `cmd/joydb`: Main entry point.
- `internal/engine`: Core database engine.
- `internal/parser`: SQL parser and lexer.
- `internal/storage`: Data persistence layer.
- `databases/`: Directory where database files are stored.

