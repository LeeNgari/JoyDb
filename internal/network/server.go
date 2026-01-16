package network

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/engine"
)

type Request struct {
	Query string `json:"query"`
}

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
	defer conn.Close()

	dbEngine := engine.New(db) // Renamed to avoid shadowing 'engine' package

	// Use Decoder instead of Scanner for network streams
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req Request
		// Decode directly from the connection
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return // Connection closed gracefully
			}
			slog.Error("decode error", "error", err)
			return
		}

		if req.Query == "exit" || req.Query == "\\q" {
			return
		}

		result, err := dbEngine.Execute(req.Query)
		if err != nil {
			_ = encoder.Encode(map[string]any{"error": err.Error()})
			continue
		}

		if err := encoder.Encode(result); err != nil {
			slog.Error("encode error", "error", err)
			return
		}
	}
}
