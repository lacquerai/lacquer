package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type FormattingApplied struct {
	Uppercase    bool `json:"uppercase"`
	BordersAdded bool `json:"borders_added"`
	Indentation  int  `json:"indentation"`
}

type Result struct {
	AdditionalMessageCount int                `json:"additional_message_count"`
	FormattingApplied      *FormattingApplied `json:"formatting_applied"`
	ProcessedOptions       bool               `json:"processed_options"`
}

func main() {
	result := Result{
		AdditionalMessageCount: 2,
		FormattingApplied: &FormattingApplied{
			Uppercase:    false,
			BordersAdded: true,
			Indentation:  2,
		},
		ProcessedOptions: true,
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
