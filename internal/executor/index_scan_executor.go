package executor

import (
	"fmt"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/index/btree"
	"github.com/leengari/mini-rdbms/internal/plan"
)

func executeIndexScan(node *plan.IndexScanNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	table, ok := ctx.Database.Tables[node.TableName]
	if !ok {
		return nil, newTableNotFoundError(node.TableName)
	}

	table.RLock()
	defer table.RUnlock()

	pkCol := table.Schema.GetPrimaryKeyColumn()
	if pkCol == nil || pkCol.Name != node.ColumnName || table.PKIndex == nil {
		return nil, fmt.Errorf("no B+Tree index found on column %s of table %s", node.ColumnName, node.TableName)
	}

	var positions []int

	switch node.Operator {
	case "=":
		pos, found := table.PKIndex.Search(node.Bound)
		if found {
			positions = append(positions, pos)
		}
	case ">":
		allPos := table.PKIndex.RangeFrom(node.Bound)
		for _, pos := range allPos {
			if pos >= 0 && pos < len(table.Rows) {
				row := table.Rows[pos]
				if btree.ToKey(row.Data[pkCol.Name]).Compare(btree.ToKey(node.Bound)) > 0 {
					positions = append(positions, pos)
				}
			}
		}
	case ">=":
		positions = table.PKIndex.RangeFrom(node.Bound)
	case "<":
		allPos := table.PKIndex.RangeTo(node.Bound)
		for _, pos := range allPos {
			if pos >= 0 && pos < len(table.Rows) {
				row := table.Rows[pos]
				if btree.ToKey(row.Data[pkCol.Name]).Compare(btree.ToKey(node.Bound)) < 0 {
					positions = append(positions, pos)
				}
			}
		}
	case "<=":
		positions = table.PKIndex.RangeTo(node.Bound)
	default:
		return nil, fmt.Errorf("unsupported operator for index scan: %s", node.Operator)
	}

	var rows []data.Row
	for _, pos := range positions {
		if pos >= 0 && pos < len(table.Rows) {
			rows = append(rows, table.Rows[pos])
		}
	}

	return &IntermediateResult{
		Rows:   rows,
		Schema: table.Schema,
		Metadata: map[string]interface{}{
			"table":      node.TableName,
			"scan_type":  "index",
			"column":     node.ColumnName,
			"operator":   node.Operator,
			"bound":      node.Bound,
			"row_count":  len(rows),
		},
	}, nil
}
