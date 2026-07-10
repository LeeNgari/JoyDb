package lexer

import (
	"testing"
)

func TestNextToken(t *testing.T) {
	input := `SELECT * FROM users WHERE id = 1;
INSERT INTO items (name, price) VALUES ('apple', 1.23);`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{SELECT, "SELECT"},
		{ASTERISK, "*"},
		{FROM, "FROM"},
		{IDENTIFIER, "users"},
		{WHERE, "WHERE"},
		{IDENTIFIER, "id"},
		{EQUALS, "="},
		{NUMBER, "1"},
		{SEMICOLON, ";"},
		{INSERT, "INSERT"},
		{INTO, "INTO"},
		{IDENTIFIER, "items"},
		{PAREN_OPEN, "("},
		{IDENTIFIER, "name"},
		{COMMA, ","},
		{IDENTIFIER, "price"},
		{PAREN_CLOSE, ")"},
		{VALUES, "VALUES"},
		{PAREN_OPEN, "("},
		{STRING, "apple"},
		{COMMA, ","},
		{NUMBER, "1.23"},
		{PAREN_CLOSE, ")"},
		{SEMICOLON, ";"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}
