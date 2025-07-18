package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Inputs struct {
	Size int `json:"size"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type Metadata struct {
	CreatedAt string `json:"created_at"`
	Source    string `json:"source"`
}

type Result struct {
	DatasetName    string    `json:"dataset_name"`
	Size           int       `json:"size"`
	DataType       string    `json:"data_type"`
	GenerationTime int       `json:"generation_time"`
	SampleTexts    []string  `json:"sample_texts"`
	Metadata       *Metadata `json:"metadata"`
}

func main() {
	var wrapper InputWrapper
	if err := json.NewDecoder(os.Stdin).Decode(&wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}

	start := time.Now()

	// Simulate yet another data generation work
	time.Sleep(1 * time.Second)

	duration := int(time.Since(start).Seconds())

	result := Result{
		DatasetName:    "dataset_c",
		Size:           wrapper.Inputs.Size,
		DataType:       "textual",
		GenerationTime: duration,
		SampleTexts:    []string{"hello", "world", "test", "data"},
		Metadata: &Metadata{
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Source:    "generator_c",
		},
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
