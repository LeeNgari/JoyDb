package engine

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/executor"
	"github.com/leengari/mini-rdbms/internal/parser"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
	"github.com/leengari/mini-rdbms/internal/planner"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

// Engine is the main entry point for the database system
type Engine struct {
	db       *schema.Database
	registry *manager.Registry
}

// New creates a new Engine instance
func New(db *schema.Database, registry *manager.Registry) *Engine {
	return &Engine{db: db, registry: registry}
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

	// 3. Handle Database Management Statements
	switch s := stmt.(type) {
	case *ast.CreateDatabaseStatement:
		if err := e.registry.Create(s.Name); err != nil {
			return nil, err
		}
		return &executor.Result{Message: fmt.Sprintf("Database '%s' created", s.Name)}, nil

	case *ast.DropDatabaseStatement:
		// If dropping currently active DB, unload it first
		if e.db != nil && e.db.Name == s.Name {
			e.db = nil
		}
		if err := e.registry.Drop(s.Name); err != nil {
			return nil, err
		}
		return &executor.Result{Message: fmt.Sprintf("Database '%s' dropped", s.Name)}, nil

	case *ast.AlterDatabaseStatement:
		// If renaming active DB, unload it (or update it, but unloading is safer for now)
		if e.db != nil && e.db.Name == s.Name {
			e.db = nil
		}
		if err := e.registry.Rename(s.Name, s.NewName); err != nil {
			return nil, err
		}
		return &executor.Result{Message: fmt.Sprintf("Database renamed from '%s' to '%s'", s.Name, s.NewName)}, nil

	case *ast.UseDatabaseStatement:
		// Load/Get new DB from registry
		newDB, err := e.registry.Get(s.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to load database '%s': %w", s.Name, err)
		}
		e.db = newDB
		return &executor.Result{Message: fmt.Sprintf("Switched to database '%s'", s.Name)}, nil
	}

	// 4. Ensure Database is Selected
	if e.db == nil {
		return nil, fmt.Errorf("no database selected. Use 'USE <database_name>' to select one")
	}

	// 5. Plan (for DML/DQL)
	planNode, err := planner.Plan(stmt, e.db)
	if err != nil {
		return nil, fmt.Errorf("planning error: %w", err)
	}

	// 6. Execute
	result, err := executor.Execute(planNode, e.db)
	if err != nil {
		return nil, fmt.Errorf("execution error: %w", err)
	}

	return result, nil
}
