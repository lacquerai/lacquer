package parser

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseError represents a parsing error with context
type ParseError struct {
	Message    string   `json:"message"`
	Position   Position `json:"position"`
	Context    string   `json:"context,omitempty"`
	Source     []byte   `json:"-"`
	Suggestion string   `json:"suggestion,omitempty"`
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

// WrapYAMLError wraps a YAML parsing error with additional context
func WrapYAMLError(err error, source []byte, filename string) error {
	if err == nil {
		return nil
	}
	
	// Handle different types of YAML errors
	switch yamlErr := err.(type) {
	case *yaml.TypeError:
		return handleTypeError(yamlErr, source, filename)
	default:
		// Try to extract position from error message
		position := extractPositionFromMessage(err.Error(), source)
		if filename != "" {
			position.File = filename
		}
		
		return &ParseError{
			Message:  err.Error(),
			Position: position,
			Context:  ExtractContext(source, position, 2),
			Source:   source,
			Suggestion: generateSuggestion(err.Error()),
		}
	}
}

// handleTypeError handles YAML type errors specifically
func handleTypeError(err *yaml.TypeError, source []byte, filename string) error {
	// For type errors, we'll use the first error in the list
	if len(err.Errors) == 0 {
		position := Position{Line: 1, Column: 1}
		if filename != "" {
			position.File = filename
		}
		return &ParseError{
			Message:  "YAML type error",
			Position: position,
			Source:   source,
		}
	}
	
	// Parse the first error message to extract position
	firstError := err.Errors[0]
	position := extractPositionFromMessage(firstError, source)
	if filename != "" {
		position.File = filename
	}
	
	return &ParseError{
		Message:    fmt.Sprintf("Type error: %s", firstError),
		Position:   position,
		Context:    ExtractContext(source, position, 2),
		Source:     source,
		Suggestion: generateSuggestion(firstError),
	}
}

// extractPositionFromMessage attempts to extract line/column from error messages
func extractPositionFromMessage(message string, source []byte) Position {
	// YAML error messages often contain "line X" patterns
	// This is a simple implementation - could be enhanced with regex
	
	lines := strings.Split(message, " ")
	for i, word := range lines {
		if word == "line" && i+1 < len(lines) {
			var line int
			if _, err := fmt.Sscanf(lines[i+1], "%d", &line); err == nil {
				return Position{Line: line, Column: 1}
			}
		}
	}
	
	// Fallback to beginning of file
	return Position{Line: 1, Column: 1}
}

// generateSuggestion provides helpful suggestions based on common errors
func generateSuggestion(errorMessage string) string {
	message := strings.ToLower(errorMessage)
	
	switch {
	case strings.Contains(message, "cannot unmarshal"):
		return "Check that the field type matches the expected value (string, number, boolean, array, or object)"
	case strings.Contains(message, "field") && strings.Contains(message, "not found"):
		return "Check for typos in field names or refer to the Lacquer DSL documentation for valid fields"
	case strings.Contains(message, "duplicate"):
		return "Remove the duplicate key or use a different name"
	case strings.Contains(message, "indent"):
		return "Check YAML indentation - use spaces, not tabs, and ensure consistent indentation"
	case strings.Contains(message, "expected"):
		return "Check YAML syntax - ensure proper use of colons, dashes, and quotes"
	case strings.Contains(message, "version"):
		return "Ensure the version field is set to \"1.0\" (in quotes)"
	case strings.Contains(message, "agents"):
		return "Check agent definitions - ensure each agent has a valid model specified"
	case strings.Contains(message, "steps"):
		return "Check workflow steps - each step needs an 'id' and either 'agent'+'prompt', 'uses', or 'action'"
	default:
		return "Check the YAML syntax and refer to the Lacquer documentation for examples"
	}
}

// ValidationError wraps validation errors from the schema validator
type ValidationError struct {
	Path       string `json:"path"`
	Message    string `json:"message"`
	Value      interface{} `json:"value,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
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