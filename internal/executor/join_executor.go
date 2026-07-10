package executor

import (
	"fmt"
	"strings"

	"github.com/leengari/joydb/internal/domain/data"
	"github.com/leengari/joydb/internal/domain/schema"
	"github.com/leengari/joydb/internal/plan"
	"github.com/leengari/joydb/internal/query/operations/join"
)

// executeJoinNode recursively executes JOIN using tree-walking pattern
// This enables multi-way JOINs by recursively executing left/right children
func executeJoinNode(node *plan.JoinNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	// Recursively execute left child
	leftResult, err := executeNode(node.Left(), ctx)
	if err != nil {
		return nil, fmt.Errorf("left child execution failed: %w", err)
	}

	// Recursively execute right child
	rightResult, err := executeNode(node.Right(), ctx)
	if err != nil {
		return nil, fmt.Errorf("right child execution failed: %w", err)
	}

	// Get table names from metadata (for qualified column names)
	leftTableName := extractTableName(node.Left())
	rightTableName := extractTableName(node.Right())

	// Build the schema for the joined result (qualified names)
	joinedSchema := &schema.TableSchema{
		Columns: make([]schema.Column, 0, len(leftResult.Schema.Columns)+len(rightResult.Schema.Columns)),
	}

	leftColIdx := -1
	rightColIdx := -1

	// Add left columns and find join column index
	for i, col := range leftResult.Schema.Columns {
		name := col.Name
		if !strings.Contains(col.Name, ".") {
			name = fmt.Sprintf("%s.%s", leftTableName, col.Name)
		}
		joinedSchema.Columns = append(joinedSchema.Columns, schema.Column{
			Name: name,
			Type: col.Type,
		})

		if col.Name == node.LeftOnCol || strings.HasSuffix(col.Name, "."+node.LeftOnCol) {
			leftColIdx = i
		}
	}

	// Add right columns and find join column index
	for i, col := range rightResult.Schema.Columns {
		name := col.Name
		if !strings.Contains(col.Name, ".") {
			name = fmt.Sprintf("%s.%s", rightTableName, col.Name)
		}
		joinedSchema.Columns = append(joinedSchema.Columns, schema.Column{
			Name: name,
			Type: col.Type,
		})

		if col.Name == node.RightOnCol || strings.HasSuffix(col.Name, "."+node.RightOnCol) {
			rightColIdx = i
		}
	}

	if leftColIdx == -1 {
		return nil, fmt.Errorf("column '%s' not found in left side", node.LeftOnCol)
	}
	if rightColIdx == -1 {
		return nil, fmt.Errorf("column '%s' not found in right side", node.RightOnCol)
	}

	var rows []data.Row

	// Dual-Strategy execution:
	// If AST ever adds inequality operators (e.g. node.Op == ">"), we will branch to Nested Loop here.
	// Currently, JoinNode only supports equijoin, so we unconditionally use Hash Join.

	// PHASE 1: Build Hash Map on the right side
	// Key: interface{}, Value: slice of row indices in rightResult.Rows
	hashTable := make(map[interface{}][]int)
	for i, row := range rightResult.Rows {
		if rightColIdx < len(row.Values) && row.Values[rightColIdx] != nil {
			key := row.Values[rightColIdx]
			hashTable[key] = append(hashTable[key], i)
		}
	}

	// PHASE 2: Probe with left side
	rightMatched := make(map[int]bool)

	for _, leftRow := range leftResult.Rows {
		var leftKey interface{}
		if leftColIdx < len(leftRow.Values) {
			leftKey = leftRow.Values[leftColIdx]
		}

		if leftKey == nil {
			if node.JoinType == join.JoinTypeLeft || node.JoinType == join.JoinTypeFull {
				// Emit left row with null right values
				newVals := make([]interface{}, len(joinedSchema.Columns))
				copy(newVals[:len(leftRow.Values)], leftRow.Values)
				rows = append(rows, data.NewRow(newVals))
			}
			continue
		}

		rightIndices, found := hashTable[leftKey]
		if found {
			for _, rIdx := range rightIndices {
				if node.JoinType == join.JoinTypeRight || node.JoinType == join.JoinTypeFull {
					rightMatched[rIdx] = true
				}
				rightRow := rightResult.Rows[rIdx]

				// Combine row values
				newVals := make([]interface{}, len(joinedSchema.Columns))
				copy(newVals[:len(leftRow.Values)], leftRow.Values)
				for j, rightVal := range rightRow.Values {
					newVals[len(leftRow.Values)+j] = rightVal
				}
				rows = append(rows, data.NewRow(newVals))
			}
		} else if node.JoinType == join.JoinTypeLeft || node.JoinType == join.JoinTypeFull {
			// No match, emit left row with null right values
			newVals := make([]interface{}, len(joinedSchema.Columns))
			copy(newVals[:len(leftRow.Values)], leftRow.Values)
			rows = append(rows, data.NewRow(newVals))
		}
	}

	// PHASE 3: Emit unmatched right side rows (for RIGHT and FULL joins)
	if node.JoinType == join.JoinTypeRight || node.JoinType == join.JoinTypeFull {
		for i, rightRow := range rightResult.Rows {
			if !rightMatched[i] {
				newVals := make([]interface{}, len(joinedSchema.Columns))
				for j, rightVal := range rightRow.Values {
					newVals[len(leftResult.Schema.Columns)+j] = rightVal
				}
				rows = append(rows, data.NewRow(newVals))
			}
		}
	}

	return &IntermediateResult{
		Rows:   rows,
		Schema: joinedSchema,
		Metadata: map[string]interface{}{
			"join_type":   node.JoinType,
			"left_rows":   len(leftResult.Rows),
			"right_rows":  len(rightResult.Rows),
			"result_rows": len(rows),
		},
	}, nil
}

// extractTableName extracts table name from a plan node
func extractTableName(node plan.Node) string {
	switch n := node.(type) {
	case *plan.ScanNode:
		return n.TableName
	case *plan.SelectNode:
		return n.TableName
	case *plan.JoinNode:
		return fmt.Sprintf("join_%p", n)
	default:
		return "temp_table"
	}
}
