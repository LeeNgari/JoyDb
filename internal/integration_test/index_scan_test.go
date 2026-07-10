package integration

import (
	"path/filepath"
	"testing"

	"github.com/leengari/mini-rdbms/internal/engine"
	storageEngine "github.com/leengari/mini-rdbms/internal/storage/engine"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

func TestSQLIndexScanQueries(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	storageEng := storageEngine.NewMemoryEngine()
	registry := manager.NewRegistry(filepath.Dir(db.Path), storageEng)
	eng := engine.New(db, registry)

	// Add more rows to users to test ranges better
	_, err := eng.Execute("INSERT INTO users (id, username, email, is_active) VALUES (3, 'user3', 'user3@test.com', true);")
	if err != nil {
		t.Fatalf("Failed to insert user 3: %v", err)
	}
	_, err = eng.Execute("INSERT INTO users (id, username, email, is_active) VALUES (4, 'user4', 'user4@test.com', false);")
	if err != nil {
		t.Fatalf("Failed to insert user 4: %v", err)
	}
	_, err = eng.Execute("INSERT INTO users (id, username, email, is_active) VALUES (5, 'user5', 'user5@test.com', true);")
	if err != nil {
		t.Fatalf("Failed to insert user 5: %v", err)
	}

	t.Run("INDEX POINT LOOKUP (=)", func(t *testing.T) {
		res, err := eng.Execute("SELECT username FROM users WHERE id = 3;")
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if len(res.Rows) != 1 {
			t.Fatalf("Expected 1 row, got %d", len(res.Rows))
		}
		if getRowMap(res.Rows[0], res.Columns)["username"] != "user3" {
			t.Errorf("Expected 'user3', got '%v'", getRowMap(res.Rows[0], res.Columns)["username"])
		}
	})

	t.Run("INDEX RANGE (>)", func(t *testing.T) {
		res, err := eng.Execute("SELECT username FROM users WHERE id > 3;")
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if len(res.Rows) != 2 {
			t.Fatalf("Expected 2 rows, got %d", len(res.Rows))
		}
		// Expect user4 (id 4), user5 (id 5)
		usernames := map[string]bool{
			getRowMap(res.Rows[0], res.Columns)["username"].(string): true,
			getRowMap(res.Rows[1], res.Columns)["username"].(string): true,
		}
		if !usernames["user4"] || !usernames["user5"] {
			t.Errorf("Expected user4 and user5, got %v", usernames)
		}
	})

	t.Run("INDEX RANGE (>=)", func(t *testing.T) {
		res, err := eng.Execute("SELECT username FROM users WHERE id >= 3;")
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if len(res.Rows) != 3 {
			t.Fatalf("Expected 3 rows, got %d", len(res.Rows))
		}
		usernames := map[string]bool{
			getRowMap(res.Rows[0], res.Columns)["username"].(string): true,
			getRowMap(res.Rows[1], res.Columns)["username"].(string): true,
			getRowMap(res.Rows[2], res.Columns)["username"].(string): true,
		}
		if !usernames["user3"] || !usernames["user4"] || !usernames["user5"] {
			t.Errorf("Expected user3, user4, and user5, got %v", usernames)
		}
	})

	t.Run("INDEX RANGE (<)", func(t *testing.T) {
		res, err := eng.Execute("SELECT username FROM users WHERE id < 3;")
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if len(res.Rows) != 2 {
			t.Fatalf("Expected 2 rows, got %d", len(res.Rows))
		}
		usernames := map[string]bool{
			getRowMap(res.Rows[0], res.Columns)["username"].(string): true,
			getRowMap(res.Rows[1], res.Columns)["username"].(string): true,
		}
		if !usernames["admin"] || !usernames["guest"] {
			t.Errorf("Expected admin and guest, got %v", usernames)
		}
	})

	t.Run("INDEX RANGE (<=)", func(t *testing.T) {
		res, err := eng.Execute("SELECT username FROM users WHERE id <= 3;")
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		if len(res.Rows) != 3 {
			t.Fatalf("Expected 3 rows, got %d", len(res.Rows))
		}
		usernames := map[string]bool{
			getRowMap(res.Rows[0], res.Columns)["username"].(string): true,
			getRowMap(res.Rows[1], res.Columns)["username"].(string): true,
			getRowMap(res.Rows[2], res.Columns)["username"].(string): true,
		}
		if !usernames["admin"] || !usernames["guest"] || !usernames["user3"] {
			t.Errorf("Expected admin, guest, and user3, got %v", usernames)
		}
	})
}
