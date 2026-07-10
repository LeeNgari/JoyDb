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
		RowsByRID: make(map[int64]data.Row),
		Indexes:   make(map[string]*data.Index),
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
		RowsByRID: make(map[int64]data.Row),
		Indexes:   make(map[string]*data.Index),
	}
	table.InsertReplay(data.NewRow([]interface{}{int64(1), "alice", "alice@example.com"}))
	table.InsertReplay(data.NewRow([]interface{}{int64(2), "bob", "bob@example.com"}))
	table.InsertReplay(data.NewRow([]interface{}{int64(3), "charlie", "charlie@example.com"}))

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
		RowsByRID: make(map[int64]data.Row),
		Indexes:   make(map[string]*data.Index),
	}
	table.InsertReplay(data.NewRow([]interface{}{int64(1), int64(1), "Laptop", 999.99}))
	table.InsertReplay(data.NewRow([]interface{}{int64(2), int64(1), "Mouse", 25.50}))
	table.InsertReplay(data.NewRow([]interface{}{int64(3), int64(2), "Keyboard", 75.00}))
	// Note: user_id 3 (charlie) has no orders

	indexing.BuildIndexes(table)
	return table
}
