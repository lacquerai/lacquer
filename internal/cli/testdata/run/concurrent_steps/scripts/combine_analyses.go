package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Inputs struct {
	NumericalMean    float64 `json:"numerical_mean"`
	CategoricalCount int     `json:"categorical_count"`
	TextualCount     int     `json:"textual_count"`
}

type InputWrapper struct {
	Inputs Inputs `json:"inputs"`
}

type NumericalInsights struct {
	MeanValue   float64 `json:"mean_value"`
	DataQuality string  `json:"data_quality"`
}

type CategoricalInsights struct {
	CategoryCount int    `json:"category_count"`
	Diversity     string `json:"diversity"`
}

type TextualInsights struct {
	TextSamples      int    `json:"text_samples"`
	LanguageDetected string `json:"language_detected"`
}

type Result struct {
	NumericalInsights   *NumericalInsights   `json:"numerical_insights"`
	CategoricalInsights *CategoricalInsights `json:"categorical_insights"`
	TextualInsights     *TextualInsights     `json:"textual_insights"`
	CombinedScore       float64              `json:"combined_score"`
	OverallAssessment   string               `json:"overall_assessment"`
}

func main() {
	var wrapper InputWrapper
	if err := json.NewDecoder(os.Stdin).Decode(&wrapper); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
		os.Exit(1)
	}

	inputs := wrapper.Inputs

	totalFeatures := inputs.CategoricalCount + inputs.TextualCount
	combinedScore := inputs.NumericalMean + float64(totalFeatures)

	result := Result{
		NumericalInsights: &NumericalInsights{
			MeanValue:   inputs.NumericalMean,
			DataQuality: "good",
		},
		CategoricalInsights: &CategoricalInsights{
			CategoryCount: inputs.CategoricalCount,
			Diversity:     "high",
		},
		TextualInsights: &TextualInsights{
			TextSamples:      inputs.TextualCount,
			LanguageDetected: "english",
		},
		CombinedScore:     combinedScore,
		OverallAssessment: "Multi-modal dataset with good variety across numerical, categorical, and textual dimensions",
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}
}
