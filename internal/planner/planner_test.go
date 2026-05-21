package planner

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/plan"
)

func TestPlanInsertColumnless(t *testing.T) {
	db := &schema.Database{
		Name: "testdb",
		Tables: map[string]*schema.Table{
			"users": {
				Name: "users",
				Schema: &schema.TableSchema{
					Columns: []schema.Column{
						{Name: "id", Type: schema.ColumnTypeInt},
						{Name: "name", Type: schema.ColumnTypeText},
						{Name: "role", Type: schema.ColumnTypeText},
					},
				},
			},
		},
	}

	tx := transaction.NewTransaction()
	defer tx.Close()

	stmt := &ast.InsertStatement{
		TableName: &ast.Identifier{TokenLiteralValue: "users", Value: "users"},
		Columns:   nil,
		Values: []ast.Expression{
			&ast.Literal{TokenLiteralValue: "1", Value: int64(1), Kind: ast.LiteralInt},
			&ast.Literal{TokenLiteralValue: "'Alice'", Value: "Alice", Kind: ast.LiteralString},
			&ast.Literal{TokenLiteralValue: "'admin'", Value: "admin", Kind: ast.LiteralString},
		},
	}

	node, err := Plan(stmt, db, tx)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	insertNode, ok := node.(*plan.InsertNode)
	if !ok {
		t.Fatalf("Expected Plan to return *plan.InsertNode, got %T", node)
	}

	expectedRow := map[string]interface{}{
		"id":   int64(1),
		"name": "Alice",
		"role": "admin",
	}

	for k, v := range expectedRow {
		val, exists := insertNode.Row.Data[k]
		if !exists {
			t.Errorf("Expected row to contain column %s", k)
			continue
		}
		if val != v {
			t.Errorf("Expected column %s to have value %v, got %v", k, v, val)
		}
	}
}
