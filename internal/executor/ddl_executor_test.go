package executor

import (
"testing"
"github.com/leengari/mini-rdbms/internal/domain/schema"
"github.com/leengari/mini-rdbms/internal/plan"
"github.com/stretchr/testify/assert"
)

func TestExecuteCreateTable(t *testing.T) {
db := &schema.Database{Name: "testdb", Path: "/tmp", Tables: make(map[string]*schema.Table)}
ctx := &ExecutionContext{
Database: db,
}

node := &plan.CreateTableNode{
TableName: "users",
Columns: []schema.Column{
{Name: "id", Type: schema.ColumnTypeInt},
{Name: "name", Type: schema.ColumnTypeText},
},
}

res, err := executeCreateTable(node, ctx)
assert.NoError(t, err)
assert.NotNil(t, res)

assert.Contains(t, db.Tables, "users")
assert.Equal(t, 2, len(db.Tables["users"].Schema.Columns))
}

func TestExecuteDropTable(t *testing.T) {
db := &schema.Database{Name: "testdb", Path: "/tmp", Tables: make(map[string]*schema.Table)}
ctx := &ExecutionContext{
Database: db,
}

// Create table first
db.Tables["users"] = &schema.Table{
Name: "users",
}

node := &plan.DropTableNode{
TableName: "users",
}

res, err := executeDropTable(node, ctx)
assert.NoError(t, err)
assert.NotNil(t, res)

assert.NotContains(t, db.Tables, "users")
}
