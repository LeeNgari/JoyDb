package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// executeDelete handles DELETE plans
func executeDelete(node *plan.DeleteNode, db *schema.Database, tx *transaction.Transaction) (*Result, error) {
	table, ok := db.Tables[node.TableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", node.TableName)
	}

	// Use domain model to delete
	rowsAffected, err := table.Delete(node.Predicate, tx)
	if err != nil {
		return nil, err
	}

	return &Result{
		Message:      fmt.Sprintf("DELETE %d", rowsAffected),
		RowsAffected: rowsAffected,
	}, nil
}
