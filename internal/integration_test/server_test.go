package integration

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"encoding/json"

	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/network"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
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
	Error        string           // Error message if any
}

func TestServerJSON(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	port := 54321

	basePath := filepath.Dir(testDBPath)
	registry := manager.NewRegistry(basePath)

	// Start server in goroutine
	go network.Start(port, registry)

	// Wait a bit for server
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	queries := []struct {
		Request       network.Request
		ExpectedError string
	}{
		{
			Request:       network.Request{Query: "USE testdb_integration"},
			ExpectedError: "",
		},
		{
			Request:       network.Request{Query: "SELECT * FROM users"},
			ExpectedError: "",
		},
		{
			Request:       network.Request{Query: "SELECT * FROM non_existent_table"},
			ExpectedError: "planning error: table not found: non_existent_table",
		},
	}

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for _, tc := range queries {
		// Send query
		if err := encoder.Encode(tc.Request); err != nil {
			t.Fatalf("Failed to send query: %v", err)
		}

		// Decode response
		var res Result
		if err := decoder.Decode(&res); err != nil {
			t.Fatalf("Failed to decode JSON: %v", err)
		}

		t.Logf("Query: %s\nResult: %+v", tc.Request.Query, res)

		if tc.ExpectedError != "" {
			if res.Error == "" {
				t.Errorf("Expected error '%s', got no error", tc.ExpectedError)
			} else if res.Error != tc.ExpectedError {
				t.Errorf("Expected error '%s', got '%s'", tc.ExpectedError, res.Error)
			}
			continue
		}

		if res.Error != "" {
			t.Errorf("Unexpected error: %s", res.Error)
			continue
		}

		// Assertions for successful query
		if strings.HasPrefix(strings.ToUpper(tc.Request.Query), "USE") {
			if res.Message == "" {
				t.Error("Expected message for USE command, got empty")
			}
			continue
		}

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
