package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type Inputs struct {
	Name      string `json:"name"`
	Style     string `json:"style"`
	Reps      int    `json:"reps"`
	IncludeTs string `json:"include_ts"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type Result struct {
	Greetings         []string `json:"greetings"`
	GreetingCount     int      `json:"greeting_count"`
	StyleUsed         string   `json:"style_used"`
	TimestampIncluded string   `json:"timestamp_included"`
}

func main() {
	var wrapper InputWrapper
	if err := json.NewDecoder(os.Stdin).Decode(&wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}

	inputs := wrapper.Inputs

	var greeting string
	switch inputs.Style {
	case "formal":
		greeting = fmt.Sprintf("Good day, Mr./Ms. %s", inputs.Name)
	case "friendly":
		greeting = fmt.Sprintf("Hello there, %s!", inputs.Name)
	case "casual":
		greeting = fmt.Sprintf("Hey %s, what's up?", inputs.Name)
	default:
		greeting = fmt.Sprintf("Hi %s", inputs.Name)
	}

	var greetings []string
	includeTimestamp := strings.ToLower(inputs.IncludeTs) == "true"

	for i := 0; i < inputs.Reps; i++ {
		currentGreeting := greeting
		if includeTimestamp {
			timestamp := time.Now().UTC().Format("15:04:05")
			currentGreeting = fmt.Sprintf("[%s] %s", timestamp, greeting)
		}
		greetings = append(greetings, currentGreeting)
	}

	result := Result{
		Greetings:         greetings,
		GreetingCount:     inputs.Reps,
		StyleUsed:         inputs.Style,
		TimestampIncluded: inputs.IncludeTs,
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
