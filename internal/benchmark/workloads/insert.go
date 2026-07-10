package workloads

import (
	"fmt"
	"math/rand"

	"github.com/leengari/mini-rdbms/internal/benchmark"
	"github.com/leengari/mini-rdbms/internal/engine"
)

// InsertSingleRow measures the throughput of single-row inserts
type InsertSingleRow struct {
	rng *rand.Rand
}

func NewInsertSingleRow() *InsertSingleRow {
	return &InsertSingleRow{
		rng: rand.New(rand.NewSource(42)),
	}
}

func (w *InsertSingleRow) Name() string {
	return "insert_single_row"
}

func (w *InsertSingleRow) Description() string {
	return "Inserts a single row per iteration. Measures raw insert throughput including validation, B+Tree insert, and index updates."
}

func (w *InsertSingleRow) Tags() []string {
	return []string{"write", "insert", "single-row"}
}

func (w *InsertSingleRow) Setup(eng *engine.Engine) error {
	createSQL := `CREATE TABLE test_insert (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name TEXT,
		amount FLOAT,
		active BOOL
	)`
	_, err := eng.Execute(createSQL)
	return err
}

func (w *InsertSingleRow) Run(eng *engine.Engine, iter int) error {
	name := benchmark.RandomText(w.rng, 10)
	amount := benchmark.RandomFloat(w.rng)
	active := benchmark.RandomBool(w.rng)

	insertSQL := fmt.Sprintf("INSERT INTO test_insert (name, amount, active) VALUES ('%s', %f, %t)",
		name, amount, active)
	
	_, err := eng.Execute(insertSQL)
	return err
}

func (w *InsertSingleRow) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE test_insert")
	return err
}

// InsertBulk100 measures amortized insert cost by inserting 100 rows per iteration
type InsertBulk100 struct {
	rng *rand.Rand
}

func NewInsertBulk100() *InsertBulk100 {
	return &InsertBulk100{
		rng: rand.New(rand.NewSource(42)),
	}
}

func (w *InsertBulk100) Name() string {
	return "insert_bulk_100"
}

func (w *InsertBulk100) Description() string {
	return "Inserts 100 rows sequentially per iteration. Measures amortized insert cost."
}

func (w *InsertBulk100) Tags() []string {
	return []string{"write", "insert", "bulk"}
}

func (w *InsertBulk100) Setup(eng *engine.Engine) error {
	createSQL := `CREATE TABLE test_bulk_insert (
		id INT PRIMARY KEY AUTO_INCREMENT,
		val TEXT
	)`
	_, err := eng.Execute(createSQL)
	return err
}

func (w *InsertBulk100) Run(eng *engine.Engine, iter int) error {
	for i := 0; i < 100; i++ {
		val := benchmark.RandomText(w.rng, 8)
		_, err := eng.Execute(fmt.Sprintf("INSERT INTO test_bulk_insert (val) VALUES ('%s')", val))
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *InsertBulk100) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE test_bulk_insert")
	return err
}

// InsertWithFK measures insert overhead when validating foreign keys
type InsertWithFK struct {
	rng *rand.Rand
}

func NewInsertWithFK() *InsertWithFK {
	return &InsertWithFK{
		rng: rand.New(rand.NewSource(42)),
	}
}

func (w *InsertWithFK) Name() string {
	return "insert_with_fk"
}

func (w *InsertWithFK) Description() string {
	return "Inserts into a child table referencing a parent table. Measures FK validation overhead."
}

func (w *InsertWithFK) Tags() []string {
	return []string{"write", "insert", "fk"}
}

func (w *InsertWithFK) Setup(eng *engine.Engine) error {
	// Create parent table
	_, err := eng.Execute(`CREATE TABLE parent (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name TEXT
	)`)
	if err != nil {
		return err
	}

	// Create child table with FK
	_, err = eng.Execute(`CREATE TABLE child (
		id INT PRIMARY KEY AUTO_INCREMENT,
		parent_id INT,
		FOREIGN KEY (parent_id) REFERENCES parent(id)
	)`)
	if err != nil {
		return err
	}

	// Seed parent table with 10K rows
	for i := 0; i < 10000; i++ {
		_, err = eng.Execute("INSERT INTO parent (name) VALUES ('parent_row')")
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *InsertWithFK) Run(eng *engine.Engine, iter int) error {
	// Pick a random valid parent_id
	parentID := benchmark.RandomInt(w.rng, 1, 10000)
	
	_, err := eng.Execute(fmt.Sprintf("INSERT INTO child (parent_id) VALUES (%d)", parentID))
	return err
}

func (w *InsertWithFK) Teardown(eng *engine.Engine) error {
	eng.Execute("DROP TABLE child")
	eng.Execute("DROP TABLE parent")
	return nil
}
