package executor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/index/btree"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// executeCreateTable executes a CREATE TABLE operation
func executeCreateTable(n *plan.CreateTableNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	// 1. Transactional check
	ctx.Database.RLock()
	_, exists := ctx.Database.Tables[n.TableName]
	ctx.Database.RUnlock()
	if exists {
		return nil, fmt.Errorf("table already exists: %s", n.TableName)
	}

	// 2. Validate schema
	tableSchema := &schema.TableSchema{
		TableName: n.TableName,
		Columns:   n.Columns,
	}
	if err := tableSchema.Validate(); err != nil {
		return nil, fmt.Errorf("invalid table schema: %w", err)
	}

	// 3. Log to WAL BEFORE mutating state (WAL-first rule)
	if n.Transaction != nil && ctx.WALManager != nil {
		if err := ctx.WALManager.LogCreateTable(n.Transaction, n.TableName, tableSchema); err != nil {
			return nil, fmt.Errorf("WAL logging failed: %w", err)
		}
	}

	// 4. Create table directory and in-memory table
	tablePath := filepath.Join(ctx.Database.Path, n.TableName)
	if err := os.MkdirAll(tablePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create table directory: %w", err)
	}

	table := &schema.Table{
		Name:    n.TableName,
		Path:    tablePath,
		Schema:  tableSchema,
		Rows:    []data.Row{},
		Indexes: make(map[string]*data.Index),
	}

	// 5. Build index structures for PRIMARY KEY and UNIQUE columns
	pkColumn := tableSchema.GetPrimaryKeyColumn()
	if pkColumn != nil {
		table.Indexes[pkColumn.Name] = &data.Index{
			Column: pkColumn.Name,
			Data:   make(map[interface{}][]int),
			Unique: true,
		}
		table.PKIndex = btree.New(btree.DefaultDegree)
	}

	for _, col := range tableSchema.Columns {
		if col.Unique && !col.PrimaryKey {
			table.Indexes[col.Name] = &data.Index{
				Column: col.Name,
				Data:   make(map[interface{}][]int),
				Unique: true,
			}
		}
	}

	// 6. Register table in database
	ctx.Database.Lock()
	ctx.Database.Tables[n.TableName] = table
	ctx.Database.Unlock()

	return &IntermediateResult{
		Metadata:     map[string]interface{}{"status": "table created"},
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
