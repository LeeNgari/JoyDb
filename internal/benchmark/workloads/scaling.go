package workloads

import (
	"fmt"
	"math/rand"

	"github.com/leengari/mini-rdbms/internal/benchmark"
	"github.com/leengari/mini-rdbms/internal/engine"
)

// InsertScaling measures B+Tree degradation at different sizes
type InsertScaling struct {
	size int
	rng  *rand.Rand
}

func NewInsertScaling(size int) *InsertScaling {
	return &InsertScaling{size: size, rng: rand.New(rand.NewSource(42))}
}

func (w *InsertScaling) Name() string { return fmt.Sprintf("insert_scaling_%d", w.size) }
func (w *InsertScaling) Description() string { return fmt.Sprintf("Insert into %d row table. Measures O(log n) scaling.", w.size) }
func (w *InsertScaling) Tags() []string { return []string{"scaling", "insert"} }

func (w *InsertScaling) Setup(eng *engine.Engine) error {
	tableName := fmt.Sprintf("test_insert_%d", w.size)
	return benchmark.SeedTable(eng, tableName, w.size, w.rng)
}

func (w *InsertScaling) Run(eng *engine.Engine, iter int) error {
	tableName := fmt.Sprintf("test_insert_%d", w.size)
	name := benchmark.RandomText(w.rng, 10)
	amount := benchmark.RandomFloat(w.rng)
	active := benchmark.RandomBool(w.rng)

	insertSQL := fmt.Sprintf("INSERT INTO %s (name, amount, active) VALUES ('%s', %f, %t)",
		tableName, name, amount, active)
	
	_, err := eng.Execute(insertSQL)
	return err
}

func (w *InsertScaling) Teardown(eng *engine.Engine) error {
	tableName := fmt.Sprintf("test_insert_%d", w.size)
	_, err := eng.Execute(fmt.Sprintf("DROP TABLE %s", tableName))
	return err
}

// SelectScanScaling measures linear degradation of seq scan
type SelectScanScaling struct {
	size int
	rng  *rand.Rand
}

func NewSelectScanScaling(size int) *SelectScanScaling {
	return &SelectScanScaling{size: size, rng: rand.New(rand.NewSource(42))}
}

func (w *SelectScanScaling) Name() string { return fmt.Sprintf("select_scan_scaling_%d", w.size) }
func (w *SelectScanScaling) Description() string { return fmt.Sprintf("Sequential scan of %d rows. Measures O(n) scaling.", w.size) }
func (w *SelectScanScaling) Tags() []string { return []string{"scaling", "select", "scan"} }

func (w *SelectScanScaling) Setup(eng *engine.Engine) error {
	tableName := fmt.Sprintf("test_scan_%d", w.size)
	return benchmark.SeedTable(eng, tableName, w.size, w.rng)
}

func (w *SelectScanScaling) Run(eng *engine.Engine, iter int) error {
	tableName := fmt.Sprintf("test_scan_%d", w.size)
	_, err := eng.Execute(fmt.Sprintf("SELECT * FROM %s", tableName))
	return err
}

func (w *SelectScanScaling) Teardown(eng *engine.Engine) error {
	tableName := fmt.Sprintf("test_scan_%d", w.size)
	_, err := eng.Execute(fmt.Sprintf("DROP TABLE %s", tableName))
	return err
}

// SelectPKScaling measures O(log n) scaling of B+Tree search
type SelectPKScaling struct {
	size int
	rng  *rand.Rand
}

func NewSelectPKScaling(size int) *SelectPKScaling {
	return &SelectPKScaling{size: size, rng: rand.New(rand.NewSource(42))}
}

func (w *SelectPKScaling) Name() string { return fmt.Sprintf("select_pk_scaling_%d", w.size) }
func (w *SelectPKScaling) Description() string { return fmt.Sprintf("PK lookup in %d rows. Measures O(log n) scaling.", w.size) }
func (w *SelectPKScaling) Tags() []string { return []string{"scaling", "select", "pk"} }

func (w *SelectPKScaling) Setup(eng *engine.Engine) error {
	tableName := fmt.Sprintf("test_pk_%d", w.size)
	return benchmark.SeedTable(eng, tableName, w.size, w.rng)
}

func (w *SelectPKScaling) Run(eng *engine.Engine, iter int) error {
	tableName := fmt.Sprintf("test_pk_%d", w.size)
	id := benchmark.RandomInt(w.rng, 1, w.size)
	_, err := eng.Execute(fmt.Sprintf("SELECT * FROM %s WHERE id = %d", tableName, id))
	return err
}

func (w *SelectPKScaling) Teardown(eng *engine.Engine) error {
	tableName := fmt.Sprintf("test_pk_%d", w.size)
	_, err := eng.Execute(fmt.Sprintf("DROP TABLE %s", tableName))
	return err
}
