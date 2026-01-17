package main

import (
"flag"
"fmt"
"io"
"log/slog"
"os"
"path/filepath"
"time"

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
seedPath := "seed_data"

// Ensure database directory exists
if err := os.MkdirAll(basePath, 0755); err != nil {
slog.Error("failed to create databases directory", "error", err)
os.Exit(1)
}

// Check if main database exists, if not and seed data exists, copy it
if err := ensureDatabaseSeeded(basePath, seedPath, "main"); err != nil {
slog.Error("failed to seed database", "error", err)
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

func ensureDatabaseSeeded(basePath, seedPath, dbName string) error {
targetDir := filepath.Join(basePath, dbName)
sourceDir := filepath.Join(seedPath, dbName)

// Check if target exists
if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
return nil // Already exists
}

// Check if source exists
if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
return nil // No seed data
}

slog.Info("Seeding database...", "database", dbName)
return copyDir(sourceDir, targetDir)
}

func copyDir(src, dst string) error {
return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
if err != nil {
return err
}

relPath, err := filepath.Rel(src, path)
if err != nil {
return err
}

dstPath := filepath.Join(dst, relPath)

if info.IsDir() {
return os.MkdirAll(dstPath, info.Mode())
}

srcFile, err := os.Open(path)
if err != nil {
return err
}
defer srcFile.Close()

dstFile, err := os.Create(dstPath)
if err != nil {
return err
}
defer dstFile.Close()

_, err = io.Copy(dstFile, srcFile)
return err
})
}
