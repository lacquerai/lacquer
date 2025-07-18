package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Inputs struct {
	Category          string `json:"category"`
	HasLargeAnalysis  bool   `json:"has_large_analysis"`
	HasMediumAnalysis bool   `json:"has_medium_analysis"`
	WasSkipped        bool   `json:"was_skipped"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type Result struct {
	Summary string `json:"summary"`
	Type    string `json:"type"`
}

func main() {
	var wrapper InputWrapper
	if err := json.NewDecoder(os.Stdin).Decode(&wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}

	inputs := wrapper.Inputs
	var result Result

	if inputs.HasLargeAnalysis {
		result = Result{
			Summary: "Large number analysis completed",
			Type:    "large",
		}
	} else if inputs.HasMediumAnalysis {
		result = Result{
			Summary: "Medium number analysis completed",
			Type:    "medium",
		}
	} else if inputs.WasSkipped {
		result = Result{
			Summary: "Small number analysis skipped",
			Type:    "small",
		}
	} else {
		result = Result{
			Summary: "No analysis performed",
			Type:    "unknown",
		}
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
