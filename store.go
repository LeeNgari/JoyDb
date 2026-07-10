package joydb

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/leengari/joydb/internal/domain/transaction"
	internalengine "github.com/leengari/joydb/internal/engine"
	storageengine "github.com/leengari/joydb/internal/storage/engine"
	"github.com/leengari/joydb/internal/storage/manager"
)

// Store owns a collection of isolated databases.
type Store struct {
	mu       sync.Mutex
	registry *manager.Registry
	dbs      map[string]*DB
	closed   bool
}

// Open opens or creates a persistent store. The special path :memory: creates a RAM-only store.
func Open(path string, options ...Option) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("joydb: store path is required")
	}
	cfg := defaultConfig()
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	if cfg.debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	var registry *manager.Registry
	if path == ":memory:" {
		registry = manager.NewRegistryWithWAL(path, storageengine.NewInMemoryEngine(), false, cfg.checkpointInterval, 0)
	} else {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, fmt.Errorf("joydb: create store: %w", err)
		}
		registry = manager.NewRegistryWithWAL(path, storageengine.NewMemoryEngine(), cfg.walEnabled, cfg.checkpointInterval, cfg.walSyncInterval)
	}
	return &Store{registry: registry, dbs: make(map[string]*DB)}, nil
}

// DB returns a cached database handle and creates the database on first access.
func (s *Store) DB(name string) (*DB, error) {
	if err := validateDatabaseName(name); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	if db := s.dbs[name]; db != nil {
		return db, nil
	}

	database, walManager, err := s.registry.GetWithWAL(name)
	if err != nil {
		names, listErr := s.registry.List()
		if listErr != nil {
			return nil, mapError(err)
		}
		exists := false
		for _, existing := range names {
			if existing == name {
				exists = true
				break
			}
		}
		if exists {
			return nil, mapError(err)
		}
		if err := s.registry.Create(name); err != nil {
			return nil, mapError(err)
		}
		database, walManager, err = s.registry.GetWithWAL(name)
		if err != nil {
			return nil, mapError(err)
		}
	}

	queryEngine := internalengine.New(database, s.registry)
	queryEngine.SetWALManager(walManager)
	db := &DB{name: name, engine: queryEngine}
	s.dbs[name] = db
	return db, nil
}

func (s *Store) ListDBs() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	names, err := s.registry.List()
	if err != nil {
		return nil, mapError(err)
	}
	sort.Strings(names)
	return names, nil
}

func (s *Store) DropDB(name string) error {
	if err := validateDatabaseName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if err := s.registry.Drop(name); err != nil {
		return mapError(err)
	}
	delete(s.dbs, name)
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	tx := transaction.NewTransaction()
	s.registry.SaveAll(tx)
	tx.Close()
	s.registry.CloseAll()
	s.closed = true
	return nil
}

func validateDatabaseName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\\`) {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	return nil
}
