package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
)

type InputWrapper struct {
	Inputs Input `json:"inputs"`
}

type Input struct {
	TestParam string `json:"test_param"`
}

type Output struct {
	Runtime string `json:"runtime"`
	Version string `json:"version"`
	Message string `json:"message"`
}

func main() {
	// Read input from stdin
	inputData, err := io.ReadAll(os.Stdin)
	var inputs InputWrapper

	if err == nil && len(inputData) > 0 {
		json.Unmarshal(inputData, &inputs)
	}

	// Set default if no input received
	testParam := inputs.Inputs.TestParam
	if testParam == "" {
		testParam = "default_value"
	}

	result := Output{
		Runtime: "go",
		Version: runtime.Version(),
		Message: fmt.Sprintf("Received input: %s", testParam),
	}

	outputJson, _ := json.Marshal(result)
	fmt.Println(string(outputJson))
}
