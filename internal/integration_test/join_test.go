package integration

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/domain/transaction"
	"github.com/leengari/mini-rdbms/internal/query/operations/join"
	"github.com/leengari/mini-rdbms/internal/query/operations/projection"
	"github.com/leengari/mini-rdbms/internal/query/operations/testutil"
)

// TestJoinOperations tests all JOIN types with real database
func TestJoinOperations(t *testing.T) {
	// Load test database
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	usersTable, ok := db.Tables["users"]
	if !ok {
		t.Fatal("users table not found")
	}

	ordersTable, ok := db.Tables["orders"]
	if !ok {
		t.Skip("orders table not found - skipping JOIN tests")
	}

	joinTests := []struct {
		name     string
		joinType join.JoinType
	}{
		{"InnerJoin", join.JoinTypeInner},
		{"LeftJoin", join.JoinTypeLeft},
		{"RightJoin", join.JoinTypeRight},
		{"FullOuterJoin", join.JoinTypeFull},
	}

	for _, tt := range joinTests {
		t.Run(tt.name, func(t *testing.T) {
			tx := transaction.NewTransaction()
			defer tx.Close()
			results, err := join.ExecuteJoin(
				usersTable, ordersTable,
				"id", "user_id",
				tt.joinType,
				nil, nil, tx,
			)

			testutil.AssertNoError(t, err, tt.name)
			if len(results) == 0 {
				t.Errorf("Expected %s results, got none", tt.name)
			}

			if tt.joinType == join.JoinTypeInner {
				// Verify joined row structure for inner join
				for _, row := range results {
					if _, exists := row.Get("users.id"); !exists {
						t.Error("Expected users.id in joined row")
					}
					if _, exists := row.Get("orders.product"); !exists {
						t.Error("Expected orders.product in joined row")
					}
				}
			}

			t.Logf("%s returned %d rows", tt.name, len(results))
		})
	}

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
