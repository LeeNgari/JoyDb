package planner

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/parser/ast"
	"github.com/leengari/mini-rdbms/internal/plan"
	"github.com/leengari/mini-rdbms/internal/planner/predicate"
	"github.com/leengari/mini-rdbms/internal/query/operations/join"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
	"github.com/leengari/mini-rdbms/internal/util/types"
)

// Plan converts an AST statement into an execution plan
func Plan(stmt ast.Statement, db *schema.Database, tx *transaction.Transaction) (plan.Node, error) {
	switch s := stmt.(type) {
	case *ast.SelectStatement:
		return planSelect(s, db, tx)
	case *ast.InsertStatement:
		return planInsert(s, db, tx)
	case *ast.UpdateStatement:
		return planUpdate(s, db, tx)
	case *ast.DeleteStatement:
		return planDelete(s, db, tx)
	default:
		return nil, fmt.Errorf("unsupported statement type: %T", stmt)
	}
}

func planSelect(stmt *ast.SelectStatement, db *schema.Database, tx *transaction.Transaction) (plan.Node, error) {
	// 1. Validate tables exist
	tableName := stmt.TableName.Value
	_, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	// 2. Build Predicate
	var pred func(data.Row) bool
	if stmt.Where != nil {
		p, err := predicate.Build(stmt.Where)
		if err != nil {
			return nil, err
		}
		pred = p
	}

	// 3. Build Joins
	var joinNodes []plan.JoinNode
	for _, joinClause := range stmt.Joins {
		// Validate join table
		joinTableName := joinClause.RightTable.Value
		_, ok := db.Tables[joinTableName]
		if !ok {
			return nil, fmt.Errorf("right table not found: %s", joinTableName)
		}

		// Parse ON condition
		binExpr, ok := joinClause.OnCondition.(*ast.BinaryExpression)
		if !ok {
			return nil, fmt.Errorf("JOIN ON condition must be a comparison expression")
		}
		if binExpr.Operator != "=" {
			return nil, fmt.Errorf("JOIN ON condition must use = operator")
		}

		leftIdent, ok := binExpr.Left.(*ast.Identifier)
		if !ok {
			return nil, fmt.Errorf("left side of JOIN condition must be an identifier")
		}
		rightIdent, ok := binExpr.Right.(*ast.Identifier)
		if !ok {
			return nil, fmt.Errorf("right side of JOIN condition must be an identifier")
		}

		// Convert string type to enum
		var jt join.JoinType
		switch joinClause.JoinType {
		case "INNER":
			jt = join.JoinTypeInner
		case "LEFT":
			jt = join.JoinTypeLeft
		case "RIGHT":
			jt = join.JoinTypeRight
		case "FULL":
			jt = join.JoinTypeFull
		default:
			return nil, fmt.Errorf("unsupported JOIN type: %s", joinClause.JoinType)
		}

		joinNodes = append(joinNodes, plan.JoinNode{
			TargetTable: joinTableName,
			JoinType:    jt,
			LeftOnCol:   leftIdent.Value,
			RightOnCol:  rightIdent.Value,
		})
	}

	// 4. Build Projection
	var proj *projection.Projection
	if len(stmt.Fields) == 1 && stmt.Fields[0].Value == "*" {
		proj = projection.NewProjection()
	} else {
		proj = &projection.Projection{
			SelectAll: false,
			Columns:   make([]projection.ColumnRef, len(stmt.Fields)),
		}
		for i, f := range stmt.Fields {
			proj.Columns[i] = projection.ColumnRef{
				Table:  f.Table,
				Column: f.Value,
			}
		}
	}

	return &plan.SelectNode{
		TableName:   tableName,
		Predicate:   pred,
		Projection:  proj,
		Joins:       joinNodes,
		Transaction: tx,
	}, nil
}

func planInsert(stmt *ast.InsertStatement, db *schema.Database, tx *transaction.Transaction) (plan.Node, error) {
	tableName := stmt.TableName.Value
	table, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	if len(stmt.Columns) != len(stmt.Values) {
		return nil, fmt.Errorf("column count (%d) does not match value count (%d)", len(stmt.Columns), len(stmt.Values))
	}

	row := make(data.Row)
	for i, col := range stmt.Columns {
		lit, ok := stmt.Values[i].(*ast.Literal)
		if !ok {
			return nil, fmt.Errorf("only literals supported in VALUES")
		}

		schemaCol := findColumnInSchema(table, col.Value)
		if schemaCol != nil {
			convertedLit, err := types.ConvertLiteralToSchemaType(lit, schemaCol.Type)
			if err != nil {
				return nil, fmt.Errorf("column '%s': %w", col.Value, err)
			}
			row[col.Value] = convertedLit.Value
		} else {
			row[col.Value] = lit.Value
		}
	}

	return &plan.InsertNode{
		TableName:   tableName,
		Row:         row,
		Transaction: tx,
	}, nil
}

func planUpdate(stmt *ast.UpdateStatement, db *schema.Database, tx *transaction.Transaction) (plan.Node, error) {
	tableName := stmt.TableName.Value
	table, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	updates := make(data.Row)
	for colName, valueExpr := range stmt.Updates {
		lit, ok := valueExpr.(*ast.Literal)
		if !ok {
			return nil, fmt.Errorf("only literals supported in SET clause")
		}

		schemaCol := findColumnInSchema(table, colName)
		if schemaCol != nil {
			convertedLit, err := types.ConvertLiteralToSchemaType(lit, schemaCol.Type)
			if err != nil {
				return nil, fmt.Errorf("column '%s': %w", colName, err)
			}
			updates[colName] = convertedLit.Value
		} else {
			updates[colName] = lit.Value
		}
	}

	var pred func(data.Row) bool
	if stmt.Where != nil {
		var err error
		pred, err = predicate.Build(stmt.Where)
		if err != nil {
			return nil, err
		}
	} else {
		pred = func(data.Row) bool { return true }
	}

	return &plan.UpdateNode{
		TableName:   tableName,
		Predicate:   pred,
		Updates:     updates,
		Transaction: tx,
	}, nil
}

func planDelete(stmt *ast.DeleteStatement, db *schema.Database, tx *transaction.Transaction) (plan.Node, error) {
	tableName := stmt.TableName.Value
	_, ok := db.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}

	var pred func(data.Row) bool
	if stmt.Where != nil {
		var err error
		pred, err = predicate.Build(stmt.Where)
		if err != nil {
			return nil, err
		}
	} else {
		pred = func(data.Row) bool { return true }
	}

	return &plan.DeleteNode{
		TableName:   tableName,
		Predicate:   pred,
		Transaction: tx,
	}, nil
}

func findColumnInSchema(table *schema.Table, colName string) *schema.Column {
	for i := range table.Schema.Columns {
		if table.Schema.Columns[i].Name == colName {
			return &table.Schema.Columns[i]
		}
	}
	return nil
}
