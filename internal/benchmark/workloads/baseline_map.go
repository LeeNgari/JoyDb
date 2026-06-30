package workloads

import (
	"sync"


	"github.com/leengari/mini-rdbms/internal/domain/data"
	"github.com/leengari/mini-rdbms/internal/engine"
)

// BaselineMap tests the absolute maximum throughput of a simple Go map protected by a Mutex.
// This serves as the theoretical speed limit for JoyDb when running purely in-memory.
type BaselineMap struct {
	mu   sync.RWMutex
	data map[int64]data.Row
}

func (w *BaselineMap) Name() string {
	return "baseline_map_mutex"
}

func (w *BaselineMap) Description() string {
	return "Theoretical maximum throughput of a simple Go map protected by a RWMutex"
}

func (w *BaselineMap) Tags() []string {
	return []string{"baseline", "write"}
}

func (w *BaselineMap) Setup(eng *engine.Engine) error {
	w.data = make(map[int64]data.Row)
	return nil
}

func (w *BaselineMap) Teardown(eng *engine.Engine) error {
	w.data = nil
	return nil
}

func (w *BaselineMap) Run(eng *engine.Engine, iteration int) error {
	id := int64(iteration)
	row := data.Row{
		Values:  []interface{}{id, "Alice", int64(30), true},
		RID:     id,
		Deleted: false,
	}

	w.mu.Lock()
	w.data[id] = row
	w.mu.Unlock()

	return nil
}

func NewBaselineMap() *BaselineMap {
	return &BaselineMap{}
}
