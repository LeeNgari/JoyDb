package parser

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
)

func TestParseCreateTable(t *testing.T) {
	input := `CREATE TABLE users (
		id INT PRIMARY KEY AUTO_INCREMENT,
		email TEXT UNIQUE NOT NULL,
		age INT
	);`

	l := lexer.New(input)
	var tokens []lexer.Token
	for tok := l.NextToken(); tok.Type != lexer.EOF; tok = l.NextToken() {
		tokens = append(tokens, tok)
	}

	p := New(tokens)
	stmt, err := p.Parse()
	assert.NoError(t, err)
	assert.NotNil(t, stmt)

	createStmt, ok := stmt.(*ast.CreateTableStatement)
	assert.True(t, ok)
	assert.Equal(t, "CREATE", createStmt.TokenLiteral())
	assert.Equal(t, "users", createStmt.TableName)
	assert.Len(t, createStmt.Columns, 3)
}

func TestParseDropTable(t *testing.T) {
	input := `DROP TABLE users;`

	l := lexer.New(input)
	var tokens []lexer.Token
	for tok := l.NextToken(); tok.Type != lexer.EOF; tok = l.NextToken() {
		tokens = append(tokens, tok)
	}

	p := New(tokens)
	stmt, err := p.Parse()
	assert.NoError(t, err)
	assert.NotNil(t, stmt)
}
