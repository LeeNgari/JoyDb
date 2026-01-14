package engine

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/executor"
	"github.com/leengari/mini-rdbms/internal/parser"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
	"github.com/leengari/mini-rdbms/internal/planner"
)

// Engine is the main entry point for the database system
type Engine struct {
	db *schema.Database
}

// New creates a new Engine instance
func New(db *schema.Database) *Engine {
	return &Engine{db: db}
}

// Execute processes a SQL string and returns the result
func (e *Engine) Execute(sql string) (*executor.Result, error) {
	// 1. Tokenize
	tokens, err := lexer.Tokenize(sql)
	if err != nil {
		return nil, fmt.Errorf("lexer error: %w", err)
	}

	// 2. Parse
	p := parser.New(tokens)
	stmt, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// 3. Plan
	planNode, err := planner.Plan(stmt, e.db)
	if err != nil {
		return nil, fmt.Errorf("planning error: %w", err)
	}

	// 4. Execute
	result, err := executor.Execute(planNode, e.db)
	if err != nil {
		return nil, fmt.Errorf("execution error: %w", err)
	}

	return result, nil
}
