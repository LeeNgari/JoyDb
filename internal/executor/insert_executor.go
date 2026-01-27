package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// executeInsert handles INSERT plans
func executeInsert(node *plan.InsertNode, db *schema.Database, tx *transaction.Transaction) (*Result, error) {
	table, ok := db.Tables[node.TableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", node.TableName)
	}

	// Insert the row using domain model
	if err := table.Insert(node.Row, tx); err != nil {
		return nil, err
	}

	return &Result{
		Message: "INSERT 1",
	}, nil
}
