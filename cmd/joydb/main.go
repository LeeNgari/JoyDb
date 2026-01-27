package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/leengari/mini-rdbms/databases"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/infrastructure/logging"
	"github.com/leengari/mini-rdbms/internal/network"
	"github.com/leengari/mini-rdbms/internal/repl"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

func main() {
	serverMode := flag.Bool("server", false, "Run in server mode")
	port := flag.Int("port", 4444, "Port to listen on")
	flag.Parse()

	logger, closeFn := logging.SetupLogger()
	defer closeFn()

	slog.SetDefault(logger)
	time.Sleep(1 * time.Second)
	fmt.Println("Starting JoyDB application...")

	// Base path for databases
	basePath := "databases"

	// Ensure database directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		slog.Error("failed to create databases directory", "error", err)
		os.Exit(1)
	}

	// Create Database Registry
	registry := manager.NewRegistry(basePath)

	// Save all loaded databases on shutdown
	defer func() {
		slog.Info("Shutting down - saving databases...")
		tx := transaction.NewTransaction()
		defer tx.Close()
		registry.SaveAll(tx)
	}()

	// Seed 'main' from embedded FS
	if err := ensureDatabaseSeeded(basePath, databases.Content, "main"); err != nil {
		slog.Error("Failed to seed main database", "error", err)
	}

	slog.Info("Application ready!", "base_path", basePath)

	if *serverMode {
		slog.Info("Starting Server mode...")
		network.Start(*port, registry)
	} else {
		slog.Info("Starting REPL mode...")
		repl.Start(registry)
	}
}

func ensureDatabaseSeeded(basePath string, seedFS fs.FS, dbName string) error {
	targetDir := filepath.Join(basePath, dbName)

	// Check if target exists
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		return nil // Already exists
	}

	slog.Info("Seeding database...", "database", dbName)

	// Walk the embedded filesystem
	return fs.WalkDir(seedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root "."
		if path == "." {
			return nil
		}

		// Calculate target path
		// Note: embedded paths will be like "main/meta.json"
		// We want to extract "main/..." to "databases/main/..."
		// Since we passed "databases.Content" which contains "main", the paths start with "main"
		
		// If we are seeding "main", and the FS has "main/...", we can just join basePath and path
		targetPath := filepath.Join(basePath, path)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// Read from embedded FS
		data, err := fs.ReadFile(seedFS, path)
		if err != nil {
			return err
		}

		// Write to disk
		return os.WriteFile(targetPath, data, 0644)
	})
}
