package join

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
)

// JoinType represents the type of JOIN operation
type JoinType int

const (
	JoinTypeInner JoinType = iota // Returns only matching rows from both tables
	JoinTypeLeft                  // Returns all rows from left table, NULLs for unmatched right rows
	JoinTypeRight                 // Returns all rows from right table, NULLs for unmatched left rows
	JoinTypeFull                  // Returns all rows from both tables, NULLs where no match
)

// String returns the string representation of the JOIN type
func (jt JoinType) String() string {
	switch jt {
	case JoinTypeInner:
		return "INNER JOIN"
	case JoinTypeLeft:
		return "LEFT JOIN"
	case JoinTypeRight:
		return "RIGHT JOIN"
	case JoinTypeFull:
		return "FULL OUTER JOIN"
	default:
		return "UNKNOWN JOIN"
	}
}

// combineRowsWithNull combines two rows with table-qualified column names
// If leftRow is nil, all left columns are set to NULL
// If rightRow is nil, all right columns are set to NULL
func combineRowsWithNull(
	leftRow data.Row,
	rightRow data.Row,
	leftTable *schema.Table,
	rightTable *schema.Table,
) data.JoinedRow {
	joined := data.NewJoinedRow()

	// Add left table columns (or NULLs if leftRow is empty)
	if len(leftRow.Data) > 0 {
		for colName, value := range leftRow.Data {
			qualifiedName := fmt.Sprintf("%s.%s", leftTable.Name, colName)
			joined.Set(qualifiedName, value)
		}
	} else {
		// Add NULL for all left columns
		for _, col := range leftTable.Schema.Columns {
			qualifiedName := fmt.Sprintf("%s.%s", leftTable.Name, col.Name)
			joined.Set(qualifiedName, nil)
		}
	}

	// Add right table columns (or NULLs if rightRow is empty)
	if len(rightRow.Data) > 0 {
		for colName, value := range rightRow.Data {
			qualifiedName := fmt.Sprintf("%s.%s", rightTable.Name, colName)
			joined.Set(qualifiedName, value)
		}
	} else {
		// Add NULL for all right columns
		for _, col := range rightTable.Schema.Columns {
			qualifiedName := fmt.Sprintf("%s.%s", rightTable.Name, col.Name)
			joined.Set(qualifiedName, nil)
		}
	}

	return joined
}
