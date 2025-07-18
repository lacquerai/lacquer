package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Inputs struct {
	Name      string `json:"name"`
	Style     string `json:"style"`
	Reps      int    `json:"reps"`
	Timestamp string `json:"timestamp"`
	Code      string `json:"code"`
	Note      string `json:"note"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type Result struct {
	ProvidedName     string `json:"provided_name"`
	GreetingStyle    string `json:"greeting_style"`
	Repetitions      int    `json:"repetitions"`
	IncludeTimestamp string `json:"include_timestamp"`
	UserCode         string `json:"user_code"`
	HasOptionalNote  bool   `json:"has_optional_note"`
	OptionalNote     string `json:"optional_note"`
	ValidationStatus string `json:"validation_status"`
}

func main() {
	var wrapper InputWrapper
	if err := json.NewDecoder(os.Stdin).Decode(&wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}

	inputs := wrapper.Inputs

	result := Result{
		ProvidedName:     inputs.Name,
		GreetingStyle:    inputs.Style,
		Repetitions:      inputs.Reps,
		IncludeTimestamp: inputs.Timestamp,
		UserCode:         inputs.Code,
		HasOptionalNote:  inputs.Note != "",
		OptionalNote:     inputs.Note,
		ValidationStatus: "passed",
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
