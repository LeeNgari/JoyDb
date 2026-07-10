package projection

// ColumnRef represents a column reference in a SELECT statement
// Can be simple (column name) or qualified (table.column)
type ColumnRef struct {
	Table  string // Optional table qualifier (e.g., "users")
	Column string // Column name (e.g., "id")
	Alias  string // Optional alias (e.g., "user_id" for "id AS user_id")
}

// Projection represents which columns to select from a query
// If SelectAll is true, all columns are returned
// Otherwise, only columns in Columns slice are returned
type Projection struct {
	Columns   []ColumnRef // Specific columns to select
	SelectAll bool        // true for SELECT *
}

// NewProjection creates a new projection for selecting all columns
func NewProjection() *Projection {
	return &Projection{
		SelectAll: true,
		Columns:   []ColumnRef{},
	}
}

// NewProjectionWithColumns creates a projection for specific columns
func NewProjectionWithColumns(columns ...ColumnRef) *Projection {
	return &Projection{
		SelectAll: false,
		Columns:   columns,
	}
}

// AddColumn adds a column to the projection
func (p *Projection) AddColumn(table, column, alias string) {
	p.Columns = append(p.Columns, ColumnRef{
		Table:  table,
		Column: column,
		Alias:  alias,
	})
	p.SelectAll = false
}
