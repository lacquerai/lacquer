package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"
)

type Input struct {
	Component string `json:"component"`
	Details   bool   `json:"details"`
}

type Output struct {
	Runtime   string                 `json:"runtime"`
	Version   string                 `json:"version"`
	Timestamp string                 `json:"timestamp"`
	System    map[string]interface{} `json:"system"`
}

func main() {
	// Get input from environment variable
	inputsJson := os.Getenv("LACQUER_INPUTS")
	if inputsJson == "" {
		inputsJson = "{}"
	}

	var inputs Input
	if err := json.Unmarshal([]byte(inputsJson), &inputs); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing inputs: %v\n", err)
		os.Exit(1)
	}

	result := Output{
		Runtime:   "go",
		Version:   runtime.Version(),
		Timestamp: time.Now().Format(time.RFC3339),
		System:    make(map[string]interface{}),
	}

	switch inputs.Component {
	case "runtime":
		result.System["go_version"] = runtime.Version()
		result.System["go_os"] = runtime.GOOS
		result.System["go_arch"] = runtime.GOARCH
		result.System["num_cpu"] = runtime.NumCPU()
		if inputs.Details {
			result.System["compiler"] = runtime.Compiler
			result.System["num_goroutines"] = runtime.NumGoroutine()
		}
	case "memory":
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		result.System["allocated_mb"] = m.Alloc / 1024 / 1024
		result.System["total_allocated_mb"] = m.TotalAlloc / 1024 / 1024
		result.System["system_mb"] = m.Sys / 1024 / 1024
		if inputs.Details {
			result.System["gc_cycles"] = m.NumGC
			result.System["heap_objects"] = m.HeapObjects
		}
	default:
		result.System["basic_info"] = map[string]interface{}{
			"os":   runtime.GOOS,
			"arch": runtime.GOARCH,
			"cpus": runtime.NumCPU(),
		}
	}

	output := map[string]interface{}{
		"outputs": result,
	}

	outputJson, err := json.Marshal(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(outputJson))
}
