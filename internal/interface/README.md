# Interface Layer

## What

The Interface Layer provides **user interaction modes** for JoyDB. It offers two ways to interact with the database:

1. **REPL (Read-Eval-Print Loop)**: Interactive command-line interface for direct SQL query execution
2. **Network Server**: TCP-based server accepting JSON-formatted SQL queries for programmatic access

Both interfaces delegate SQL execution to the Engine layer, providing a clean separation between user interaction and query processing.

## Why

### Design Rationale



**Why separate from Engine?**
- Allows adding new interfaces (HTTP API, WebSocket, etc.) without modifying core logic
- Simplifies testing - Engine can be tested without UI concerns
- Enables different output formats (table display vs. JSON) for the same queries

**Why JSON for network protocol?**
- Simple and widely supported
- Human-readable for debugging
- Easy to integrate with any programming language

## How

### REPL Mode

**Location**: `internal/repl/repl.go`

**Mechanism**:
1. Starts an interactive prompt
2. Reads SQL input line-by-line
3. Passes SQL to Engine for execution
4. Formats and displays results in table format
5. Handles special commands (`exit`, `\q`)

**Key Features**:
- Line-based input (no multi-line support currently)
- Pretty-printed table output
- Error messages displayed inline
- Graceful exit handling

**Usage**:
```bash
./joydb
# or
make repl
```

**Example Session**:
```
> USE mydb;
Switched to database 'mydb'

> SELECT * FROM users WHERE age > 18;
Returned 3 rows
id   username   age   email
---  --------   ---   -----
1    alice      25    alice@example.com
2    bob        30    bob@example.com
5    eve        22    eve@example.com

> exit
```

---

### Network Server Mode

**Location**: `internal/network/server.go`

**Mechanism**:
1. Listens on TCP port (default: 4444)
2. Accepts client connections
3. Reads JSON-formatted requests
4. Executes SQL via Engine
5. Returns JSON-formatted responses
6. Maintains database context per connection

**Protocol**:

**Request Format**:
```json
{"query": "SELECT * FROM users WHERE id = 5"}
```

**Response Format**:
```json
{
  "Columns": ["id", "username", "email"],
  "Rows": [
    {"id": 5, "username": "alice", "email": "alice@example.com"}
  ],
  "Error": ""
}
```

**Error Response**:
```json
{
  "Columns": null,
  "Rows": null,
  "Error": "table not found: invalid_table"
}
```

**Usage**:
```bash
./joydb --server --port 4444
# or
make server
```

**Client Example** (using `nc`):
```bash
echo '{"query": "SELECT * FROM users"}' | nc localhost 4444
```

**Client Example** (Go):
```go
conn, _ := net.Dial("tcp", "localhost:4444")
defer conn.Close()

// Send query
request := map[string]string{"query": "SELECT * FROM users"}
json.NewEncoder(conn).Encode(request)

// Read response
var response struct {
    Columns []string
    Rows    []map[string]interface{}
    Error   string
}
json.NewDecoder(conn).Decode(&response)
```

## Interactions

### With Engine Layer

Both interfaces interact with the Engine in the same way:

```go
// Create engine with database registry
engine := engine.New(currentDB, registry)

// Execute SQL
result, err := engine.Execute(sqlQuery)

// Handle result
if err != nil {
    // Display error
} else {
    // Display result.Rows, result.Columns, or result.Message
}
```

**Key Points**:
- Interfaces don't parse SQL - they delegate to Engine
- Interfaces don't access database directly - they go through Engine
- Database context (current database) is managed by Engine

### With Storage Layer

Indirectly through Engine:
- REPL/Network → Engine → Storage (for database management commands)
- Database registry is shared across connections

## Key Components

### REPL (`repl/repl.go`)

**Main Function**: `Start(registry *manager.Registry)`

**Responsibilities**:
- Read user input from stdin
- Detect special commands (`exit`, `\q`)
- Create Engine instance
- Execute queries and display results
- Handle graceful shutdown

**Output Formatting**:
- Tables displayed with column headers
- Row count shown for SELECT queries
- Success messages for INSERT/UPDATE/DELETE
- Error messages in red (if terminal supports colors)

---

### Network Server (`network/server.go`)

**Main Function**: `Start(port int, registry *manager.Registry)`

**Responsibilities**:
- Listen on TCP port
- Accept client connections
- Parse JSON requests
- Execute queries via Engine
- Encode JSON responses
- Handle connection errors

**Connection Handling**:
- One goroutine per connection
- Each connection maintains its own database context
- Connections are independent (no shared state except registry)

**Error Handling**:
- Malformed JSON → Error response
- SQL errors → Error in response.Error field
- Connection errors → Close connection

## Design Decisions

### Why No Multi-Line Support in REPL?
**Trade-off**: Simplicity vs. convenience
- Current: Simple line-based input
- Future: Could add multi-line with `;` terminator or special mode

### Why Newline-Delimited JSON?
**Trade-off**: Simplicity vs. efficiency
- Each request/response is a single JSON object per line
- Easy to parse and debug
- Could use streaming JSON or binary protocol for better performance

### Why No Authentication?
**Current limitation**: No user authentication or authorization
- Suitable for local development
- Future: Could add authentication layer

### Why No Connection Pooling?
**Current design**: One connection per client
- Simple and sufficient for current use case
- Future: Could add connection pooling for better resource management

## Limitations

### Current Limitations
1. **No multi-line queries** in REPL
2. **No authentication** in network mode
3. **No TLS/SSL** for encrypted connections
4. **No query history** in REPL
5. **No tab completion** in REPL
6. **No connection pooling** in server mode
7. **No rate limiting** or resource quotas

### Future Enhancements
- **REPL improvements**: Multi-line input, history, tab completion, syntax highlighting
- **Network improvements**: TLS, authentication, connection pooling, HTTP API
- **Protocol improvements**: Binary protocol, streaming results, prepared statements

## Testing

### REPL Testing
Manual testing recommended:
```bash
./joydb
> CREATE DATABASE test;
> USE test;
> CREATE TABLE users (id INT, name TEXT);
> INSERT INTO users (id, name) VALUES (1, 'Alice');
> SELECT * FROM users;
```

### Network Testing
Use integration tests in `internal/integration_test/`:
```go
// Start server in test
go network.Start(testPort, registry)

// Connect and send queries
conn, _ := net.Dial("tcp", fmt.Sprintf("localhost:%d", testPort))
// ... send queries and verify responses
```

## Related Documentation

- [Engine Layer](../engine/README.md) - Query execution orchestrator (coming soon)
- [ARCHITECTURE.md](../../ARCHITECTURE.md) - System architecture overview
- [SQL_REFERENCE.md](../../SQL_REFERENCE.md) - Supported SQL syntax
