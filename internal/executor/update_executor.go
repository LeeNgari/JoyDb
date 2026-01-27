package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/plan"
)

// executeUpdate handles UPDATE plans
func executeUpdate(node *plan.UpdateNode, db *schema.Database, tx *transaction.Transaction) (*Result, error) {
	table, ok := db.Tables[node.TableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", node.TableName)
	}

	// Use domain model to update
	rowsAffected, err := table.Update(node.Predicate, node.Updates, tx)
	if err != nil {
		return nil, err
	}

	return &Result{
		Message:      fmt.Sprintf("UPDATE %d", rowsAffected),
		RowsAffected: rowsAffected,
	}, nil
}
