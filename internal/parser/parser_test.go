package parser

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

func TestParseSelect(t *testing.T) {
	input := "SELECT id, name FROM users WHERE id = 1;"
	tokens, err := lexer.Tokenize(input)
	if err != nil {
		t.Fatalf("Lexer error: %v", err)
	}

	p := New(tokens)
	stmt, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	sel, ok := stmt.(*ast.SelectStatement)
	if !ok {
		t.Fatalf("Expected SelectStatement, got %T", stmt)
	}

	if len(sel.Fields) != 2 {
		t.Fatalf("Expected 2 fields, got %d", len(sel.Fields))
	}
	if sel.Fields[0].(*ast.Identifier).Value != "id" {
		t.Errorf("Expected field 0 to be id, got %s", sel.Fields[0].(*ast.Identifier).Value)
	}
	if sel.Fields[1].(*ast.Identifier).Value != "name" {
		t.Errorf("Expected field 1 to be name, got %s", sel.Fields[1].(*ast.Identifier).Value)
	}

	if sel.TableName.Value != "users" {
		t.Errorf("Expected table users, got %s", sel.TableName.Value)
	}

	if sel.Where == nil {
		t.Fatal("Expected Where clause, got nil")
	}

	binExpr, ok := sel.Where.(*ast.BinaryExpression)
	if !ok {
		t.Fatalf("Expected BinaryExpression in Where, got %T", sel.Where)
	}

	if binExpr.Left.(*ast.Identifier).Value != "id" {
		t.Errorf("Expected left side id, got %s", binExpr.Left)
	}
	if binExpr.Operator != "=" {
		t.Errorf("Expected operator =, got %s", binExpr.Operator)
	}
	if binExpr.Right.(*ast.Literal).Value.(int) != 1 {
		t.Errorf("Expected right side 1, got %v", binExpr.Right)
	}
}

func TestParseSelectLimitOffset(t *testing.T) {
	input := "SELECT id FROM users LIMIT 10 OFFSET 5;"
	tokens, err := lexer.Tokenize(input)
	if err != nil {
		t.Fatalf("Lexer error: %v", err)
	}

	p := New(tokens)
	stmt, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	sel, ok := stmt.(*ast.SelectStatement)
	if !ok {
		t.Fatalf("Expected SelectStatement, got %T", stmt)
	}

	if sel.Limit == nil || *sel.Limit != 10 {
		t.Errorf("Expected Limit 10, got %v", sel.Limit)
	}

	if sel.Offset == nil || *sel.Offset != 5 {
		t.Errorf("Expected Offset 5, got %v", sel.Offset)
	}
}

func TestParseInsert(t *testing.T) {
	input := "INSERT INTO items (name, price) VALUES ('apple', 1.23);"
	tokens, err := lexer.Tokenize(input)
	if err != nil {
		t.Fatalf("Lexer error: %v", err)
	}

	p := New(tokens)
	stmt, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	ins, ok := stmt.(*ast.InsertStatement)
	if !ok {
		t.Fatalf("Expected InsertStatement, got %T", stmt)
	}

	if ins.TableName.Value != "items" {
		t.Errorf("Expected table items, got %s", ins.TableName.Value)
	}

	if len(ins.Columns) != 2 {
		t.Fatalf("Expected 2 columns, got %d", len(ins.Columns))
	}
	if ins.Columns[0].Value != "name" {
		t.Errorf("Expected col 0 to be name, got %s", ins.Columns[0].Value)
	}

	if len(ins.Values) != 2 {
		t.Fatalf("Expected 2 values, got %d", len(ins.Values))
	}
	
	val1, ok := ins.Values[0].(*ast.Literal)
	if !ok || val1.Value != "apple" {
		t.Errorf("Expected value 0 to be 'apple', got %v", ins.Values[0])
	}

	val2, ok := ins.Values[1].(*ast.Literal)
	if !ok || val2.Value != 1.23 {
		t.Errorf("Expected value 1 to be 1.23, got %v", ins.Values[1])
	}
}

func TestParseUpdate(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedTable string
		expectedCol   string
		expectedVal   interface{}
		hasWhere      bool
	}{
		{
			name:          "UPDATE with WHERE",
			input:         "UPDATE users SET email = 'new@test.com' WHERE id = 5;",
			expectedTable: "users",
			expectedCol:   "email",
			expectedVal:   "new@test.com",
			hasWhere:      true,
		},
		{
			name:          "UPDATE without WHERE",
			input:         "UPDATE products SET price = 99.99;",
			expectedTable: "products",
			expectedCol:   "price",
			expectedVal:   99.99,
			hasWhere:      false,
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

			if upd.TableName.Value != tt.expectedTable {
				t.Errorf("Expected table %s, got %s", tt.expectedTable, upd.TableName.Value)
			}

			expr, exists := upd.Updates[tt.expectedCol]
			if !exists {
				t.Fatalf("Expected column %s in updates", tt.expectedCol)
			}

			lit, ok := expr.(*ast.Literal)
			if !ok {
				t.Fatalf("Expected literal value, got %T", expr)
			}

			if lit.Value != tt.expectedVal {
				t.Errorf("Expected value %v, got %v", tt.expectedVal, lit.Value)
			}

			if tt.hasWhere && upd.Where == nil {
				t.Error("Expected WHERE clause, got nil")
			}
			if !tt.hasWhere && upd.Where != nil {
				t.Error("Expected no WHERE clause, but got one")
			}
		})
	}
}

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

			if del.TableName.Value != tt.expectedTable {
				t.Errorf("Expected table %s, got %s", tt.expectedTable, del.TableName.Value)
			}

			if tt.hasWhere && del.Where == nil {
				t.Error("Expected WHERE clause, got nil")
			}
			if !tt.hasWhere && del.Where != nil {
				t.Error("Expected no WHERE clause, but got one")
			}
		})
	}
}
