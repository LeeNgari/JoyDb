package integration

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/query/indexing"
	"github.com/leengari/mini-rdbms/internal/query/operations/join"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
	"github.com/leengari/mini-rdbms/internal/query/operations/testutil"
	"github.com/leengari/mini-rdbms/internal/storage/loader"
)

// TestJoinOperations tests all JOIN types with real database
func TestJoinOperations(t *testing.T) {
	// Load test database
	db, err := loader.LoadDatabase("../../databases/testdb")
	if err != nil {
		t.Fatalf("Failed to load database: %v", err)
	}

	// Build indexes
	if err := indexing.BuildDatabaseIndexes(db); err != nil {
		t.Fatalf("Failed to build indexes: %v", err)
	}

	usersTable, ok := db.Tables["users"]
	if !ok {
		t.Fatal("users table not found")
	}

	ordersTable, ok := db.Tables["orders"]
	if !ok {
		t.Skip("orders table not found - skipping JOIN tests")
	}

	t.Run("InnerJoin", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		results, err := join.ExecuteJoin(
			usersTable, ordersTable,
			"id", "user_id",
			join.JoinTypeInner,
			nil, nil, tx,
		)

		testutil.AssertNoError(t, err, "INNER JOIN")
		if len(results) == 0 {
			t.Error("Expected JOIN results, got none")
		}

		// Verify joined row structure
		for _, row := range results {
			if _, exists := row.Get("users.id"); !exists {
				t.Error("Expected users.id in joined row")
			}
			if _, exists := row.Get("orders.product"); !exists {
				t.Error("Expected orders.product in joined row")
			}
		}

		t.Logf("INNER JOIN returned %d rows", len(results))
	})

	t.Run("LeftJoin", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		results, err := join.ExecuteJoin(
			usersTable, ordersTable,
			"id", "user_id",
			join.JoinTypeLeft,
			nil, nil, tx,
		)

		testutil.AssertNoError(t, err, "LEFT JOIN")
		
		// LEFT JOIN should return at least as many rows as INNER JOIN
		if len(results) == 0 {
			t.Error("Expected LEFT JOIN results, got none")
		}

		t.Logf("LEFT JOIN returned %d rows", len(results))
	})

	t.Run("RightJoin", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		results, err := join.ExecuteJoin(
			usersTable, ordersTable,
			"id", "user_id",
			join.JoinTypeRight,
			nil, nil, tx,
		)

		testutil.AssertNoError(t, err, "RIGHT JOIN")
		if len(results) == 0 {
			t.Error("Expected RIGHT JOIN results, got none")
		}

		t.Logf("RIGHT JOIN returned %d rows", len(results))
	})

	t.Run("FullOuterJoin", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		results, err := join.ExecuteJoin(
			usersTable, ordersTable,
			"id", "user_id",
			join.JoinTypeFull,
			nil, nil, tx,
		)

		testutil.AssertNoError(t, err, "FULL OUTER JOIN")
		if len(results) == 0 {
			t.Error("Expected FULL JOIN results, got none")
		}

		t.Logf("FULL OUTER JOIN returned %d rows", len(results))
	})

	t.Run("JoinWithProjection", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		proj := projection.NewProjectionWithColumns(
			projection.ColumnRef{Table: "users", Column: "username"},
			projection.ColumnRef{Table: "orders", Column: "product"},
			projection.ColumnRef{Table: "orders", Column: "amount"},
		)

		results, err := join.ExecuteJoin(
			usersTable, ordersTable,
			"id", "user_id",
			join.JoinTypeInner,
			nil, proj, tx,
		)

		testutil.AssertNoError(t, err, "JOIN with projection")
		if len(results) == 0 {
			t.Error("Expected JOIN results, got none")
		}

		// Verify only projected columns exist
		for _, row := range results {
			if len(row.Data) != 3 {
				t.Errorf("Expected 3 columns, got %d", len(row.Data))
			}
			if _, exists := row.Get("users.username"); !exists {
				t.Error("Expected users.username in projected row")
			}
			if _, exists := row.Get("orders.product"); !exists {
				t.Error("Expected orders.product in projected row")
			}
			if _, exists := row.Get("orders.amount"); !exists {
				t.Error("Expected orders.amount in projected row")
			}
		}

		t.Logf("JOIN with projection returned %d rows", len(results))
	})

	t.Run("JoinWithPredicate", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		// Only orders with amount > 50
		predicate := func(row data.JoinedRow) bool {
			amount, exists := row.Get("orders.amount")
			if !exists {
				return false
			}
			amountVal, ok := amount.(float64)
			return ok && amountVal > 50.0
		}

		results, err := join.ExecuteJoin(
			usersTable, ordersTable,
			"id", "user_id",
			join.JoinTypeInner,
			predicate, nil, tx,
		)

		testutil.AssertNoError(t, err, "JOIN with predicate")
		
		// Verify all results match predicate
		for _, row := range results {
			amount, _ := row.Get("orders.amount")
			if amountVal, ok := amount.(float64); ok && amountVal <= 50.0 {
				t.Errorf("Expected amount > 50, got %f", amountVal)
			}
		}

		t.Logf("JOIN with predicate returned %d rows", len(results))
	})

	// Edge case: Empty tables
	t.Run("JoinEmptyTables", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		emptyUsers := testutil.CreateUsersTable()
		emptyUsers.Rows = []data.Row{}
		
		emptyOrders := testutil.CreateOrdersTable()
		emptyOrders.Rows = []data.Row{}
		
		results, err := join.ExecuteJoin(
			emptyUsers, emptyOrders,
			"id", "user_id",
			join.JoinTypeInner,
			nil, nil, tx,
		)
		
		testutil.AssertNoError(t, err, "JOIN with empty tables")
		testutil.AssertRowCount(t, len(results), 0, "Empty JOIN results")
	})

	// Edge case: LEFT JOIN with empty right table
	t.Run("LeftJoinEmptyRight", func(t *testing.T) {
		tx := transaction.NewTransaction()
		defer tx.Close()
		testUsers := testutil.CreateUsersTable()
		
		emptyOrders := testutil.CreateOrdersTable()
		emptyOrders.Rows = []data.Row{}
		
		results, err := join.ExecuteJoin(
			testUsers, emptyOrders,
			"id", "user_id",
			join.JoinTypeLeft,
			nil, nil, tx,
		)
		
		testutil.AssertNoError(t, err, "LEFT JOIN with empty right table")
		// Should return all users with NULL order columns
		testutil.AssertRowCount(t, len(results), 3, "LEFT JOIN results")
		
		// All rows should have NULL order columns
		for _, row := range results {
			product, _ := row.Get("orders.product")
			testutil.AssertNullValue(t, product, "Product in empty right JOIN")
		}
	})
}
