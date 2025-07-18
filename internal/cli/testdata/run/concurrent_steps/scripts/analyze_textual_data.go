package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Inputs struct {
	Texts       string `json:"texts"`
	DatasetName string `json:"dataset_name"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type Result struct {
	TextCount     int    `json:"text_count"`
	AverageLength int    `json:"average_length"`
	Language      string `json:"language"`
	Analysis      string `json:"analysis"`
}

func main() {
	var wrapper InputWrapper
	if err := json.NewDecoder(os.Stdin).Decode(&wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}

	inputs := wrapper.Inputs

	// Simple text analysis
	textCount := len(strings.Split(inputs.Texts, ","))
	avgLength := 5 // Average length of sample texts

	result := Result{
		TextCount:     textCount,
		AverageLength: avgLength,
		Language:      "english",
		Analysis:      fmt.Sprintf("Dataset %s contains %d text samples with average length %d", inputs.DatasetName, textCount, avgLength),
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
