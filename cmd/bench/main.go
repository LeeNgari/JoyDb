package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/leengari/mini-rdbms/internal/benchmark"
	"github.com/leengari/mini-rdbms/internal/benchmark/workloads"
)

const defaultHistoryPath = "benchmarks/history.json"

func main() {
	tagsStr := flag.String("tags", "", "Filter workloads by tags (comma-separated)")
	iterations := flag.Int("iterations", 0, "Number of iterations per workload (default: auto using duration)")
	duration := flag.Duration("duration", 3*time.Second, "Run each workload for this duration (default: 3s)")
	warmup := flag.Int("warmup", 100, "Warmup iterations (default: 100)")
	runs := flag.Int("runs", 1, "Number of runs to perform per workload, reporting the median result (default: 1)")
	jsonOut := flag.Bool("json", false, "Output results as JSON")
	outputFile := flag.String("output", "", "Write results to file (default: stdout)")
	baselineFile := flag.String("baseline", "", "Compare against a baseline JSON report")
	walEnabled := flag.Bool("wal", false, "Enable WAL for benchmarks (default: false)")
	concurrency := flag.Int("concurrency", 1, "Concurrent workers - no-op scaffold, always 1 (default: 1)")
	label := flag.String("label", "", "Label for this benchmark run (e.g. 'baseline', 'phase3', 'after-wal-fix')")
	historyPath := flag.String("history", defaultHistoryPath, "Path to the history JSON file")
	noHistory := flag.Bool("no-history", false, "Don't save this run to the history file")
	showHistory := flag.Bool("show-history", false, "Show the benchmark history trend table and exit")
	
	flag.Parse()

	// Handle --show-history: just print and exit
	if *showHistory {
		history, err := benchmark.LoadHistory(*historyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load history: %v\n", err)
			os.Exit(1)
		}
		if err := benchmark.WriteHistoryText(history, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write history: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Suppress standard logging during benchmark runs
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	tags := []string{}
	if *tagsStr != "" {
		for _, t := range strings.Split(*tagsStr, ",") {
			tags = append(tags, strings.TrimSpace(t))
		}
	}

	opts := benchmark.RunOptions{
		WarmupIterations: *warmup,
		Iterations:       *iterations,
		Duration:         *duration,
		Concurrency:      *concurrency,
		ResetBetweenRuns: false,
	}

	engOpts := benchmark.DefaultEngineOptions()
	engOpts.WALEnabled = *walEnabled

	// Register all available workloads
	allWorkloads := []benchmark.Workload{
		workloads.NewInsertSingleRow(),
		workloads.NewInsertBulk100(),
		workloads.NewInsertWithFK(),
		
		workloads.NewSelectPKLookup(),
		workloads.NewSelectFullScan1K(),
		workloads.NewSelectFullScan10K(),
		workloads.NewSelectWithPredicate(),
		workloads.NewSelectWithProjection(),
		workloads.NewSelectRangeScan100(),
		workloads.NewSelectRangeScan1K(),
		
		workloads.NewUpdateByPK(),
		workloads.NewUpdateBulk(),
		
		workloads.NewDeleteByPK(),
		workloads.NewDeleteBulk(),
		
		workloads.NewJoinNestedLoop100x100(),
		workloads.NewJoinNestedLoop1Kx1K(),
		
		workloads.NewMixedReadWrite80_20(),
		workloads.NewMixedReadWrite50_50(),
		
		workloads.NewCreateDropTable(),
		
		workloads.NewInsertWithWAL(),
		workloads.NewInsertNoWAL(),

		workloads.NewInsertScaling(100),
		workloads.NewInsertScaling(1000),
		workloads.NewInsertScaling(10000),
		
		workloads.NewSelectScanScaling(100),
		workloads.NewSelectScanScaling(1000),
		workloads.NewSelectScanScaling(10000),
		
		workloads.NewSelectPKScaling(100),
		workloads.NewSelectPKScaling(1000),
		workloads.NewSelectPKScaling(10000),
	}

	// Filter by tags
	selectedWorkloads := []benchmark.Workload{}
	for _, w := range allWorkloads {
		if len(tags) == 0 || hasIntersection(tags, w.Tags()) {
			selectedWorkloads = append(selectedWorkloads, w)
		}
	}

	if len(selectedWorkloads) == 0 {
		fmt.Println("No workloads selected.")
		os.Exit(0)
	}

	harness := benchmark.NewHarness(engOpts)
	var results []benchmark.WorkloadResult

	if !*jsonOut {
		if *runs > 1 {
			fmt.Printf("Running %d workloads (%d runs each, reporting median TPS)...\n", len(selectedWorkloads), *runs)
		} else {
			fmt.Printf("Running %d workloads...\n", len(selectedWorkloads))
		}
	}

	for _, w := range selectedWorkloads {
		// Check if workload wants custom engine options (like WAL tests)
		if provider, ok := w.(workloads.CustomEngineOptionsProvider); ok {
			harness = benchmark.NewHarness(provider.GetEngineOptions())
		} else {
			harness = benchmark.NewHarness(engOpts)
		}

		numRuns := *runs
		if numRuns < 1 {
			numRuns = 1
		}

		var runResults []benchmark.WorkloadResult
		for r := 0; r < numRuns; r++ {
			res := harness.Run(w, opts)
			runResults = append(runResults, *res)
		}

		// Sort by TPS to find the median result
		sort.Slice(runResults, func(i, j int) bool {
			return runResults[i].Stats.TPS < runResults[j].Stats.TPS
		})

		// Pick median run
		medianRes := runResults[len(runResults)/2]
		results = append(results, medianRes)
	}

	report := benchmark.NewReport(results)
	report.Label = *label

	// Auto-save to history unless --no-history is set
	if !*noHistory {
		if err := benchmark.AppendToHistory(*historyPath, report); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save to history: %v\n", err)
			// Don't exit — still show the report
		} else if !*jsonOut {
			fmt.Printf("Results saved to history: %s\n", *historyPath)
		}
	}

	var outWriter = os.Stdout
	if *outputFile != "" {
		f, err := os.Create(*outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open output file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		outWriter = f
	}

	if *baselineFile != "" {
		baseline, err := benchmark.LoadReport(*baselineFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load baseline: %v\n", err)
			os.Exit(1)
		}
		
		if err := benchmark.CompareReports(baseline, report, outWriter); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write comparison: %v\n", err)
			os.Exit(1)
		}
	} else if *jsonOut {
		if err := report.WriteJSON(outWriter); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write JSON: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := report.WriteText(outWriter); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write text report: %v\n", err)
			os.Exit(1)
		}
	}
}

func hasIntersection(filterTags, workloadTags []string) bool {
	for _, ft := range filterTags {
		for _, wt := range workloadTags {
			if ft == wt {
				return true
			}
		}
	}
	return false
}
