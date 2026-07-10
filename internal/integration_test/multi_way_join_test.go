package integration

import (
	"path/filepath"
	"testing"

	"github.com/leengari/mini-rdbms/internal/engine"
	storageEngine "github.com/leengari/mini-rdbms/internal/storage/engine"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

func TestSQLMultiWayJoin(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	storageEng := storageEngine.NewMemoryEngine()
	registry := manager.NewRegistry(filepath.Dir(db.Path), storageEng)
	eng := engine.New(db, registry)

	// 1. Create a third table for products
	_, err := eng.Execute("CREATE TABLE products (id INT PRIMARY KEY, name TEXT, price FLOAT);")
	if err != nil {
		t.Fatalf("Failed to create products table: %v", err)
	}

	// 2. Insert rows into products
	_, err = eng.Execute("INSERT INTO products (id, name, price) VALUES (1, 'Laptop', 1200.50);")
	if err != nil {
		t.Fatalf("Failed to insert product 1: %v", err)
	}
	_, err = eng.Execute("INSERT INTO products (id, name, price) VALUES (2, 'Mouse', 25.99);")
	if err != nil {
		t.Fatalf("Failed to insert product 2: %v", err)
	}

	// 3. Execute three-table JOIN
	query := `
		SELECT users.username, orders.product, products.price
		FROM users
		INNER JOIN orders ON users.id = orders.user_id
		INNER JOIN products ON orders.product = products.name;
	`
	res, err := eng.Execute(query)
	if err != nil {
		t.Fatalf("Three-table JOIN query failed: %v", err)
	}

	// 4. Verify results
	if len(res.Rows) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(res.Rows))
	}

	// Verify columns exist in the output rows without pointers
	for _, row := range res.Rows {
		// Output column keys should be just users.username, orders.product, products.price
		rowMap := getRowMap(row, res.Columns)
		username, exists := rowMap["username"]
		if !exists {
			t.Errorf("Expected 'username' in result, but row only had: %+v", rowMap)
		}
		product, exists := rowMap["product"]
		if !exists {
			t.Errorf("Expected 'product' in result, but row only had: %+v", rowMap)
		}
		price, exists := rowMap["price"]
		if !exists {
			t.Errorf("Expected 'price' in result, but row only had: %+v", rowMap)
		}

		if username == "admin" {
			if product == "Laptop" && price != 1200.50 {
				t.Errorf("Expected Laptop price to be 1200.50, got %v", price)
			}
			if product == "Mouse" && price != 25.99 {
				t.Errorf("Expected Mouse price to be 25.99, got %v", price)
			}
		} else {
			t.Errorf("Expected user to be admin, got %v", username)
		}
	}
}
