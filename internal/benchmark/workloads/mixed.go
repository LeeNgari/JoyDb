package workloads

import (
	"fmt"
	"math/rand"

	"github.com/leengari/joydb/internal/benchmark"
	"github.com/leengari/joydb/internal/engine"
)

// MixedReadWrite simulates an OLTP-like workload with a specific read/write ratio
type MixedReadWrite struct {
	name        string
	readPercent int
	rng         *rand.Rand
	currMax     int
}

func NewMixedReadWrite80_20() *MixedReadWrite {
	return &MixedReadWrite{
		name:        "mixed_read_write_80_20",
		readPercent: 80,
		rng:         rand.New(rand.NewSource(42)),
	}
}

func NewMixedReadWrite50_50() *MixedReadWrite {
	return &MixedReadWrite{
		name:        "mixed_read_write_50_50",
		readPercent: 50,
		rng:         rand.New(rand.NewSource(42)),
	}
}

func (w *MixedReadWrite) Name() string { return w.name }
func (w *MixedReadWrite) Description() string {
	return fmt.Sprintf("%d%% reads (PK lookup) / %d%% writes (insert). Simulates OLTP pattern.", w.readPercent, 100-w.readPercent)
}
func (w *MixedReadWrite) Tags() []string { return []string{"mixed", "oltp"} }

func (w *MixedReadWrite) Setup(eng *engine.Engine) error {
	w.currMax = 10000
	return benchmark.SeedTable(eng, "test_mixed", 10000, w.rng)
}

func (w *MixedReadWrite) Run(eng *engine.Engine, iter int) error {
	// Randomly decide whether to read or write based on configured percentage
	isRead := benchmark.RandomInt(w.rng, 1, 100) <= w.readPercent

	if isRead {
		// Read: Random PK lookup
		id := benchmark.RandomInt(w.rng, 1, w.currMax)
		_, err := eng.Execute(fmt.Sprintf("SELECT * FROM test_mixed WHERE id = %d", id))
		return err
	}

	// Write: Insert a new row
	name := benchmark.RandomText(w.rng, 10)
	amount := benchmark.RandomFloat(w.rng)
	active := benchmark.RandomBool(w.rng)

	insertSQL := fmt.Sprintf("INSERT INTO test_mixed (name, amount, active) VALUES ('%s', %f, %t)",
		name, amount, active)
	
	_, err := eng.Execute(insertSQL)
	if err == nil {
		w.currMax++
	}
	return err
}

func (w *MixedReadWrite) Teardown(eng *engine.Engine) error {
	_, err := eng.Execute("DROP TABLE test_mixed")
	return err
}
