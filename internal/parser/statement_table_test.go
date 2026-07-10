package parser

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
	"github.com/stretchr/testify/assert"
)

func TestParseCreateTable(t *testing.T) {
	sql := "CREATE TABLE users (id INT PRIMARY KEY AUTO_INCREMENT, name TEXT NOT NULL UNIQUE, email TEXT);"

	tokens, err := lexer.Tokenize(sql)
	assert.NoError(t, err)

	p := New(tokens)
	stmt, err := p.Parse()

	assert.NoError(t, err)
	assert.NotNil(t, stmt)

	createStmt, ok := stmt.(*ast.CreateTableStatement)
	assert.True(t, ok)
	assert.Equal(t, "users", createStmt.TableName)
	assert.Equal(t, 3, len(createStmt.Columns))

	assert.Equal(t, "id", createStmt.Columns[0].Name)
	assert.Equal(t, "INT", createStmt.Columns[0].Type)
	assert.True(t, createStmt.Columns[0].PrimaryKey)
	assert.True(t, createStmt.Columns[0].AutoIncrement)

	assert.Equal(t, "name", createStmt.Columns[1].Name)
	assert.Equal(t, "TEXT", createStmt.Columns[1].Type)
	assert.True(t, createStmt.Columns[1].NotNull)
	assert.True(t, createStmt.Columns[1].Unique)

	assert.Equal(t, "email", createStmt.Columns[2].Name)
	assert.Equal(t, "TEXT", createStmt.Columns[2].Type)
	assert.False(t, createStmt.Columns[2].NotNull)
}

func TestParseDropTable(t *testing.T) {
	sql := "DROP TABLE users;"

	tokens, err := lexer.Tokenize(sql)
	assert.NoError(t, err)

	p := New(tokens)
	stmt, err := p.Parse()

	assert.NoError(t, err)
	assert.NotNil(t, stmt)

	dropStmt, ok := stmt.(*ast.DropTableStatement)
	assert.True(t, ok)
	assert.Equal(t, "users", dropStmt.TableName)
}

func TestParseCreateTable_ForeignKeys(t *testing.T) {
	// 1. Table-level FOREIGN KEY
	sqlTableLevel := "CREATE TABLE orders (id INT PRIMARY KEY, user_id INT, FOREIGN KEY (user_id) REFERENCES users(id));"
	tokens1, err := lexer.Tokenize(sqlTableLevel)
	assert.NoError(t, err)
	p1 := New(tokens1)
	stmt1, err := p1.Parse()
	assert.NoError(t, err)
	createStmt1, ok := stmt1.(*ast.CreateTableStatement)
	assert.True(t, ok)
	assert.Equal(t, "orders", createStmt1.TableName)
	assert.Equal(t, 2, len(createStmt1.Columns))
	assert.Equal(t, 1, len(createStmt1.ForeignKeys))
	assert.Equal(t, "user_id", createStmt1.ForeignKeys[0].ColumnName)
	assert.Equal(t, "users", createStmt1.ForeignKeys[0].RefTableName)
	assert.Equal(t, "id", createStmt1.ForeignKeys[0].RefColumnName)

	// 2. Column-level (inline) REFERENCES
	sqlColumnLevel := "CREATE TABLE orders (id INT PRIMARY KEY, user_id INT REFERENCES users(id));"
	tokens2, err := lexer.Tokenize(sqlColumnLevel)
	assert.NoError(t, err)
	p2 := New(tokens2)
	stmt2, err := p2.Parse()
	assert.NoError(t, err)
	createStmt2, ok := stmt2.(*ast.CreateTableStatement)
	assert.True(t, ok)
	assert.Equal(t, "orders", createStmt2.TableName)
	assert.Equal(t, 2, len(createStmt2.Columns))
	assert.Equal(t, 1, len(createStmt2.ForeignKeys))
	assert.Equal(t, "user_id", createStmt2.ForeignKeys[0].ColumnName)
	assert.Equal(t, "users", createStmt2.ForeignKeys[0].RefTableName)
	assert.Equal(t, "id", createStmt2.ForeignKeys[0].RefColumnName)
}
