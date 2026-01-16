package integration

import (
	"fmt"
	"net"
	"testing"
	"time"

	"encoding/json"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/network"
)

type ColumnMetadata struct {
	Name string // Column name
	Type string // Data type as string
}

// Result represents the outcome of executing a SQL statement
type Result struct {
	Columns      []string         // Column names
	Metadata     []ColumnMetadata // Column metadata
	Rows         []data.Row       // Result rows
	Message      string           // Status message
	RowsAffected int              // Rows affected by INSERT/UPDATE/DELETE
}

func TestServerJSON(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	port := 54321

	// Start server in goroutine
	go network.Start(port, db)

	// Wait a bit for server
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	queries := []network.Request{
    {Query: "SELECT * FROM users"},
}

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for _, req := range queries {
		// Send query
		if err := encoder.Encode(req); err != nil {
        t.Fatalf("Failed to send query: %v", err)
    }

		// Decode response
		var res Result
		if err := decoder.Decode(&res); err != nil {
			t.Fatalf("Failed to decode JSON: %v", err)
		}

		t.Logf("Query: %s\nResult: %+v", req, res)

		// Assertions
		foundAdmin := false
		for _, row := range res.Rows {
			if row["username"] == "admin" {
				foundAdmin = true
				break
			}
		}
		if !foundAdmin {
			t.Errorf("Expected row with username 'admin', got: %+v", res.Rows)
		}

		headerFound := false
		for _, col := range res.Columns {
			if col == "id" {
				headerFound = true
				break
			}
		}
		if !headerFound {
			t.Errorf("Expected 'id' in columns, got: %+v", res.Columns)
		}
	}
}
