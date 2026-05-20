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

// Server represents the TCP database server
type Server struct {
	listener net.Listener
	registry *manager.Registry
}

// NewServer creates a new TCP database server instance but does not start listening
func NewServer(port int, registry *manager.Registry) (*Server, error) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &Server{
		listener: listener,
		registry: registry,
	}, nil
}

// Start starts the server loop
func (s *Server) Start() {
	slog.Info("Running on port", "port", s.listener.Addr().(*net.TCPAddr).Port)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// When the listener is closed, Accept will return an error
			slog.Debug("Server listener closed or accept error", "error", err)
			return
		}
		go handleConnection(conn, s.registry)
	}
}

// Close gracefully closes the server listener
func (s *Server) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Start starts the TCP database server in a blocking way (retained for backward compatibility)
func Start(port int, registry *manager.Registry) {
	s, err := NewServer(port, registry)
	if err != nil {
		slog.Error("Failed to bind to port", "port", port, "error", err)
		return
	}
	s.Start()
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
