package parser

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// TestParseComparisonOperators tests parsing of all comparison operators
func TestParseComparisonOperators(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedOperator string
	}{
		{
			name:             "Equals operator",
			input:            "SELECT * FROM users WHERE age = 25;",
			expectedOperator: "=",
		},
		{
			name:             "Less than operator",
			input:            "SELECT * FROM users WHERE age < 30;",
			expectedOperator: "<",
		},
		{
			name:             "Greater than operator",
			input:            "SELECT * FROM users WHERE age > 18;",
			expectedOperator: ">",
		},
		{
			name:             "Less than or equal operator",
			input:            "SELECT * FROM users WHERE age <= 65;",
			expectedOperator: "<=",
		},
		{
			name:             "Greater than or equal operator",
			input:            "SELECT * FROM users WHERE age >= 21;",
			expectedOperator: ">=",
		},
		{
			name:             "Not equal operator (!=)",
			input:            "SELECT * FROM users WHERE status != 'inactive';",
			expectedOperator: "!=",
		},
		{
			name:             "Not equal operator (<>)",
			input:            "SELECT * FROM users WHERE status <> 'deleted';",
			expectedOperator: "<>",
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
				t.Fatalf("Parser error: %v", err)
			}

			sel, ok := stmt.(*ast.SelectStatement)
			if !ok {
				t.Fatalf("Expected SelectStatement, got %T", stmt)
			}

			if sel.Where == nil {
				t.Fatal("Expected WHERE clause, got nil")
			}

			binExpr, ok := sel.Where.(*ast.BinaryExpression)
			if !ok {
				t.Fatalf("Expected BinaryExpression in WHERE, got %T", sel.Where)
			}

			if binExpr.Operator != tt.expectedOperator {
				t.Errorf("Expected operator %s, got %s", tt.expectedOperator, binExpr.Operator)
			}
		})
	}
}

// TestComparisonOperatorsInUpdate tests comparison operators in UPDATE statements
func TestComparisonOperatorsInUpdate(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedOperator string
	}{
		{
			name:             "UPDATE with < operator",
			input:            "UPDATE products SET price = 0 WHERE price < 10;",
			expectedOperator: "<",
		},
		{
			name:             "UPDATE with >= operator",
			input:            "UPDATE users SET premium = true WHERE age >= 18;",
			expectedOperator: ">=",
		},
		{
			name:             "UPDATE with != operator",
			input:            "UPDATE orders SET status = 'cancelled' WHERE status != 'completed';",
			expectedOperator: "!=",
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
				t.Fatalf("Parser error: %v", err)
			}

			upd, ok := stmt.(*ast.UpdateStatement)
			if !ok {
				t.Fatalf("Expected UpdateStatement, got %T", stmt)
			}

			if upd.Where == nil {
				t.Fatal("Expected WHERE clause, got nil")
			}

			binExpr, ok := upd.Where.(*ast.BinaryExpression)
			if !ok {
				t.Fatalf("Expected BinaryExpression in WHERE, got %T", upd.Where)
			}

			if binExpr.Operator != tt.expectedOperator {
				t.Errorf("Expected operator %s, got %s", tt.expectedOperator, binExpr.Operator)
			}
		})
	}
}

// TestComparisonOperatorsInDelete tests comparison operators in DELETE statements
func TestComparisonOperatorsInDelete(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedOperator string
	}{
		{
			name:             "DELETE with > operator",
			input:            "DELETE FROM logs WHERE timestamp > 1000000;",
			expectedOperator: ">",
		},
		{
			name:             "DELETE with <= operator",
			input:            "DELETE FROM temp_data WHERE age <= 0;",
			expectedOperator: "<=",
		},
		{
			name:             "DELETE with <> operator",
			input:            "DELETE FROM users WHERE role <> 'admin';",
			expectedOperator: "<>",
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
				t.Fatalf("Parser error: %v", err)
			}

			del, ok := stmt.(*ast.DeleteStatement)
			if !ok {
				t.Fatalf("Expected DeleteStatement, got %T", stmt)
			}

			if del.Where == nil {
				t.Fatal("Expected WHERE clause, got nil")
			}

			binExpr, ok := del.Where.(*ast.BinaryExpression)
			if !ok {
				t.Fatalf("Expected BinaryExpression in WHERE, got %T", del.Where)
			}

			if binExpr.Operator != tt.expectedOperator {
				t.Errorf("Expected operator %s, got %s", tt.expectedOperator, binExpr.Operator)
			}
		})
	}
}
