package testutil

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
)

// AssertRowCount checks if the result has the expected number of rows
func AssertRowCount(t *testing.T, actual, expected int, context string) {
	t.Helper()
	if actual != expected {
		t.Errorf("%s: expected %d rows, got %d", context, expected, actual)
	}
}

// AssertColumnCount checks if a row has the expected number of columns
func AssertColumnCount(t *testing.T, actual, expected int, context string) {
	t.Helper()
	if actual != expected {
		t.Errorf("%s: expected %d columns, got %d", context, expected, actual)
	}
}

// AssertColumnExists checks if a column exists in a row
func AssertColumnExists(t *testing.T, row data.Row, column, context string) {
	t.Helper()
	if _, exists := row.Data[column]; !exists {
		t.Errorf("%s: expected column '%s' to exist", context, column)
	}
}

// AssertColumnNotExists checks if a column does not exist in a row
func AssertColumnNotExists(t *testing.T, row data.Row, column, context string) {
	t.Helper()
	if _, exists := row.Data[column]; exists {
		t.Errorf("%s: did not expect column '%s' to exist", context, column)
	}
}

// AssertNoError checks that an error is nil
func AssertNoError(t *testing.T, err error, context string) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: expected no error, got: %v", context, err)
	}
}

// AssertError checks that an error is not nil
func AssertError(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected an error, got nil", context)
	}
}

// AssertNullValue checks if a value is nil
func AssertNullValue(t *testing.T, value interface{}, context string) {
	t.Helper()
	if value != nil {
		t.Errorf("%s: expected NULL value, got: %v", context, value)
	}
}

// AssertNotNullValue checks if a value is not nil
func AssertNotNullValue(t *testing.T, value interface{}, context string) {
	t.Helper()
	if value == nil {
		t.Errorf("%s: expected non-NULL value, got nil", context)
	}
}

// AssertJoinedColumnExists checks if a column exists in a JoinedRow
func AssertJoinedColumnExists(t *testing.T, row map[string]interface{}, column, context string) {
	t.Helper()
	if _, exists := row[column]; !exists {
		t.Errorf("%s: expected column '%s' to exist", context, column)
	}
}

// AssertJoinedColumnNotExists checks if a column does not exist in a JoinedRow
func AssertJoinedColumnNotExists(t *testing.T, row map[string]interface{}, column, context string) {
	t.Helper()
	if _, exists := row[column]; exists {
		t.Errorf("%s: did not expect column '%s' to exist", context, column)
	}
}
