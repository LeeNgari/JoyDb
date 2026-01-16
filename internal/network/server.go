package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/engine"
)

// Start starts the TCP database server
func Start(port int, db *schema.Database) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to bind TCP listener", "addr", addr, "error", err)
		return
	}
	defer listener.Close()

	slog.Info("TCP DB server listening", "addr", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Error("failed to accept connection", "error", err)
			continue
		}

		slog.Info("client connected", "remote", conn.RemoteAddr())
		go handleConnection(conn, db)
	}
}

func handleConnection(conn net.Conn, db *schema.Database) {
	defer func() {
		slog.Info("client disconnected", "remote", conn.RemoteAddr())
		conn.Close()
	}()

	engine := engine.New(db)

	scanner := bufio.NewScanner(conn)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		query := strings.TrimSpace(scanner.Text())

		if query == "" {
			continue
		}

		if query == "exit" || query == "\\q" {
			return
		}

		result, err := engine.Execute(query)
		if err != nil {
			// Send structured error response
			_ = encoder.Encode(map[string]any{
				"error": err.Error(),
			})
			continue
		}

		// Send JSON result
		if err := encoder.Encode(result); err != nil {
			slog.Error(
				"failed to encode response",
				"remote", conn.RemoteAddr(),
				"error", err,
			)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error(
			"connection read error",
			"remote", conn.RemoteAddr(),
			"error", err,
		)
	}
}
