package integration

import (
	"path/filepath"
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/query/indexing"
	storageEngine "github.com/leengari/mini-rdbms/internal/storage/engine"
)

// setupTestDB creates a fresh test database for integration tests
func setupTestDB(t *testing.T) *schema.Database {
	t.Helper()

	basePath := t.TempDir()
	dbName := "testdb_integration"
	testDBPath := filepath.Join(basePath, dbName)

	engine := storageEngine.NewMemoryEngine()
	if err := engine.CreateDatabase(dbName, basePath); err != nil {
		t.Fatalf("Failed to create test database directory: %v", err)
	}

	db := &schema.Database{
		Name:   "testdb_integration",
		Path:   testDBPath,
		Tables: make(map[string]*schema.Table),
	}

	usersTable := &schema.Table{
		Name: "users",
		Schema: &schema.TableSchema{
			TableName: "users",
			Columns: []schema.Column{
				{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true, Unique: true, NotNull: true, AutoIncrement: true},
				{Name: "username", Type: schema.ColumnTypeText, PrimaryKey: false, Unique: true, NotNull: true},
				{Name: "email", Type: schema.ColumnTypeText, PrimaryKey: false, Unique: true, NotNull: true},
				{Name: "is_active", Type: schema.ColumnTypeBool, PrimaryKey: false, Unique: false, NotNull: false},
			},
		},
		LastInsertID: 2,
		Rows: []data.Row{
			{Data: map[string]interface{}{"id": int64(1), "username": "admin", "email": "admin@example.com", "is_active": true}},
			{Data: map[string]interface{}{"id": int64(2), "username": "guest", "email": "guest@example.com", "is_active": false}},
		},
	}
	usersTable.Path = testDBPath
	db.Tables["users"] = usersTable

	ordersTable := &schema.Table{
		Name: "orders",
		Schema: &schema.TableSchema{
			TableName: "orders",
			Columns: []schema.Column{
				{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true, Unique: true, NotNull: true, AutoIncrement: true},
				{Name: "user_id", Type: schema.ColumnTypeInt, PrimaryKey: false, Unique: false, NotNull: true},
				{Name: "product", Type: schema.ColumnTypeText, PrimaryKey: false, Unique: false, NotNull: true},
				{Name: "amount", Type: schema.ColumnTypeFloat, PrimaryKey: false, Unique: false, NotNull: true},
			},
		},
		LastInsertID: 2,
		Rows: []data.Row{
			{Data: map[string]interface{}{"id": int64(1), "user_id": int64(1), "product": "Laptop", "amount": float64(1200.50)}},
			{Data: map[string]interface{}{"id": int64(2), "user_id": int64(1), "product": "Mouse", "amount": float64(25.99)}},
		},
	}
	ordersTable.Path = testDBPath
	db.Tables["orders"] = ordersTable

	// Save the database snapshot so engine can load it later
	if _, _, err := engine.CreateSnapshot(db, testDBPath); err != nil {
		t.Fatalf("Failed to create test database snapshot: %v", err)
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
	engine := storageEngine.NewMemoryEngine()
	engine.CreateSnapshot(db, db.Path)
}
