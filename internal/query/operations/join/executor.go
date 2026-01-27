package join

import (
	"fmt"
	"log/slog"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
)

// JoinPredicate tests whether a joined row matches certain criteria
type JoinPredicate func(data.JoinedRow) bool

// ExecuteJoin performs a JOIN operation with the specified type
// This is the unified API for all JOIN types (INNER, LEFT, RIGHT, FULL)
// Supports optional predicate filtering and column projection
func ExecuteJoin(
	leftTable *schema.Table,
	rightTable *schema.Table,
	leftColumn string,
	rightColumn string,
	joinType JoinType,
	pred JoinPredicate,
	proj *projection.Projection,
	tx *transaction.Transaction,
) ([]data.JoinedRow, error) {
	if tx != nil {
		slog.Debug("ExecuteJoin operation", "type", joinType, "tx_id", tx.ID)
	}
	// Validate join condition
	if err := validateJoinCondition(leftTable, rightTable, leftColumn, rightColumn); err != nil {
		return nil, err
	}

	var results []data.JoinedRow
	var err error

	// Execute the appropriate JOIN type
	switch joinType {
	case JoinTypeInner:
		results, err = executeInnerJoin(leftTable, rightTable, leftColumn, rightColumn, pred)
	case JoinTypeLeft:
		results, err = executeLeftJoin(leftTable, rightTable, leftColumn, rightColumn, pred)
	case JoinTypeRight:
		results, err = executeRightJoin(leftTable, rightTable, leftColumn, rightColumn, pred)
	case JoinTypeFull:
		results, err = executeFullJoin(leftTable, rightTable, leftColumn, rightColumn, pred)
	default:
		return nil, fmt.Errorf("unknown JOIN type: %v", joinType)
	}

	if err != nil {
		return nil, err
	}

	// Apply projection if specified
	if proj != nil && !proj.SelectAll {
		projectedResults := make([]data.JoinedRow, len(results))
		for i, row := range results {
			projectedResults[i] = projection.ProjectJoinedRow(row, proj)
		}
		return projectedResults, nil
	}

	return results, nil
}

// executeInnerJoin performs INNER JOIN 
func executeInnerJoin(
	leftTable *schema.Table,
	rightTable *schema.Table,
	leftColumn string,
	rightColumn string,
	pred JoinPredicate,
) ([]data.JoinedRow, error) {
	// Acquire read locks on both tables
	leftTable.RLock()
	defer leftTable.RUnlock()
	rightTable.RLock()
	defer rightTable.RUnlock()

	slog.Debug("Starting INNER JOIN",
		slog.String("left_table", leftTable.Name),
		slog.String("right_table", rightTable.Name),
		slog.String("left_column", leftColumn),
		slog.String("right_column", rightColumn),
	)

	// Build hash index on right table
	hashIndex, reusedIndex := buildJoinIndex(rightTable, rightColumn)
	if reusedIndex {
		slog.Debug("Reusing existing index", slog.String("column", rightColumn))
	}

	results := make([]data.JoinedRow, 0)
	skippedByPredicate := 0

	// Probe left table and combine matches
	for _, leftRow := range leftTable.Rows {
		leftValue, exists := leftRow[leftColumn]
		if !exists {
			continue // Skip rows with NULL join column
		}

		rightPositions, found := hashIndex[leftValue]
		if !found {
			continue // No matches (INNER JOIN excludes)
		}

		for _, rightPos := range rightPositions {
			rightRow := rightTable.Rows[rightPos]
			joined := combineRows(leftRow, rightRow, leftTable.Name, rightTable.Name)

			if pred != nil && !pred(joined) {
				skippedByPredicate++
				continue
			}

			results = append(results, joined)
		}
	}

	slog.Info("INNER JOIN completed",
		slog.String("left_table", leftTable.Name),
		slog.String("right_table", rightTable.Name),
		slog.Int("result_rows", len(results)),
		slog.Int("filtered_by_predicate", skippedByPredicate),
	)

	return results, nil
}

// executeLeftJoin performs LEFT OUTER JOIN
func executeLeftJoin(
	leftTable *schema.Table,
	rightTable *schema.Table,
	leftColumn string,
	rightColumn string,
	pred JoinPredicate,
) ([]data.JoinedRow, error) {
	leftTable.RLock()
	defer leftTable.RUnlock()
	rightTable.RLock()
	defer rightTable.RUnlock()

	slog.Debug("Starting LEFT JOIN",
		slog.String("left_table", leftTable.Name),
		slog.String("right_table", rightTable.Name),
	)

	hashIndex, _ := buildJoinIndex(rightTable, rightColumn)
	results := make([]data.JoinedRow, 0)
	matchedLeftRows := make(map[int]bool)

	// Phase 1: INNER JOIN
	for leftPos, leftRow := range leftTable.Rows {
		leftValue, exists := leftRow[leftColumn]
		if !exists {
			continue
		}

		rightPositions, found := hashIndex[leftValue]
		if found {
			matchedLeftRows[leftPos] = true
			for _, rightPos := range rightPositions {
				rightRow := rightTable.Rows[rightPos]
				joined := combineRows(leftRow, rightRow, leftTable.Name, rightTable.Name)

				if pred == nil || pred(joined) {
					results = append(results, joined)
				}
			}
		}
	}

	// Phase 2: Add unmatched left rows
	for leftPos, leftRow := range leftTable.Rows {
		if !matchedLeftRows[leftPos] {
			joined := combineRowsWithNull(leftRow, nil, leftTable, rightTable)
			if pred == nil || pred(joined) {
				results = append(results, joined)
			}
		}
	}

	slog.Info("LEFT JOIN completed",
		slog.Int("result_rows", len(results)),
		slog.Int("unmatched_left", len(leftTable.Rows)-len(matchedLeftRows)),
	)

	return results, nil
}

// executeRightJoin performs RIGHT OUTER JOIN
func executeRightJoin(
	leftTable *schema.Table,
	rightTable *schema.Table,
	leftColumn string,
	rightColumn string,
	pred JoinPredicate,
) ([]data.JoinedRow, error) {
	leftTable.RLock()
	defer leftTable.RUnlock()
	rightTable.RLock()
	defer rightTable.RUnlock()

	slog.Debug("Starting RIGHT JOIN",
		slog.String("left_table", leftTable.Name),
		slog.String("right_table", rightTable.Name),
	)

	hashIndex, _ := buildJoinIndex(rightTable, rightColumn)
	results := make([]data.JoinedRow, 0)
	matchedRightRows := make(map[int]bool)

	// Phase 1: INNER JOIN
	for _, leftRow := range leftTable.Rows {
		leftValue, exists := leftRow[leftColumn]
		if !exists {
			continue
		}

		rightPositions, found := hashIndex[leftValue]
		if found {
			for _, rightPos := range rightPositions {
				matchedRightRows[rightPos] = true
				rightRow := rightTable.Rows[rightPos]
				joined := combineRows(leftRow, rightRow, leftTable.Name, rightTable.Name)

				if pred == nil || pred(joined) {
					results = append(results, joined)
				}
			}
		}
	}

	// Phase 2: Add unmatched right rows
	for rightPos, rightRow := range rightTable.Rows {
		if !matchedRightRows[rightPos] {
			joined := combineRowsWithNull(nil, rightRow, leftTable, rightTable)
			if pred == nil || pred(joined) {
				results = append(results, joined)
			}
		}
	}

	slog.Info("RIGHT JOIN completed",
		slog.Int("result_rows", len(results)),
		slog.Int("unmatched_right", len(rightTable.Rows)-len(matchedRightRows)),
	)

	return results, nil
}

// executeFullJoin performs FULL OUTER JOIN
func executeFullJoin(
	leftTable *schema.Table,
	rightTable *schema.Table,
	leftColumn string,
	rightColumn string,
	pred JoinPredicate,
) ([]data.JoinedRow, error) {
	leftTable.RLock()
	defer leftTable.RUnlock()
	rightTable.RLock()
	defer rightTable.RUnlock()

	slog.Debug("Starting FULL OUTER JOIN",
		slog.String("left_table", leftTable.Name),
		slog.String("right_table", rightTable.Name),
	)

	hashIndex, _ := buildJoinIndex(rightTable, rightColumn)
	results := make([]data.JoinedRow, 0)
	matchedLeftRows := make(map[int]bool)
	matchedRightRows := make(map[int]bool)

	// Phase 1: INNER JOIN
	for leftPos, leftRow := range leftTable.Rows {
		leftValue, exists := leftRow[leftColumn]
		if !exists {
			continue
		}

		rightPositions, found := hashIndex[leftValue]
		if found {
			matchedLeftRows[leftPos] = true
			for _, rightPos := range rightPositions {
				matchedRightRows[rightPos] = true
				rightRow := rightTable.Rows[rightPos]
				joined := combineRows(leftRow, rightRow, leftTable.Name, rightTable.Name)

				if pred == nil || pred(joined) {
					results = append(results, joined)
				}
			}
		}
	}

	// Phase 2: Add unmatched left rows
	for leftPos, leftRow := range leftTable.Rows {
		if !matchedLeftRows[leftPos] {
			joined := combineRowsWithNull(leftRow, nil, leftTable, rightTable)
			if pred == nil || pred(joined) {
				results = append(results, joined)
			}
		}
	}

	// Phase 3: Add unmatched right rows
	for rightPos, rightRow := range rightTable.Rows {
		if !matchedRightRows[rightPos] {
			joined := combineRowsWithNull(nil, rightRow, leftTable, rightTable)
			if pred == nil || pred(joined) {
				results = append(results, joined)
			}
		}
	}

	slog.Info("FULL OUTER JOIN completed",
		slog.Int("result_rows", len(results)),
		slog.Int("unmatched_left", len(leftTable.Rows)-len(matchedLeftRows)),
		slog.Int("unmatched_right", len(rightTable.Rows)-len(matchedRightRows)),
	)

	return results, nil
}
