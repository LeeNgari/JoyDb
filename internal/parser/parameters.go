package parser

import (
	"fmt"
	"strconv"

	"github.com/leengari/joydb/internal/parser/ast"
)

// BindParameters replaces positional parameter nodes with typed literals.
func BindParameters(statement ast.Statement, args []interface{}) error {
	used := make(map[int]bool)
	bind := func(expression ast.Expression) (ast.Expression, error) {
		return bindExpression(expression, args, used)
	}

	switch typed := statement.(type) {
	case *ast.SelectStatement:
		var err error
		typed.Where, err = bind(typed.Where)
		if err != nil {
			return err
		}
		for _, join := range typed.Joins {
			join.OnCondition, err = bind(join.OnCondition)
			if err != nil {
				return err
			}
		}
	case *ast.InsertStatement:
		for i, value := range typed.Values {
			bound, err := bind(value)
			if err != nil {
				return err
			}
			typed.Values[i] = bound
		}
	case *ast.UpdateStatement:
		for column, value := range typed.Updates {
			bound, err := bind(value)
			if err != nil {
				return err
			}
			typed.Updates[column] = bound
		}
		var err error
		typed.Where, err = bind(typed.Where)
		if err != nil {
			return err
		}
	case *ast.DeleteStatement:
		var err error
		typed.Where, err = bind(typed.Where)
		if err != nil {
			return err
		}
	}

	if len(used) != len(args) {
		return fmt.Errorf("parameter count mismatch: query uses %d, received %d", len(used), len(args))
	}
	return nil
}

func bindExpression(expression ast.Expression, args []interface{}, used map[int]bool) (ast.Expression, error) {
	if expression == nil {
		return nil, nil
	}
	switch typed := expression.(type) {
	case *ast.ParameterExpression:
		if typed.Index >= len(args) {
			return nil, fmt.Errorf("missing value for parameter %d", typed.Index+1)
		}
		used[typed.Index] = true
		return literalForParameter(args[typed.Index])
	case *ast.BinaryExpression:
		left, err := bindExpression(typed.Left, args, used)
		if err != nil {
			return nil, err
		}
		right, err := bindExpression(typed.Right, args, used)
		if err != nil {
			return nil, err
		}
		typed.Left, typed.Right = left, right
	case *ast.LogicalExpression:
		left, err := bindExpression(typed.Left, args, used)
		if err != nil {
			return nil, err
		}
		right, err := bindExpression(typed.Right, args, used)
		if err != nil {
			return nil, err
		}
		typed.Left, typed.Right = left, right
	}
	return expression, nil
}

func literalForParameter(value interface{}) (*ast.Literal, error) {
	switch typed := value.(type) {
	case nil:
		return &ast.Literal{TokenLiteralValue: "NULL", Kind: ast.LiteralNull}, nil
	case string:
		return &ast.Literal{TokenLiteralValue: typed, Value: typed, Kind: ast.LiteralString}, nil
	case bool:
		return &ast.Literal{TokenLiteralValue: strconv.FormatBool(typed), Value: typed, Kind: ast.LiteralBool}, nil
	case int:
		return &ast.Literal{TokenLiteralValue: strconv.Itoa(typed), Value: typed, Kind: ast.LiteralInt}, nil
	case int8:
		value := int(typed)
		return &ast.Literal{TokenLiteralValue: strconv.Itoa(value), Value: value, Kind: ast.LiteralInt}, nil
	case int16:
		value := int(typed)
		return &ast.Literal{TokenLiteralValue: strconv.Itoa(value), Value: value, Kind: ast.LiteralInt}, nil
	case int32:
		value := int(typed)
		return &ast.Literal{TokenLiteralValue: strconv.Itoa(value), Value: value, Kind: ast.LiteralInt}, nil
	case int64:
		return &ast.Literal{TokenLiteralValue: strconv.FormatInt(typed, 10), Value: typed, Kind: ast.LiteralInt}, nil
	case float32:
		value := float64(typed)
		return &ast.Literal{TokenLiteralValue: strconv.FormatFloat(value, 'g', -1, 64), Value: value, Kind: ast.LiteralFloat}, nil
	case float64:
		return &ast.Literal{TokenLiteralValue: strconv.FormatFloat(typed, 'g', -1, 64), Value: typed, Kind: ast.LiteralFloat}, nil
	default:
		return nil, fmt.Errorf("unsupported parameter type %T", value)
	}
}
