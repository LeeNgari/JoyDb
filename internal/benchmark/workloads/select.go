package workloads

import (
	"fmt"
	"math/rand"

	"github.com/leengari/mini-rdbms/internal/benchmark"
	"github.com/leengari/mini-rdbms/internal/engine"
)

// SelectPKLookup measures the latency of a point lookup
type SelectPKLookup struct {
	rng *rand.Rand
}

func NewSelectPKLookup() *SelectPKLookup {
	return &SelectPKLookup{rng: rand.New(rand.NewSource(42))}
}

func (w *SelectPKLookup) Name() string { return "select_pk_lookup" }
func (w *SelectPKLookup) Description() string { return "Point lookup by primary key on a 10K-row table. Exercises B+Tree Search()." }
func (w *SelectPKLookup) Tags() []string { return []string{"read", "select", "index", "pk"} }

func (w *SelectPKLookup) Setup(eng *engine.Engine) error {
	return benchmark.SeedTable(eng, "test_select_pk", 10000, w.rng)
}

func (w *SelectPKLookup) Run(eng *engine.Engine, iter int) error {
	// Query random ID between 1 and 10000
	id := benchmark.RandomInt(w.rng, 1, 10000)
	_, err := eng.Execute(fmt.Sprintf("SELECT * FROM test_select_pk WHERE id = %d", id))
	return err
}

func (w *SelectPKLookup) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE test_select_pk")
	return err
}

// SelectFullScan1K measures sequential scan on 1K rows
type SelectFullScan1K struct {
	rng *rand.Rand
}

func NewSelectFullScan1K() *SelectFullScan1K { return &SelectFullScan1K{rng: rand.New(rand.NewSource(42))} }
func (w *SelectFullScan1K) Name() string { return "select_full_scan_1k" }
func (w *SelectFullScan1K) Description() string { return "SELECT * on 1K rows. Measures scan throughput." }
func (w *SelectFullScan1K) Tags() []string { return []string{"read", "select", "scan"} }
func (w *SelectFullScan1K) Setup(eng *engine.Engine) error { return benchmark.SeedTable(eng, "scan_1k", 1000, w.rng) }
func (w *SelectFullScan1K) Run(eng *engine.Engine, iter int) error {
	_, err := eng.Execute("SELECT * FROM scan_1k")
	return err
}
func (w *SelectFullScan1K) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE scan_1k")
	return err
}

// SelectFullScan10K measures sequential scan on 10K rows
type SelectFullScan10K struct {
	rng *rand.Rand
}

func NewSelectFullScan10K() *SelectFullScan10K { return &SelectFullScan10K{rng: rand.New(rand.NewSource(42))} }
func (w *SelectFullScan10K) Name() string { return "select_full_scan_10k" }
func (w *SelectFullScan10K) Description() string { return "SELECT * on 10K rows." }
func (w *SelectFullScan10K) Tags() []string { return []string{"read", "select", "scan"} }
func (w *SelectFullScan10K) Setup(eng *engine.Engine) error { return benchmark.SeedTable(eng, "scan_10k", 10000, w.rng) }
func (w *SelectFullScan10K) Run(eng *engine.Engine, iter int) error {
	_, err := eng.Execute("SELECT * FROM scan_10k")
	return err
}
func (w *SelectFullScan10K) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE scan_10k")
	return err
}

// SelectWithPredicate measures predicate evaluation
type SelectWithPredicate struct {
	rng *rand.Rand
}

func NewSelectWithPredicate() *SelectWithPredicate { return &SelectWithPredicate{rng: rand.New(rand.NewSource(42))} }
func (w *SelectWithPredicate) Name() string { return "select_with_predicate" }
func (w *SelectWithPredicate) Description() string { return "SELECT * WHERE amount > 500.0 on 10K rows. Measures filtering overhead." }
func (w *SelectWithPredicate) Tags() []string { return []string{"read", "select", "filter"} }
func (w *SelectWithPredicate) Setup(eng *engine.Engine) error { return benchmark.SeedTable(eng, "select_pred", 10000, w.rng) }
func (w *SelectWithPredicate) Run(eng *engine.Engine, iter int) error {
	_, err := eng.Execute("SELECT * FROM select_pred WHERE amount > 500.0")
	return err
}
func (w *SelectWithPredicate) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE select_pred")
	return err
}

// SelectWithProjection measures projection overhead
type SelectWithProjection struct {
	rng *rand.Rand
}

func NewSelectWithProjection() *SelectWithProjection { return &SelectWithProjection{rng: rand.New(rand.NewSource(42))} }
func (w *SelectWithProjection) Name() string { return "select_with_projection" }
func (w *SelectWithProjection) Description() string { return "SELECT id, name FROM t on 10K rows. Measures projection overhead." }
func (w *SelectWithProjection) Tags() []string { return []string{"read", "select", "projection"} }
func (w *SelectWithProjection) Setup(eng *engine.Engine) error { return benchmark.SeedTable(eng, "select_proj", 10000, w.rng) }
func (w *SelectWithProjection) Run(eng *engine.Engine, iter int) error {
	_, err := eng.Execute("SELECT id, name FROM select_proj")
	return err
}
func (w *SelectWithProjection) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE select_proj")
	return err
}
