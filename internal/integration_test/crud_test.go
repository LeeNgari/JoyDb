package integration

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
	"github.com/leengari/mini-rdbms/internal/query/operations/testutil"
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
		rows := usersTable.SelectAll()
		if len(rows) == 0 {
			t.Error("Expected rows, got none")
		}
		t.Logf("Found %d users", len(rows))
	})

	t.Run("SelectWithProjection", func(t *testing.T) {
		proj := projection.NewProjectionWithColumns(
			projection.ColumnRef{Column: "id"},
			projection.ColumnRef{Column: "username"},
		)
		
		// Get all rows then apply projection manually (simulating executor)
		allRows := usersTable.SelectAll()
		var rows []data.Row
		for _, row := range allRows {
			rows = append(rows, projection.ProjectRow(row, proj, usersTable.Name))
		}
		
		if len(rows) == 0 {
			t.Error("Expected rows, got none")
		}

		// Verify only projected columns exist
		for i, row := range rows {
			testutil.AssertColumnExists(t, row, "id", "Row "+string(rune(i)))
			testutil.AssertColumnExists(t, row, "username", "Row "+string(rune(i)))
			testutil.AssertColumnNotExists(t, row, "email", "Row "+string(rune(i)))
		}
	})

	t.Run("SelectWhere", func(t *testing.T) {
		// Find users with specific username
		rows := usersTable.Select(func(row data.Row) bool {
			username, ok := row["username"].(string)
			return ok && username == "guest"
		})

		if len(rows) != 1 {
			t.Errorf("Expected 1 user named guest, got %d", len(rows))
		}
	})

	t.Run("SelectByUniqueIndex", func(t *testing.T) {
		// First, get all rows to find a valid ID
		allRows := usersTable.SelectAll()
		if len(allRows) == 0 {
			t.Skip("No users in database to test with")
		}

		// Get the first user's ID
		firstUserID, ok := allRows[0]["id"].(int64)
		if !ok {
			t.Fatal("First user doesn't have a valid ID")
		}

		// Now test SelectByIndex with that ID
		row, found := usersTable.SelectByIndex("id", firstUserID)
		if !found {
			t.Errorf("Expected to find user with id=%d", firstUserID)
		}
		if row == nil {
			t.Error("Expected non-nil row")
		}
		
		// Verify we got the right user
		if row != nil {
			if rowID, ok := row["id"].(int64); ok && rowID != firstUserID {
				t.Errorf("Expected id=%d, got id=%d", firstUserID, rowID)
			}
		}
	})

	t.Run("Insert", func(t *testing.T) {
		// Insert a new user without specifying ID (let auto-increment handle it)
		newUser := data.Row{
			"username": "newuser",
			"email":    "new@example.com",
		}
		
		err := usersTable.Insert(newUser)
		testutil.AssertNoError(t, err, "Insert operation")
		
		// Get the auto-generated ID
		newID := usersTable.LastInsertID
		
		// Verify insertion
		row, found := usersTable.SelectByIndex("id", newID)
		if !found {
			t.Error("Expected to find newly inserted user")
		}
		if row != nil {
			if username, ok := row["username"].(string); !ok || username != "newuser" {
				t.Errorf("Expected username 'newuser', got '%v'", row["username"])
			}
		}
	})

	t.Run("Update", func(t *testing.T) {
		// Update a user's email
		updated, err := usersTable.Update(func(row data.Row) bool {
			id, ok := row["id"].(int64)
			return ok && id == int64(2)
		}, data.Row{
			"email": "newemail@example.com",
		})

		testutil.AssertNoError(t, err, "Update operation")
		if updated == 0 {
			t.Error("Expected to update at least 1 row")
		}

		// Verify update
		row, found := usersTable.SelectByIndex("id", int64(2))
		if !found {
			t.Fatal("User not found after update")
		}
		if email, ok := row["email"].(string); !ok || email != "newemail@example.com" {
			t.Errorf("Expected email to be updated, got: %v", row["email"])
		}
	})

	t.Run("Delete", func(t *testing.T) {
		// Get initial count
		initialRows := usersTable.SelectAll()
		initialCount := len(initialRows)
		
		// Delete a specific user (use ID 1 which should exist in fresh DB)
		deleted, err := usersTable.Delete(func(row data.Row) bool {
			id, ok := row["id"].(int64)
			return ok && id == int64(1)
		})
		
		testutil.AssertNoError(t, err, "Delete operation")
		if deleted == 0 {
			t.Error("Expected to delete at least 1 row")
		}
		
		// Verify deletion
		finalRows := usersTable.SelectAll()
		if len(finalRows) != initialCount-deleted {
			t.Errorf("Expected %d rows after delete, got %d", 
				initialCount-deleted, len(finalRows))
		}
		
		// Verify user no longer exists
		_, found := usersTable.SelectByIndex("id", int64(1))
		if found {
			t.Error("Expected user to be deleted")
		}
	})
}
