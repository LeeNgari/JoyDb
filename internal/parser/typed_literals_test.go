package parser

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

func TestParseTypedLiterals(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantKind  ast.LiteralKind
		wantValue string
		wantErr   bool
	}{
		{
			name:      "Valid DATE literal",
			input:     "SELECT * FROM events WHERE event_date = DATE '2024-01-13';",
			wantKind:  ast.LiteralDate,
			wantValue: "2024-01-13",
			wantErr:   false,
		},
		{
			name:      "Valid TIME literal with seconds",
			input:     "SELECT * FROM logs WHERE log_time = TIME '14:30:45';",
			wantKind:  ast.LiteralTime,
			wantValue: "14:30:45",
			wantErr:   false,
		},
		{
			name:      "Valid TIME literal without seconds",
			input:     "SELECT * FROM logs WHERE log_time = TIME '14:30';",
			wantKind:  ast.LiteralTime,
			wantValue: "14:30",
			wantErr:   false,
		},
		{
			name:      "Valid EMAIL literal",
			input:     "SELECT * FROM users WHERE email = EMAIL 'user@example.com';",
			wantKind:  ast.LiteralEmail,
			wantValue: "user@example.com",
			wantErr:   false,
		},
		{
			name:    "Invalid DATE format",
			input:   "SELECT * FROM events WHERE event_date = DATE '2024-13-01';",
			wantErr: true,
		},
		{
			name:    "Invalid TIME format",
			input:   "SELECT * FROM logs WHERE log_time = TIME '25:00:00';",
			wantErr: true,
		},
		{
			name:    "Invalid EMAIL format - no @",
			input:   "SELECT * FROM users WHERE email = EMAIL 'notanemail';",
			wantErr: true,
		},
		{
			name:    "Invalid EMAIL format - no domain",
			input:   "SELECT * FROM users WHERE email = EMAIL 'user@';",
			wantErr: true,
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

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			selectStmt, ok := stmt.(*ast.SelectStatement)
			if !ok {
				t.Fatalf("Expected SelectStatement, got %T", stmt)
			}

			if selectStmt.Where == nil {
				t.Fatal("Expected WHERE clause")
			}

			binExpr, ok := selectStmt.Where.(*ast.BinaryExpression)
			if !ok {
				t.Fatalf("Expected BinaryExpression in WHERE, got %T", selectStmt.Where)
			}

			lit, ok := binExpr.Right.(*ast.Literal)
			if !ok {
				t.Fatalf("Expected Literal on right side, got %T", binExpr.Right)
			}

			if lit.Kind != tt.wantKind {
				t.Errorf("Expected kind %s, got %s", tt.wantKind, lit.Kind)
			}

			if lit.Value != tt.wantValue {
				t.Errorf("Expected value %s, got %v", tt.wantValue, lit.Value)
			}
		})
	}
}

func TestInsertWithTypedLiterals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKind ast.LiteralKind
		wantErr  bool
	}{
		{
			name:     "INSERT with DATE",
			input:    "INSERT INTO events (id, event_date) VALUES (1, DATE '2024-01-13');",
			wantKind: ast.LiteralDate,
			wantErr:  false,
		},
		{
			name:     "INSERT with TIME",
			input:    "INSERT INTO logs (id, log_time) VALUES (1, TIME '14:30:00');",
			wantKind: ast.LiteralTime,
			wantErr:  false,
		},
		{
			name:     "INSERT with EMAIL",
			input:    "INSERT INTO users (id, email) VALUES (1, EMAIL 'user@example.com');",
			wantKind: ast.LiteralEmail,
			wantErr:  false,
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

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			insertStmt, ok := stmt.(*ast.InsertStatement)
			if !ok {
				t.Fatalf("Expected InsertStatement, got %T", stmt)
			}

			// Check the second value (the typed literal)
			if len(insertStmt.Values) < 2 {
				t.Fatal("Expected at least 2 values")
			}

			lit, ok := insertStmt.Values[1].(*ast.Literal)
			if !ok {
				t.Fatalf("Expected Literal, got %T", insertStmt.Values[1])
			}

			if lit.Kind != tt.wantKind {
				t.Errorf("Expected kind %s, got %s", tt.wantKind, lit.Kind)
			}
		})
	}
}
