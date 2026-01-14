# Lexer Package

The lexer package performs **lexical analysis** (tokenization) of SQL input strings, converting raw text into a stream of tokens that the parser can understand.

## Responsibility

The lexer's job is to:
1. Read SQL text character by character
2. Recognize keywords, identifiers, operators, and literals
3. Produce a sequence of tokens with type and position information
4. Report lexical errors (e.g., unterminated strings)

## Architecture

```
SQL String → Lexer → Token Stream → Parser
```

### Files

| File | Responsibility |
|------|---------------|
| `lexer.go` | Tokenization logic and token types |

## Token Types

### Keywords
```
SELECT, FROM, WHERE, INSERT, INTO, VALUES, UPDATE, SET, DELETE,
AND, OR, TRUE, FALSE, JOIN, INNER, LEFT, RIGHT, FULL, OUTER, ON,
DATE, TIME, EMAIL
```

### Operators & Punctuation
```
* , ( ) = < > <= >= != <> . ;
```

### Literals
```
IDENTIFIER  - table names, column names (e.g., users, id)
STRING      - string literals (e.g., 'hello')
NUMBER      - numeric literals (e.g., 42, 3.14)
```

### Special
```
EOF     - End of file
ILLEGAL - Invalid token
```

## Usage

### Basic Tokenization

```go
import "github.com/leengari/mini-rdbms/internal/parser/lexer"

// Tokenize SQL string
tokens, err := lexer.Tokenize("SELECT * FROM users WHERE id = 5")
if err != nil {
    // Handle lexical error
}

// Iterate through tokens
for _, tok := range tokens {
    fmt.Printf("%s: %q\n", tok.Type, tok.Literal)
}
```

### Manual Lexing

```go
l := lexer.New("SELECT * FROM users")

for {
    tok := l.NextToken()
    if tok.Type == lexer.EOF {
        break
    }
    fmt.Println(tok)
}
```

## Token Structure

```go
type Token struct {
    Type    TokenType  // Token type (SELECT, IDENTIFIER, etc.)
    Literal string     // Actual text (e.g., "users", "5")
    Line    int        // Line number (1-indexed)
    Column  int        // Column number (1-indexed)
}
```

## Adding a New Keyword

To add support for a new SQL keyword (e.g., `LIMIT`):

### 1. Add Token Type

Edit `lexer.go`:
```go
const (
    // ... existing token types
    LIMIT
)
```

### 2. Add to Keywords Map

```go
var keywords = map[string]TokenType{
    // ... existing keywords
    "LIMIT": LIMIT,
}
```

That's it! The lexer will now recognize `LIMIT` as a keyword token.

### 3. Use in Parser

The parser can now check for this token:
```go
if p.curTok.Type == lexer.LIMIT {
    // Parse LIMIT clause
}
```

## Adding a New Operator

To add a new operator (e.g., `%` for modulo):

### 1. Add Token Type

```go
const (
    // ... existing operators
    MODULO  // %
)
```

### 2. Add to NextToken()

```go
func (l *Lexer) NextToken() Token {
    // ... existing cases
    case '%':
        tok = newToken(MODULO, l.ch, l.line, l.column)
    // ...
}
```

## Lexer Features

### Case-Insensitive Keywords

Keywords are case-insensitive:
```sql
SELECT = select = SeLeCt
```

Implementation:
```go
func LookupIdent(ident string) TokenType {
    if tok, ok := keywords[strings.ToUpper(ident)]; ok {
        return tok
    }
    return IDENTIFIER
}
```

### Multi-Character Operators

The lexer handles operators like `<=`, `>=`, `!=`, `<>`:

```go
case '<':
    if l.peekChar() == '=' {
        // <=
        ch := l.ch
        l.readChar()
        tok = Token{Type: LESS_EQUAL, Literal: string(ch) + string(l.ch)}
    } else if l.peekChar() == '>' {
        // <>
        ch := l.ch
        l.readChar()
        tok = Token{Type: NOT_EQUAL, Literal: string(ch) + string(l.ch)}
    } else {
        // <
        tok = newToken(LESS_THAN, l.ch)
    }
```

### String Literals

Strings are enclosed in single quotes:
```sql
'hello world'
'user@example.com'
```

The lexer handles string reading:
```go
func (l *Lexer) readString() string {
    position := l.position + 1
    for {
        l.readChar()
        if l.ch == '\'' || l.ch == 0 {
            break
        }
    }
    return l.input[position:l.position]
}
```

### Number Literals

Supports integers and floats:
```sql
42      -- integer
3.14    -- float
0.5     -- float
```

```go
func (l *Lexer) readNumber() string {
    position := l.position
    for isDigit(l.ch) {
        l.readChar()
    }
    // Support floats
    if l.ch == '.' && isDigit(l.peekChar()) {
        l.readChar()
        for isDigit(l.ch) {
            l.readChar()
        }
    }
    return l.input[position:l.position]
}
```

### Position Tracking

The lexer tracks line and column numbers for error reporting:

```go
func (l *Lexer) readChar() {
    // ... read character
    l.column++
}

func (l *Lexer) skipWhitespace() {
    for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
        if l.ch == '\n' {
            l.line++
            l.column = 0
        }
        l.readChar()
    }
}
```

## Error Handling

The `Tokenize` helper function reports illegal tokens:

```go
func Tokenize(input string) ([]Token, error) {
    l := New(input)
    var tokens []Token
    for {
        tok := l.NextToken()
        if tok.Type == EOF {
            break
        }
        if tok.Type == ILLEGAL {
            return nil, fmt.Errorf("illegal token at line %d, col %d: %s",
                tok.Line, tok.Column, tok.Literal)
        }
        tokens = append(tokens, tok)
    }
    return tokens, nil
}
```

## Design Principles

1. **Single Responsibility**: Only tokenization, no parsing
2. **Stateless**: Each token is independent
3. **Lookahead**: Uses `peekChar()` for multi-character operators
4. **Error Reporting**: Provides line/column information

## Testing

Run lexer tests:
```bash
go test ./internal/parser/lexer/...
```

Test specific functionality:
```go
func TestTokenizeSelect(t *testing.T) {
    input := "SELECT * FROM users"
    tokens, err := lexer.Tokenize(input)
    
    if err != nil {
        t.Fatalf("Tokenize error: %v", err)
    }
    
    expected := []lexer.TokenType{
        lexer.SELECT,
        lexer.ASTERISK,
        lexer.FROM,
        lexer.IDENTIFIER,
    }
    
    for i, tok := range tokens {
        if tok.Type != expected[i] {
            t.Errorf("Token %d: expected %v, got %v", i, expected[i], tok.Type)
        }
    }
}
```

## Common Patterns

### Keyword vs Identifier

```go
// Read identifier
ident := l.readIdentifier()

// Check if it's a keyword
tokType := LookupIdent(ident)

// tokType is either a keyword token or IDENTIFIER
```

### Whitespace Handling

```go
// Skip whitespace before each token
l.skipWhitespace()

// Then read the actual token
tok := l.NextToken()
```

## Related Packages

- `parser/` - Consumes tokens to build AST
- `ast/` - Defines node types that tokens become
