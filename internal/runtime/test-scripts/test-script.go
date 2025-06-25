package main

import (
	"encoding/json"
	"os"
)

type ExecutionInput struct {
	Input string `json:"input"`
}

type ExecutionOutput struct {
	Result string `json:"result"`
}

func main() {
	var input ExecutionInput
	json.NewDecoder(os.Stdin).Decode(&input)

	output := ExecutionOutput{
		Result: "Processed: " + input.Input,
	}

	json.NewEncoder(os.Stdout).Encode(output)
}
