package repl

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/leengari/mini-rdbms/internal/domain/schema"
	"github.com/leengari/mini-rdbms/internal/engine"
	"github.com/leengari/mini-rdbms/internal/executor"
)

func Start(db *schema.Database) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Welcome to M")
	fmt.Println("Type 'exit' or '\\q' to quit.")

	eng := engine.New(db)

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
		printResult(result)
	}
}

func printResult(res *executor.Result) {
	if res.Message != "" {
		fmt.Println(res.Message)
	}

	if len(res.Rows) > 0 || len(res.Columns) > 0 {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		
		// Header - show type if metadata available
		for i, col := range res.Columns {
			if i < len(res.Metadata) && res.Metadata[i].Type != "" {
				// Show column with type
				fmt.Fprintf(w, "%s (%s)", col, res.Metadata[i].Type)
			} else {
				// Just column name
				fmt.Fprintf(w, "%s", col)
			}
			if i < len(res.Columns)-1 {
				fmt.Fprintf(w, "\t")
			}
		}
		fmt.Fprintln(w)

		// Separator
		for i := range res.Columns {
			fmt.Fprintf(w, "---")
			if i < len(res.Columns)-1 {
				fmt.Fprintf(w, "\t")
			}
		}
		fmt.Fprintln(w)

		// Rows
		for _, row := range res.Rows {
			for i, col := range res.Columns {
				val, ok := row[col]
				if !ok {
					fmt.Fprintf(w, "NULL")
				} else {
					fmt.Fprintf(w, "%v", val)
				}
				if i < len(res.Columns)-1 {
					fmt.Fprintf(w, "\t")
				}
			}
			fmt.Fprintln(w)
		}
		w.Flush()
	}
}
