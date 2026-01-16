package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/leengari/mini-rdbms/internal/infrastructure/logging"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
	"flag"

	"github.com/leengari/mini-rdbms/internal/repl"
	"github.com/leengari/mini-rdbms/internal/network"
)

func main() {
	serverMode := flag.Bool("server", false, "Run in server mode")
	port := flag.Int("port", 4444, "Port to listen on")
	flag.Parse()

	logger, closeFn := logging.SetupLogger()
	defer closeFn()

	slog.SetDefault(logger)
	time.Sleep(1 * time.Second)
	slog.Info("Starting RDBMS application...")

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
		registry.SaveAll()
	}()

	// Application is ready
	// Run integration tests with: go test ./internal/integration_test/...
	slog.Info("Application ready!", "base_path", basePath)

	if *serverMode {
		slog.Info("Starting Server mode...")
		network.Start(*port, registry)
	} else {
		slog.Info("Starting REPL mode...")
		repl.Start(registry)
	}
}
