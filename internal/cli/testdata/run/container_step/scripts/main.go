package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Inputs struct {
	Message        string `json:"message"`
	ProcessingType string `json:"processing_type"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type Result struct {
	Result string `json:"result"`
}

func main() {
	var wrapper InputWrapper
	reader := strings.NewReader(os.Getenv("LACQUER_INPUTS"))
	if err := json.NewDecoder(reader).Decode(&wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}

	var result Result

	// reverse the message
	runes := []rune(wrapper.Inputs.Message)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	result.Result = string(runes)

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
