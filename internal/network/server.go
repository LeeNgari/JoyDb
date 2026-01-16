package network

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/engine"
	"github.com/leengari/mini-rdbms/internal/repl"
)

// Start starts the TCP server on the given port
func Start(port int, db *schema.Database) {
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
		go handleConnection(conn, db)
	}
}

func handleConnection(conn net.Conn, db *schema.Database) {
	defer conn.Close()
	eng := engine.New(db)
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
