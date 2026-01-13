package parser

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

// TestParseLogicalAND tests parsing of AND expressions
func TestParseLogicalAND(t *testing.T) {
	input := "SELECT * FROM users WHERE age > 18 AND active = true;"
	tokens, err := lexer.Tokenize(input)
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

	logExpr, ok := sel.Where.(*ast.LogicalExpression)
	if !ok {
		t.Fatalf("Expected LogicalExpression, got %T", sel.Where)
	}

	if logExpr.Operator != "AND" {
		t.Errorf("Expected AND operator, got %s", logExpr.Operator)
	}

	// Verify left side is a comparison
	_, ok = logExpr.Left.(*ast.BinaryExpression)
	if !ok {
		t.Errorf("Expected left side to be BinaryExpression, got %T", logExpr.Left)
	}

	// Verify right side is a comparison
	_, ok = logExpr.Right.(*ast.BinaryExpression)
	if !ok {
		t.Errorf("Expected right side to be BinaryExpression, got %T", logExpr.Right)
	}
}

// TestParseLogicalOR tests parsing of OR expressions
func TestParseLogicalOR(t *testing.T) {
	input := "SELECT * FROM orders WHERE status = 'pending' OR status = 'processing';"
	tokens, err := lexer.Tokenize(input)
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

	logExpr, ok := sel.Where.(*ast.LogicalExpression)
	if !ok {
		t.Fatalf("Expected LogicalExpression, got %T", sel.Where)
	}

	if logExpr.Operator != "OR" {
		t.Errorf("Expected OR operator, got %s", logExpr.Operator)
	}
}

// TestParseComplexLogical tests complex logical expressions with precedence
func TestParseComplexLogical(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, ast.Expression)
	}{
		{
			name:  "AND with OR (OR has lower precedence)",
			input: "SELECT * FROM users WHERE age > 18 AND active = true OR premium = true;",
			validate: func(t *testing.T, expr ast.Expression) {
				// Should parse as: (age > 18 AND active = true) OR premium = true
				logExpr, ok := expr.(*ast.LogicalExpression)
				if !ok {
					t.Fatalf("Expected LogicalExpression, got %T", expr)
				}
				if logExpr.Operator != "OR" {
					t.Errorf("Expected top-level OR, got %s", logExpr.Operator)
				}
				// Left side should be AND expression
				leftLog, ok := logExpr.Left.(*ast.LogicalExpression)
				if !ok {
					t.Errorf("Expected left side to be LogicalExpression (AND), got %T", logExpr.Left)
				} else if leftLog.Operator != "AND" {
					t.Errorf("Expected left side to be AND, got %s", leftLog.Operator)
				}
			},
		},
		{
			name:  "Multiple ANDs",
			input: "SELECT * FROM users WHERE age > 18 AND active = true AND verified = true;",
			validate: func(t *testing.T, expr ast.Expression) {
				// Should parse as: (age > 18 AND active = true) AND verified = true
				logExpr, ok := expr.(*ast.LogicalExpression)
				if !ok {
					t.Fatalf("Expected LogicalExpression, got %T", expr)
				}
				if logExpr.Operator != "AND" {
					t.Errorf("Expected AND operator, got %s", logExpr.Operator)
				}
			},
		},
		{
			name:  "Parenthesized expression",
			input: "SELECT * FROM users WHERE (age > 18 OR premium = true) AND active = true;",
			validate: func(t *testing.T, expr ast.Expression) {
				// Should parse as: (age > 18 OR premium = true) AND active = true
				logExpr, ok := expr.(*ast.LogicalExpression)
				if !ok {
					t.Fatalf("Expected LogicalExpression, got %T", expr)
				}
				if logExpr.Operator != "AND" {
					t.Errorf("Expected top-level AND, got %s", logExpr.Operator)
				}
				// Left side should be OR expression (from parentheses)
				leftLog, ok := logExpr.Left.(*ast.LogicalExpression)
				if !ok {
					t.Errorf("Expected left side to be LogicalExpression (OR), got %T", logExpr.Left)
				} else if leftLog.Operator != "OR" {
					t.Errorf("Expected left side to be OR, got %s", leftLog.Operator)
				}
			},
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

			tt.validate(t, sel.Where)
		})
	}
}

// TestLogicalInUpdateDelete tests AND/OR in UPDATE and DELETE statements
func TestLogicalInUpdateDelete(t *testing.T) {
	t.Run("UPDATE with AND", func(t *testing.T) {
		input := "UPDATE users SET active = false WHERE age < 18 AND verified = false;"
		tokens, _ := lexer.Tokenize(input)
		p := New(tokens)
		stmt, err := p.Parse()
		if err != nil {
			t.Fatalf("Parser error: %v", err)
		}

		upd, ok := stmt.(*ast.UpdateStatement)
		if !ok {
			t.Fatalf("Expected UpdateStatement, got %T", stmt)
		}

		logExpr, ok := upd.Where.(*ast.LogicalExpression)
		if !ok {
			t.Fatalf("Expected LogicalExpression, got %T", upd.Where)
		}

		if logExpr.Operator != "AND" {
			t.Errorf("Expected AND, got %s", logExpr.Operator)
		}
	})

	t.Run("DELETE with OR", func(t *testing.T) {
		input := "DELETE FROM logs WHERE level = 'debug' OR level = 'trace';"
		tokens, _ := lexer.Tokenize(input)
		p := New(tokens)
		stmt, err := p.Parse()
		if err != nil {
			t.Fatalf("Parser error: %v", err)
		}

		del, ok := stmt.(*ast.DeleteStatement)
		if !ok {
			t.Fatalf("Expected DeleteStatement, got %T", stmt)
		}

		logExpr, ok := del.Where.(*ast.LogicalExpression)
		if !ok {
			t.Fatalf("Expected LogicalExpression, got %T", del.Where)
		}

		if logExpr.Operator != "OR" {
			t.Errorf("Expected OR, got %s", logExpr.Operator)
		}
	})
}
