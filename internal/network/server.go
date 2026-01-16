package network

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/leengari/mini-rdbms/internal/engine"
	"github.com/leengari/mini-rdbms/internal/executor"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

type Request struct {
	Query string `json:"query"`
}

// Start starts the TCP database server
func Start(port int, registry *manager.Registry) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("Failed to bind to port", "port", port, "error", err)
		return
	}
	defer listener.Close()

	slog.Info("Running on port", "port", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Error("Failed to accept connection", "error", err)
			continue
		}
		go handleConnection(conn, registry)
	}
}

func handleConnection(conn net.Conn, registry *manager.Registry) {
	defer conn.Close()

	dbEngine := engine.New(nil, registry)

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
			// Return error as a Result object
			errResult := &executor.Result{
				Error: err.Error(),
			}
			if err := encoder.Encode(errResult); err != nil {
				slog.Error("encode error", "error", err)
				return
			}
			continue
		}

		if err := encoder.Encode(result); err != nil {
			slog.Error("encode error", "error", err)
			return
		}
	}
}
