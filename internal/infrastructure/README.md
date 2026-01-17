# Infrastructure Layer

## What

The Infrastructure Layer provides **cross-cutting technical concerns** that support the entire application. Currently, it focuses on structured logging.

**Components**:
- **Logging** (`infrastructure/logging/`): Structured logging configuration using `slog`

## Why

**Why separate infrastructure from business logic?**
- **Separation of Concerns**: Technical infrastructure doesn't belong in domain or application layers
- **Reusability**: Logging configuration can be reused across all components
- **Testability**: Can mock or disable logging for tests
- **Maintainability**: Changes to logging don't affect business logic

## How

### Logging

**Location**: `infrastructure/logging/logger.go`

**Purpose**: Configure structured logging with `slog`

**Setup**:
```go
logger, closeFn := logging.SetupLogger()
defer closeFn()

slog.SetDefault(logger)
```

**Usage Throughout Application**:
```go
import "log/slog"

slog.Info("Starting server", "port", 4444)
slog.Error("Failed to load table", "table", "users", "error", err)
slog.Debug("Query executed", "sql", query, "duration_ms", duration)
```

**Log Levels**:
- `DEBUG`: Detailed diagnostic information
- `INFO`: General informational messages
- `WARN`: Warning messages
- `ERROR`: Error messages

**Structured Fields**:
```go
slog.Info("message", "key1", value1, "key2", value2)
// Output: time=2024-01-17T12:00:00 level=INFO msg=message key1=value1 key2=value2
```

## Design Decisions

**Why slog instead of other logging libraries?**
- **Standard library**: No external dependencies
- **Structured logging**: Key-value pairs for better parsing
- **Performance**: Fast and efficient
- **Modern**: Introduced in Go 1.21

## Future Enhancements

- **Configuration management**: Environment-based configuration
- **Metrics**: Prometheus metrics for monitoring
- **Tracing**: Distributed tracing support
- **Health checks**: Application health endpoints

## Related Documentation

- [ARCHITECTURE.md](../../ARCHITECTURE.md) - System architecture overview
