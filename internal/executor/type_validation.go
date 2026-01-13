package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
)

// findColumnInSchema finds a column by name in the table schema
func findColumnInSchema(table *schema.Table, colName string) *schema.Column {
	for i := range table.Schema.Columns {
		if table.Schema.Columns[i].Name == colName {
			return &table.Schema.Columns[i]
		}
	}
	return nil
}

// validateLiteralType checks if a literal's type matches the expected column type
func validateLiteralType(lit *ast.Literal, expectedType schema.ColumnType) error {
	switch expectedType {
	case schema.ColumnTypeInt:
		if lit.Kind != ast.LiteralInt {
			return fmt.Errorf("expected INT, got %s", lit.Kind)
		}
	case schema.ColumnTypeFloat:
		if lit.Kind != ast.LiteralInt && lit.Kind != ast.LiteralFloat {
			return fmt.Errorf("expected FLOAT or INT, got %s", lit.Kind)
		}
	case schema.ColumnTypeText:
		if lit.Kind != ast.LiteralString {
			return fmt.Errorf("expected TEXT, got %s", lit.Kind)
		}
	case schema.ColumnTypeBool:
		if lit.Kind != ast.LiteralBool {
			return fmt.Errorf("expected BOOL, got %s", lit.Kind)
		}
	case schema.ColumnTypeDate:
		if lit.Kind != ast.LiteralDate {
			return fmt.Errorf("expected DATE, got %s", lit.Kind)
		}
	case schema.ColumnTypeTime:
		if lit.Kind != ast.LiteralTime {
			return fmt.Errorf("expected TIME, got %s", lit.Kind)
		}
	case schema.ColumnTypeEmail:
		if lit.Kind != ast.LiteralEmail {
			return fmt.Errorf("expected EMAIL, got %s", lit.Kind)
		}
	}
	return nil
}
