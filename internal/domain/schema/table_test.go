package schema

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/errors"
	"github.com/leengari/mini-rdbms/internal/index/btree"
	"github.com/stretchr/testify/assert"
)

func TestTable_InsertAndAutoIncrement(t *testing.T) {
	columns := []Column{
		{Name: "id", Type: ColumnTypeInt, PrimaryKey: true, AutoIncrement: true},
		{Name: "name", Type: ColumnTypeText},
	}
	ts := &TableSchema{
		TableName: "users",
		Columns:   columns,
	}
	table := &Table{
		Name:      "users",
		Schema:    ts,
		RowsByRID: make(map[int64]data.Row),
		Indexes:   make(map[string]*data.Index),
		PKIndex:   btree.New(btree.DefaultDegree),
	}

	// First insert: id should auto-increment to 1
	row1 := data.Row{Values: []interface{}{nil, "Alice"}}
	err := table.Insert(row1, nil)
	assert.NoError(t, err)

	liveRows := table.LiveRows()
	assert.Len(t, liveRows, 1)
	assert.Equal(t, int64(1), liveRows[0].RID)
	assert.Equal(t, int64(1), liveRows[0].Values[0])

	// Second insert: id should auto-increment to 2
	row2 := data.Row{Values: []interface{}{nil, "Bob"}}
	err = table.Insert(row2, nil)
	assert.NoError(t, err)

	liveRows = table.LiveRows()
	assert.Len(t, liveRows, 2)
	assert.Equal(t, int64(2), liveRows[1].Values[0])
}

func TestTable_UpdateUniqueConstraints(t *testing.T) {
	columns := []Column{
		{Name: "id", Type: ColumnTypeInt, PrimaryKey: true},
		{Name: "email", Type: ColumnTypeText, Unique: true},
	}
	ts := &TableSchema{
		TableName: "users",
		Columns:   columns,
	}
	table := &Table{
		Name:      "users",
		Schema:    ts,
		RowsByRID: make(map[int64]data.Row),
		Indexes:   make(map[string]*data.Index),
		PKIndex:   btree.New(btree.DefaultDegree),
	}
	table.Indexes["email"] = &data.Index{
		Column: "email",
		Data:   make(map[interface{}][]int64),
		Unique: true,
	}

	// Insert Bob (id=1, email="bob@gmail.com")
	err := table.Insert(data.Row{
		Values: []interface{}{int64(1), "bob@gmail.com"},
	}, nil)
	assert.NoError(t, err)

	// Insert Alice (id=2, email="alice@gmail.com")
	err = table.Insert(data.Row{
		Values: []interface{}{int64(2), "alice@gmail.com"},
	}, nil)
	assert.NoError(t, err)

	// Update Bob to have the same email as Alice -> should FAIL
	updatePred := func(r data.Row) bool {
		idVal, ok := normalizeToInt64(r.Values[0])
		return ok && idVal == 1
	}
	updates := map[string]interface{}{"email": "alice@gmail.com"}

	count, err := table.Update(updatePred, updates, nil)
	assert.Error(t, err)
	assert.Equal(t, 0, count)

	constraintErr, ok := err.(*errors.ConstraintError)
	assert.True(t, ok)
	assert.Equal(t, "unique", constraintErr.Constraint)
	assert.Equal(t, "email", constraintErr.Column)

	// Verify Bob's email is still "bob@gmail.com" in index and table
	row, found := table.SelectByIndex("email", "bob@gmail.com", nil)
	assert.True(t, found)
	assert.Equal(t, int64(1), row.RID)

	// Update Bob to a new unique email -> should SUCCEED
	updatesValid := map[string]interface{}{"email": "bob_new@gmail.com"}
	count, err = table.Update(updatePred, updatesValid, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify Bob's email is updated to "bob_new@gmail.com"
	_, foundOld := table.SelectByIndex("email", "bob@gmail.com", nil)
	assert.False(t, foundOld)

	rowNew, foundNew := table.SelectByIndex("email", "bob_new@gmail.com", nil)
	assert.True(t, foundNew)
	assert.Equal(t, int64(1), rowNew.RID)
}
