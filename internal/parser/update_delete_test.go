package parser

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// TestParseUpdate tests parsing of UPDATE statements
func TestParseUpdate(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedTable string
		expectedSets  map[string]interface{} // column -> expected value
		hasWhere      bool
	}{
		{
			name:          "UPDATE with WHERE",
			input:         "UPDATE users SET email = 'new@test.com' WHERE id = 5;",
			expectedTable: "users",
			expectedSets:  map[string]interface{}{"email": "new@test.com"},
			hasWhere:      true,
		},
		{
			name:          "UPDATE without WHERE",
			input:         "UPDATE products SET price = 99.99;",
			expectedTable: "products",
			expectedSets:  map[string]interface{}{"price": 99.99},
			hasWhere:      false,
		},
		{
			name:          "UPDATE multiple columns",
			input:         "UPDATE users SET email = 'test@example.com', active = true WHERE id = 10;",
			expectedTable: "users",
			expectedSets:  map[string]interface{}{"email": "test@example.com", "active": true},
			hasWhere:      true,
		},
		{
			name:          "UPDATE with number",
			input:         "UPDATE items SET quantity = 42 WHERE name = 'widget';",
			expectedTable: "items",
			expectedSets:  map[string]interface{}{"quantity": 42},
			hasWhere:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.Tokenize(tt.input)
			if err != nil {
				t.Fatalf("Lexer error: %v", err)
			}

			p := New(tokens)
			stmt, err := p.Parse()
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			upd, ok := stmt.(*ast.UpdateStatement)
			if !ok {
				t.Fatalf("Expected UpdateStatement, got %T", stmt)
			}

			// Check table name
			if upd.TableName.Value != tt.expectedTable {
				t.Errorf("Expected table %s, got %s", tt.expectedTable, upd.TableName.Value)
			}

			// Check SET clause
			if len(upd.Updates) != len(tt.expectedSets) {
				t.Errorf("Expected %d updates, got %d", len(tt.expectedSets), len(upd.Updates))
			}

			for col, expectedVal := range tt.expectedSets {
				expr, exists := upd.Updates[col]
				if !exists {
					t.Errorf("Expected column %s in updates, but not found", col)
					continue
				}

				lit, ok := expr.(*ast.Literal)
				if !ok {
					t.Errorf("Expected literal value for column %s, got %T", col, expr)
					continue
				}

				if lit.Value != expectedVal {
					t.Errorf("Expected value %v for column %s, got %v", expectedVal, col, lit.Value)
				}
			}

			// Check WHERE clause
			if tt.hasWhere {
				if upd.Where == nil {
					t.Error("Expected WHERE clause, got nil")
				}
			} else {
				if upd.Where != nil {
					t.Error("Expected no WHERE clause, but got one")
				}
			}
		})
	}
}

// TestParseDelete tests parsing of DELETE statements
func TestParseDelete(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedTable string
		hasWhere      bool
	}{
		{
			name:          "DELETE with WHERE",
			input:         "DELETE FROM users WHERE active = false;",
			expectedTable: "users",
			hasWhere:      true,
		},
		{
			name:          "DELETE without WHERE",
			input:         "DELETE FROM temp_data;",
			expectedTable: "temp_data",
			hasWhere:      false,
		},
		{
			name:          "DELETE with numeric WHERE",
			input:         "DELETE FROM orders WHERE id = 123;",
			expectedTable: "orders",
			hasWhere:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.Tokenize(tt.input)
			if err != nil {
				t.Fatalf("Lexer error: %v", err)
			}

			p := New(tokens)
			stmt, err := p.Parse()
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			del, ok := stmt.(*ast.DeleteStatement)
			if !ok {
				t.Fatalf("Expected DeleteStatement, got %T", stmt)
			}

			// Check table name
			if del.TableName.Value != tt.expectedTable {
				t.Errorf("Expected table %s, got %s", tt.expectedTable, del.TableName.Value)
			}

			// Check WHERE clause
			if tt.hasWhere {
				if del.Where == nil {
					t.Error("Expected WHERE clause, got nil")
				}
			} else {
				if del.Where != nil {
					t.Error("Expected no WHERE clause, but got one")
				}
			}
		})
	}
}

// TestParseUpdateErrors tests error cases for UPDATE parsing
func TestParseUpdateErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Missing SET keyword",
			input: "UPDATE users email = 'test@example.com';",
		},
		{
			name:  "Missing table name",
			input: "UPDATE SET email = 'test@example.com';",
		},
		{
			name:  "Missing equals in SET",
			input: "UPDATE users SET email 'test@example.com';",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.Tokenize(tt.input)
			if err != nil {
				// Lexer error is acceptable for some malformed inputs
				return
			}

			p := New(tokens)
			_, err = p.Parse()
			if err == nil {
				t.Error("Expected parse error, but got nil")
			}
		})
	}
}

// TestParseDeleteErrors tests error cases for DELETE parsing
func TestParseDeleteErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Missing FROM keyword",
			input: "DELETE users;",
		},
		{
			name:  "Missing table name",
			input: "DELETE FROM;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.Tokenize(tt.input)
			if err != nil {
				// Lexer error is acceptable
				return
			}

			p := New(tokens)
			_, err = p.Parse()
			if err == nil {
				t.Error("Expected parse error, but got nil")
			}
		})
	}
}
