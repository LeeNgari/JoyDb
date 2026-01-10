package main

import (
	"log/slog"
	"os"

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

	// 5. Demo: Insert new users (only if they don't exist)
	// Note: do NOT provide "id" if it's auto-increment!
	demoUsers := []data.Row{
		{
			"username":  "frank",
			"email":     "frank@newuser.com",
			"is_active": true,
		},
		{
			"username":  "grace",
			"email":     "grace@secure.mail",
			"is_active": false,
		},
	}

	slog.Info("Demo: Attempting to insert test users...")
	for i, row := range demoUsers {
		err := operations.Insert(usersTable, row)
		if err != nil {
			// Check if it's a duplicate - that's okay, just skip
			slog.Warn("skipping user insert (likely already exists)",
				"index", i+1,
				"username", row["username"],
				"reason", err.Error(),
			)
			continue // Don't exit, just skip this user
		}
		insertedRow := usersTable.Rows[len(usersTable.Rows)-1]

		slog.Info("successfully inserted user",
			"username", insertedRow["username"],
			"email", insertedRow["email"],
			"new_id", insertedRow["id"],
		)
	}

	// 6. Select All (after inserts)
	allRows := operations.SelectAll(usersTable)
	slog.Info("all rows after insert",
		"count", len(allRows),
		"rows", allRows,
	)

	// 7. Select with Predicate (example)
	graceUser := operations.SelectWhere(usersTable, func(r data.Row) bool {
		return r["username"] == "grace"
	})
	slog.Info("found grace", "results", graceUser)

	// 8. Select by Unique Index (using the new auto-generated IDs)
	if row, found := operations.SelectByUniqueIndex(usersTable, "id", 6); found {
		slog.Info("found user by id=6 (first new insert)", "data", row)
	} else {
		slog.Warn("user id=6 not found")
	}

	slog.Info("Application ready")
}
