package projection_test

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
	"github.com/leengari/mini-rdbms/internal/query/operations/testutil"
)

// TestProjection_SelectAll tests selecting all columns
func TestProjection_SelectAll(t *testing.T) {
	table := testutil.CreateTestTable("users")
	table.InsertReplay(data.NewRow([]interface{}{int64(1), "Alice", "alice@example.com", int64(30)}))
	table.InsertReplay(data.NewRow([]interface{}{int64(2), "Bob", "bob@example.com", int64(25)}))

	// SELECT * (all columns)
	proj := projection.NewProjection()
	liveRows := table.LiveRows()
	results := make([]data.Row, len(liveRows))
	for i, row := range liveRows {
		results[i] = projection.ProjectRow(row, proj, table.Schema)
	}

	testutil.AssertRowCount(t, len(results), 2, "SELECT *")
	testutil.AssertColumnCount(t, len(results[0].Values), 4, "First row")
}

// TestProjection_SelectSpecificColumns tests selecting specific columns
func TestProjection_SelectSpecificColumns(t *testing.T) {
	table := testutil.CreateTestTable("users")
	table.InsertReplay(data.NewRow([]interface{}{int64(1), "Alice", "alice@example.com", int64(30)}))
	table.InsertReplay(data.NewRow([]interface{}{int64(2), "Bob", "bob@example.com", int64(25)}))

	// SELECT id, name
	proj := projection.NewProjectionWithColumns(
		projection.ColumnRef{Column: "id"},
		projection.ColumnRef{Column: "name"},
	)

	liveRows := table.LiveRows()
	results := make([]data.Row, len(liveRows))
	for i, row := range liveRows {
		results[i] = projection.ProjectRow(row, proj, table.Schema)
	}

	projectedSchema := &schema.TableSchema{
		Columns: []schema.Column{
			{Name: "id"},
			{Name: "name"},
		},
	}

	testutil.AssertRowCount(t, len(results), 2, "SELECT id, name")
	testutil.AssertColumnCount(t, len(results[0].Values), 2, "Projected row")
	testutil.AssertColumnExists(t, results[0], projectedSchema, "id", "Projected row")
	testutil.AssertColumnExists(t, results[0], projectedSchema, "name", "Projected row")
	testutil.AssertColumnNotExists(t, results[0], projectedSchema, "email", "Projected row")
}

// TestProjection_WithAlias tests column aliasing
func TestProjection_WithAlias(t *testing.T) {
	table := testutil.CreateTestTable("users")
	table.InsertReplay(data.NewRow([]interface{}{int64(1), "Alice", "alice@example.com", int64(30)}))

	// SELECT id AS user_id, name AS username
	proj := projection.NewProjectionWithColumns(
		projection.ColumnRef{Column: "id", Alias: "user_id"},
		projection.ColumnRef{Column: "name", Alias: "username"},
	)

	result := projection.ProjectRow(table.LiveRows()[0], proj, table.Schema)

	projectedSchema := &schema.TableSchema{
		Columns: []schema.Column{
			{Name: "user_id"},
			{Name: "username"},
		},
	}

	testutil.AssertColumnExists(t, result, projectedSchema, "user_id", "Aliased projection")
	testutil.AssertColumnExists(t, result, projectedSchema, "username", "Aliased projection")
	testutil.AssertColumnNotExists(t, result, projectedSchema, "id", "Aliased projection")
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
	table.InsertReplay(data.NewRow([]interface{}{int64(1), "Alice", "alice@example.com", int64(30)}))

	// nil projection should return all columns
	result := projection.ProjectRow(table.LiveRows()[0], nil, table.Schema)

	testutil.AssertColumnCount(t, len(result.Values), 4, "Nil projection")
}
