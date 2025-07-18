package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Inputs struct {
	Number int `json:"number"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type Result struct {
	IsLarge       bool   `json:"is_large"`
	Category      string `json:"category"`
	ShouldAnalyze bool   `json:"should_analyze"`
}

func main() {
	var wrapper InputWrapper
	if err := json.NewDecoder(os.Stdin).Decode(&wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}

	number := wrapper.Inputs.Number
	var result Result

	if number > 50 {
		result = Result{
			IsLarge:       true,
			Category:      "large",
			ShouldAnalyze: true,
		}
	} else if number > 20 {
		result = Result{
			IsLarge:       false,
			Category:      "medium",
			ShouldAnalyze: true,
		}
	} else {
		result = Result{
			IsLarge:       false,
			Category:      "small",
			ShouldAnalyze: false,
		}
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
