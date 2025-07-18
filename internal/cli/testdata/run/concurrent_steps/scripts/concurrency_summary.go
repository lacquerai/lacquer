package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type Inputs struct {
	GenTimeA int `json:"gen_time_a"`
	GenTimeB int `json:"gen_time_b"`
	GenTimeC int `json:"gen_time_c"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type GenerationTimes struct {
	DatasetA int `json:"dataset_a"`
	DatasetB int `json:"dataset_b"`
	DatasetC int `json:"dataset_c"`
}

type ExecutionAnalysis struct {
	SequentialTimeEstimate int    `json:"sequential_time_estimate"`
	ConcurrentTimeActual   int    `json:"concurrent_time_actual"`
	TimeSaved              int    `json:"time_saved"`
	EfficiencyGain         string `json:"efficiency_gain"`
}

type Result struct {
	GenerationTimes     *GenerationTimes   `json:"generation_times"`
	ExecutionAnalysis   *ExecutionAnalysis `json:"execution_analysis"`
	ConcurrencyBenefits []string           `json:"concurrency_benefits"`
}

func main() {
	var wrapper InputWrapper
	if err := json.NewDecoder(os.Stdin).Decode(&wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}

	inputs := wrapper.Inputs

	// Calculate what sequential vs concurrent execution would look like
	sequentialTime := inputs.GenTimeA + inputs.GenTimeB + inputs.GenTimeC
	concurrentTime := int(math.Max(math.Max(float64(inputs.GenTimeA), float64(inputs.GenTimeB)), float64(inputs.GenTimeC)))
	timeSaved := sequentialTime - concurrentTime

	var efficiencyGain string
	if sequentialTime > 0 {
		efficiencyGain = fmt.Sprintf("%.2f%%", float64(timeSaved)*100/float64(sequentialTime))
	} else {
		efficiencyGain = "0.00%"
	}

	result := Result{
		GenerationTimes: &GenerationTimes{
			DatasetA: inputs.GenTimeA,
			DatasetB: inputs.GenTimeB,
			DatasetC: inputs.GenTimeC,
		},
		ExecutionAnalysis: &ExecutionAnalysis{
			SequentialTimeEstimate: sequentialTime,
			ConcurrentTimeActual:   concurrentTime,
			TimeSaved:              timeSaved,
			EfficiencyGain:         efficiencyGain,
		},
		ConcurrencyBenefits: []string{
			"Independent steps executed in parallel",
			"Dependent steps waited for prerequisites",
			"Overall workflow time reduced through parallelization",
		},
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
