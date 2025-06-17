package parser

import (
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnhancedError_Error(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := &EnhancedError{
			ID:       "test_1_1",
			Severity: SeverityError,
			Title:    "Invalid syntax",
			Message:  "Expected a colon after the key",
			Position: ast.Position{Line: 1, Column: 10, File: "test.yaml"},
			Category: "yaml",
		}

		output := err.Error()
		assert.Contains(t, output, "test.yaml:1:10:")
		assert.Contains(t, output, "error: Invalid syntax")
		assert.Contains(t, output, "Expected a colon after the key")
	})

	t.Run("error with context", func(t *testing.T) {
		err := &EnhancedError{
			ID:       "test_2_5",
			Severity: SeverityError,
			Title:    "Missing field",
			Position: ast.Position{Line: 2, Column: 5},
			Context: &ErrorContext{
				Lines: []ContextLine{
					{Number: 1, Content: "version: \"1.0\"", IsError: false},
					{Number: 2, Content: "agents", IsError: true},
					{Number: 3, Content: "  assistant:", IsError: false},
				},
				Highlight: HighlightInfo{StartColumn: 5, EndColumn: 11, Length: 6},
			},
			Category: "yaml",
		}

		output := err.Error()
		assert.Contains(t, output, "→   2 | agents")
		assert.Contains(t, output, "    1 | version: \"1.0\"")
		assert.Contains(t, output, "    3 |   assistant:")
		assert.Contains(t, output, "^^^^^^") // highlight indicator
	})

	t.Run("error with suggestion", func(t *testing.T) {
		err := &EnhancedError{
			ID:       "test_3_1",
			Severity: SeverityError,
			Title:    "Missing version field",
			Position: ast.Position{Line: 3, Column: 1},
			Suggestion: &ErrorSuggestion{
				Title:       "Add version field",
				Description: "Lacquer workflows require a version field set to \"1.0\"",
				Examples:    []string{"version: \"1.0\""},
				DocsURL:     "https://docs.lacquer.ai/reference/schema",
			},
			Category: "schema",
		}

		output := err.Error()
		assert.Contains(t, output, "Suggestion: Add version field")
		assert.Contains(t, output, "Example:")
		assert.Contains(t, output, "version: \"1.0\"")
		assert.Contains(t, output, "See: https://docs.lacquer.ai/reference/schema")
	})
}

func TestErrorReporter(t *testing.T) {
	source := []byte(`version: "1.0"

metadata:
  name test-workflow

agents:
  assistant:
    model: gpt-4

workflow:
  steps:
    - id: step1
      agent: unknown_agent
      prompt: "Hello"`)

	t.Run("add simple error", func(t *testing.T) {
		reporter := NewErrorReporter(source, "test.yaml")

		reporter.AddSimpleError("Unknown agent 'unknown_agent'", ast.Position{Line: 13, Column: 13}, "semantic")

		assert.True(t, reporter.HasErrors())
		assert.False(t, reporter.HasWarnings())

		errors := reporter.GetErrors()
		require.Len(t, errors, 1)

		err := errors[0]
		assert.Equal(t, SeverityError, err.Severity)
		assert.Equal(t, "Unknown agent 'unknown_agent'", err.Title)
		assert.Equal(t, "semantic", err.Category)
		assert.Equal(t, "test.yaml", err.Position.File)
		assert.NotNil(t, err.Context)
		assert.NotNil(t, err.Suggestion)
	})

	t.Run("build context", func(t *testing.T) {
		reporter := NewErrorReporter(source, "test.yaml")

		// Test context around line 4 (name test-workflow - missing colon)
		context := reporter.buildContext(ast.Position{Line: 4, Column: 8}, 2)

		require.NotNil(t, context)
		// The context includes more lines due to the radius, just check that we have content
		assert.GreaterOrEqual(t, len(context.Lines), 3)

		// Find the error line (line 4)
		var errorLine *ContextLine
		for i := range context.Lines {
			if context.Lines[i].Number == 4 {
				errorLine = &context.Lines[i]
				break
			}
		}

		require.NotNil(t, errorLine)
		assert.Equal(t, "  name test-workflow", errorLine.Content)
		assert.True(t, errorLine.IsError)
	})

	t.Run("to error conversion", func(t *testing.T) {
		reporter := NewErrorReporter(source, "test.yaml")

		// Add multiple errors
		reporter.AddSimpleError("First error", ast.Position{Line: 1, Column: 1}, "yaml")
		reporter.AddSimpleError("Second error", ast.Position{Line: 3, Column: 5}, "schema")

		err := reporter.ToError()
		require.NotNil(t, err)

		multiErr, ok := err.(*MultiErrorEnhanced)
		require.True(t, ok)

		assert.Len(t, multiErr.Errors, 2)
		assert.Equal(t, "test.yaml", multiErr.Filename)

		// Check that errors are sorted by position
		assert.Equal(t, 1, multiErr.Errors[0].Position.Line)
		assert.Equal(t, 3, multiErr.Errors[1].Position.Line)
	})

	t.Run("no errors returns nil", func(t *testing.T) {
		reporter := NewErrorReporter(source, "test.yaml")

		assert.False(t, reporter.HasErrors())
		assert.Nil(t, reporter.ToError())
	})
}

func TestErrorSuggestions(t *testing.T) {
	reporter := NewErrorReporter(nil, "test.yaml")

	t.Run("YAML suggestions", func(t *testing.T) {
		tests := []struct {
			message  string
			expected string
		}{
			{"indentation error", "Fix YAML indentation"},
			{"duplicate key found", "Remove duplicate keys"},
			{"cannot unmarshal string into int", "Check data type"},
			{"unknown yaml error", "Check YAML syntax"},
		}

		for _, tt := range tests {
			suggestion := reporter.generateYAMLSuggestion(tt.message)
			assert.Contains(t, suggestion.Title, tt.expected)
			assert.NotEmpty(t, suggestion.Description)
		}
	})

	t.Run("Schema suggestions", func(t *testing.T) {
		tests := []struct {
			message  string
			expected string
		}{
			{"missing version field", "Set the version field"},
			{"invalid agents definition", "Check agent definitions"},
			{"steps validation failed", "Check step definitions"},
			{"unknown schema error", "Check the schema requirements"},
		}

		for _, tt := range tests {
			suggestion := reporter.generateSchemaSuggestion(tt.message)
			assert.Contains(t, suggestion.Title, tt.expected)
			assert.NotEmpty(t, suggestion.Description)
		}
	})

	t.Run("Semantic suggestions", func(t *testing.T) {
		tests := []struct {
			message  string
			expected string
		}{
			{"agent 'foo' not found", "Define the agent"},
			{"circular dependency detected", "Remove circular dependencies"},
			{"undefined variable reference", "Check variable references"},
			{"unknown semantic error", "Check workflow logic"},
		}

		for _, tt := range tests {
			suggestion := reporter.generateSemanticSuggestion(tt.message)
			assert.Contains(t, suggestion.Title, tt.expected)
			assert.NotEmpty(t, suggestion.Description)
		}
	})
}

func TestMultiErrorEnhanced(t *testing.T) {
	t.Run("single error", func(t *testing.T) {
		err := &MultiErrorEnhanced{
			Errors: []*EnhancedError{
				{
					Title:    "Test error",
					Severity: SeverityError,
					Position: ast.Position{Line: 1, Column: 1},
				},
			},
			Filename: "test.yaml",
		}

		output := err.Error()
		assert.Contains(t, output, "parsing test.yaml:")
		assert.Contains(t, output, "error: Test error")
	})

	t.Run("multiple errors", func(t *testing.T) {
		err := &MultiErrorEnhanced{
			Errors: []*EnhancedError{
				{
					Title:    "First error",
					Severity: SeverityError,
					Position: ast.Position{Line: 1, Column: 1},
				},
				{
					Title:    "Second error",
					Severity: SeverityError,
					Position: ast.Position{Line: 2, Column: 1},
				},
			},
			Filename: "test.yaml",
		}

		output := err.Error()
		assert.Contains(t, output, "Multiple errors (2):")
		assert.Contains(t, output, "1. 1:1: error: First error")
		assert.Contains(t, output, "2. 2:1: error: Second error")
	})

	t.Run("get all issues", func(t *testing.T) {
		err := &MultiErrorEnhanced{
			Errors: []*EnhancedError{
				{Title: "Error 1", Severity: SeverityError},
			},
			Warnings: []*EnhancedError{
				{Title: "Warning 1", Severity: SeverityWarning},
			},
		}

		issues := err.GetAllIssues()
		assert.Len(t, issues, 2)
		assert.Equal(t, "Error 1", issues[0].Title)
		assert.Equal(t, "Warning 1", issues[1].Title)
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("generateErrorID", func(t *testing.T) {
		id := generateErrorID("yaml", ast.Position{Line: 5, Column: 10})
		assert.Equal(t, "yaml_5_10", id)
	})

	t.Run("isWordChar", func(t *testing.T) {
		assert.True(t, isWordChar('a'))
		assert.True(t, isWordChar('Z'))
		assert.True(t, isWordChar('5'))
		assert.True(t, isWordChar('_'))
		assert.True(t, isWordChar('-'))
		assert.False(t, isWordChar(' '))
		assert.False(t, isWordChar(':'))
		assert.False(t, isWordChar('.'))
	})

	t.Run("max and min", func(t *testing.T) {
		assert.Equal(t, 5, max(3, 5))
		assert.Equal(t, 3, max(3, 3))
		assert.Equal(t, 3, min(3, 5))
		assert.Equal(t, 3, min(3, 3))
	})
}

func TestErrorContextHighlighting(t *testing.T) {
	source := []byte(`version: "1.0"
metadata:
  name: test-workflow
agents:
  assistant:
    model: gpt-4`)

	reporter := NewErrorReporter(source, "test.yaml")

	t.Run("highlight word boundaries", func(t *testing.T) {
		// Test highlighting "assistant" on line 5, column 3
		context := reporter.buildContext(ast.Position{Line: 5, Column: 3}, 1)

		require.NotNil(t, context)
		assert.Equal(t, 3, context.Highlight.StartColumn)
		assert.Equal(t, 12, context.Highlight.EndColumn) // "assistant" is 9 chars
		assert.Equal(t, 9, context.Highlight.Length)
	})

	t.Run("highlight single character", func(t *testing.T) {
		// Test highlighting colon on line 1, column 8
		context := reporter.buildContext(ast.Position{Line: 1, Column: 8}, 0)

		require.NotNil(t, context)
		// The highlighting extends from the start of the word to the specified position
		assert.Equal(t, 1, context.Highlight.StartColumn)
		assert.Equal(t, 8, context.Highlight.EndColumn) // Including the colon at column 8
		assert.Equal(t, 7, context.Highlight.Length)    // Length is end - start
	})
}

// Integration test with real YAML content
func TestErrorReporter_Integration(t *testing.T) {
	invalidYaml := []byte(`version: "1.0"

metadata:
  name test-workflow  # Missing colon

agents:
  assistant
    model: gpt-4      # Wrong indentation

workflow:
  steps:
    - id: step1
      agent: nonexistent
      prompt: "test"`)

	reporter := NewErrorReporter(invalidYaml, "invalid.laq.yaml")

	// Simulate various errors that would be found during parsing
	reporter.AddSimpleError("Expected ':' after key", ast.Position{Line: 4, Column: 8}, "yaml")
	reporter.AddSimpleError("Wrong indentation level", ast.Position{Line: 7, Column: 5}, "yaml")
	reporter.AddSimpleError("Agent 'nonexistent' not found", ast.Position{Line: 12, Column: 13}, "semantic")

	assert.True(t, reporter.HasErrors())
	assert.Len(t, reporter.GetErrors(), 3)

	// Convert to error and check output
	err := reporter.ToError()
	require.NotNil(t, err)

	output := err.Error()

	// Check that all errors are included
	assert.Contains(t, output, "Expected ':' after key")
	assert.Contains(t, output, "Wrong indentation level")
	assert.Contains(t, output, "Agent 'nonexistent' not found")

	// Check that context is included
	assert.Contains(t, output, "name test-workflow")
	assert.Contains(t, output, "agent: nonexistent")

	// Check that suggestions are included
	assert.Contains(t, output, "Suggestion:")
}
