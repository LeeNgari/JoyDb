package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/infrastructure/logging"
	"github.com/leengari/mini-rdbms/internal/query/indexing"
	"github.com/leengari/mini-rdbms/internal/query/operations"
	"github.com/leengari/mini-rdbms/internal/storage/loader"
	"github.com/leengari/mini-rdbms/internal/storage/writer"
)

func main() {
	logger, closeFn := logging.SetupLogger()
	defer closeFn()

	// Set as default logger for entire application
	slog.SetDefault(logger)
	time.Sleep(1 * time.Second)
	slog.Info("Starting application...")

	// 1. Load Database
	db, err := loader.LoadDatabase("databases/testdb")
	if err != nil {
		slog.Error("failed to load database", "error", err)
		closeFn()
		os.Exit(1)
	}

	// 2. Save database and all dirty tables on shutdown
	defer func() {
		slog.Info("Shutting down - saving database...")
		if err := writer.SaveDatabase(db); err != nil {
			slog.Error("shutdown save failed", "error", err)
		}
	}()

	// 3. Build Indexes (after loading rows)
	if err := indexing.BuildDatabaseIndexes(db); err != nil {
		slog.Error("Index building failed", "error", err)
		closeFn()
		os.Exit(1)
	}

	// 4. Get users table
	usersTable, ok := db.Tables["users"]
	if !ok {
		slog.Error("table 'users' not found")
		closeFn()
		os.Exit(1)
	}

	slog.Info("=== Testing CRUD Operations ===")

	// 5. SELECT All (before operations)
	allRows := operations.SelectAll(usersTable)
	slog.Info("Initial row count", "count", len(allRows))

	// 6. Test UPDATE operation
	slog.Info("=== Testing UPDATE ===")
	
	// Update alice's email
	updated, err := operations.Update(usersTable, func(r data.Row) bool {
		return r["username"] == "alice"
	}, data.Row{
		"email":     "alice.updated@example.com",
		"is_active": false,
	})
	if err != nil {
		slog.Error("UPDATE failed", "error", err)
	} else {
		slog.Info("UPDATE successful", "rows_updated", updated)
		
		// Verify update
		if row, found := operations.SelectByUniqueIndex(usersTable, "username", "alice"); found {
			slog.Info("Verified alice's updated data", 
				"email", row["email"],
				"is_active", row["is_active"],
			)
		}
	}

	// 7. Test UPDATE by ID
	slog.Info("=== Testing UPDATE by ID ===")
	err = operations.UpdateByID(usersTable, int64(2), data.Row{
		"email": "bob.new@example.com",
	})
	if err != nil {
		slog.Error("UPDATE by ID failed", "error", err)
	} else {
		slog.Info("UPDATE by ID successful")
		if row, found := operations.SelectByUniqueIndex(usersTable, "id", int64(2)); found {
			slog.Info("Verified bob's updated email", "email", row["email"])
		}
	}

	// 8. Test DELETE operation
	slog.Info("=== Testing DELETE ===")
	
	// Delete inactive users
	deleted, err := operations.Delete(usersTable, func(r data.Row) bool {
		isActive, ok := r["is_active"].(bool)
		return ok && !isActive
	})
	if err != nil {
		slog.Error("DELETE failed", "error", err)
	} else {
		slog.Info("DELETE successful", "rows_deleted", deleted)
	}

	// 9. Test DELETE by ID
	slog.Info("=== Testing DELETE by ID ===")
	err = operations.DeleteByID(usersTable, int64(3))
	if err != nil {
		slog.Error("DELETE by ID failed", "error", err)
	} else {
		slog.Info("DELETE by ID successful (charlie deleted)")
	}

	// 10. Final SELECT to show remaining rows
	finalRows := operations.SelectAll(usersTable)
	slog.Info("Final row count after operations", "count", len(finalRows))
	
	for _, row := range finalRows {
		slog.Info("Remaining user",
			"id", row["id"],
			"username", row["username"],
			"email", row["email"],
			"is_active", row["is_active"],
		)
	}

	slog.Info("Application ready - all CRUD operations tested!")
}
