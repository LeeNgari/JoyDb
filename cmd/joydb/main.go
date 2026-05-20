package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/infrastructure/logging"
	"github.com/leengari/mini-rdbms/internal/network"
	"github.com/leengari/mini-rdbms/internal/repl"
	"github.com/leengari/mini-rdbms/internal/storage/engine"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

func main() {
	serverMode := flag.Bool("server", false, "Run in server mode")
	port := flag.Int("port", 4444, "Port to listen on")
	noWAL := flag.Bool("no-wal", false, "Disable Write-Ahead Logging (reduces durability)")
	walSyncInterval := flag.Duration("wal-sync-interval", 0, "Interval between WAL fsyncs (0 = sync on every commit, e.g., '100ms', '1s')")
	checkpointInterval := flag.Duration("checkpoint-interval", 5*time.Second, "Interval between automatic checkpoints (e.g., '5s', '1m')")
	flag.Parse()

	logger, closeFn := logging.SetupLogger()
	defer closeFn()

	slog.SetDefault(logger)
	time.Sleep(1 * time.Second)
	fmt.Println("Starting JoyDB application...")

	// WAL is enabled by default, disabled with --no-wal flag
	walEnabled := !*noWAL
	if walEnabled {
		slog.Info("WAL enabled for crash recovery")
		if *walSyncInterval > 0 {
			// TODO: Wire this into WALManager for periodic fsync
			slog.Info("WAL sync interval configured", "interval", *walSyncInterval)
		}
		slog.Info("Checkpoint interval configured", "interval", *checkpointInterval)
	} else {
		slog.Warn("WAL disabled - data may be lost on crash")
	}

	// Base path for databases
	basePath := "databases"

	if err := os.MkdirAll(basePath, 0755); err != nil {
		slog.Error("failed to create databases directory", "error", err)
		os.Exit(1)
	}

	// Use Memory storage engine (binary snapshots)
	var storageEngine engine.StorageEngine = engine.NewMemoryEngine()
	slog.Info("Using Memory storage engine (binary snapshots)")

	registry := manager.NewRegistryWithWAL(basePath, storageEngine, walEnabled, *checkpointInterval)

	defer func() {
		slog.Info("Shutting down - saving databases...")
		tx := transaction.NewTransaction()
		defer tx.Close()
		registry.SaveAll(tx)
		registry.CloseAll()
	}()

	slog.Info("Application ready!", "base_path", basePath, "wal_enabled", walEnabled)

	if *serverMode {
		slog.Info("Starting Server mode...")
		network.Start(*port, registry)
	} else {
		slog.Info("Starting REPL mode...")
		repl.Start(registry)
	}
}
