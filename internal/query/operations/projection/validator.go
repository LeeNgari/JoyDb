package projection

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
)

// ValidateProjection checks if all columns in the projection exist in the table schema
// Returns error if any column is not found
func ValidateProjection(table *schema.Table, proj *Projection) error {
	if proj == nil || proj.SelectAll {
		return nil
	}

	for _, colRef := range proj.Columns {
		// Skip validation for columns from other tables (for JOIN queries)
		if colRef.Table != "" && colRef.Table != table.Name {
			continue
		}

		found := false
		for _, col := range table.Schema.Columns {
			if col.Name == colRef.Column {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("column '%s' does not exist in table '%s'", colRef.Column, table.Name)
		}
	}

	return nil
}
