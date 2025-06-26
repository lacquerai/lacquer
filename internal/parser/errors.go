package parser

import (
	"fmt"
	"strings"

	"github.com/lacquerai/lacquer/internal/ast"
)

// ParseError represents a parsing error with context
type ParseError struct {
	Message    string       `json:"message"`
	Position   ast.Position `json:"position"`
	Context    string       `json:"context,omitempty"`
	Source     []byte       `json:"-"`
	Suggestion string       `json:"suggestion,omitempty"`
}

// Error implements the error interface
func (e *ParseError) Error() string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("Parse error at %s: %s", e.Position.String(), e.Message))

	if e.Suggestion != "" {
		result.WriteString(fmt.Sprintf("\nSuggestion: %s", e.Suggestion))
	}

	if e.Context != "" {
		result.WriteString(fmt.Sprintf("\n\nContext:\n%s", e.Context))
	}

	return result.String()
}

// extractPositionFromMessage attempts to extract line/column from error messages
func extractPositionFromMessage(message string, source []byte) ast.Position {
	// YAML error messages often contain "line X" patterns
	// This is a simple implementation - could be enhanced with regex

	lines := strings.Split(message, " ")
	for i, word := range lines {
		if word == "line" && i+1 < len(lines) {
			var line int
			if _, err := fmt.Sscanf(lines[i+1], "%d", &line); err == nil {
				return ast.Position{Line: line, Column: 1}
			}
		}
	}

	// Fallback to beginning of file
	return ast.Position{Line: 1, Column: 1}
}

// MultiError represents multiple parsing or validation errors
type MultiError struct {
	Errors []error `json:"errors"`
}

// Error implements the error interface for MultiError
func (e *MultiError) Error() string {
	if len(e.Errors) == 0 {
		return "no errors"
	}

	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Multiple errors (%d):\n", len(e.Errors)))

	for i, err := range e.Errors {
		result.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
	}

	return result.String()
}

// Add adds an error to the MultiError
func (e *MultiError) Add(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

// HasErrors returns true if there are any errors
func (e *MultiError) HasErrors() bool {
	return len(e.Errors) > 0
}

// ToError returns the MultiError as an error if there are errors, nil otherwise
func (e *MultiError) ToError() error {
	if !e.HasErrors() {
		return nil
	}
	return e
}
