package workloads

import (
	"fmt"
	"math/rand"

	"github.com/leengari/joydb/internal/benchmark"
	"github.com/leengari/joydb/internal/engine"
)

// DeleteByPK measures single-row delete throughput
type DeleteByPK struct {
	rng     *rand.Rand
	currMax int // keeps track of max ID inserted
}

func NewDeleteByPK() *DeleteByPK {
	return &DeleteByPK{
		rng:     rand.New(rand.NewSource(42)),
		currMax: 10000,
	}
}

func (w *DeleteByPK) Name() string { return "delete_by_pk" }
func (w *DeleteByPK) Description() string { return "Delete a single row by PK. Measures delete + array compaction + index rebuild." }
func (w *DeleteByPK) Tags() []string { return []string{"write", "delete", "single-row"} }

func (w *DeleteByPK) Setup(eng *engine.Engine) error {
	w.currMax = 10000
	return benchmark.SeedTable(eng, "test_delete_pk", 10000, w.rng)
}

func (w *DeleteByPK) Run(eng *engine.Engine, iter int) error {
	if w.currMax <= 100 {
		if err := w.Teardown(eng); err != nil {
			return err
		}
		if err := w.Setup(eng); err != nil {
			return err
		}
	}

	idToDelete := w.currMax
	w.currMax--

	_, err := eng.Execute(fmt.Sprintf("DELETE FROM test_delete_pk WHERE id = %d", idToDelete))
	return err
}

func (w *DeleteByPK) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE test_delete_pk")
	return err
}

// DeleteBulk measures bulk delete performance
type DeleteBulk struct {
	rng     *rand.Rand
	currMax int
}

func NewDeleteBulk() *DeleteBulk {
	return &DeleteBulk{
		rng:     rand.New(rand.NewSource(42)),
		currMax: 10000,
	}
}

func (w *DeleteBulk) Name() string { return "delete_bulk" }
func (w *DeleteBulk) Description() string { return "Delete 10 rows per iteration." }
func (w *DeleteBulk) Tags() []string { return []string{"write", "delete", "bulk"} }

func (w *DeleteBulk) Setup(eng *engine.Engine) error {
	w.currMax = 10000
	return benchmark.SeedTable(eng, "test_delete_bulk", 10000, w.rng)
}

func (w *DeleteBulk) Run(eng *engine.Engine, iter int) error {
	if w.currMax <= 100 {
		if err := w.Teardown(eng); err != nil {
			return err
		}
		if err := w.Setup(eng); err != nil {
			return err
		}
	}

	startID := w.currMax - 10
	endID := w.currMax
	w.currMax -= 10

	_, err := eng.Execute(fmt.Sprintf("DELETE FROM test_delete_bulk WHERE id > %d AND id <= %d", startID, endID))
	return err
}

func (w *DeleteBulk) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE test_delete_bulk")
	return err
}
