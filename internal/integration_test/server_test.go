package integration

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/leengari/mini-rdbms/internal/network"
)

func TestServer(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	// Pick a random high port
	port := 54321

	// Start server in goroutine
	go network.Start(port, db)

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Connect
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

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

			// Check if we have received the expected tabular output
			// We look for "Returned" message AND the header separator or content
			if strings.Contains(output, "Returned") && strings.Contains(output, "---") {
				break
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
