package projection

import (
	"fmt"

	"github.com/leengari/joydb/internal/domain/data"
	"github.com/leengari/joydb/internal/domain/schema"
)

// ProjectRow applies projection to a single row
// Returns a new row containing only the requested columns
// If projection is nil or SelectAll is true, returns a copy of the entire row
func ProjectRow(row data.Row, proj *Projection, tableSchema *schema.TableSchema) data.Row {
	// If no projection or SELECT *, return full row
	if proj == nil || proj.SelectAll {
		return row.Copy()
	}

	projectedVals := make([]interface{}, len(proj.Columns))

	for i, colRef := range proj.Columns {
		idx := -1
		if tableSchema != nil {
			name := colRef.Column
			if colRef.Table != "" {
				name = fmt.Sprintf("%s.%s", colRef.Table, colRef.Column)
			}
			idx = tableSchema.GetColumnIndex(name)
			if idx == -1 {
				// Fallback to unqualified name
				idx = tableSchema.GetColumnIndex(colRef.Column)
			}
		}

		if idx == -1 || idx >= len(row.Values) {
			// Column doesn't exist in this row - skip it (nil value)
			continue
		}

		projectedVals[i] = row.Values[idx]
	}

	return data.NewRow(projectedVals)
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
		if !exists {
			// Fallback for simple table scans where row keys are unqualified
			if colRef.Table != "" {
				value, exists = row.Data[colRef.Column]
			}

			// Fallback for unqualified column names or general matching
			if !exists {
				for key, val := range row.Data {
					if key == colRef.Column || len(key) > len(colRef.Column) && key[len(key)-len(colRef.Column)-1:] == "."+colRef.Column {
						value = val
						exists = true
						break
					}
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
