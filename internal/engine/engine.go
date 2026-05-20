package engine

import (
	"fmt"
	"time"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/executor"
	"github.com/leengari/mini-rdbms/internal/parser"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/parser/lexer"
	"github.com/leengari/mini-rdbms/internal/plan"
	"github.com/leengari/mini-rdbms/internal/planner"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

// Engine is the main entry point for the database system
type Engine struct {
	db         *schema.Database
	registry   *manager.Registry
	walManager *manager.WALManager // WAL manager for current database
	observers  []Observer          // Observers for lifecycle events
}

func New(db *schema.Database, registry *manager.Registry) *Engine {
	return &Engine{
		db:        db,
		registry:  registry,
		observers: make([]Observer, 0),
	}
}

func (e *Engine) SetWALManager(wm *manager.WALManager) {
	e.walManager = wm
}
func (e *Engine) GetWALManager() *manager.WALManager {
	return e.walManager
}

// Execute processes a SQL string and returns the result
func (e *Engine) Execute(sql string) (*executor.Result, error) {
	//Start Transaction
	tx := transaction.NewTransaction()
	defer tx.Close()

	//Tokenize
	e.notify(Event{Type: EventLexStart, TxID: tx.ID, Data: sql})
	tokens, err := lexer.Tokenize(sql)
	if err != nil {
		return nil, fmt.Errorf("lexer error: %w", err)
	}
	e.notify(Event{Type: EventLexEnd, TxID: tx.ID, Data: len(tokens)})

	//Parse
	e.notify(Event{Type: EventParseStart, TxID: tx.ID})
	p := parser.New(tokens)
	stmt, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	e.notify(Event{Type: EventParseEnd, TxID: tx.ID, Data: fmt.Sprintf("%T", stmt)})

	switch s := stmt.(type) {
	case *ast.CreateDatabaseStatement:
		if err := e.registry.Create(s.Name); err != nil {
			return nil, err
		}
		return &executor.Result{Message: fmt.Sprintf("Database '%s' created", s.Name)}, nil

	case *ast.DropDatabaseStatement:
		if e.db != nil && e.db.Name == s.Name {
			e.db = nil
			e.walManager = nil
		}
		if err := e.registry.Drop(s.Name); err != nil {
			return nil, err
		}
		return &executor.Result{Message: fmt.Sprintf("Database '%s' dropped", s.Name)}, nil

	case *ast.AlterDatabaseStatement:
		if e.db != nil && e.db.Name == s.Name {
			e.db = nil
			e.walManager = nil
		}
		if err := e.registry.Rename(s.Name, s.NewName); err != nil {
			return nil, err
		}
		return &executor.Result{Message: fmt.Sprintf("Database renamed from '%s' to '%s'", s.Name, s.NewName)}, nil

	case *ast.UseDatabaseStatement:
		newDB, walMgr, err := e.registry.GetWithWAL(s.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to load database '%s': %w", s.Name, err)
		}
		e.db = newDB
		e.walManager = walMgr
		return &executor.Result{Message: fmt.Sprintf("Switched to database '%s'", s.Name)}, nil
	}

	if e.db == nil {
		return nil, fmt.Errorf("no database selected. Use 'USE <database_name>' to select one")
	}
	e.notify(Event{Type: EventPlanStart, TxID: tx.ID})
	planNode, err := planner.Plan(stmt, e.db, tx)
	if err != nil {
		return nil, fmt.Errorf("planning error: %w", err)
	}
	e.notify(Event{Type: EventPlanEnd, TxID: tx.ID, Data: fmt.Sprintf("%T", planNode)})

	needsWAL := false
	switch planNode.(type) {
	case *plan.InsertNode, *plan.UpdateNode, *plan.DeleteNode, *plan.CreateTableNode, *plan.DropTableNode:
		needsWAL = true
	}

	if e.walManager != nil && needsWAL {
		if err := e.walManager.BeginTransaction(tx); err != nil {
			return nil, fmt.Errorf("WAL begin failed: %w", err)
		}
	}

	e.notify(Event{Type: EventExecStart, TxID: tx.ID})
	var walMgr *manager.WALManager
	if needsWAL {
		walMgr = e.walManager
	}
	result, err := executor.ExecuteWithWAL(planNode, e.db, tx, walMgr)
	if err != nil {
		if e.walManager != nil && needsWAL {
			e.walManager.Abort(tx)
		}
		return nil, fmt.Errorf("execution error: %w", err)
	}
	e.notify(Event{Type: EventExecEnd, TxID: tx.ID, Data: map[string]interface{}{
		"rows_affected": result.RowsAffected,
		"rows_returned": len(result.Rows),
	}})

	if e.walManager != nil && needsWAL {
		if err := e.walManager.Commit(tx); err != nil {
			return nil, fmt.Errorf("WAL commit failed: %w", err)
		}
	}

	return result, nil
}

func (e *Engine) ListTables() ([]string, error) {
	if e.db == nil {
		return nil, fmt.Errorf("no database selected")
	}

	var tables []string
	for tableName := range e.db.Tables {
		tables = append(tables, tableName)
	}
	return tables, nil
}

func (e *Engine) AddObserver(observer Observer) {
	e.observers = append(e.observers, observer)
}
func (e *Engine) RemoveObserver(observer Observer) {
	for i, o := range e.observers {
		if o == observer {
			e.observers = append(e.observers[:i], e.observers[i+1:]...)
			return
		}
	}
}

func (e *Engine) notify(event Event) {
	event.Timestamp = time.Now()
	for _, observer := range e.observers {
		observer.OnEvent(event)
	}
}
