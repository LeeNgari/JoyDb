package integration

import (
	"os"
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/query/indexing"
	"github.com/leengari/mini-rdbms/internal/storage/bootstrap"
	"github.com/leengari/mini-rdbms/internal/storage/loader"
	"github.com/leengari/mini-rdbms/internal/storage/writer"
)

const testDBPath = "../../databases/testdb_integration"

// setupTestDB creates a fresh test database for integration tests
func setupTestDB(t *testing.T) *schema.Database {
	t.Helper()

	// Clean up any existing test database
	os.RemoveAll(testDBPath)

	// Bootstrap fresh database
	if err := bootstrap.EnsureDatabase(testDBPath, "testdb_integration"); err != nil {
		t.Fatalf("Failed to bootstrap test database: %v", err)
	}

	// Load the database
	db, err := loader.LoadDatabase(testDBPath)
	if err != nil {
		t.Fatalf("Failed to load test database: %v", err)
	}

	// Build indexes
	if err := indexing.BuildDatabaseIndexes(db); err != nil {
		t.Fatalf("Failed to build indexes: %v", err)
	}

	return db
}

// teardownTestDB cleans up the test database
func teardownTestDB(t *testing.T, db *schema.Database) {
	t.Helper()

	// Save database before cleanup (optional, for debugging)
	tx := transaction.NewTransaction()
	defer tx.Close()
	if err := writer.SaveDatabase(db, tx); err != nil {
		t.Logf("Warning: Failed to save test database: %v", err)
	}

	// Remove test database directory
	if err := os.RemoveAll(testDBPath); err != nil {
		t.Logf("Warning: Failed to remove test database: %v", err)
	}
}
