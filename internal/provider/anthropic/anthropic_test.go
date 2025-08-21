package anthropic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_ModelAlias(t *testing.T) {
	// Create a provider instance for testing
	provider := &Provider{
		name: "anthropic",
	}

	tests := []struct {
		name            string
		inputModel      string
		availableModels []string
		expectedModel   string
		expectError     bool
		description     string
	}{
		{
			name:            "no_matches_returns_original",
			inputModel:      "claude-opus-4",
			availableModels: []string{"claude-3-5-sonnet-20241022", "claude-3-haiku-20240307"},
			expectedModel:   "claude-opus-4",
			expectError:     false,
			description:     "When no models match the prefix, should return the original model",
		},
		{
			name:            "single_match_returns_match",
			inputModel:      "claude-opus-4",
			availableModels: []string{"claude-opus-4-20250514", "claude-3-5-sonnet-20241022"},
			expectedModel:   "claude-opus-4-20250514",
			expectError:     false,
			description:     "When exactly one model matches the prefix, should return that model",
		},
		{
			name:       "multiple_matches_returns_latest",
			inputModel: "claude-opus-4",
			availableModels: []string{
				"claude-opus-4-20250514",
				"claude-opus-4-20250520",
				"claude-opus-4-20250510",
			},
			expectedModel: "claude-opus-4-20250520",
			expectError:   false,
			description:   "When multiple models match, should return the one with the latest date",
		},
		{
			name:       "mixed_matches_with_latest",
			inputModel: "claude-sonnet-4",
			availableModels: []string{
				"claude-sonnet-4-20250514",
				"claude-opus-4-20250520",
				"claude-sonnet-4-20250601",
				"claude-3-5-sonnet-20241022",
			},
			expectedModel: "claude-sonnet-4-20250601",
			expectError:   false,
			description:   "Should only consider models with matching prefix and return latest",
		},
		{
			name:            "empty_model_list_returns_original",
			inputModel:      "claude-opus-4",
			availableModels: []string{},
			expectedModel:   "claude-opus-4",
			expectError:     false,
			description:     "Empty model list should return the original model",
		},
		{
			name:       "partial_prefix_match",
			inputModel: "claude-3",
			availableModels: []string{
				"claude-3-5-sonnet-20241022",
				"claude-3-haiku-20240307",
				"claude-3-opus-20240229",
			},
			expectedModel: "claude-3-5-sonnet-20241022",
			expectError:   false,
			description:   "Should match all models with the prefix and return latest",
		},
		{
			name:       "exact_model_name_in_list",
			inputModel: "claude-3-5-sonnet-20241022",
			availableModels: []string{
				"claude-3-5-sonnet-20241022",
				"claude-3-haiku-20240307",
			},
			expectedModel: "claude-3-5-sonnet-20241022",
			expectError:   false,
			description:   "If exact model name exists in list, should return it",
		},
		{
			name:       "models_without_date_suffix",
			inputModel: "claude-test",
			availableModels: []string{
				"claude-test-model",
				"claude-test-beta",
			},
			expectedModel: "claude-test-beta", // Lexicographically first when no valid dates
			expectError:   false,
			description:   "Models without valid date suffixes should still work",
		},
		{
			name:       "case_sensitive_matching",
			inputModel: "Claude-Opus-4",
			availableModels: []string{
				"claude-opus-4-20250514",
				"Claude-Opus-4-20250520",
			},
			expectedModel: "Claude-Opus-4-20250520",
			expectError:   false,
			description:   "Matching should be case-sensitive",
		},
		{
			name:       "very_old_dates",
			inputModel: "claude-opus-4",
			availableModels: []string{
				"claude-opus-4-20200101",
				"claude-opus-4-20250514",
				"claude-opus-4-19990101",
			},
			expectedModel: "claude-opus-4-20250514",
			expectError:   false,
			description:   "Should correctly sort very old and new dates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.ModelAlias(tt.inputModel, tt.availableModels)

			if tt.expectError {
				require.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedModel, result, tt.description)
			}
		})
	}
}

func TestProvider_ModelAlias_DateSorting(t *testing.T) {
	provider := &Provider{name: "anthropic"}

	// Test specifically the date sorting logic with a complex scenario
	models := []string{
		"claude-opus-4-20250101", // New Year's Day 2025
		"claude-opus-4-20241231", // New Year's Eve 2024
		"claude-opus-4-20250630", // Mid 2025
		"claude-opus-4-20250105", // Early January 2025
		"claude-opus-4-20250229", // Leap day 2025 (invalid, but tests parsing)
	}

	result, err := provider.ModelAlias("claude-opus-4", models)
	require.NoError(t, err)

	// Should return the latest valid date
	assert.Equal(t, "claude-opus-4-20250630", result)
}

func TestProvider_ModelAlias_EmptyInputs(t *testing.T) {
	provider := &Provider{name: "anthropic"}

	tests := []struct {
		name        string
		inputModel  string
		models      []string
		expected    string
		expectError bool
		description string
	}{
		{
			name:        "empty_input_model",
			inputModel:  "",
			models:      []string{"claude-opus-4-20250514"},
			expected:    "",
			expectError: true,
			description: "Empty input model should return empty string",
		},
		{
			name:        "nil_models_list",
			inputModel:  "claude-opus-4",
			models:      nil,
			expected:    "claude-opus-4",
			description: "Nil models list should return original model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := provider.ModelAlias(tt.inputModel, tt.models)
			if tt.expectError {
				require.Error(t, err, tt.description)
			} else {
				require.NoError(t, err, tt.description)
			}
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestModelSuffixRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		match    string
	}{
		{
			name:     "valid_date_suffix",
			input:    "claude-opus-4-20250514",
			expected: true,
			match:    "-20250514",
		},
		{
			name:     "no_date_suffix",
			input:    "claude-opus-4",
			expected: false,
			match:    "",
		},
		{
			name:     "invalid_date_format",
			input:    "claude-opus-4-202505",
			expected: false,
			match:    "",
		},
		{
			name:     "too_long_suffix",
			input:    "claude-opus-4-202505141",
			expected: false,
			match:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := modelSuffix.FindStringSubmatch(tt.input)
			if tt.expected {
				require.NotEmpty(t, matches, "Expected to find a match")
				assert.Equal(t, tt.match, matches[0])
			} else {
				assert.Empty(t, matches, "Expected no matches")
			}
		})
	}
}
