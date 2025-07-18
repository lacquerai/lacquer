package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Inputs struct {
	CurrentValue int    `json:"current_value"`
	Multiplier   int    `json:"multiplier,omitempty"`
	Addition     int    `json:"addition,omitempty"`
	Operation    string `json:"operation"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type Result struct {
	CalculatedValue int    `json:"calculated_value"`
	Operation       string `json:"operation"`
}

func main() {
	var wrapper InputWrapper
	if err := json.NewDecoder(os.Stdin).Decode(&wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}

	inputs := wrapper.Inputs
	var result Result

	switch inputs.Operation {
	case "multiply":
		result.CalculatedValue = inputs.CurrentValue * inputs.Multiplier
		result.Operation = "multiply"
	case "add":
		result.CalculatedValue = inputs.CurrentValue + inputs.Addition
		result.Operation = "add"
	default:
		fmt.Fprintf(os.Stderr, "Unknown operation: %s\n", inputs.Operation)
		os.Exit(1)
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
