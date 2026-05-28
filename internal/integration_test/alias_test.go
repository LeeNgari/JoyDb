package integration

import (
	"path/filepath"
	"testing"

	"github.com/leengari/mini-rdbms/internal/engine"
	storageEngine "github.com/leengari/mini-rdbms/internal/storage/engine"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

func TestSQLAliases(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	storageEng := storageEngine.NewMemoryEngine()
	registry := manager.NewRegistry(filepath.Dir(db.Path), storageEng)
	eng := engine.New(db, registry)

	t.Run("Table and Column Alias", func(t *testing.T) {
		query := "SELECT u.username AS name, u.email AS mail FROM users AS u WHERE u.id = 1;"
		res, err := eng.Execute(query)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(res.Rows) != 1 {
			t.Fatalf("Expected 1 row, got %d", len(res.Rows))
		}

		// Verify column names match aliases
		if len(res.Columns) != 2 || res.Columns[0] != "name" || res.Columns[1] != "mail" {
			t.Errorf("Expected columns ['name', 'mail'], got %v", res.Columns)
		}

		// Verify values
		row := res.Rows[0]
		if row.Data["name"] != "admin" || row.Data["mail"] != "admin@example.com" {
			t.Errorf("Values mismatch: %+v", row.Data)
		}
	})

	t.Run("JOIN with Table Aliases", func(t *testing.T) {
		query := `
			SELECT u.username AS user, o.product AS prod
			FROM users AS u
			INNER JOIN orders AS o ON u.id = o.user_id;
		`
		res, err := eng.Execute(query)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(res.Rows) != 2 {
			t.Fatalf("Expected 2 rows, got %d", len(res.Rows))
		}

		if len(res.Columns) != 2 || res.Columns[0] != "user" || res.Columns[1] != "prod" {
			t.Errorf("Expected columns ['user', 'prod'], got %v", res.Columns)
		}

		// Expect Laptop (user admin) and Mouse (user admin)
		row0 := res.Rows[0]
		row1 := res.Rows[1]
		if row0.Data["user"] != "admin" || row1.Data["user"] != "admin" {
			t.Errorf("Expected users to be admin, got %v and %v", row0.Data["user"], row1.Data["user"])
		}
	})

	t.Run("Invalid Table Alias Reference", func(t *testing.T) {
		// x.username is invalid since we only aliased users AS u
		query := "SELECT x.username FROM users AS u;"
		_, err := eng.Execute(query)
		if err == nil {
			t.Error("Expected error for invalid table alias reference, got nil")
		}
	})
}
