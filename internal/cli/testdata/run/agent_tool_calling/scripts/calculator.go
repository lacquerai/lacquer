package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Inputs struct {
	Operation string `json:"operation"`
	Num1      int    `json:"num1"`
	Num2      int    `json:"num2"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type Result struct {
	Result int    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
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
	case "add":
		result.Result = inputs.Num1 + inputs.Num2
	case "subtract":
		result.Result = inputs.Num1 - inputs.Num2
	case "multiply":
		result.Result = inputs.Num1 * inputs.Num2
	case "divide":
		if inputs.Num2 == 0 {
			result.Error = "Division by zero"
			if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
				os.Exit(1)
			}
			os.Exit(1)
		}
		result.Result = inputs.Num1 / inputs.Num2
	default:
		result.Error = fmt.Sprintf("Unknown operation: %s", inputs.Operation)
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
			os.Exit(1)
		}
		os.Exit(1)
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
