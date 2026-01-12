package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/leengari/mini-rdbms/internal/infrastructure/logging"
	"github.com/leengari/mini-rdbms/internal/query/indexing"
	"github.com/leengari/mini-rdbms/internal/storage/loader"
	"github.com/leengari/mini-rdbms/internal/storage/writer"
	"github.com/leengari/mini-rdbms/internal/storage/bootstrap"
	"github.com/leengari/mini-rdbms/internal/repl"
)

func main() {
	logger, closeFn := logging.SetupLogger()
	defer closeFn()

	slog.SetDefault(logger)
	time.Sleep(1 * time.Second)
	slog.Info("Starting RDBMS application...")

	// Bootstrap database if not exists
	dbPath := "databases/testdb"
	if err := bootstrap.EnsureDatabase(dbPath); err != nil {
		slog.Error("failed to bootstrap database", "error", err)
		os.Exit(1)
	}

	// Load Database
	db, err := loader.LoadDatabase(dbPath)
	if err != nil {
		slog.Error("failed to load database", "error", err)
		closeFn()
		os.Exit(1)
	}

	// Save database on shutdown
	defer func() {
		slog.Info("Shutting down - saving database...")
		if err := writer.SaveDatabase(db); err != nil {
			slog.Error("shutdown save failed", "error", err)
		}
	}()

	// Build Indexes
	if err := indexing.BuildDatabaseIndexes(db); err != nil {
		slog.Error("Index building failed", "error", err)
		closeFn()
		os.Exit(1)
	}

	slog.Info("Database loaded successfully",
		"name", db.Name,
		"tables", len(db.Tables),
	)

	// Application is ready
	// Run integration tests with: go test ./internal/integration_test/...
	slog.Info("Application ready!")
	slog.Info("Starting REPL mode...")
	
	repl.Start(db)
}
