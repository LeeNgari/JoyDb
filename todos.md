# Project TODOs and Tasks

This document tracks `TODO` comments and tasks identified in the codebase that are left to be done later.

## High Priority / Core Features



### 3. Transaction ID Consolidation
- **Location**: `internal/domain/transaction/transaction.go:34`
- **Task**: Phase out the UUID-based `ID` string in favor of the numeric `TxID` (uint64) required for WAL integration.
- **Context**: `ID string // Unique transaction identifier (UUID - to be phased out)`

---

## Performance & Scalability

### 4. Row-Level Locking
- **Location**: `internal/domain/data/row.go:12`
- **Task**: Implement actual row-level locking. Currently, there is only a placeholder mutex.
- **Context**: `// mu is a placeholder for future row-level locking implementation`

---

## Reliability & Crash Recovery

### 5. WAL Recovery Improvements
- **Location**: `internal/storage/manager/wal_crash_test.go:256`
- **Task**: Improve `RecoverFromScratch` to "recover as much as possible" from a corrupted WAL file instead of failing early.
- **Context**: `// This needs to be improved if we want "recover as much as possible".`

### 6. Optimized State Recovery
- **Location**: `internal/wal/state_recovery.go:59`
- **Task**: Optimize the WAL state recovery implementation. Currently, it just seeks to the end.

---

## SQL Features & Constraints

### 7. Foreign Key Support
- **Location**: `internal/domain/errors/constraint.go:9`
- **Task**: Implement foreign key constraints and validation.
- **Context**: `// (unique, primary key, not null, type mismatch, foreign key later, etc.)`

---
