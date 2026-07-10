package integration

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/engine"
	"github.com/leengari/mini-rdbms/internal/executor"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
	"github.com/leengari/mini-rdbms/internal/query/operations/testutil"
	storageEngine "github.com/leengari/mini-rdbms/internal/storage/engine"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

// TestCRUDOperations tests all CRUD operations with isolated test database
func TestCRUDOperations(t *testing.T) {
	// Setup fresh test database
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	usersTable, ok := db.Tables["users"]
	if !ok {
		t.Fatal("users table not found")
	}

	t.Run("SelectAll", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		rows := usersTable.SelectAll(tx)
		if len(rows) == 0 {
			t.Error("Expected rows, got none")
		}
		t.Logf("Found %d users", len(rows))
	})

	t.Run("SelectWithProjection", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		proj := projection.NewProjectionWithColumns(
			projection.ColumnRef{Column: "id"},
			projection.ColumnRef{Column: "username"},
		)
		
		// Get all rows then apply projection manually (simulating executor)
		allRows := usersTable.SelectAll(tx)
		var rows []data.Row
		for _, row := range allRows {
			rows = append(rows, projection.ProjectRow(row, proj, usersTable.Schema))
		}
		
		if len(rows) == 0 {
			t.Error("Expected rows, got none")
		}

		projSchema := &schema.TableSchema{
			Columns: []schema.Column{
				{Name: "id", Type: schema.ColumnTypeInt},
				{Name: "username", Type: schema.ColumnTypeText},
			},
		}

		// Verify only projected columns exist
		for i, row := range rows {
			testutil.AssertColumnExists(t, row, projSchema, "id", "Row "+string(rune(i)))
			testutil.AssertColumnExists(t, row, projSchema, "username", "Row "+string(rune(i)))
			testutil.AssertColumnNotExists(t, row, projSchema, "email", "Row "+string(rune(i)))
		}
	})

	t.Run("SelectWhere", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		// Find users with specific username
		rows := usersTable.Select(func(row data.Row) bool {
			rMap := getRowMap(row, usersTable.Schema.ColumnNames())
			username, ok := rMap["username"].(string)
			return ok && username == "guest"
		}, tx)

		if len(rows) != 1 {
			t.Errorf("Expected 1 user named guest, got %d", len(rows))
		}
	})

	t.Run("SelectByUniqueIndex", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		// First, get all rows to find a valid ID
		allRows := usersTable.SelectAll(tx)
		if len(allRows) == 0 {
			t.Skip("No users in database to test with")
		}

		// Get the first user's ID
		firstUserID, ok := getRowMap(allRows[0], usersTable.Schema.ColumnNames())["id"].(int64)
		if !ok {
			t.Fatal("First user doesn't have a valid ID")
		}

		// Now test SelectByIndex with that ID
		row, found := usersTable.SelectByIndex("id", firstUserID, tx)
		if !found {
			t.Errorf("Expected to find user with id=%d", firstUserID)
		}
		if len(row.Values) == 0 {
			t.Error("Expected non-nil row")
		}
		
		// Verify we got the right user
		if len(row.Values) > 0 {
			rMap := getRowMap(row, usersTable.Schema.ColumnNames())
			if rowID, ok := rMap["id"].(int64); ok && rowID != firstUserID {
				t.Errorf("Expected id=%d, got id=%d", firstUserID, rowID)
			}
		}
	})

	t.Run("Insert", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		// Insert a new user without specifying ID (let auto-increment handle it)
		newUser := data.NewRow([]interface{}{nil, "newuser", "new@example.com", nil})
		
		err := usersTable.Insert(newUser, tx)
		testutil.AssertNoError(t, err, "Insert operation")
		
		// Get the auto-generated ID
		newID := usersTable.LastInsertID
		
		// Verify insertion
		row, found := usersTable.SelectByIndex("id", newID, tx)
		if !found {
			t.Error("Expected to find newly inserted user")
		}
		if len(row.Values) > 0 {
			rMap := getRowMap(row, usersTable.Schema.ColumnNames())
			if username, ok := rMap["username"].(string); !ok || username != "newuser" {
				t.Errorf("Expected username 'newuser', got '%v'", rMap["username"])
			}
		}
	})

	t.Run("Update", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		// Update a user's email
		updated, err := usersTable.Update(func(row data.Row) bool {
			rMap := getRowMap(row, usersTable.Schema.ColumnNames())
			id, ok := rMap["id"].(int64)
			return ok && id == int64(2)
		}, map[string]interface{}{
			"email": "newemail@example.com",
		}, tx)

		testutil.AssertNoError(t, err, "Update operation")
		if updated == 0 {
			t.Error("Expected to update at least 1 row")
		}

		// Verify update
		row, found := usersTable.SelectByIndex("id", int64(2), tx)
		if !found {
			t.Fatal("User not found after update")
		}
		rMap := getRowMap(row, usersTable.Schema.ColumnNames())
		if email, ok := rMap["email"].(string); !ok || email != "newemail@example.com" {
			t.Errorf("Expected email to be updated, got: %v", rMap["email"])
		}
	})

	t.Run("Delete", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		// Get initial count
		initialRows := usersTable.SelectAll(tx)
		initialCount := len(initialRows)
		
		// Delete a specific user (use ID 1 which should exist in fresh DB)
		deleted, err := usersTable.Delete(func(row data.Row) bool {
			rMap := getRowMap(row, usersTable.Schema.ColumnNames())
			id, ok := rMap["id"].(int64)
			return ok && id == int64(1)
		}, tx)
		
		testutil.AssertNoError(t, err, "Delete operation")
		if deleted == 0 {
			t.Error("Expected to delete at least 1 row")
		}
		
		// Verify deletion
		finalRows := usersTable.SelectAll(tx)
		if len(finalRows) != initialCount-deleted {
			t.Errorf("Expected %d rows after delete, got %d", 
				initialCount-deleted, len(finalRows))
		}
		
		// Verify user no longer exists
		_, found := usersTable.SelectByIndex("id", int64(1), tx)
		if found {
			t.Error("Expected user to be deleted")
		}
	})
}

func TestLimitOffset(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	storageEng := storageEngine.NewMemoryEngine()
	registry := manager.NewRegistry(filepath.Dir(db.Path), storageEng)
	eng := engine.New(db, registry)

	exec := func(t *testing.T, sql string) *executor.Result {
		res, err := eng.Execute(sql)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		return res
	}

	// 1. Create table and insert data
	exec(t, "CREATE TABLE paginated (id INT PRIMARY KEY, name STRING);")
	exec(t, "INSERT INTO paginated (id, name) VALUES (1, 'A');")
	exec(t, "INSERT INTO paginated (id, name) VALUES (2, 'B');")
	exec(t, "INSERT INTO paginated (id, name) VALUES (3, 'C');")
	exec(t, "INSERT INTO paginated (id, name) VALUES (4, 'D');")
	exec(t, "INSERT INTO paginated (id, name) VALUES (5, 'E');")

	// 2. Test LIMIT only
	res := exec(t, "SELECT id FROM paginated LIMIT 2;")
	if len(res.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(res.Rows))
	}

	// 3. Test OFFSET only
	res = exec(t, "SELECT id FROM paginated OFFSET 3;")
	if len(res.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(res.Rows))
	}

	// 4. Test LIMIT and OFFSET
	res = exec(t, "SELECT id FROM paginated LIMIT 2 OFFSET 1;")
	if len(res.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(res.Rows))
	}

	// 5. Test OFFSET past end
	res = exec(t, "SELECT id FROM paginated OFFSET 10;")
	if len(res.Rows) != 0 {
		t.Errorf("Expected 0 rows, got %d", len(res.Rows))
	}
}
func TestOrderBy(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	storageEng := storageEngine.NewMemoryEngine()
	registry := manager.NewRegistry(filepath.Dir(db.Path), storageEng)
	eng := engine.New(db, registry)

	exec := func(t *testing.T, sql string) *executor.Result {
		res, err := eng.Execute(sql)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		return res
	}

	exec(t, "CREATE TABLE sort_test (id INT PRIMARY KEY, name STRING, score INT);")
	exec(t, "INSERT INTO sort_test (id, name, score) VALUES (1, 'Alice', 90);")
	exec(t, "INSERT INTO sort_test (id, name, score) VALUES (2, 'Bob', 80);")
	exec(t, "INSERT INTO sort_test (id, name, score) VALUES (3, 'Charlie', 90);")
	exec(t, "INSERT INTO sort_test (id, name, score) VALUES (4, 'Dave', 70);")

	// 1. Single column ASC
	res := exec(t, "SELECT id FROM sort_test ORDER BY score ASC;")
	if len(res.Rows) != 4 {
		t.Errorf("Expected 4 rows, got %d", len(res.Rows))
	}
	if fmt.Sprintf("%v", res.Rows[0].Values[0]) != "4" { // Dave (70)
		t.Errorf("Expected first row to be id 4, got %v", res.Rows[0].Values[0])
	}

	// 2. Single column DESC
	res = exec(t, "SELECT id FROM sort_test ORDER BY score DESC;")
	firstId := fmt.Sprintf("%v", res.Rows[0].Values[0])
	if firstId != "1" && firstId != "3" { // Alice or Charlie (90)
		t.Errorf("Expected first row to be id 1 or 3, got %v", res.Rows[0].Values[0])
	}

	// 3. Multi-column sort (score DESC, name ASC)
	res = exec(t, "SELECT name FROM sort_test ORDER BY score DESC, name ASC;")
	if fmt.Sprintf("%v", res.Rows[0].Values[0]) != "Alice" || fmt.Sprintf("%v", res.Rows[1].Values[0]) != "Charlie" {
		t.Errorf("Expected Alice then Charlie, got %v and %v", res.Rows[0].Values[0], res.Rows[1].Values[0])
	}
}

func TestAggregations(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	storageEng := storageEngine.NewMemoryEngine()
	registry := manager.NewRegistry(filepath.Dir(db.Path), storageEng)
	eng := engine.New(db, registry)

	exec := func(t *testing.T, sql string) *executor.Result {
		res, err := eng.Execute(sql)
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		return res
	}

	exec(t, "CREATE TABLE agg_test (id INT PRIMARY KEY, amount FLOAT, qty INT);")
	exec(t, "INSERT INTO agg_test (id, amount, qty) VALUES (1, 10.5, 2);")
	exec(t, "INSERT INTO agg_test (id, amount, qty) VALUES (2, 20.0, 4);")
	exec(t, "INSERT INTO agg_test (id, amount, qty) VALUES (3, 30.5, 6);")
	exec(t, "INSERT INTO agg_test (id, amount, qty) VALUES (4, 10.0, NULL);")

	// 1. Test COUNT(*)
	res := exec(t, "SELECT COUNT(*) FROM agg_test;")
	if len(res.Rows) != 1 || fmt.Sprintf("%v", res.Rows[0].Values[0]) != "4" {
		t.Errorf("Expected COUNT(*) = 4, got %v", res.Rows[0].Values)
	}

	// 2. Test COUNT(col) ignoring NULLs
	res = exec(t, "SELECT COUNT(qty) FROM agg_test;")
	if len(res.Rows) != 1 || fmt.Sprintf("%v", res.Rows[0].Values[0]) != "3" {
		t.Errorf("Expected COUNT(qty) = 3, got %v", res.Rows[0].Values)
	}

	// 3. Test SUM
	res = exec(t, "SELECT SUM(amount) FROM agg_test;")
	if len(res.Rows) != 1 || fmt.Sprintf("%v", res.Rows[0].Values[0]) != "71" {
		t.Errorf("Expected SUM(amount) = 71, got %v", res.Rows[0].Values)
	}

	// 4. Test AVG
	res = exec(t, "SELECT AVG(qty) FROM agg_test;")
	if len(res.Rows) != 1 || fmt.Sprintf("%v", res.Rows[0].Values[0]) != "4" {
		t.Errorf("Expected AVG(qty) = 4, got %v", res.Rows[0].Values)
	}

	// 5. Test MIN and MAX with alias
	res = exec(t, "SELECT MIN(amount) AS min_amt, MAX(amount) AS max_amt FROM agg_test;")
	if len(res.Rows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(res.Rows))
	}
	if fmt.Sprintf("%v", res.Rows[0].Values[0]) != "10" {
		t.Errorf("Expected MIN(amount) = 10, got %v", res.Rows[0].Values[0])
	}
	if fmt.Sprintf("%v", res.Rows[0].Values[1]) != "30.5" {
		t.Errorf("Expected MAX(amount) = 30.5, got %v", res.Rows[0].Values[1])
	}
}
