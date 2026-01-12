package bootstrap

import (
	"os"
)

// EnsureDatabase checks if the database exists at the given path.
// If it does not exist, it creates a default database with sample data.
func EnsureDatabase(path string) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return nil // Database exists
	}

	// Create database directory
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}

	// Create meta.json for database
	dbMeta := `{"name": "testdb", "created_at": "2024-01-01T00:00:00Z"}`
	if err := os.WriteFile(path+"/meta.json", []byte(dbMeta), 0644); err != nil {
		return err
	}

	// Create users table
	usersPath := path + "/users"
	if err := os.MkdirAll(usersPath, 0755); err != nil {
		return err
	}

	usersSchema := `{
  "name": "users",
  "columns": [
    {"name": "id", "type": "INT", "primary_key": true, "unique": true, "not_null": true, "auto_increment": true},
    {"name": "username", "type": "TEXT", "primary_key": false, "unique": true, "not_null": true},
    {"name": "email", "type": "TEXT", "primary_key": false, "unique": true, "not_null": true},
    {"name": "is_active", "type": "BOOL", "primary_key": false, "unique": false, "not_null": false}
  ],
  "last_insert_id": 2,
  "row_count": 2
}`
	if err := os.WriteFile(usersPath+"/meta.json", []byte(usersSchema), 0644); err != nil {
		return err
	}

	usersData := `[
  {"id": 1, "username": "admin", "email": "admin@example.com", "is_active": true},
  {"id": 2, "username": "guest", "email": "guest@example.com", "is_active": false}
]`
	if err := os.WriteFile(usersPath+"/data.json", []byte(usersData), 0644); err != nil {
		return err
	}

	return nil
}
