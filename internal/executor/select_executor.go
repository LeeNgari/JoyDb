package executor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/plan"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
)

// executeSelectNode handles SelectNode using tree-walking pattern
// Returns IntermediateResult for composition with other nodes
func executeSelectNode(node *plan.SelectNode, ctx *ExecutionContext) (*IntermediateResult, error) {
	var rows []data.Row

	var resultSchema *schema.TableSchema
	if len(node.Children()) > 0 {
		// Execute child (JOIN tree or other operation) recursively
		childResult, err := executeNode(node.Children()[0], ctx)
		if err != nil {
			return nil, err
		}
		rows = childResult.Rows
		resultSchema = childResult.Schema
	} else {
		// No children - simple table scan
		scanNode := &plan.ScanNode{
			TableName:   node.TableName,
			Predicate:   node.Predicate,
			Transaction: node.Transaction,
		}
		scanResult, err := executeScan(scanNode, ctx)
		if err != nil {
			return nil, err
		}
		rows = scanResult.Rows
		resultSchema = scanResult.Schema
	}

	// Apply predicate
	if node.Predicate != nil {
		filteredRows := make([]data.Row, 0)
		for _, row := range rows {
			if node.Predicate(row) {
				filteredRows = append(filteredRows, row)
			}
		}
		rows = filteredRows
	}

	// Apply ORDER BY BEFORE projection so we have access to all columns
	if len(node.OrderBy) > 0 {
		sort.SliceStable(rows, func(i, j int) bool {
			rowI := rows[i]
			rowJ := rows[j]

			// We need to compare based on the OrderBy clauses
			for _, ob := range node.OrderBy {
				// Find column index in the resultSchema
				colIdx := -1
				for idx, col := range resultSchema.Columns {
					if col.Name == ob.Column || col.Name == "*."+ob.Column || strings.HasSuffix(col.Name, "."+ob.Column) {
						colIdx = idx
						break
					}
				}
				if colIdx == -1 {
					// Fallback: assume sort order is equal if column not found
					continue
				}

				valI := rowI.Values[colIdx]
				valJ := rowJ.Values[colIdx]

				// Null handling
				if valI == nil && valJ != nil {
					return !ob.Desc
				}
				if valI != nil && valJ == nil {
					return ob.Desc
				}
				if valI == nil && valJ == nil {
					continue
				}

				// Compare values based on type
				cmp := compareValues(valI, valJ)
				if cmp == 0 {
					continue // Go to next ORDER BY clause
				}

				if ob.Desc {
					return cmp > 0
				}
				return cmp < 0
			}
			return false // All order by columns equal
		})
	}

	// Apply Aggregations
	if len(node.Aggregates) > 0 {
		var syntheticValues []interface{}
		var syntheticColumns []schema.Column
		
		for _, agg := range node.Aggregates {
			colIdx := -1
			if agg.Column != "*" {
				for idx, col := range resultSchema.Columns {
					if col.Name == agg.Column || col.Name == "*."+agg.Column || strings.HasSuffix(col.Name, "."+agg.Column) {
						colIdx = idx
						break
					}
				}
				if colIdx == -1 {
					return nil, fmt.Errorf("column not found for aggregate: %s", agg.Column)
				}
			}

			var val interface{}
			
			switch agg.Function {
			case "COUNT":
				count := 0
				for _, r := range rows {
					if colIdx == -1 {
						count++ // COUNT(*)
					} else if r.Values[colIdx] != nil {
						count++
					}
				}
				val = int64(count)
			case "SUM":
				var sum float64
				for _, r := range rows {
					if colIdx != -1 && r.Values[colIdx] != nil {
						sum += toFloat64(r.Values[colIdx])
					}
				}
				val = sum
			case "AVG":
				var sum float64
				var count int
				for _, r := range rows {
					if colIdx != -1 && r.Values[colIdx] != nil {
						sum += toFloat64(r.Values[colIdx])
						count++
					}
				}
				if count > 0 {
					val = sum / float64(count)
				} else {
					val = nil
				}
			case "MIN":
				var min interface{}
				for _, r := range rows {
					if colIdx != -1 && r.Values[colIdx] != nil {
						if min == nil || compareValues(r.Values[colIdx], min) < 0 {
							min = r.Values[colIdx]
						}
					}
				}
				val = min
			case "MAX":
				var max interface{}
				for _, r := range rows {
					if colIdx != -1 && r.Values[colIdx] != nil {
						if max == nil || compareValues(r.Values[colIdx], max) > 0 {
							max = r.Values[colIdx]
						}
					}
				}
				val = max
			}
			
			syntheticValues = append(syntheticValues, val)
			
			colType := schema.ColumnTypeFloat
			if agg.Function == "COUNT" {
				colType = schema.ColumnTypeInt
			}
			syntheticColumns = append(syntheticColumns, schema.Column{
				Name: agg.Alias,
				Type: colType,
			})
		}
		
		syntheticRow := data.Row{
			Values: syntheticValues,
		}
		
		return &IntermediateResult{
			Rows: []data.Row{syntheticRow},
			Schema: &schema.TableSchema{
				TableName: "synthetic",
				Columns:   syntheticColumns,
			},
			Metadata: map[string]interface{}{
				"row_count": 1,
				"is_aggregate": true,
			},
		}, nil
	}

	// Apply projection
	projectedRows := make([]data.Row, len(rows))
	for i, r := range rows {
		projectedRows[i] = projection.ProjectRow(r, node.Projection, resultSchema)
	}

	// Apply Offset and Limit
	finalRows := projectedRows
	if node.Offset != nil {
		offset := *node.Offset
		if offset > len(finalRows) {
			finalRows = nil
		} else {
			finalRows = finalRows[offset:]
		}
	}
	if node.Limit != nil {
		limit := *node.Limit
		if limit < len(finalRows) {
			finalRows = finalRows[:limit]
		}
	}

	return &IntermediateResult{
		Rows:   finalRows,
		Schema: resultSchema,
		Metadata: map[string]interface{}{
			"projection": node.Projection,
			"row_count":  len(projectedRows),
			"filtered":   node.Predicate != nil,
		},
	}, nil
}

// compareValues returns -1 if a < b, 0 if a == b, 1 if a > b
func compareValues(a, b interface{}) int {
	switch aVal := a.(type) {
	case int:
		// Attempt to handle both int and int64 comparison
		var bValInt int
		switch bVal := b.(type) {
		case int:
			bValInt = bVal
		case int64:
			bValInt = int(bVal)
		default:
			return 0
		}
		if aVal < bValInt {
			return -1
		} else if aVal > bValInt {
			return 1
		}
		return 0
	case int64:
		var bValInt int64
		switch bVal := b.(type) {
		case int:
			bValInt = int64(bVal)
		case int64:
			bValInt = bVal
		default:
			return 0
		}
		if aVal < bValInt {
			return -1
		} else if aVal > bValInt {
			return 1
		}
		return 0
	case float64:
		bVal, ok := b.(float64)
		if !ok {
			return 0
		}
		if aVal < bVal {
			return -1
		} else if aVal > bVal {
			return 1
		}
		return 0
	case string:
		bVal, ok := b.(string)
		if !ok {
			return 0
		}
		if aVal < bVal {
			return -1
		} else if aVal > bVal {
			return 1
		}
		return 0
	case bool:
		bVal, ok := b.(bool)
		if !ok {
			return 0
		}
		if !aVal && bVal {
			return -1 // false < true
		} else if aVal && !bVal {
			return 1
		}
		return 0
	default:
		// Cannot compare unsupported types
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float64:
		return val
	default:
		return 0
	}
}
