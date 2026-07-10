package workloads

import (
	"fmt"
	"math/rand"

	"github.com/leengari/joydb/internal/benchmark"
	"github.com/leengari/joydb/internal/engine"
)

// UpdateByPK measures single-row update throughput
type UpdateByPK struct {
	rng *rand.Rand
}

func NewUpdateByPK() *UpdateByPK {
	return &UpdateByPK{rng: rand.New(rand.NewSource(42))}
}

func (w *UpdateByPK) Name() string { return "update_by_pk" }
func (w *UpdateByPK) Description() string { return "Update a single row by PK. Measures update + index rebuild cost." }
func (w *UpdateByPK) Tags() []string { return []string{"write", "update", "single-row"} }

func (w *UpdateByPK) Setup(eng *engine.Engine) error {
	return benchmark.SeedTable(eng, "test_update_pk", 10000, w.rng)
}

func (w *UpdateByPK) Run(eng *engine.Engine, iter int) error {
	id := benchmark.RandomInt(w.rng, 1, 10000)
	newAmt := benchmark.RandomFloat(w.rng)
	_, err := eng.Execute(fmt.Sprintf("UPDATE test_update_pk SET amount = %f WHERE id = %d", newAmt, id))
	return err
}

func (w *UpdateByPK) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE test_update_pk")
	return err
}

// UpdateBulk measures bulk update performance
type UpdateBulk struct {
	rng *rand.Rand
}

func NewUpdateBulk() *UpdateBulk {
	return &UpdateBulk{rng: rand.New(rand.NewSource(42))}
}

func (w *UpdateBulk) Name() string { return "update_bulk" }
func (w *UpdateBulk) Description() string { return "Update ~100 rows per iteration." }
func (w *UpdateBulk) Tags() []string { return []string{"write", "update", "bulk"} }

func (w *UpdateBulk) Setup(eng *engine.Engine) error {
	return benchmark.SeedTable(eng, "test_update_bulk", 10000, w.rng)
}

func (w *UpdateBulk) Run(eng *engine.Engine, iter int) error {
	// We'll update rows within a 100 ID range
	startID := benchmark.RandomInt(w.rng, 1, 9900)
	endID := startID + 100
	newAmt := benchmark.RandomFloat(w.rng)
	
	// JoyDB predicate might not handle range natively easily with current SQL,
	// so we can simulate a predicate that hits about 100 rows.
	// We seeded amount randomly, so we update where active = true (hits ~50%) 
	// then we just do a simpler range if we can.
	// Actually JoyDB supports id > X and id < Y. Let's use that.
	_, err := eng.Execute(fmt.Sprintf("UPDATE test_update_bulk SET amount = %f WHERE id > %d AND id < %d", newAmt, startID, endID))
	return err
}

func (w *UpdateBulk) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE test_update_bulk")
	return err
}
