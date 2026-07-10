package integration

import (
	"reflect"
	"strings"
	"testing"

	joydb "github.com/leengari/joydb"
)

func openGroupByDB(t *testing.T) *joydb.DB {
	t.Helper()
	store, err := joydb.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db, err := store.DB("group_by")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestGroupByAggregatesOrderAndPagination(t *testing.T) {
	db := openGroupByDB(t)
	statements := []string{
		"CREATE TABLE employees (id INT PRIMARY KEY, department TEXT, role TEXT, salary FLOAT)",
		"INSERT INTO employees (id, department, role, salary) VALUES (1, 'engineering', 'developer', 100.0)",
		"INSERT INTO employees (id, department, role, salary) VALUES (2, 'engineering', 'developer', 120.0)",
		"INSERT INTO employees (id, department, role, salary) VALUES (3, 'engineering', 'manager', 150.0)",
		"INSERT INTO employees (id, department, role, salary) VALUES (4, 'sales', 'seller', 80.0)",
		"INSERT INTO employees (id, department, role, salary) VALUES (5, 'sales', 'seller', NULL)",
		"INSERT INTO employees (id, department, role, salary) VALUES (6, 'support', 'agent', 60.0)",
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}

	rows, err := db.Query("SELECT department, COUNT(*) AS total, COUNT(salary) AS paid, SUM(salary) AS payroll, AVG(salary) AS average, MIN(salary) AS minimum, MAX(salary) AS maximum FROM employees GROUP BY department ORDER BY total DESC, department ASC LIMIT 2")
	if err != nil {
		t.Fatal(err)
	}
	if got := rows.Columns(); !reflect.DeepEqual(got, []string{"department", "total", "paid", "payroll", "average", "minimum", "maximum"}) {
		t.Fatalf("columns = %v", got)
	}
	if !rows.Next() {
		t.Fatal("expected engineering group")
	}
	if rows.String("department") != "engineering" || rows.Int("total") != 3 || rows.Int("paid") != 3 || rows.Float("payroll") != 370 || rows.Float("average") != 370.0/3.0 || rows.Float("minimum") != 100 || rows.Float("maximum") != 150 {
		t.Fatal("unexpected engineering aggregate values")
	}
	if !rows.Next() {
		t.Fatal("expected sales group")
	}
	if rows.String("department") != "sales" || rows.Int("total") != 2 || rows.Int("paid") != 1 || rows.Float("payroll") != 80 {
		t.Fatal("unexpected sales aggregate values")
	}
	if rows.Next() {
		t.Fatal("LIMIT did not apply to grouped rows")
	}

	rows, err = db.Query("SELECT department, role, COUNT(*) AS total FROM employees GROUP BY department, role ORDER BY department, role LIMIT 1 OFFSET 1")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.String("department") != "engineering" || rows.String("role") != "manager" || rows.Int("total") != 1 {
		t.Fatal("unexpected multi-column group after OFFSET")
	}
}

func TestGroupByNullsDistinctAndEmptyInput(t *testing.T) {
	db := openGroupByDB(t)
	statements := []string{
		"CREATE TABLE items (id INT PRIMARY KEY, category TEXT)",
		"INSERT INTO items (id, category) VALUES (1, NULL)",
		"INSERT INTO items (id, category) VALUES (2, NULL)",
		"INSERT INTO items (id, category) VALUES (3, 'book')",
		"INSERT INTO items (id, category) VALUES (4, 'book')",
		"INSERT INTO items (id, category) VALUES (5, 'game')",
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := db.Query("SELECT category, COUNT(*) AS total FROM items GROUP BY category ORDER BY total DESC")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for rows.Next() {
		key := rows.String("category")
		if rows.IsNull("category") {
			key = "<null>"
		}
		counts[key] = rows.Int("total")
	}
	if !reflect.DeepEqual(counts, map[string]int64{"<null>": 2, "book": 2, "game": 1}) {
		t.Fatalf("unexpected NULL grouping: %v", counts)
	}

	rows, err = db.Query("SELECT category FROM items GROUP BY category")
	if err != nil {
		t.Fatal(err)
	}
	var distinctCount int
	for rows.Next() {
		distinctCount++
	}
	if distinctCount != 3 {
		t.Fatalf("GROUP BY without aggregates returned %d rows", distinctCount)
	}

	rows, err = db.Query("SELECT category, COUNT(*) AS total FROM items WHERE id < 0 GROUP BY category")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() {
		t.Fatal("grouped empty input should return no rows")
	}
	rows, err = db.Query("SELECT COUNT(*) AS total FROM items WHERE id < 0 LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.Int("total") != 0 {
		t.Fatal("ungrouped empty COUNT should return zero")
	}
}

func TestGroupByValidationAndJoin(t *testing.T) {
	db := openGroupByDB(t)
	statements := []string{
		"CREATE TABLE departments (id INT PRIMARY KEY, name TEXT)",
		"CREATE TABLE employees (id INT PRIMARY KEY, department_id INT, name TEXT)",
		"INSERT INTO departments (id, name) VALUES (1, 'engineering')",
		"INSERT INTO departments (id, name) VALUES (2, 'sales')",
		"INSERT INTO employees (id, department_id, name) VALUES (1, 1, 'Ada')",
		"INSERT INTO employees (id, department_id, name) VALUES (2, 1, 'Lin')",
		"INSERT INTO employees (id, department_id, name) VALUES (3, 2, 'Sam')",
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := db.Query("SELECT d.name, COUNT(*) AS total FROM employees AS e INNER JOIN departments AS d ON e.department_id = d.id GROUP BY d.name ORDER BY total DESC")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.String("name") != "engineering" || rows.Int("total") != 2 {
		t.Fatal("unexpected joined engineering group")
	}
	if !rows.Next() || rows.String("name") != "sales" || rows.Int("total") != 1 {
		t.Fatal("unexpected joined sales group")
	}

	invalidQueries := []string{
		"SELECT name, COUNT(*) FROM employees GROUP BY department_id",
		"SELECT name, COUNT(*) FROM employees",
		"SELECT * FROM employees GROUP BY department_id",
		"SELECT missing, COUNT(*) FROM employees GROUP BY missing",
	}
	for _, query := range invalidQueries {
		if _, err := db.Query(query); err == nil {
			t.Errorf("expected query to fail: %s", query)
		} else if strings.TrimSpace(err.Error()) == "" {
			t.Errorf("query returned an empty error: %s", query)
		}
	}
}
