package testutil

import (
	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/query/indexing"
)

// CreateTestTable creates a basic test table with common columns
func CreateTestTable(name string) *schema.Table {
	table := &schema.Table{
		Name: name,
		Schema: &schema.TableSchema{
			TableName: name,
			Columns: []schema.Column{
				{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true, NotNull: true},
				{Name: "name", Type: schema.ColumnTypeText, NotNull: true},
				{Name: "email", Type: schema.ColumnTypeText},
				{Name: "age", Type: schema.ColumnTypeInt},
			},
		},
		Rows:    []data.Row{},
		Indexes: make(map[string]*data.Index),
	}
	return table
}

// CreateUsersTable creates a users table with sample data for testing
func CreateUsersTable() *schema.Table {
	table := &schema.Table{
		Name: "users",
		Schema: &schema.TableSchema{
			TableName: "users",
			Columns: []schema.Column{
				{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true, NotNull: true},
				{Name: "username", Type: schema.ColumnTypeText, NotNull: true},
				{Name: "email", Type: schema.ColumnTypeText},
			},
		},
		Rows: []data.Row{
			data.NewRow(map[string]interface{}{"id": int64(1), "username": "alice", "email": "alice@example.com"}),
			data.NewRow(map[string]interface{}{"id": int64(2), "username": "bob", "email": "bob@example.com"}),
			data.NewRow(map[string]interface{}{"id": int64(3), "username": "charlie", "email": "charlie@example.com"}),
		},
		Indexes: make(map[string]*data.Index),
	}
	indexing.BuildIndexes(table)
	return table
}

// CreateOrdersTable creates an orders table with sample data for testing
func CreateOrdersTable() *schema.Table {
	table := &schema.Table{
		Name: "orders",
		Schema: &schema.TableSchema{
			TableName: "orders",
			Columns: []schema.Column{
				{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true, NotNull: true},
				{Name: "user_id", Type: schema.ColumnTypeInt, NotNull: true},
				{Name: "product", Type: schema.ColumnTypeText, NotNull: true},
				{Name: "amount", Type: schema.ColumnTypeFloat},
			},
		},
		Rows: []data.Row{
			data.NewRow(map[string]interface{}{"id": int64(1), "user_id": int64(1), "product": "Laptop", "amount": 999.99}),
			data.NewRow(map[string]interface{}{"id": int64(2), "user_id": int64(1), "product": "Mouse", "amount": 25.50}),
			data.NewRow(map[string]interface{}{"id": int64(3), "user_id": int64(2), "product": "Keyboard", "amount": 75.00}),
			// Note: user_id 3 (charlie) has no orders
		},
		Indexes: make(map[string]*data.Index),
	}
	indexing.BuildIndexes(table)
	return table
}
