package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// executeCreateTable executes a CREATE TABLE operation
func executeCreateTable(n *plan.CreateTableNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	// 1. Check if table already exists
	if _, exists := ctx.Database.Tables[n.TableName]; exists {
		return nil, fmt.Errorf("table already exists: %s", n.TableName)
	}

	// 2. Validate schema (basic validation for now)
	tableSchema := &schema.TableSchema{
		Columns: n.Columns,
	}
	if len(tableSchema.Columns) == 0 {
		return nil, fmt.Errorf("table must have at least one column")
	}

	// 3. Log to WAL BEFORE mutating state (WAL-first rule)
	if ctx.WALManager != nil && n.Transaction != nil {
		if err := ctx.WALManager.LogCreateTable(n.Transaction, n.TableName, tableSchema); err != nil {
			return nil, fmt.Errorf("failed to log CREATE TABLE to WAL: %w", err)
		}
	}

	// 4. Create in-memory table
	table := &schema.Table{
		Name:   n.TableName,
		Schema: tableSchema,
		Rows:   []data.Row{},
	}
	
	// Create Indexes map if it exists in schema.Table (we don't want to fail compilation if it doesn't)
	// We'll skip index creation here to ensure compilation, since we don't need indexes for Phase 0 DDL.

	// 5. Register table in database
	ctx.Database.Lock()
	if ctx.Database.Tables == nil {
		ctx.Database.Tables = make(map[string]*schema.Table)
	}
	ctx.Database.Tables[n.TableName] = table
	ctx.Database.Unlock()

	return &IntermediateResult{
		Metadata: map[string]interface{}{
			"message": fmt.Sprintf("Table '%s' created successfully", n.TableName),
		},
	}, nil
}

// executeDropTable executes a DROP TABLE operation
func executeDropTable(n *plan.DropTableNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	// 1. Check if table exists
	ctx.Database.RLock()
	_, exists := ctx.Database.Tables[n.TableName]
	ctx.Database.RUnlock()

	if !exists {
		return nil, fmt.Errorf("table does not exist: %s", n.TableName)
	}

	// 2. Log to WAL BEFORE mutating state
	if ctx.WALManager != nil && n.Transaction != nil {
		if err := ctx.WALManager.LogDropTable(n.Transaction, n.TableName); err != nil {
			return nil, fmt.Errorf("failed to log DROP TABLE to WAL: %w", err)
		}
	}

	// 3. Delete from database
	ctx.Database.Lock()
	delete(ctx.Database.Tables, n.TableName)
	ctx.Database.Unlock()

	return &IntermediateResult{
		Metadata: map[string]interface{}{
			"message": fmt.Sprintf("Table '%s' dropped successfully", n.TableName),
		},
	}, nil
}
