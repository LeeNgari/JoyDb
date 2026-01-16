package integration

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leengari/mini-rdbms/internal/network"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

func TestServer(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	// Pick a random high port
	port := 54321

	// Start server in goroutine
	// testDBPath is defined in testdb_helper.go ("../../databases/testdb_integration")
	basePath := filepath.Dir(testDBPath)
	registry := manager.NewRegistry(basePath)
	// Pre-load the test database into the registry so it's available and indexed
	// (Although setupTestDB already created it on disk, registry needs to know about it/load it if we want it shared)
	// Actually, USE command will load it via registry. So we just need to pass the registry.
	go network.Start(port, registry)

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Connect
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Select the database
	fmt.Fprintf(conn, "USE testdb_integration\n")
	// Read response (Switched to database...)
	tempBuf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	conn.Read(tempBuf)

	queries := []string{
		"SELECT * FROM users",
	}

	for _, query := range queries {
		// Send query
		_, err := fmt.Fprintf(conn, "%s\n", query)
		if err != nil {
			t.Fatalf("Failed to write to connection: %v", err)
		}

		// Read response loop
		output := ""
		buf := make([]byte, 1024)
		timeout := time.Now().Add(2 * time.Second)

		for time.Now().Before(timeout) {
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, err := conn.Read(buf)
			if n > 0 {
				output += string(buf[:n])
			}

			// We need to wait until we get the actual rows.
			// "Returned X rows" is first.
			// Then headers, then "---".
			// Then rows.
			// Simple heuristic: if we have "Returned" and "---", wait a bit more for rows to arrive
			// unless we already have "admin" (which is what we look for).
			if strings.Contains(output, "Returned") && strings.Contains(output, "---") {
				// If we found the user we are looking for, we can stop
				if strings.Contains(output, "admin") {
					break
				}
				// Otherwise, keep reading until timeout (to ensure we get all data)
				// or until we hit a reasonable size.
			}

			if err != nil {
				// If we got an error that is not a timeout, or if we got EOF (server closed)
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				break
			}
		}

		t.Logf("Query: %s\nOutput:\n%s", query, output)

		if !strings.Contains(output, "admin") {
			t.Errorf("Expected 'admin' in output, got: %s", output)
		}
		if !strings.Contains(output, "id (INT)") {
			t.Errorf("Expected header 'id (INT)' in output, got: %s", output)
		}
	}
}
