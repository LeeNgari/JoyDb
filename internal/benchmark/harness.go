package benchmark

import (
	"math"
	"runtime"
	"runtime/debug"
	"sort"
	"time"
)

// RunOptions configures how a workload is executed
type RunOptions struct {
	WarmupIterations int           // Iterations to run before measuring
	Iterations       int           // Measured iterations (0 = auto-calibrate or use Duration)
	Duration         time.Duration // Run for this duration (if Iterations == 0)
	Concurrency      int           // No-op scaffold (default: 1). Activated in Phase 5
	ResetBetweenRuns bool          // Re-seed the database between runs (for destructive ops like DELETE)
}

// DefaultRunOptions returns reasonable defaults for a quick benchmark
func DefaultRunOptions() RunOptions {
	return RunOptions{
		WarmupIterations: 10,
		Iterations:       0, // Will use duration
		Duration:         3 * time.Second,
		Concurrency:      1, // No-op scaffold
		ResetBetweenRuns: false,
	}
}

// Stats holds computed latency and throughput statistics
type Stats struct {
	P50    time.Duration
	P90    time.Duration
	P95    time.Duration
	P99    time.Duration
	Mean   time.Duration
	StdDev time.Duration
	TPS    float64 // Transactions Per Second (ops/sec)
}

// WorkloadResult contains the raw and computed results of a benchmark run
type WorkloadResult struct {
	Name       string
	Latencies  []time.Duration
	TotalTime  time.Duration
	Iterations int
	Stats      Stats
	Errors     int
	Metadata   map[string]interface{}
}

// BenchmarkHarness manages the execution and measurement of workloads
type BenchmarkHarness struct {
	engOpts EngineOptions
}

// NewHarness creates a new benchmark harness
func NewHarness(opts EngineOptions) *BenchmarkHarness {
	return &BenchmarkHarness{
		engOpts: opts,
	}
}

// GetEngineOpts returns the engine options configured for this harness
func (h *BenchmarkHarness) GetEngineOpts() EngineOptions {
	return h.engOpts
}

// Run executes a workload and measures its performance
func (h *BenchmarkHarness) Run(w Workload, opts RunOptions) *WorkloadResult {
	// 1. Setup Engine
	eng, _, err := NewBenchEngine(h.engOpts)
	if err != nil {
		return &WorkloadResult{Name: w.Name(), Errors: 1, Metadata: map[string]interface{}{"error": err.Error()}}
	}

	// 2. Setup Workload
	if err := w.Setup(eng); err != nil {
		return &WorkloadResult{Name: w.Name(), Errors: 1, Metadata: map[string]interface{}{"error": err.Error()}}
	}
	defer w.Teardown(eng)

	// 3. Warmup
	for i := 0; i < opts.WarmupIterations; i++ {
		w.Run(eng, i)
	}

	// 4. Measure
	// Determine mode: Fixed Iterations vs Fixed Duration
	useDuration := opts.Iterations <= 0
	targetDuration := opts.Duration
	if targetDuration <= 0 {
		targetDuration = 3 * time.Second // fallback
	}

	// Pre-allocate slice capacity to eliminate hot-path allocation overhead
	capacity := 100000
	if !useDuration && opts.Iterations > 0 {
		capacity = opts.Iterations
	}
	latencies := make([]time.Duration, 0, capacity)
	errors := 0

	// Trigger manual GC to start with a clean heap and disable GOGC during timed loop
	runtime.GC()
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	startTotal := time.Now()
	i := 0

	// Note: Concurrency option is currently a no-op scaffold. 
	// It will be activated in Phase 5 (Row-level locking).
	// For now, everything runs in this single goroutine.
	
	for {
		if !useDuration && i >= opts.Iterations {
			break
		}
		if useDuration && time.Since(startTotal) >= targetDuration {
			break
		}

		startOp := time.Now()
		err := w.Run(eng, i)
		latencies = append(latencies, time.Since(startOp))
		
		if err != nil {
			errors++
		}
		i++
	}

	totalTime := time.Since(startTotal)

	// 5. Compute Stats
	stats := computeStats(latencies, totalTime)

	return &WorkloadResult{
		Name:       w.Name(),
		Latencies:  latencies,
		TotalTime:  totalTime,
		Iterations: len(latencies),
		Stats:      stats,
		Errors:     errors,
		Metadata:   make(map[string]interface{}),
	}
}

func computeStats(latencies []time.Duration, totalTime time.Duration) Stats {
	if len(latencies) == 0 {
		return Stats{}
	}

	// Sort copy for percentiles
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	n := float64(len(sorted))
	
	// Mean
	var sum int64
	for _, l := range sorted {
		sum += int64(l)
	}
	mean := time.Duration(float64(sum) / n)

	// StdDev
	var sumSq float64
	for _, l := range sorted {
		diff := float64(l - mean)
		sumSq += diff * diff
	}
	stdDev := time.Duration(math.Sqrt(sumSq / n))

	// Percentiles
	idx50 := int(math.Floor(n * 0.50))
	idx90 := int(math.Floor(n * 0.90))
	idx95 := int(math.Floor(n * 0.95))
	idx99 := int(math.Floor(n * 0.99))

	// TPS
	tps := n / totalTime.Seconds()

	return Stats{
		P50:    sorted[idx50],
		P90:    sorted[idx90],
		P95:    sorted[idx95],
		P99:    sorted[idx99],
		Mean:   mean,
		StdDev: stdDev,
		TPS:    tps,
	}
}
