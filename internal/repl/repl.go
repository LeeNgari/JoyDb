package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/leengari/mini-rdbms/internal/engine"
	"github.com/leengari/mini-rdbms/internal/executor"
	"github.com/leengari/mini-rdbms/internal/storage/manager"
)

func Start(registry *manager.Registry) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Welcome to Mini-RDBMS")
	fmt.Println("Type 'exit' or '\\q' to quit.")

	// Start with no database selected
	eng := engine.New(nil, registry)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			return
		}
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			continue
		}

		if line == "exit" || line == "\\q" {
			break
		}

		// Execute using Engine
		result, err := eng.Execute(line)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Print Result
		PrintResult(os.Stdout, result)
	}
}

func PrintResult(w io.Writer, res *executor.Result) {
	if res.Message != "" {
		fmt.Fprintln(w, res.Message)
	}

	if len(res.Rows) > 0 || len(res.Columns) > 0 {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

		// Header - show type if metadata available
		for i, col := range res.Columns {
			if i < len(res.Metadata) && res.Metadata[i].Type != "" {
				// Show column with type
				fmt.Fprintf(tw, "%s (%s)", col, res.Metadata[i].Type)
			} else {
				// Just column name
				fmt.Fprintf(tw, "%s", col)
			}
			if i < len(res.Columns)-1 {
				fmt.Fprintf(tw, "\t")
			}
		}
		fmt.Fprintln(tw)

		// Separator
		for i := range res.Columns {
			fmt.Fprintf(tw, "---")
			if i < len(res.Columns)-1 {
				fmt.Fprintf(tw, "\t")
			}
		}
		fmt.Fprintln(tw)

		// Rows
		for _, row := range res.Rows {
			for i, col := range res.Columns {
				val, ok := row[col]
				if !ok {
					fmt.Fprintf(tw, "NULL")
				} else {
					fmt.Fprintf(tw, "%v", val)
				}
				if i < len(res.Columns)-1 {
					fmt.Fprintf(tw, "\t")
				}
			}
			fmt.Fprintln(tw)
		}
		tw.Flush()
	}
}
