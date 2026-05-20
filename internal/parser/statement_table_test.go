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
