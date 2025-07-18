package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Inputs struct {
	Creative   string `json:"creative"`
	Analytical string `json:"analytical"`
	Synthesis  string `json:"synthesis"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type ResponseLengths struct {
	Creative   int `json:"creative"`
	Analytical int `json:"analytical"`
	Synthesis  int `json:"synthesis"`
}

type ExecutionAnalysis struct {
	SequentialTimeEstimate int    `json:"sequential_time_estimate"`
	ConcurrentTimeActual   int    `json:"concurrent_time_actual"`
	TimeSaved              int    `json:"time_saved"`
	EfficiencyGain         string `json:"efficiency_gain"`
}

type AgentComparison struct {
	CreativeAgentCharacteristics   string `json:"creative_agent_characteristics"`
	AnalyticalAgentCharacteristics string `json:"analytical_agent_characteristics"`
	BalancedAgentCharacteristics   string `json:"balanced_agent_characteristics"`
}

type Result struct {
	ResponseLengths *ResponseLengths `json:"response_lengths"`
	LongestResponse string           `json:"longest_response"`
	TotalWords      int              `json:"total_words"`
	AgentComparison *AgentComparison `json:"agent_comparison"`
}

func countWords(text string) int {
	if text == "" {
		return 0
	}
	return len(strings.Fields(text))
}

func main() {
	var wrapper InputWrapper
	if err := json.NewDecoder(os.Stdin).Decode(&wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}

	inputs := wrapper.Inputs

	creativeLength := countWords(inputs.Creative)
	analyticalLength := countWords(inputs.Analytical)
	synthesisLength := countWords(inputs.Synthesis)

	var longestResponse string
	if creativeLength >= analyticalLength && creativeLength >= synthesisLength {
		longestResponse = "creative"
	} else if analyticalLength >= synthesisLength {
		longestResponse = "analytical"
	} else {
		longestResponse = "synthesis"
	}

	result := Result{
		ResponseLengths: &ResponseLengths{
			Creative:   creativeLength,
			Analytical: analyticalLength,
			Synthesis:  synthesisLength,
		},
		LongestResponse: longestResponse,
		TotalWords:      creativeLength + analyticalLength + synthesisLength,
		AgentComparison: &AgentComparison{
			CreativeAgentCharacteristics:   "High temperature (0.8), enthusiastic tone",
			AnalyticalAgentCharacteristics: "Low temperature (0.1), precise structure",
			BalancedAgentCharacteristics:   "Medium temperature (0.5), combined approach",
		},
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
