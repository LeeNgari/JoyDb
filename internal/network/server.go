package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/leengari/joydb/internal/engine"
	"github.com/leengari/joydb/internal/executor"
	"github.com/leengari/joydb/internal/storage/manager"
)

type Request struct {
	Query string `json:"query"`
}

type Server struct {
	listener net.Listener
	registry *manager.Registry
}

// StartServer starts the TCP database server and returns a stoppable Server instance
func StartServer(port int, registry *manager.Registry) (*Server, error) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind to port: %w", err)
	}

	srv := &Server{
		listener: listener,
		registry: registry,
	}

	slog.Info("Running on port", "port", port)

	go srv.serve()
	return srv, nil
}

func (s *Server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				slog.Error("Failed to accept connection", "error", err)
			}
			return
		}
		go handleConnection(conn, s.registry)
	}
}

// Stop stops the server from accepting new connections
func (s *Server) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func handleConnection(conn net.Conn, registry *manager.Registry) {
	defer conn.Close()

	dbEngine := engine.New(nil, registry)

	loggingObserver := engine.NewLoggingObserver()
	dbEngine.AddObserver(loggingObserver)

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req Request
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return
			}
			slog.Error("decode error", "error", err)

			errResult := &executor.Result{
				Error: fmt.Sprintf("Invalid request format: %v", err),
			}
			_ = encoder.Encode(errResult)
			return
		}

		if req.Query == "exit" || req.Query == "\\q" {
			return
		}

		result, err := dbEngine.Execute(req.Query)
		if err != nil {

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
