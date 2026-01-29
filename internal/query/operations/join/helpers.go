package join

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
)

// validateJoinCondition checks if the join is valid
func validateJoinCondition(
	leftTable *schema.Table,
	rightTable *schema.Table,
	leftColumn *string,
	rightColumn *string,
) error {
	if leftTable == nil {
		return fmt.Errorf("left table is nil")
	}
	if rightTable == nil {
		return fmt.Errorf("right table is nil")
	}

	// Find columns in schemas (supporting both qualified and unqualified names)
	var leftCol, rightCol *schema.Column
	for i := range leftTable.Schema.Columns {
		if leftTable.Schema.Columns[i].Name == *leftColumn {
			leftCol = &leftTable.Schema.Columns[i]
			break
		}
		// Try matching by suffix if it's a join result (e.g., "users.id" matches "id")
		if strings.HasSuffix(leftTable.Schema.Columns[i].Name, "."+*leftColumn) {
			leftCol = &leftTable.Schema.Columns[i]
			break
		}
	}
	for i := range rightTable.Schema.Columns {
		if rightTable.Schema.Columns[i].Name == *rightColumn {
			rightCol = &rightTable.Schema.Columns[i]
			break
		}
		if strings.HasSuffix(rightTable.Schema.Columns[i].Name, "."+*rightColumn) {
			rightCol = &rightTable.Schema.Columns[i]
			break
		}
	}

	if leftCol == nil {
		return fmt.Errorf("column '%s' not found in table '%s'", *leftColumn, leftTable.Name)
	}
	if rightCol == nil {
		return fmt.Errorf("column '%s' not found in table '%s'", *rightColumn, rightTable.Name)
	}

	// Update the columns to use the actual fully qualified names discovered
	// This ensures BuildJoinIndex uses the correct key in Row.Data
	*leftColumn = leftCol.Name
	*rightColumn = rightCol.Name

	// Validate type compatibility
	if leftCol.Type != rightCol.Type {
		return fmt.Errorf("cannot join incompatible types: %s.%s (%s) with %s.%s (%s)",
			leftTable.Name, *leftColumn, leftCol.Type,
			rightTable.Name, *rightColumn, rightCol.Type,
		)
	}

	// Warn if joining on non-indexed columns
	if _, leftIndexed := leftTable.Indexes[leftCol.Name]; !leftIndexed {
		slog.Warn("Joining on non-indexed column (consider adding index)",
			slog.String("table", leftTable.Name),
			slog.String("column", leftCol.Name),
		)
	}
	if _, rightIndexed := rightTable.Indexes[rightCol.Name]; !rightIndexed {
		slog.Warn("Joining on non-indexed column (consider adding index)",
			slog.String("table", rightTable.Name),
			slog.String("column", rightCol.Name),
		)
	}

	return nil
}

// executeJoinWithDisambiguation is a helper used by innerJoin to find the right column name
func resolveJoinColumn(table *schema.Table, colName string) string {
	for i := range table.Schema.Columns {
		if table.Schema.Columns[i].Name == colName {
			return colName
		}
		if strings.HasSuffix(table.Schema.Columns[i].Name, "."+colName) {
			return table.Schema.Columns[i].Name
		}
	}
	return colName
}

// buildJoinIndex creates a hash index for the join column
// Returns the index and a boolean indicating if an existing index was reused
func buildJoinIndex(table *schema.Table, columnName string) (map[interface{}][]int, bool) {
	// Try to reuse existing index
	if idx, exists := table.Indexes[columnName]; exists {
		return idx.Data, true
	}

	// Build temporary index
	hashIndex := make(map[interface{}][]int)
	for i, row := range table.Rows {
		value, exists := row.Data[columnName]
		if !exists {
			continue // Skip NULL values
		}
		hashIndex[value] = append(hashIndex[value], i)
	}

	return hashIndex, false
}

// combineRows merges two rows with table-qualified column names
func combineRows(
	leftRow data.Row,
	rightRow data.Row,
	leftTableName string,
	rightTableName string,
) data.JoinedRow {
	joined := data.NewJoinedRow()

	// Add left table columns
	for colName, value := range leftRow.Data {
		qualifiedName := colName
		if !strings.Contains(colName, ".") {
			qualifiedName = fmt.Sprintf("%s.%s", leftTableName, colName)
		}
		joined.Set(qualifiedName, value)
	}

	// Add right table columns
	for colName, value := range rightRow.Data {
		qualifiedName := colName
		if !strings.Contains(colName, ".") {
			qualifiedName = fmt.Sprintf("%s.%s", rightTableName, colName)
		}
		joined.Set(qualifiedName, value)
	}

	return joined
}
