package planner

import (
	"strings"
	"testing"

	"github.com/leengari/joydb/internal/domain/schema"
	"github.com/leengari/joydb/internal/domain/transaction"
	"github.com/leengari/joydb/internal/index/btree"
	"github.com/leengari/joydb/internal/parser/ast"
	"github.com/leengari/joydb/internal/plan"
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
		val, exists := insertNode.RawData[k]
		if !exists {
			t.Errorf("Expected row to contain column %s", k)
			continue
		}
		if val != v {
			t.Errorf("Expected column %s to have value %v, got %v", k, v, val)
		}
	}
}

func TestPlanSelectGroupBy(t *testing.T) {
	db := &schema.Database{Tables: map[string]*schema.Table{
		"employees": {
			Name: "employees",
			Schema: &schema.TableSchema{Columns: []schema.Column{
				{Name: "id", Type: schema.ColumnTypeInt},
				{Name: "department", Type: schema.ColumnTypeText},
			}},
		},
	}}
	tx := transaction.NewTransaction()
	defer tx.Close()

	stmt := &ast.SelectStatement{
		TableName: &ast.Identifier{Value: "employees"},
		Fields: []ast.Expression{
			&ast.Identifier{Value: "department"},
			&ast.AggregateFunctionCall{Function: "COUNT", Argument: &ast.Identifier{Value: "*"}, Alias: "total"},
		},
		GroupBy: []*ast.Identifier{{Value: "department"}},
	}

	node, err := Plan(stmt, db, tx)
	if err != nil {
		t.Fatal(err)
	}
	selectNode := node.(*plan.SelectNode)
	if len(selectNode.GroupBy) != 1 || selectNode.GroupBy[0].Table != "employees" {
		t.Fatalf("unexpected GROUP BY plan: %+v", selectNode.GroupBy)
	}
	if len(selectNode.SelectItems) != 2 || selectNode.SelectItems[0].Column == nil || selectNode.SelectItems[1].Aggregate == nil {
		t.Fatalf("SELECT item order was not preserved: %+v", selectNode.SelectItems)
	}
}

func TestPlanSelectRejectsNonGroupedColumn(t *testing.T) {
	db := &schema.Database{Tables: map[string]*schema.Table{
		"employees": {
			Name: "employees",
			Schema: &schema.TableSchema{Columns: []schema.Column{
				{Name: "department", Type: schema.ColumnTypeText},
				{Name: "name", Type: schema.ColumnTypeText},
			}},
		},
	}}
	tx := transaction.NewTransaction()
	defer tx.Close()

	stmt := &ast.SelectStatement{
		TableName: &ast.Identifier{Value: "employees"},
		Fields: []ast.Expression{
			&ast.Identifier{Value: "name"},
			&ast.AggregateFunctionCall{Function: "COUNT", Argument: &ast.Identifier{Value: "*"}},
		},
		GroupBy: []*ast.Identifier{{Value: "department"}},
	}

	_, err := Plan(stmt, db, tx)
	if err == nil || !strings.Contains(err.Error(), "must appear in GROUP BY") {
		t.Fatalf("expected GROUP BY validation error, got %v", err)
	}
}

func TestPlanSelectIndexScan(t *testing.T) {
	db := &schema.Database{
		Name: "testdb",
		Tables: map[string]*schema.Table{
			"users": {
				Name: "users",
				Schema: &schema.TableSchema{
					Columns: []schema.Column{
						{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true},
						{Name: "name", Type: schema.ColumnTypeText},
					},
				},
				PKIndex: btree.New(32),
			},
		},
	}

	tx := transaction.NewTransaction()
	defer tx.Close()

	// 1. SELECT * FROM users WHERE id > 10
	stmt := &ast.SelectStatement{
		TableName: &ast.Identifier{TokenLiteralValue: "users", Value: "users"},
		Fields:    []ast.Expression{&ast.Identifier{TokenLiteralValue: "*", Value: "*"}},
		Where: &ast.BinaryExpression{
			Left:     &ast.Identifier{TokenLiteralValue: "id", Value: "id"},
			Operator: ">",
			Right:    &ast.Literal{TokenLiteralValue: "10", Value: int64(10), Kind: ast.LiteralInt},
		},
	}

	node, err := Plan(stmt, db, tx)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	selectNode, ok := node.(*plan.SelectNode)
	if !ok {
		t.Fatalf("Expected *plan.SelectNode, got %T", node)
	}

	if len(selectNode.Children()) != 1 {
		t.Fatalf("Expected exactly 1 child (IndexScanNode), got %d", len(selectNode.Children()))
	}

	indexScan, ok := selectNode.Children()[0].(*plan.IndexScanNode)
	if !ok {
		t.Fatalf("Expected child to be *plan.IndexScanNode, got %T", selectNode.Children()[0])
	}

	if indexScan.TableName != "users" || indexScan.ColumnName != "id" || indexScan.Operator != ">" || indexScan.Bound != int64(10) {
		t.Fatalf("IndexScanNode properties mismatch: %+v", indexScan)
	}

	// 2. Non-indexed column SELECT * FROM users WHERE name = 'Alice'
	stmtSeq := &ast.SelectStatement{
		TableName: &ast.Identifier{TokenLiteralValue: "users", Value: "users"},
		Fields:    []ast.Expression{&ast.Identifier{TokenLiteralValue: "*", Value: "*"}},
		Where: &ast.BinaryExpression{
			Left:     &ast.Identifier{TokenLiteralValue: "name", Value: "name"},
			Operator: "=",
			Right:    &ast.Literal{TokenLiteralValue: "'Alice'", Value: "Alice", Kind: ast.LiteralString},
		},
	}

	nodeSeq, err := Plan(stmtSeq, db, tx)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	selectNodeSeq, ok := nodeSeq.(*plan.SelectNode)
	if !ok {
		t.Fatalf("Expected *plan.SelectNode, got %T", nodeSeq)
	}

	if len(selectNodeSeq.Children()) != 0 {
		t.Fatalf("Expected no children (fallback to sequential scan), got %d", len(selectNodeSeq.Children()))
	}
}
