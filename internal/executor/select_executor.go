package executor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/leengari/joydb/internal/domain/data"
	"github.com/leengari/joydb/internal/domain/schema"
	"github.com/leengari/joydb/internal/plan"
	"github.com/leengari/joydb/internal/query/operations/projection"
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

	if len(node.GroupBy) > 0 || len(node.Aggregates) > 0 {
		return executeAggregateSelect(node, rows, resultSchema)
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

type aggregateGroup struct {
	rows []data.Row
}

func executeAggregateSelect(node *plan.SelectNode, rows []data.Row, inputSchema *schema.TableSchema) (*IntermediateResult, error) {
	groups := make([]aggregateGroup, 0)
	if len(node.GroupBy) == 0 {
		groups = append(groups, aggregateGroup{rows: rows})
	} else {
		groupIndexes := make([]int, len(node.GroupBy))
		for i, column := range node.GroupBy {
			index, err := resolveResultColumn(inputSchema, column.Table, column.Column)
			if err != nil {
				return nil, fmt.Errorf("GROUP BY: %w", err)
			}
			groupIndexes[i] = index
		}

		groupLookup := make(map[string]int)
		for _, row := range rows {
			values := make([]interface{}, len(groupIndexes))
			for i, index := range groupIndexes {
				values[i] = row.Values[index]
			}
			key, err := encodeGroupKey(values)
			if err != nil {
				return nil, err
			}
			groupIndex, exists := groupLookup[key]
			if !exists {
				groupIndex = len(groups)
				groupLookup[key] = groupIndex
				groups = append(groups, aggregateGroup{})
			}
			groups[groupIndex].rows = append(groups[groupIndex].rows, row)
		}
	}

	outputSchema, err := aggregateOutputSchema(node, inputSchema)
	if err != nil {
		return nil, err
	}
	outputRows := make([]data.Row, 0, len(groups))
	for _, group := range groups {
		values := make([]interface{}, 0, len(node.SelectItems))
		for _, item := range node.SelectItems {
			switch {
			case item.Column != nil:
				index, err := resolveResultColumn(inputSchema, item.Column.Table, item.Column.Column)
				if err != nil {
					return nil, err
				}
				values = append(values, group.rows[0].Values[index])
			case item.Aggregate != nil:
				value, err := computeAggregate(*item.Aggregate, group.rows, inputSchema)
				if err != nil {
					return nil, err
				}
				values = append(values, value)
			}
		}
		outputRows = append(outputRows, data.NewRow(values))
	}

	if err := sortResultRows(outputRows, outputSchema, node.OrderBy); err != nil {
		return nil, err
	}
	outputRows = applyOffsetLimit(outputRows, node.Offset, node.Limit)

	return &IntermediateResult{
		Rows:   outputRows,
		Schema: outputSchema,
		Metadata: map[string]interface{}{
			"row_count":    len(outputRows),
			"is_aggregate": len(node.Aggregates) > 0,
			"is_grouped":   len(node.GroupBy) > 0,
		},
	}, nil
}

func aggregateOutputSchema(node *plan.SelectNode, inputSchema *schema.TableSchema) (*schema.TableSchema, error) {
	columns := make([]schema.Column, 0, len(node.SelectItems))
	for _, item := range node.SelectItems {
		switch {
		case item.Column != nil:
			index, err := resolveResultColumn(inputSchema, item.Column.Table, item.Column.Column)
			if err != nil {
				return nil, err
			}
			name := item.Column.Column
			if item.Column.Alias != "" {
				name = item.Column.Alias
			}
			columns = append(columns, schema.Column{Name: name, Type: inputSchema.Columns[index].Type})
		case item.Aggregate != nil:
			columnType := schema.ColumnTypeFloat
			if item.Aggregate.Function == "COUNT" {
				columnType = schema.ColumnTypeInt
			} else if item.Aggregate.Function == "MIN" || item.Aggregate.Function == "MAX" {
				index, err := resolveResultColumn(inputSchema, item.Aggregate.Table, item.Aggregate.Column)
				if err != nil {
					return nil, err
				}
				columnType = inputSchema.Columns[index].Type
			}
			columns = append(columns, schema.Column{Name: item.Aggregate.Alias, Type: columnType})
		}
	}
	return &schema.TableSchema{TableName: "aggregate", Columns: columns}, nil
}

func computeAggregate(spec plan.AggregateSpec, rows []data.Row, inputSchema *schema.TableSchema) (interface{}, error) {
	columnIndex := -1
	if spec.Column != "*" {
		var err error
		columnIndex, err = resolveResultColumn(inputSchema, spec.Table, spec.Column)
		if err != nil {
			return nil, fmt.Errorf("aggregate %s: %w", spec.Function, err)
		}
	}

	switch spec.Function {
	case "COUNT":
		var count int64
		for _, row := range rows {
			if columnIndex < 0 || row.Values[columnIndex] != nil {
				count++
			}
		}
		return count, nil
	case "SUM":
		var sum float64
		for _, row := range rows {
			if columnIndex >= 0 && row.Values[columnIndex] != nil {
				sum += toFloat64(row.Values[columnIndex])
			}
		}
		return sum, nil
	case "AVG":
		var sum float64
		var count int
		for _, row := range rows {
			if columnIndex >= 0 && row.Values[columnIndex] != nil {
				sum += toFloat64(row.Values[columnIndex])
				count++
			}
		}
		if count == 0 {
			return nil, nil
		}
		return sum / float64(count), nil
	case "MIN", "MAX":
		var selected interface{}
		for _, row := range rows {
			if columnIndex < 0 || row.Values[columnIndex] == nil {
				continue
			}
			if selected == nil || spec.Function == "MIN" && compareValues(row.Values[columnIndex], selected) < 0 || spec.Function == "MAX" && compareValues(row.Values[columnIndex], selected) > 0 {
				selected = row.Values[columnIndex]
			}
		}
		return selected, nil
	default:
		return nil, fmt.Errorf("unsupported aggregate function: %s", spec.Function)
	}
}

func resolveResultColumn(resultSchema *schema.TableSchema, table, column string) (int, error) {
	qualified := column
	if table != "" {
		qualified = table + "." + column
	}
	match := -1
	for index, candidate := range resultSchema.Columns {
		if candidate.Name == qualified || table != "" && candidate.Name == column || table == "" && (candidate.Name == column || strings.HasSuffix(candidate.Name, "."+column)) {
			if match >= 0 && match != index {
				return -1, fmt.Errorf("column is ambiguous: %s", column)
			}
			match = index
		}
	}
	if match < 0 {
		return -1, fmt.Errorf("column not found: %s", qualified)
	}
	return match, nil
}

func encodeGroupKey(values []interface{}) (string, error) {
	var buffer bytes.Buffer
	for _, value := range values {
		switch typed := value.(type) {
		case nil:
			buffer.WriteByte(0)
		case int:
			buffer.WriteByte(1)
			_ = binary.Write(&buffer, binary.LittleEndian, int64(typed))
		case int64:
			buffer.WriteByte(1)
			_ = binary.Write(&buffer, binary.LittleEndian, typed)
		case float64:
			buffer.WriteByte(2)
			_ = binary.Write(&buffer, binary.LittleEndian, math.Float64bits(typed))
		case string:
			buffer.WriteByte(3)
			_ = binary.Write(&buffer, binary.LittleEndian, uint64(len(typed)))
			buffer.WriteString(typed)
		case bool:
			buffer.WriteByte(4)
			if typed {
				buffer.WriteByte(1)
			} else {
				buffer.WriteByte(0)
			}
		default:
			return "", fmt.Errorf("unsupported GROUP BY value type %T", value)
		}
	}
	return buffer.String(), nil
}

func sortResultRows(rows []data.Row, resultSchema *schema.TableSchema, orderBy []plan.OrderByClause) error {
	indexes := make([]int, len(orderBy))
	for i, clause := range orderBy {
		index, err := resolveResultColumn(resultSchema, "", clause.Column)
		if err != nil {
			return fmt.Errorf("ORDER BY: %w", err)
		}
		indexes[i] = index
	}
	sort.SliceStable(rows, func(i, j int) bool {
		for clauseIndex, clause := range orderBy {
			left := rows[i].Values[indexes[clauseIndex]]
			right := rows[j].Values[indexes[clauseIndex]]
			if left == nil || right == nil {
				if left == nil && right == nil {
					continue
				}
				return (left == nil) != clause.Desc
			}
			comparison := compareValues(left, right)
			if comparison == 0 {
				continue
			}
			if clause.Desc {
				return comparison > 0
			}
			return comparison < 0
		}
		return false
	})
	return nil
}

func applyOffsetLimit(rows []data.Row, offset, limit *int) []data.Row {
	if offset != nil {
		if *offset >= len(rows) {
			return nil
		}
		rows = rows[*offset:]
	}
	if limit != nil && *limit < len(rows) {
		rows = rows[:*limit]
	}
	return rows
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
