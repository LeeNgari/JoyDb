package projection_test

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
	"github.com/leengari/mini-rdbms/internal/query/operations/testutil"
)

// TestProjection_SelectAll tests selecting all columns
func TestProjection_SelectAll(t *testing.T) {
	table := testutil.CreateTestTable("users")
	table.Rows = []data.Row{
		data.NewRow(map[string]interface{}{"id": int64(1), "name": "Alice", "email": "alice@example.com", "age": int64(30)}),
		data.NewRow(map[string]interface{}{"id": int64(2), "name": "Bob", "email": "bob@example.com", "age": int64(25)}),
	}

	// SELECT * (all columns)
	proj := projection.NewProjection()
	results := make([]data.Row, len(table.Rows))
	for i, row := range table.Rows {
		results[i] = projection.ProjectRow(row, proj, table.Name)
	}

	testutil.AssertRowCount(t, len(results), 2, "SELECT *")
	testutil.AssertColumnCount(t, len(results[0].Data), 4, "First row")
}

// TestProjection_SelectSpecificColumns tests selecting specific columns
func TestProjection_SelectSpecificColumns(t *testing.T) {
	table := testutil.CreateTestTable("users")
	table.Rows = []data.Row{
		data.NewRow(map[string]interface{}{"id": int64(1), "name": "Alice", "email": "alice@example.com", "age": int64(30)}),
		data.NewRow(map[string]interface{}{"id": int64(2), "name": "Bob", "email": "bob@example.com", "age": int64(25)}),
	}

	// SELECT id, name
	proj := projection.NewProjectionWithColumns(
		projection.ColumnRef{Column: "id"},
		projection.ColumnRef{Column: "name"},
	)

	results := make([]data.Row, len(table.Rows))
	for i, row := range table.Rows {
		results[i] = projection.ProjectRow(row, proj, table.Name)
	}

	testutil.AssertRowCount(t, len(results), 2, "SELECT id, name")
	testutil.AssertColumnCount(t, len(results[0].Data), 2, "Projected row")
	testutil.AssertColumnExists(t, results[0], "id", "Projected row")
	testutil.AssertColumnExists(t, results[0], "name", "Projected row")
	testutil.AssertColumnNotExists(t, results[0], "email", "Projected row")
}

// TestProjection_WithAlias tests column aliasing
func TestProjection_WithAlias(t *testing.T) {
	table := testutil.CreateTestTable("users")
	table.Rows = []data.Row{
		data.NewRow(map[string]interface{}{"id": int64(1), "name": "Alice", "email": "alice@example.com"}),
	}

	// SELECT id AS user_id, name AS username
	proj := projection.NewProjectionWithColumns(
		projection.ColumnRef{Column: "id", Alias: "user_id"},
		projection.ColumnRef{Column: "name", Alias: "username"},
	)

	result := projection.ProjectRow(table.Rows[0], proj, table.Name)

	testutil.AssertColumnExists(t, result, "user_id", "Aliased projection")
	testutil.AssertColumnExists(t, result, "username", "Aliased projection")
	testutil.AssertColumnNotExists(t, result, "id", "Aliased projection")
}

// TestProjection_ValidateProjection tests projection validation
func TestProjection_ValidateProjection(t *testing.T) {
	table := testutil.CreateTestTable("users")

	// Valid projection
	validProj := projection.NewProjectionWithColumns(
		projection.ColumnRef{Column: "id"},
		projection.ColumnRef{Column: "name"},
	)

	err := projection.ValidateProjection(table, validProj)
	testutil.AssertNoError(t, err, "Valid projection")

	// Invalid projection (non-existent column)
	invalidProj := projection.NewProjectionWithColumns(
		projection.ColumnRef{Column: "nonexistent"},
	)

	err = projection.ValidateProjection(table, invalidProj)
	testutil.AssertError(t, err, "Invalid projection")
}

// TestProjection_EmptyProjection tests nil projection (returns all columns)
func TestProjection_EmptyProjection(t *testing.T) {
	table := testutil.CreateTestTable("users")
	table.Rows = []data.Row{
		data.NewRow(map[string]interface{}{"id": int64(1), "name": "Alice", "email": "alice@example.com"}),
	}

	// nil projection should return all columns
	result := projection.ProjectRow(table.Rows[0], nil, table.Name)

	testutil.AssertColumnCount(t, len(result.Data), 3, "Nil projection")
}
