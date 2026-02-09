package wal

import (
	"os"
	"testing"
)

// Helpers used by multiple test files

func createTempWAL(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "wal_test_*.wal")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	f.Close()
	os.Remove(name) // Let NewWAL create it
	return name
}

func removeTempWAL(t *testing.T, path string) {
	t.Helper()
	os.Remove(path)
}
