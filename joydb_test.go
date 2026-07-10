package joydb_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	joydb "github.com/leengari/joydb"
)

func TestInMemoryStoreCRUDAndScanning(t *testing.T) {
	store, err := joydb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	db, err := store.DB("app")
	if err != nil {
		t.Fatal(err)
	}
	sameDB, err := store.DB("app")
	if err != nil {
		t.Fatal(err)
	}
	if db != sameDB {
		t.Fatal("expected DB handles to be cached")
	}

	if _, err := db.Exec("CREATE TABLE users (id INT PRIMARY KEY AUTO_INCREMENT, name TEXT NOT NULL, age INT)"); err != nil {
		t.Fatal(err)
	}
	insert, err := db.Exec("INSERT INTO users (name, age) VALUES ('Alice', 30)")
	if err != nil {
		t.Fatal(err)
	}
	if insert.RowsAffected() != 1 {
		t.Fatalf("RowsAffected = %d, want 1", insert.RowsAffected())
	}
	if insert.LastInsertID() != 1 {
		t.Fatalf("LastInsertID = %d, want 1", insert.LastInsertID())
	}

	rows, err := db.Query("SELECT id, name, age FROM users")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal("expected one row")
	}
	var id int64
	var name string
	var age int
	if err := rows.Scan(&id, &name, &age); err != nil {
		t.Fatal(err)
	}
	if id != 1 || name != "Alice" || age != 30 {
		t.Fatalf("unexpected row: %d %q %d", id, name, age)
	}

	rows, err = db.Query("SELECT id, name, age FROM users")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal("expected one row")
	}
	var user struct {
		ID   int64  `joydb:"id"`
		Name string `joydb:"name"`
		Age  int    `joydb:"age"`
	}
	if err := rows.StructScan(&user); err != nil {
		t.Fatal(err)
	}
	if user.Name != "Alice" || rows.Int("id") != 1 || rows.String("name") != "Alice" {
		t.Fatalf("unexpected struct/accessors: %+v", user)
	}

	names, err := store.ListDBs()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"app"}) {
		t.Fatalf("ListDBs = %v", names)
	}
}

func TestInMemoryDatabasesAreIsolated(t *testing.T) {
	store, err := joydb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	first, err := store.DB("first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.DB("second")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Exec("CREATE TABLE items (id INT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Query("SELECT * FROM items"); !errors.Is(err, joydb.ErrTableNotFound) {
		t.Fatalf("expected ErrTableNotFound, got %v", err)
	}
}

func TestPersistentStoreReopensData(t *testing.T) {
	path := t.TempDir()
	store, err := joydb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.DB("app")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE items (id INT PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO items (id, name) VALUES (1, 'saved')"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = joydb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db, err = store.DB("app")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query("SELECT name FROM items WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.String("name") != "saved" {
		t.Fatal("persisted row was not recovered")
	}
}

func TestWithoutWALPersistsOnClose(t *testing.T) {
	path := t.TempDir()
	store, err := joydb.Open(path, joydb.WithoutWAL())
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.DB("app")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE items (id INT PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO items (id, name) VALUES (?, ?)", 1, "snapshot"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = joydb.Open(path, joydb.WithoutWAL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db, err = store.DB("app")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query("SELECT name FROM items WHERE id = ?", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.String("name") != "snapshot" {
		t.Fatal("no-WAL store did not persist on Close")
	}
}

func TestParameterizedQueriesBindTypedValues(t *testing.T) {
	store, err := joydb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db, err := store.DB("params")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE notes (id INT PRIMARY KEY, body TEXT, score FLOAT, active BOOL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO notes (id, body, score, active) VALUES (?, ?, ?, ?)", 1, "Who is this? O'Brien", 4.5, true); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query("SELECT body FROM notes WHERE body = ?", "Who is this? O'Brien")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.String("body") != "Who is this? O'Brien" {
		t.Fatal("quoted parameter did not round trip")
	}

	if _, err := db.Exec("INSERT INTO notes (id, body, score, active) VALUES (?, ?, ?, ?)", 2, "Who?", 1.0, false); err != nil {
		t.Fatal(err)
	}
	rows, err = db.Query("SELECT id FROM notes WHERE body = 'Who?' AND id = ?", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.Int("id") != 2 {
		t.Fatal("question mark inside literal was treated as a parameter")
	}

	if _, err := db.Exec("UPDATE notes SET body = ? WHERE id = ?", nil, 1); err != nil {
		t.Fatal(err)
	}
	rows, err = db.Query("SELECT body FROM notes WHERE id = ?", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || !rows.IsNull("body") {
		t.Fatal("NULL parameter was not stored")
	}
	if _, err := db.Exec("DELETE FROM notes WHERE id = ?", 1); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Query("SELECT * FROM notes WHERE id = ?"); err == nil || !strings.Contains(err.Error(), "missing value") {
		t.Fatalf("expected missing parameter error, got %v", err)
	}
	if _, err := db.Query("SELECT * FROM notes", 1); err == nil || !strings.Contains(err.Error(), "parameter count mismatch") {
		t.Fatalf("expected extra parameter error, got %v", err)
	}
	if _, err := db.Query("SELECT * FROM notes WHERE id = ?", struct{}{}); err == nil || !strings.Contains(err.Error(), "unsupported parameter type") {
		t.Fatalf("expected unsupported parameter error, got %v", err)
	}
}

func TestTransactionRollbackRestoresDDLAndDML(t *testing.T) {
	store, err := joydb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db, err := store.DB("tx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE users (id INT PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO users (id, name) VALUES (?, ?)", 1, "before"); err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("UPDATE users SET name = ? WHERE id = ?", "during", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("INSERT INTO users (id, name) VALUES (?, ?)", 2, "new"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("CREATE TABLE temporary (id INT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); !errors.Is(err, joydb.ErrTxDone) {
		t.Fatalf("second rollback = %v", err)
	}

	rows, err := db.Query("SELECT id, name FROM users ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.Int("id") != 1 || rows.String("name") != "before" {
		t.Fatal("rollback did not restore original row")
	}
	if rows.Next() {
		t.Fatal("rollback retained inserted row")
	}
	if _, err := db.Query("SELECT * FROM temporary"); !errors.Is(err, joydb.ErrTableNotFound) {
		t.Fatalf("rolled-back table still exists: %v", err)
	}
}

func TestTransactionCommitPersists(t *testing.T) {
	path := t.TempDir()
	store, err := joydb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.DB("tx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE users (id INT PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("INSERT INTO users (id, name) VALUES (?, ?)", 1, "committed"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); !errors.Is(err, joydb.ErrTxDone) {
		t.Fatalf("second commit = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = joydb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db, err = store.DB("tx")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query("SELECT name FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.String("name") != "committed" {
		t.Fatal("committed row was not recovered")
	}
}

func TestContextCancellation(t *testing.T) {
	store, err := joydb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db, err := store.DB("context")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := db.ExecContext(ctx, "CREATE TABLE users (id INT PRIMARY KEY)"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecContext = %v", err)
	}
	if _, err := db.BeginTx(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("BeginTx = %v", err)
	}
}
