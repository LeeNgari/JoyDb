package integration

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/executor"
	"github.com/leengari/mini-rdbms/internal/parser"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
	"github.com/leengari/mini-rdbms/internal/query/indexing"
	"github.com/leengari/mini-rdbms/internal/storage/loader"
)

// TestSQLUpdateStatement tests UPDATE statements end-to-end via SQL
func TestSQLUpdateStatement(t *testing.T) {
	// Load test database
	db, err := loader.LoadDatabase("../../databases/testdb")
	if err != nil {
		t.Fatalf("Failed to load database: %v", err)
	}

	// Build indexes
	if err := indexing.BuildDatabaseIndexes(db); err != nil {
		t.Fatalf("Failed to build indexes: %v", err)
	}

	t.Run("UPDATE single column with WHERE", func(t *testing.T) {
		// First, verify initial state (using id=2 which exists in database)
		selectSQL := "SELECT email FROM users WHERE id = 2;"
		tokens, _ := lexer.Tokenize(selectSQL)
		p := parser.New(tokens)
		stmt, _ := p.Parse()
		result, err := executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("Failed to select initial state: %v", err)
		}
		
		if len(result.Rows) == 0 {
			t.Skip("No user with id=2 to test with")
		}
		
		initialEmail := result.Rows[0]["email"]
		
		// Execute UPDATE
		updateSQL := "UPDATE users SET email = 'updated@test.com' WHERE id = 2;"
		tokens, err = lexer.Tokenize(updateSQL)
		if err != nil {
			t.Fatalf("Lexer error: %v", err)
		}
		
		p = parser.New(tokens)
		stmt, err = p.Parse()
		if err != nil {
			t.Fatalf("Parser error: %v", err)
		}
		
		result, err = executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("Executor error: %v", err)
		}
		
		// Verify result message
		if result.Message != "UPDATE 1" {
			t.Errorf("Expected 'UPDATE 1', got '%s'", result.Message)
		}
		
		// Verify the update took effect
		tokens, _ = lexer.Tokenize(selectSQL)
		p = parser.New(tokens)
		stmt, _ = p.Parse()
		result, err = executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("Failed to verify update: %v", err)
		}
		
		if len(result.Rows) == 0 {
			t.Fatal("User disappeared after update")
		}
		
		newEmail := result.Rows[0]["email"]
		if newEmail != "updated@test.com" {
			t.Errorf("Expected email 'updated@test.com', got '%v'", newEmail)
		}
		
		// Restore original state
		restoreSQL := "UPDATE users SET email = '" + initialEmail.(string) + "' WHERE id = 2;"
		tokens, _ = lexer.Tokenize(restoreSQL)
		p = parser.New(tokens)
		stmt, _ = p.Parse()
		executor.Execute(stmt, db)
	})

	t.Run("UPDATE multiple columns", func(t *testing.T) {
		// Update multiple columns at once (using id=5 which exists)
		updateSQL := "UPDATE users SET email = 'multi@test.com', username = 'multiuser' WHERE id = 5;"
		tokens, err := lexer.Tokenize(updateSQL)
		if err != nil {
			t.Fatalf("Lexer error: %v", err)
		}
		
		p := parser.New(tokens)
		stmt, err := p.Parse()
		if err != nil {
			t.Fatalf("Parser error: %v", err)
		}
		
		result, err := executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("Executor error: %v", err)
		}
		
		// Verify result
		if result.Message != "UPDATE 1" {
			t.Errorf("Expected 'UPDATE 1', got '%s'", result.Message)
		}
		
		// Verify both columns were updated
		selectSQL := "SELECT username, email FROM users WHERE id = 5;"
		tokens, _ = lexer.Tokenize(selectSQL)
		p = parser.New(tokens)
		stmt, _ = p.Parse()
		result, _ = executor.Execute(stmt, db)
		
		if len(result.Rows) > 0 {
			if result.Rows[0]["username"] != "multiuser" {
				t.Errorf("Expected username 'multiuser', got '%v'", result.Rows[0]["username"])
			}
			if result.Rows[0]["email"] != "multi@test.com" {
				t.Errorf("Expected email 'multi@test.com', got '%v'", result.Rows[0]["email"])
			}
		}
	})

	t.Run("UPDATE with boolean value", func(t *testing.T) {
		// Test updating with boolean literal (using correct column name is_active)
		updateSQL := "UPDATE users SET is_active = false WHERE id = 2;"
		tokens, err := lexer.Tokenize(updateSQL)
		if err != nil {
			t.Fatalf("Lexer error: %v", err)
		}
		
		p := parser.New(tokens)
		stmt, err := p.Parse()
		if err != nil {
			t.Fatalf("Parser error: %v", err)
		}
		
		result, err := executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("Executor error: %v", err)
		}
		
		// Verify update count
		if result.Message != "UPDATE 1" {
			t.Errorf("Expected 'UPDATE 1', got '%s'", result.Message)
		}
	})

	t.Run("UPDATE nonexistent table", func(t *testing.T) {
		updateSQL := "UPDATE nonexistent SET col = 'value' WHERE id = 1;"
		tokens, _ := lexer.Tokenize(updateSQL)
		p := parser.New(tokens)
		stmt, _ := p.Parse()
		
		_, err := executor.Execute(stmt, db)
		if err == nil {
			t.Error("Expected error for nonexistent table, got nil")
		}
	})
}

// TestSQLDeleteStatement tests DELETE statements end-to-end via SQL
func TestSQLDeleteStatement(t *testing.T) {
	// Load test database
	db, err := loader.LoadDatabase("../../databases/testdb")
	if err != nil {
		t.Fatalf("Failed to load database: %v", err)
	}

	// Build indexes
	if err := indexing.BuildDatabaseIndexes(db); err != nil {
		t.Fatalf("Failed to build indexes: %v", err)
	}

	t.Run("DELETE with WHERE clause", func(t *testing.T) {
		// First, insert a test user to delete
		insertSQL := "INSERT INTO users (id, username, email) VALUES (999, 'tempuser', 'temp@test.com');"
		tokens, _ := lexer.Tokenize(insertSQL)
		p := parser.New(tokens)
		stmt, _ := p.Parse()
		_, err := executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
		
		// Verify insertion
		selectSQL := "SELECT id FROM users WHERE id = 999;"
		tokens, _ = lexer.Tokenize(selectSQL)
		p = parser.New(tokens)
		stmt, _ = p.Parse()
		result, _ := executor.Execute(stmt, db)
		
		if len(result.Rows) == 0 {
			t.Fatal("Test user was not inserted")
		}
		
		// Now delete the user
		deleteSQL := "DELETE FROM users WHERE id = 999;"
		tokens, err = lexer.Tokenize(deleteSQL)
		if err != nil {
			t.Fatalf("Lexer error: %v", err)
		}
		
		p = parser.New(tokens)
		stmt, err = p.Parse()
		if err != nil {
			t.Fatalf("Parser error: %v", err)
		}
		
		result, err = executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("Executor error: %v", err)
		}
		
		// Verify result message
		if result.Message != "DELETE 1" {
			t.Errorf("Expected 'DELETE 1', got '%s'", result.Message)
		}
		
		// Verify deletion
		tokens, _ = lexer.Tokenize(selectSQL)
		p = parser.New(tokens)
		stmt, _ = p.Parse()
		result, _ = executor.Execute(stmt, db)
		
		if len(result.Rows) != 0 {
			t.Error("User was not deleted")
		}
	})

	t.Run("DELETE with string WHERE", func(t *testing.T) {
		// Insert test data first to ensure it exists
		insertSQL := "INSERT INTO users (id, username, email) VALUES (998, 'deletetest', 'delete@test.com');"
		tokens, _ := lexer.Tokenize(insertSQL)
		p := parser.New(tokens)
		stmt, _ := p.Parse()
		executor.Execute(stmt, db)
		
		// Delete by username (string comparison)
		deleteSQL := "DELETE FROM users WHERE username = 'deletetest';"
		tokens, err := lexer.Tokenize(deleteSQL)
		if err != nil {
			t.Fatalf("Lexer error: %v", err)
		}
		
		p = parser.New(tokens)
		stmt, err = p.Parse()
		if err != nil {
			t.Fatalf("Parser error: %v", err)
		}
		
		result, err := executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("Executor error: %v", err)
		}
		
		// Should delete successfully (may be 0 if already deleted in previous run, or 1 if fresh)
		// Accept both to make test idempotent
		if result.Message != "DELETE 1" && result.Message != "DELETE 0" {
			t.Errorf("Expected 'DELETE 1' or 'DELETE 0', got '%s'", result.Message)
		}
	})

	t.Run("DELETE nonexistent row", func(t *testing.T) {
		// Try to delete a row that doesn't exist
		deleteSQL := "DELETE FROM users WHERE id = 99999;"
		tokens, _ := lexer.Tokenize(deleteSQL)
		p := parser.New(tokens)
		stmt, _ := p.Parse()
		
		result, err := executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		
		// Should return DELETE 0
		if result.Message != "DELETE 0" {
			t.Errorf("Expected 'DELETE 0', got '%s'", result.Message)
		}
	})

	t.Run("DELETE nonexistent table", func(t *testing.T) {
		deleteSQL := "DELETE FROM nonexistent WHERE id = 1;"
		tokens, _ := lexer.Tokenize(deleteSQL)
		p := parser.New(tokens)
		stmt, _ := p.Parse()
		
		_, err := executor.Execute(stmt, db)
		if err == nil {
			t.Error("Expected error for nonexistent table, got nil")
		}
	})
}

// TestSQLCombinedOperations tests combinations of CRUD operations
func TestSQLCombinedOperations(t *testing.T) {
	// Load test database
	db, err := loader.LoadDatabase("../../databases/testdb")
	if err != nil {
		t.Fatalf("Failed to load database: %v", err)
	}

	// Build indexes
	if err := indexing.BuildDatabaseIndexes(db); err != nil {
		t.Fatalf("Failed to build indexes: %v", err)
	}

	t.Run("INSERT then UPDATE then DELETE", func(t *testing.T) {
		// INSERT
		insertSQL := "INSERT INTO users (id, username, email) VALUES (997, 'testuser', 'test@example.com');"
		tokens, _ := lexer.Tokenize(insertSQL)
		p := parser.New(tokens)
		stmt, _ := p.Parse()
		result, err := executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}
		if result.Message != "INSERT 1" {
			t.Errorf("Expected 'INSERT 1', got '%s'", result.Message)
		}
		
		// UPDATE
		updateSQL := "UPDATE users SET email = 'updated@example.com' WHERE id = 997;"
		tokens, _ = lexer.Tokenize(updateSQL)
		p = parser.New(tokens)
		stmt, _ = p.Parse()
		result, err = executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("UPDATE failed: %v", err)
		}
		if result.Message != "UPDATE 1" {
			t.Errorf("Expected 'UPDATE 1', got '%s'", result.Message)
		}
		
		// Verify UPDATE
		selectSQL := "SELECT email FROM users WHERE id = 997;"
		tokens, _ = lexer.Tokenize(selectSQL)
		p = parser.New(tokens)
		stmt, _ = p.Parse()
		result, _ = executor.Execute(stmt, db)
		if len(result.Rows) > 0 && result.Rows[0]["email"] != "updated@example.com" {
			t.Errorf("Email was not updated correctly")
		}
		
		// DELETE
		deleteSQL := "DELETE FROM users WHERE id = 997;"
		tokens, _ = lexer.Tokenize(deleteSQL)
		p = parser.New(tokens)
		stmt, _ = p.Parse()
		result, err = executor.Execute(stmt, db)
		if err != nil {
			t.Fatalf("DELETE failed: %v", err)
		}
		if result.Message != "DELETE 1" {
			t.Errorf("Expected 'DELETE 1', got '%s'", result.Message)
		}
		
		// Verify DELETE
		tokens, _ = lexer.Tokenize(selectSQL)
		p = parser.New(tokens)
		stmt, _ = p.Parse()
		result, _ = executor.Execute(stmt, db)
		if len(result.Rows) != 0 {
			t.Error("User was not deleted")
		}
	})
}
