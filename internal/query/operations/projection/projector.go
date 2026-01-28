package projection

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
)

// ProjectRow applies projection to a single row
// Returns a new row containing only the requested columns
// If projection is nil or SelectAll is true, returns a copy of the entire row
func ProjectRow(row data.Row, proj *Projection, tableName string) data.Row {
	// If no projection or SELECT *, return full row
	if proj == nil || proj.SelectAll {
		return row.Copy()
	}

	projected := make(map[string]interface{})

	for _, colRef := range proj.Columns {
		// Skip columns from other tables (for JOIN queries)
		if colRef.Table != "" && colRef.Table != tableName {
			continue
		}

		value, exists := row.Data[colRef.Column]
		if !exists {
			// Column doesn't exist in this row - skip it
			// This can happen in JOIN queries where not all columns are present
			continue
		}

		// Use alias if provided, otherwise use column name
		key := colRef.Column
		if colRef.Alias != "" {
			key = colRef.Alias
		}

		projected[key] = value
	}

	return data.NewRow(projected)
}

// ProjectJoinedRow applies projection to a joined row
// Handles table-qualified column names (e.g., "users.id", "orders.product")
func ProjectJoinedRow(row data.JoinedRow, proj *Projection) data.JoinedRow {
	// If no projection or SELECT *, return full row
	if proj == nil || proj.SelectAll {
		return row
	}

	projected := data.NewJoinedRow()

	for _, colRef := range proj.Columns {
		// Build the qualified column name
		var qualifiedName string
		if colRef.Table != "" {
			qualifiedName = fmt.Sprintf("%s.%s", colRef.Table, colRef.Column)
		} else {
			// If no table qualifier, we need to search for the column
			// This handles ambiguous columns - user should qualify them
			qualifiedName = colRef.Column
		}

		value, exists := row.Get(qualifiedName)
		if !exists && colRef.Table == "" {
			// Try to find the column in any table
			// This is a fallback for unqualified column names
			for key, val := range row.Data {
				if key == colRef.Column || len(key) > len(colRef.Column) && key[len(key)-len(colRef.Column)-1:] == "."+colRef.Column {
					value = val
					exists = true
					break
				}
			}
		}

		if !exists {
			// Column doesn't exist - skip it
			continue
		}

		// Use alias if provided, otherwise use qualified name
		key := qualifiedName
		if colRef.Alias != "" {
			key = colRef.Alias
		}

		projected.Set(key, value)
	}

	return projected
}
