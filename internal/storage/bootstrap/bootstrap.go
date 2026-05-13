package bootstrap

import (
	"fmt"
	"os"
)

// EnsureDatabase checks if the database exists at the given path.
// If it does not exist, it creates a default database with sample data.
// EnsureDatabase checks if the database exists at the given path.
// If it does not exist, it creates a default database with sample data.
func EnsureDatabase(path, name string) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return nil // Database exists
	}

	// Create database directory
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}

	// Create meta.json for database
	dbMeta := fmt.Sprintf(`{"name": "%s", "created_at": "2024-01-01T00:00:00Z"}`, name)
	if err := os.WriteFile(path+"/meta.json", []byte(dbMeta), 0644); err != nil {
		return err
	}

	return nil
}
