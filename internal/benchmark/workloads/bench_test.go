package workloads

import (
	"testing"

	"github.com/leengari/mini-rdbms/internal/benchmark"
)

// RunGoBench bridges our custom harness to Go's standard testing.B
func runGoBench(h *benchmark.BenchmarkHarness, b *testing.B, w benchmark.Workload) {
	// Setup Engine
	eng, _, err := benchmark.NewBenchEngine(h.GetEngineOpts())
	if err != nil {
		b.Fatalf("Failed to create engine: %v", err)
	}

	// Setup Workload
	if err := w.Setup(eng); err != nil {
		b.Fatalf("Failed to setup workload: %v", err)
	}
	defer w.Teardown(eng)

	// Go's benchmark framework handles warmup and iterations
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		if err := w.Run(eng, i); err != nil {
			b.Fatalf("Workload run failed at iteration %d: %v", i, err)
		}
	}
	
	b.StopTimer()
}

// ---------------------------------------------------------------------
// Go Benchmark Functions
// ---------------------------------------------------------------------

func runBench(b *testing.B, w benchmark.Workload) {
	h := benchmark.NewHarness(benchmark.DefaultEngineOptions())
	runGoBench(h, b, w)
}

func BenchmarkInsertSingleRow(b *testing.B) { runBench(b, NewInsertSingleRow()) }
func BenchmarkInsertBulk100(b *testing.B)   { runBench(b, NewInsertBulk100()) }
func BenchmarkInsertWithFK(b *testing.B)    { runBench(b, NewInsertWithFK()) }

func BenchmarkSelectPKLookup(b *testing.B)       { runBench(b, NewSelectPKLookup()) }
func BenchmarkSelectFullScan1K(b *testing.B)     { runBench(b, NewSelectFullScan1K()) }
func BenchmarkSelectFullScan10K(b *testing.B)    { runBench(b, NewSelectFullScan10K()) }
func BenchmarkSelectWithPredicate(b *testing.B)  { runBench(b, NewSelectWithPredicate()) }
func BenchmarkSelectWithProjection(b *testing.B) { runBench(b, NewSelectWithProjection()) }

func BenchmarkUpdateByPK(b *testing.B) { runBench(b, NewUpdateByPK()) }
func BenchmarkUpdateBulk(b *testing.B) { runBench(b, NewUpdateBulk()) }

func BenchmarkDeleteByPK(b *testing.B) { runBench(b, NewDeleteByPK()) }
func BenchmarkDeleteBulk(b *testing.B) { runBench(b, NewDeleteBulk()) }

func BenchmarkJoinNestedLoop100x100(b *testing.B) { runBench(b, NewJoinNestedLoop100x100()) }
func BenchmarkJoinNestedLoop1Kx1K(b *testing.B)   { runBench(b, NewJoinNestedLoop1Kx1K()) }

func BenchmarkMixedReadWrite80_20(b *testing.B) { runBench(b, NewMixedReadWrite80_20()) }
func BenchmarkMixedReadWrite50_50(b *testing.B) { runBench(b, NewMixedReadWrite50_50()) }

func BenchmarkCreateDropTable(b *testing.B) { runBench(b, NewCreateDropTable()) }

func BenchmarkInsertWithWAL(b *testing.B) { runBench(b, NewInsertWithWAL()) }
func BenchmarkInsertNoWAL(b *testing.B)   { runBench(b, NewInsertNoWAL()) }

func BenchmarkInsertScaling100(b *testing.B)   { runBench(b, NewInsertScaling(100)) }
func BenchmarkInsertScaling1K(b *testing.B)    { runBench(b, NewInsertScaling(1000)) }
func BenchmarkInsertScaling10K(b *testing.B)   { runBench(b, NewInsertScaling(10000)) }

func BenchmarkSelectScanScaling100(b *testing.B) { runBench(b, NewSelectScanScaling(100)) }
func BenchmarkSelectScanScaling1K(b *testing.B)  { runBench(b, NewSelectScanScaling(1000)) }
func BenchmarkSelectScanScaling10K(b *testing.B) { runBench(b, NewSelectScanScaling(10000)) }

func BenchmarkSelectPKScaling100(b *testing.B)   { runBench(b, NewSelectPKScaling(100)) }
func BenchmarkSelectPKScaling1K(b *testing.B)    { runBench(b, NewSelectPKScaling(1000)) }
func BenchmarkSelectPKScaling10K(b *testing.B)   { runBench(b, NewSelectPKScaling(10000)) }
