# Executor Package

The executor package is responsible for executing **query plans** against the database. It acts as the final stage of the query engine pipeline.

## Architecture

```
Plan Node → Executor → Query Operations → Database
```

The executor:
1. Receives **Plan Nodes** from the planner (not AST)
2. Executes the described operation using the `domain` or `query/operations` packages
3. Returns formatted results

## File Organization

| File | Responsibility |
|------|---------------|
| `executor.go` | Main entry point, Execute() dispatcher |
| `select_executor.go` | SELECT execution logic |
| `insert_executor.go` | INSERT execution logic |
| `update_executor.go` | UPDATE execution logic |
| `delete_executor.go` | DELETE execution logic |
| `join_executor.go` | JOIN execution logic |

## Usage

The executor is typically called by the `engine` package, which orchestrates the pipeline.

```go
import (
    "github.com/leengari/mini-rdbms/internal/executor"
    "github.com/leengari/mini-rdbms/internal/planner"
)

// ... obtain AST ...

// Create Execution Plan
planNode, err := planner.Plan(astStmt, database)

// Execute Plan
result, err := executor.Execute(planNode, database)
```

## Statement Execution Flow

### SELECT
```
Plan SelectNode
  ↓
select_executor.go
  ↓
crud.SelectWhere() or join.ExecuteJoin() (using pre-built predicate)
  ↓
Result with Rows
```

### INSERT
```
Plan InsertNode
  ↓
insert_executor.go
  ↓
crud.Insert() (using pre-converted values)
  ↓
Result with Message
```

## Adding a New Statement Executor

To add support for a new operation (e.g., `TRUNCATE`):

1.  Add `TruncateNode` to `internal/plan`.
2.  Update `internal/planner` to convert AST to `TruncateNode`.
3.  Add `truncate_executor.go` in `internal/executor` that accepts `TruncateNode`.
4.  Update `executor.go` dispatcher.
