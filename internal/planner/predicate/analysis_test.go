package predicate

import (
	"testing"

	"github.com/leengari/joydb/internal/parser/ast"
)

func TestAnalyzeForIndexScan(t *testing.T) {
	tests := []struct {
		name     string
		expr     ast.Expression
		expected *PredicateInfo
	}{
		{
			name: "Simple equality",
			expr: &ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "id", Table: "users"},
				Operator: "=",
				Right:    &ast.Literal{Value: int64(10), TokenLiteralValue: "10", Kind: ast.LiteralInt},
			},
			expected: &PredicateInfo{
				Column:   "id",
				Table:    "users",
				Operator: "=",
				Value:    int64(10),
			},
		},
		{
			name: "Simple greater than",
			expr: &ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "price"},
				Operator: ">",
				Right:    &ast.Literal{Value: float64(19.99), TokenLiteralValue: "19.99", Kind: ast.LiteralFloat},
			},
			expected: &PredicateInfo{
				Column:   "price",
				Table:    "",
				Operator: ">",
				Value:    float64(19.99),
			},
		},
		{
			name: "Unsupported operator (!=)",
			expr: &ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "id"},
				Operator: "!=",
				Right:    &ast.Literal{Value: int64(5)},
			},
			expected: nil,
		},
		{
			name: "Non-binary expression",
			expr: &ast.Identifier{
				Value: "name",
			},
			expected: nil,
		},
		{
			name: "Complex binary left side (not identifier)",
			expr: &ast.BinaryExpression{
				Left: &ast.BinaryExpression{
					Left:     &ast.Identifier{Value: "a"},
					Operator: "+",
					Right:    &ast.Identifier{Value: "b"},
				},
				Operator: "=",
				Right:    &ast.Literal{Value: int64(10)},
			},
			expected: nil,
		},
		{
			name: "Binary right side not literal",
			expr: &ast.BinaryExpression{
				Left:     &ast.Identifier{Value: "a"},
				Operator: "=",
				Right:    &ast.Identifier{Value: "b"},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzeForIndexScan(tt.expr)
			if tt.expected == nil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %+v, got nil", tt.expected)
			}
			if got.Column != tt.expected.Column ||
				got.Table != tt.expected.Table ||
				got.Operator != tt.expected.Operator ||
				got.Value != tt.expected.Value {
				t.Fatalf("got %+v, want %+v", got, tt.expected)
			}
		})
	}
}
