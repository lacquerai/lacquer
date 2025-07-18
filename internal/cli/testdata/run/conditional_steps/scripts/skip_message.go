package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Result struct {
	Message string `json:"message"`
	Skipped bool   `json:"skipped"`
}

func main() {
	result := Result{
		Message: "Number too small for analysis",
		Skipped: true,
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
