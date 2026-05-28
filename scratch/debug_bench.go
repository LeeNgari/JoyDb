package main

import (
	"fmt"
	"os"

	"github.com/leengari/mini-rdbms/internal/benchmark"
	"github.com/leengari/mini-rdbms/internal/benchmark/workloads"
)

func main() {
	opts := benchmark.DefaultEngineOptions()
	opts.WALEnabled = true
	
	eng, dir, err := benchmark.NewBenchEngine(opts)
	if err != nil {
		fmt.Printf("Engine creation failed: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	
	wl := workloads.NewInsertWithWAL()
	if err := wl.Setup(eng); err != nil {
		fmt.Printf("Setup failed: %v\n", err)
		os.Exit(1)
	}
	defer wl.Teardown(eng)
	
	err = wl.Run(eng, 0)
	if err != nil {
		fmt.Printf("Run failed: %v\n", err)
	} else {
		fmt.Println("Run succeeded!")
	}
}
