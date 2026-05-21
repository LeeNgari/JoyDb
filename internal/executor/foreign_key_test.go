package executor

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/errors"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/plan"
	"github.com/stretchr/testify/assert"
)

func TestForeignKey_ValidationFlow(t *testing.T) {
	db := &schema.Database{
		Name:   "testdb",
		Path:   "/tmp",
		Tables: make(map[string]*schema.Table),
	}
	ctx := &ExecutionContext{
		Database: db,
	}

	// 1. Create parent table 'users'
	createParentNode := &plan.CreateTableNode{
		TableName: "users",
		Columns: []schema.Column{
			{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true},
			{Name: "name", Type: schema.ColumnTypeText},
		},
	}
	_, err := executeCreateTable(createParentNode, ctx)
	assert.NoError(t, err)

	// 2. Create child table 'orders' referencing 'users(id)'
	createChildNode := &plan.CreateTableNode{
		TableName: "orders",
		Columns: []schema.Column{
			{Name: "id", Type: schema.ColumnTypeInt, PrimaryKey: true},
			{Name: "user_id", Type: schema.ColumnTypeInt},
		},
		ForeignKeys: []schema.ForeignKey{
			{ColumnName: "user_id", RefTableName: "users", RefColumnName: "id"},
		},
	}
	_, err = executeCreateTable(createChildNode, ctx)
	assert.NoError(t, err)

	// 3. Insert user record (id=1, name='Alice')
	usersTable := db.Tables["users"]
	err = usersTable.Insert(data.Row{
		Data: map[string]interface{}{"id": int64(1), "name": "Alice"},
	}, nil)
	assert.NoError(t, err)

	// 4. Try to insert order referencing non-existent user (user_id=2) -> should fail
	insertInvalidOrder := &plan.InsertNode{
		TableName: "orders",
		Row: data.Row{
			Data: map[string]interface{}{"id": int64(10), "user_id": int64(2)},
		},
	}
	_, err = executeInsertNode(insertInvalidOrder, ctx)
	assert.Error(t, err)
	constraintErr, ok := err.(*errors.ConstraintError)
	assert.True(t, ok)
	assert.Equal(t, "foreign_key", constraintErr.Constraint)

	// 5. Insert order referencing valid user (user_id=1) -> should succeed
	insertValidOrder := &plan.InsertNode{
		TableName: "orders",
		Row: data.Row{
			Data: map[string]interface{}{"id": int64(11), "user_id": int64(1)},
		},
	}
	_, err = executeInsertNode(insertValidOrder, ctx)
	assert.NoError(t, err)

	// 6. Try to update order's user_id to an invalid user (user_id=3) -> should fail
	updateInvalidOrder := &plan.UpdateNode{
		TableName: "orders",
		Predicate: func(r data.Row) bool {
			id, _ := normalizeNumeric(r.Data["id"])
			return id == 11
		},
		Updates: data.Row{
			Data: map[string]interface{}{"user_id": int64(3)},
		},
	}
	_, err = executeUpdateNode(updateInvalidOrder, ctx)
	assert.Error(t, err)

	// 7. Try to delete the parent user (id=1) which is referenced by order -> should fail (restricted)
	deleteParentNode := &plan.DeleteNode{
		TableName: "users",
		Predicate: func(r data.Row) bool {
			id, _ := normalizeNumeric(r.Data["id"])
			return id == 1
		},
	}
	_, err = executeDeleteNode(deleteParentNode, ctx)
	assert.Error(t, err)

	// 8. Delete the referencing order first
	deleteChildNode := &plan.DeleteNode{
		TableName: "orders",
		Predicate: func(r data.Row) bool {
			id, _ := normalizeNumeric(r.Data["id"])
			return id == 11
		},
	}
	_, err = executeDeleteNode(deleteChildNode, ctx)
	assert.NoError(t, err)

	// 9. Now delete parent user -> should succeed since referencing row is gone
	_, err = executeDeleteNode(deleteParentNode, ctx)
	assert.NoError(t, err)
}
