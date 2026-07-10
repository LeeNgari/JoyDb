package parser

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// TestParseComparisonExpressions tests parsing of all comparison operators
func TestParseComparisonExpressions(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedOperator string
		stmtType         string // "SELECT", "UPDATE", or "DELETE"
	}{
		// SELECT with comparisons
		{name: "SELECT with =", input: "SELECT * FROM users WHERE age = 25;", expectedOperator: "=", stmtType: "SELECT"},
		{name: "SELECT with <", input: "SELECT * FROM users WHERE age < 30;", expectedOperator: "<", stmtType: "SELECT"},
		{name: "SELECT with >", input: "SELECT * FROM users WHERE age > 18;", expectedOperator: ">", stmtType: "SELECT"},
		{name: "SELECT with <=", input: "SELECT * FROM users WHERE age <= 65;", expectedOperator: "<=", stmtType: "SELECT"},
		{name: "SELECT with >=", input: "SELECT * FROM users WHERE age >= 21;", expectedOperator: ">=", stmtType: "SELECT"},
		{name: "SELECT with !=", input: "SELECT * FROM users WHERE status != 'inactive';", expectedOperator: "!=", stmtType: "SELECT"},
		{name: "SELECT with <>", input: "SELECT * FROM users WHERE status <> 'deleted';", expectedOperator: "<>", stmtType: "SELECT"},
		
		// UPDATE with comparisons
		{name: "UPDATE with <", input: "UPDATE products SET price = 0 WHERE price < 10;", expectedOperator: "<", stmtType: "UPDATE"},
		{name: "UPDATE with >=", input: "UPDATE users SET premium = true WHERE age >= 18;", expectedOperator: ">=", stmtType: "UPDATE"},
		
		// DELETE with comparisons
		{name: "DELETE with >", input: "DELETE FROM logs WHERE timestamp > 1000000;", expectedOperator: ">", stmtType: "DELETE"},
		{name: "DELETE with <>", input: "DELETE FROM users WHERE role <> 'admin';", expectedOperator: "<>", stmtType: "DELETE"},
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
				t.Fatalf("Parser error: %v", err)
			}

			var whereClause ast.Expression
			switch tt.stmtType {
			case "SELECT":
				sel, ok := stmt.(*ast.SelectStatement)
				if !ok {
					t.Fatalf("Expected SelectStatement, got %T", stmt)
				}
				whereClause = sel.Where
			case "UPDATE":
				upd, ok := stmt.(*ast.UpdateStatement)
				if !ok {
					t.Fatalf("Expected UpdateStatement, got %T", stmt)
				}
				whereClause = upd.Where
			case "DELETE":
				del, ok := stmt.(*ast.DeleteStatement)
				if !ok {
					t.Fatalf("Expected DeleteStatement, got %T", stmt)
				}
				whereClause = del.Where
			}

			if whereClause == nil {
				t.Fatal("Expected WHERE clause, got nil")
			}

			binExpr, ok := whereClause.(*ast.BinaryExpression)
			if !ok {
				t.Fatalf("Expected BinaryExpression in WHERE, got %T", whereClause)
			}

			if binExpr.Operator != tt.expectedOperator {
				t.Errorf("Expected operator %s, got %s", tt.expectedOperator, binExpr.Operator)
			}
		})
	}
}

// TestParseLogicalExpressions tests parsing of AND/OR logical operators
func TestParseLogicalExpressions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		operator string // "AND" or "OR"
		stmtType string // "SELECT", "UPDATE", or "DELETE"
	}{
		// Basic AND/OR
		{name: "SELECT with AND", input: "SELECT * FROM users WHERE age > 18 AND active = true;", operator: "AND", stmtType: "SELECT"},
		{name: "SELECT with OR", input: "SELECT * FROM orders WHERE status = 'pending' OR status = 'processing';", operator: "OR", stmtType: "SELECT"},
		
		// Complex expressions
		{name: "Multiple ANDs", input: "SELECT * FROM users WHERE age > 18 AND active = true AND verified = true;", operator: "AND", stmtType: "SELECT"},
		{name: "Parenthesized OR with AND", input: "SELECT * FROM users WHERE (age > 18 OR premium = true) AND active = true;", operator: "AND", stmtType: "SELECT"},
		
		// UPDATE/DELETE with logical operators
		{name: "UPDATE with AND", input: "UPDATE users SET active = false WHERE age < 18 AND verified = false;", operator: "AND", stmtType: "UPDATE"},
		{name: "DELETE with OR", input: "DELETE FROM logs WHERE level = 'debug' OR level = 'trace';", operator: "OR", stmtType: "DELETE"},
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
				t.Fatalf("Parser error: %v", err)
			}

			var whereClause ast.Expression
			switch tt.stmtType {
			case "SELECT":
				sel, ok := stmt.(*ast.SelectStatement)
				if !ok {
					t.Fatalf("Expected SelectStatement, got %T", stmt)
				}
				whereClause = sel.Where
			case "UPDATE":
				upd, ok := stmt.(*ast.UpdateStatement)
				if !ok {
					t.Fatalf("Expected UpdateStatement, got %T", stmt)
				}
				whereClause = upd.Where
			case "DELETE":
				del, ok := stmt.(*ast.DeleteStatement)
				if !ok {
					t.Fatalf("Expected DeleteStatement, got %T", stmt)
				}
				whereClause = del.Where
			}

			if whereClause == nil {
				t.Fatal("Expected WHERE clause, got nil")
			}

			logExpr, ok := whereClause.(*ast.LogicalExpression)
			if !ok {
				t.Fatalf("Expected LogicalExpression, got %T", whereClause)
			}

			if logExpr.Operator != tt.operator {
				t.Errorf("Expected %s operator, got %s", tt.operator, logExpr.Operator)
			}
		})
	}
}

// TestParseTypedLiterals tests parsing of DATE, TIME, and EMAIL literals
func TestParseTypedLiterals(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantKind  ast.LiteralKind
		wantValue string
		wantErr   bool
	}{
		// Valid typed literals
		{name: "Valid DATE", input: "SELECT * FROM events WHERE event_date = DATE '2024-01-13';", wantKind: ast.LiteralDate, wantValue: "2024-01-13"},
		{name: "Valid TIME with seconds", input: "SELECT * FROM logs WHERE log_time = TIME '14:30:45';", wantKind: ast.LiteralTime, wantValue: "14:30:45"},
		{name: "Valid TIME without seconds", input: "SELECT * FROM logs WHERE log_time = TIME '14:30';", wantKind: ast.LiteralTime, wantValue: "14:30"},
		{name: "Valid EMAIL", input: "SELECT * FROM users WHERE email = EMAIL 'user@example.com';", wantKind: ast.LiteralEmail, wantValue: "user@example.com"},
		
		// INSERT with typed literals
		{name: "INSERT with DATE", input: "INSERT INTO events (id, event_date) VALUES (1, DATE '2024-01-13');", wantKind: ast.LiteralDate, wantValue: "2024-01-13"},
		{name: "INSERT with TIME", input: "INSERT INTO logs (id, log_time) VALUES (1, TIME '14:30:00');", wantKind: ast.LiteralTime, wantValue: "14:30:00"},
		{name: "INSERT with EMAIL", input: "INSERT INTO users (id, email) VALUES (1, EMAIL 'user@example.com');", wantKind: ast.LiteralEmail, wantValue: "user@example.com"},
		
		// Invalid formats
		{name: "Invalid DATE format", input: "SELECT * FROM events WHERE event_date = DATE '2024-13-01';", wantErr: true},
		{name: "Invalid TIME format", input: "SELECT * FROM logs WHERE log_time = TIME '25:00:00';", wantErr: true},
		{name: "Invalid EMAIL - no @", input: "SELECT * FROM users WHERE email = EMAIL 'notanemail';", wantErr: true},
		{name: "Invalid EMAIL - no domain", input: "SELECT * FROM users WHERE email = EMAIL 'user@';", wantErr: true},
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

			var lit *ast.Literal
			switch s := stmt.(type) {
			case *ast.SelectStatement:
				if s.Where == nil {
					t.Fatal("Expected WHERE clause")
				}
				binExpr, ok := s.Where.(*ast.BinaryExpression)
				if !ok {
					t.Fatalf("Expected BinaryExpression in WHERE, got %T", s.Where)
				}
				lit, ok = binExpr.Right.(*ast.Literal)
				if !ok {
					t.Fatalf("Expected Literal on right side, got %T", binExpr.Right)
				}
			case *ast.InsertStatement:
				if len(s.Values) < 2 {
					t.Fatal("Expected at least 2 values")
				}
				var ok bool
				lit, ok = s.Values[1].(*ast.Literal)
				if !ok {
					t.Fatalf("Expected Literal, got %T", s.Values[1])
				}
			default:
				t.Fatalf("Unexpected statement type: %T", stmt)
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
