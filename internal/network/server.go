package network

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"github.com/leengari/mini-rdbms/internal/engine"
	"github.com/leengari/mini-rdbms/internal/repl"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

// Start starts the TCP server on the given port
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
	// Each connection starts with no DB selected, but shares the Registry
	eng := engine.New(nil, registry)
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			continue
		}

		if line == "exit" || line == "\\q" {
			break
		}

		result, err := eng.Execute(line)
		if err != nil {
			io.WriteString(conn, fmt.Sprintf("Error: %v\n", err))
			continue
		}

		repl.PrintResult(conn, result)
	}

	if err := scanner.Err(); err != nil {
		slog.Error("Connection error", "remote_addr", conn.RemoteAddr(), "error", err)
	}
}
