package benchmark

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"text/tabwriter"
	"time"
)

// Report contains the results of a benchmark suite run
type Report struct {
	Timestamp   time.Time        `json:"timestamp"`
	Label       string           `json:"label,omitempty"` // e.g. "baseline", "after-wal-rework", "phase3"
	OS          string           `json:"os"`
	Arch        string           `json:"arch"`
	CPUCores    int              `json:"cpu_cores"`
	GoVersion   string           `json:"go_version"`
	Results     []WorkloadResult `json:"results"`
}

// NewReport creates a new report structure populated with environment metadata
func NewReport(results []WorkloadResult) *Report {
	return &Report{
		Timestamp: time.Now(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		CPUCores:  runtime.NumCPU(),
		GoVersion: runtime.Version(),
		Results:   results,
	}
}

// WriteJSON writes the report as JSON to the provided writer
func (r *Report) WriteJSON(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}

// SaveJSON saves the report as JSON to a file
func (r *Report) SaveJSON(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return r.WriteJSON(f)
}

// LoadReport loads a previously saved JSON report
func LoadReport(path string) (*Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var report Report
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&report); err != nil {
		return nil, err
	}
	return &report, nil
}

// formatDuration neatly formats duration for the text report
func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%d ns", d.Nanoseconds())
	} else if d < time.Millisecond {
		return fmt.Sprintf("%.2f µs", float64(d.Nanoseconds())/1e3)
	} else if d < time.Second {
		return fmt.Sprintf("%.2f ms", float64(d.Nanoseconds())/1e6)
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
}

// WriteText writes a human-readable table of the results
func (r *Report) WriteText(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
	
	fmt.Fprintln(w, "JoyDB Benchmark Report")
	if r.Label != "" {
		fmt.Fprintf(w, "Label: %s\n", r.Label)
	}
	fmt.Fprintf(w, "Run on %s, %s %s (%d cores)\n", r.Timestamp.Format(time.RFC1123), r.OS, r.Arch, r.CPUCores)
	fmt.Fprintln(w, "---------------------------------------------------------------------------------------------------------")
	
	// Header
	fmt.Fprintln(tw, "Workload\tOps/sec\tp50\tp90\tp95\tp99\tMean\tErrors\t")
	fmt.Fprintln(tw, "--------\t-------\t---\t---\t---\t---\t----\t------\t")
	
	for _, res := range r.Results {
		if res.Errors > 0 && len(res.Latencies) == 0 {
			// Failed workload
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t\n",
				res.Name, "ERROR", "-", "-", "-", "-", "-", res.Errors)
			continue
		}

		fmt.Fprintf(tw, "%s\t%.1f\t%s\t%s\t%s\t%s\t%s\t%d\t\n",
			res.Name,
			res.Stats.TPS,
			formatDuration(res.Stats.P50),
			formatDuration(res.Stats.P90),
			formatDuration(res.Stats.P95),
			formatDuration(res.Stats.P99),
			formatDuration(res.Stats.Mean),
			res.Errors,
		)
	}
	
	return tw.Flush()
}

// CompareReports compares a baseline report against a current report
func CompareReports(baseline, current *Report, w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
	
	fmt.Fprintln(w, "JoyDB Benchmark Comparison")
	fmt.Fprintf(w, "Baseline: %s\n", baseline.Timestamp.Format(time.RFC1123))
	fmt.Fprintf(w, "Current:  %s\n", current.Timestamp.Format(time.RFC1123))
	fmt.Fprintln(w, "---------------------------------------------------------------------------------------------------------")
	
	fmt.Fprintln(tw, "Workload\tBaseline TPS\tCurrent TPS\tDelta %\tBaseline p95\tCurrent p95\t")
	fmt.Fprintln(tw, "--------\t------------\t-----------\t-------\t------------\t-----------\t")

	baselineMap := make(map[string]WorkloadResult)
	for _, res := range baseline.Results {
		baselineMap[res.Name] = res
	}

	for _, cur := range current.Results {
		base, exists := baselineMap[cur.Name]
		if !exists {
			fmt.Fprintf(tw, "%s\t-\t%.1f\t-\t-\t%s\t\n",
				cur.Name, cur.Stats.TPS, formatDuration(cur.Stats.P95))
			continue
		}

		deltaTPS := ((cur.Stats.TPS - base.Stats.TPS) / base.Stats.TPS) * 100.0
		
		fmt.Fprintf(tw, "%s\t%.1f\t%.1f\t%+.1f%%\t%s\t%s\t\n",
			cur.Name,
			base.Stats.TPS,
			cur.Stats.TPS,
			deltaTPS,
			formatDuration(base.Stats.P95),
			formatDuration(cur.Stats.P95),
		)
	}

	return tw.Flush()
}

// ---------------------------------------------------------------------------
// Benchmark History — append-only tracking of all runs in a single file
// ---------------------------------------------------------------------------

// BenchmarkHistory contains a list of benchmark reports over time
type BenchmarkHistory struct {
	Runs []Report `json:"runs"`
}

// LoadHistory loads the history file, returning an empty history if the file doesn't exist
func LoadHistory(path string) (*BenchmarkHistory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &BenchmarkHistory{}, nil
		}
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	var history BenchmarkHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to parse history file: %w", err)
	}
	return &history, nil
}

// AppendToHistory adds a report to the history file and saves it.
// Creates the directory and file if they don't exist.
func AppendToHistory(path string, report *Report) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	history, err := LoadHistory(path)
	if err != nil {
		return err
	}

	// Strip raw latencies from the history copy to keep the file manageable
	historyReport := *report
	trimmedResults := make([]WorkloadResult, len(report.Results))
	for i, r := range report.Results {
		trimmed := r
		trimmed.Latencies = nil // Don't store raw latency arrays in history
		trimmedResults[i] = trimmed
	}
	historyReport.Results = trimmedResults

	history.Runs = append(history.Runs, historyReport)

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}
	return nil
}

// WriteHistoryText prints a trend table showing how each workload's TPS changed over time
func WriteHistoryText(history *BenchmarkHistory, w io.Writer) error {
	if len(history.Runs) == 0 {
		fmt.Fprintln(w, "No benchmark history found.")
		return nil
	}

	fmt.Fprintln(w, "JoyDB Benchmark History")
	fmt.Fprintf(w, "Total runs: %d\n", len(history.Runs))
	fmt.Fprintln(w, "=========================================================================================================")

	// Collect all unique workload names across all runs
	workloadNames := []string{}
	seen := map[string]bool{}
	for _, run := range history.Runs {
		for _, res := range run.Results {
			if !seen[res.Name] {
				seen[res.Name] = true
				workloadNames = append(workloadNames, res.Name)
			}
		}
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight|tabwriter.Debug)

	// Header row: Workload | Run1 (label/date) | Run2 | ...
	header := "Workload"
	for _, run := range history.Runs {
		label := run.Label
		if label == "" {
			label = run.Timestamp.Format("2006-01-02 15:04")
		}
		header += "\t" + label
	}
	fmt.Fprintln(tw, header+"\t")

	// Separator
	sep := "--------"
	for range history.Runs {
		sep += "\t" + "--------"
	}
	fmt.Fprintln(tw, sep+"\t")

	// Data rows: one per workload, showing TPS at each run
	for _, wName := range workloadNames {
		row := wName
		var prevTPS float64
		for i, run := range history.Runs {
			found := false
			for _, res := range run.Results {
				if res.Name == wName {
					if i > 0 && prevTPS > 0 {
						delta := ((res.Stats.TPS - prevTPS) / prevTPS) * 100.0
						row += fmt.Sprintf("\t%.0f (%+.1f%%)", res.Stats.TPS, delta)
					} else {
						row += fmt.Sprintf("\t%.0f", res.Stats.TPS)
					}
					prevTPS = res.Stats.TPS
					found = true
					break
				}
			}
			if !found {
				row += "\t-"
			}
		}
		fmt.Fprintln(tw, row+"\t")
	}

	return tw.Flush()
}
