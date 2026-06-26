package benchmark

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/engine"
	storageEngine "github.com/leengari/mini-rdbms/internal/storage/engine"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

// EngineOptions configures the creation of a benchmark database engine
type EngineOptions struct {
	WALEnabled      bool
	WALSyncInterval time.Duration
	BasePath        string // If empty, a temporary directory is created
	DBName          string
}

// DefaultEngineOptions returns standard options for in-memory pure engine benchmarks
func DefaultEngineOptions() EngineOptions {
	return EngineOptions{
		WALEnabled:      false,
		WALSyncInterval: 0,
		BasePath:        "", // Will use a temp dir
		DBName:          "benchdb",
	}
}

// NewBenchEngine creates a fresh JoyDB engine configured for benchmarking
func NewBenchEngine(opts EngineOptions) (*engine.Engine, string, error) {
	basePath := opts.BasePath
	if basePath == "" {
		var err error
		basePath, err = os.MkdirTemp("", "joydb-bench-*")
		if err != nil {
			return nil, "", fmt.Errorf("failed to create temp dir: %w", err)
		}
	}

	storageEng := storageEngine.NewMemoryEngine()

	// Create database using the storage engine first
	if err := storageEng.CreateDatabase(opts.DBName, basePath); err != nil {
		return nil, basePath, fmt.Errorf("failed to create database: %w", err)
	}

	// Create a dummy database object for the engine initialization
	db := &schema.Database{
		Name:   opts.DBName,
		Path:   basePath + "/" + opts.DBName,
		Tables: make(map[string]*schema.Table),
	}

	registry := manager.NewRegistryWithWAL(
		basePath,
		storageEng,
		opts.WALEnabled,
		5*time.Second,
		opts.WALSyncInterval,
	)

	// In memory engine, we just create the engine and set up the WAL manager manually
	// to avoid relying heavily on USE command internals for setup
	eng := engine.New(db, registry)

	// Since we are mocking the db setup, we need to explicitly initialize the WAL manager
	// by grabbing it from the registry
	if opts.WALEnabled {
		// Use GetWithWAL to ensure WALManager is created
		_, walMgr, err := registry.GetWithWAL(opts.DBName)
		if err == nil && walMgr != nil {
			eng.SetWALManager(walMgr)
		}
	}

	return eng, basePath, nil
}

// DataGenerators provides reproducible random data

// RandomInt returns a random integer between min and max
func RandomInt(rng *rand.Rand, min, max int) int {
	return rng.Intn(max-min+1) + min
}

// RandomFloat returns a random float64
func RandomFloat(rng *rand.Rand) float64 {
	return rng.Float64() * 1000.0
}

// RandomBool returns a random boolean
func RandomBool(rng *rand.Rand) bool {
	return rng.Intn(2) == 1
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// RandomText returns a random string of length n
func RandomText(rng *rand.Rand, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rng.Intn(len(letterBytes))]
	}
	return string(b)
}

// SeedTable creates a table and fills it with rowCount random rows.
// Assumes table schema format: id (PK, AutoIncrement), name (TEXT), amount (FLOAT), active (BOOL)
func SeedTable(eng *engine.Engine, tableName string, rowCount int, rng *rand.Rand) error {
	createSQL := fmt.Sprintf(`CREATE TABLE %s (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name TEXT,
		amount FLOAT,
		active BOOL
	)`, tableName)

	if _, err := eng.Execute(createSQL); err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	// For bulk seeding, it's faster to generate multiple insert statements
	// but the parser only supports single statements per Execute right now.
	for i := 0; i < rowCount; i++ {
		name := RandomText(rng, 10)
		amount := RandomFloat(rng)
		active := RandomBool(rng)

		insertSQL := fmt.Sprintf("INSERT INTO %s (name, amount, active) VALUES ('%s', %f, %t)",
			tableName, name, amount, active)

		if _, err := eng.Execute(insertSQL); err != nil {
			return fmt.Errorf("failed to insert row %d: %w", i, err)
		}
	}

	return nil
}
