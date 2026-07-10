package executor

import (
	"fmt"

	"github.com/leengari/joydb/internal/domain/data"
	"github.com/leengari/joydb/internal/index/btree"
	"github.com/leengari/joydb/internal/plan"
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

	var rids []int64

	switch node.Operator {
	case "=":
		rid, found := table.PKIndex.Search(node.Bound)
		if found {
			rids = append(rids, rid)
		}
	case ">":
		pkIdx := table.Schema.GetColumnIndex(pkCol.Name)
		allRIDs := table.PKIndex.RangeFrom(node.Bound)
		for _, rid := range allRIDs {
			if row, found := table.RowsByRID[rid]; found && !row.Deleted && pkIdx != -1 && pkIdx < len(row.Values) {
				if btree.ToKey(row.Values[pkIdx]).Compare(btree.ToKey(node.Bound)) > 0 {
					rids = append(rids, rid)
				}
			}
		}
	case ">=":
		rids = table.PKIndex.RangeFrom(node.Bound)
	case "<":
		pkIdx := table.Schema.GetColumnIndex(pkCol.Name)
		allRIDs := table.PKIndex.RangeTo(node.Bound)
		for _, rid := range allRIDs {
			if row, found := table.RowsByRID[rid]; found && !row.Deleted && pkIdx != -1 && pkIdx < len(row.Values) {
				if btree.ToKey(row.Values[pkIdx]).Compare(btree.ToKey(node.Bound)) < 0 {
					rids = append(rids, rid)
				}
			}
		}
	case "<=":
		rids = table.PKIndex.RangeTo(node.Bound)
	default:
		return nil, fmt.Errorf("unsupported operator for index scan: %s", node.Operator)
	}

	var rows []data.Row
	for _, rid := range rids {
		if row, found := table.RowsByRID[rid]; found && !row.Deleted {
			rows = append(rows, row.Copy())
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
