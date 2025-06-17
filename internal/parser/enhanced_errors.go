package parser

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lacquerai/lacquer/internal/ast"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	SeverityError   ErrorSeverity = "error"
	SeverityWarning ErrorSeverity = "warning"
	SeverityInfo    ErrorSeverity = "info"
)

// EnhancedError represents a detailed error with rich context
type EnhancedError struct {
	ID            string           `json:"id"`
	Severity      ErrorSeverity    `json:"severity"`
	Title         string           `json:"title"`
	Message       string           `json:"message"`
	Position      ast.Position     `json:"position"`
	Context       *ErrorContext    `json:"context,omitempty"`
	Suggestion    *ErrorSuggestion `json:"suggestion,omitempty"`
	RelatedErrors []string         `json:"related_errors,omitempty"`
	Category      string           `json:"category"`
}

// ErrorContext provides source code context around the error
type ErrorContext struct {
	Lines      []ContextLine `json:"lines"`
	Highlight  HighlightInfo `json:"highlight"`
	SourceFile string        `json:"source_file,omitempty"`
}

// ContextLine represents a line of source code with context
type ContextLine struct {
	Number  int    `json:"number"`
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}

// HighlightInfo specifies how to highlight the error in the source
type HighlightInfo struct {
	StartColumn int `json:"start_column"`
	EndColumn   int `json:"end_column"`
	Length      int `json:"length"`
}

// ErrorSuggestion provides actionable advice for fixing the error
type ErrorSuggestion struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Fixes       []SuggestedFix `json:"fixes,omitempty"`
	Examples    []string       `json:"examples,omitempty"`
	DocsURL     string         `json:"docs_url,omitempty"`
}

// SuggestedFix represents a specific fix that could be applied
type SuggestedFix struct {
	Description string        `json:"description"`
	OldText     string        `json:"old_text,omitempty"`
	NewText     string        `json:"new_text"`
	Position    *ast.Position `json:"position,omitempty"`
}

// Error implements the error interface
func (e *EnhancedError) Error() string {
	var result strings.Builder

	// Main error line
	if e.Position.File != "" {
		result.WriteString(fmt.Sprintf("%s:%d:%d: ", e.Position.File, e.Position.Line, e.Position.Column))
	} else {
		result.WriteString(fmt.Sprintf("%d:%d: ", e.Position.Line, e.Position.Column))
	}

	// Severity and title
	result.WriteString(fmt.Sprintf("%s: %s", e.Severity, e.Title))

	if e.Message != "" && e.Message != e.Title {
		result.WriteString(fmt.Sprintf("\n%s", e.Message))
	}

	// Context lines
	if e.Context != nil && len(e.Context.Lines) > 0 {
		result.WriteString("\n\n")
		for _, line := range e.Context.Lines {
			if line.IsError {
				result.WriteString(fmt.Sprintf("→ %3d | %s\n", line.Number, line.Content))
				// Add highlight indicator
				if e.Context.Highlight.StartColumn > 0 {
					padding := strings.Repeat(" ", 7+e.Context.Highlight.StartColumn-1)
					highlight := strings.Repeat("^", max(1, e.Context.Highlight.Length))
					result.WriteString(fmt.Sprintf("%s%s\n", padding, highlight))
				}
			} else {
				result.WriteString(fmt.Sprintf("  %3d | %s\n", line.Number, line.Content))
			}
		}
	}

	// Suggestion
	if e.Suggestion != nil {
		result.WriteString(fmt.Sprintf("\nSuggestion: %s", e.Suggestion.Title))
		if e.Suggestion.Description != "" {
			result.WriteString(fmt.Sprintf("\n%s", e.Suggestion.Description))
		}

		// Examples
		if len(e.Suggestion.Examples) > 0 {
			result.WriteString("\n\nExample:")
			for _, example := range e.Suggestion.Examples {
				result.WriteString(fmt.Sprintf("\n  %s", example))
			}
		}

		// Documentation link
		if e.Suggestion.DocsURL != "" {
			result.WriteString(fmt.Sprintf("\n\nSee: %s", e.Suggestion.DocsURL))
		}
	}

	return result.String()
}

// ErrorReporter collects and formats multiple errors
type ErrorReporter struct {
	errors   []*EnhancedError
	warnings []*EnhancedError
	source   []byte
	filename string
}

// NewErrorReporter creates a new error reporter
func NewErrorReporter(source []byte, filename string) *ErrorReporter {
	return &ErrorReporter{
		errors:   make([]*EnhancedError, 0),
		warnings: make([]*EnhancedError, 0),
		source:   source,
		filename: filename,
	}
}

// AddError adds an error to the reporter
func (r *ErrorReporter) AddError(err *EnhancedError) {
	if err.Position.File == "" && r.filename != "" {
		err.Position.File = r.filename
	}

	// Add context if not present
	if err.Context == nil && r.source != nil {
		err.Context = r.buildContext(err.Position, 2)
	}

	switch err.Severity {
	case SeverityError:
		r.errors = append(r.errors, err)
	case SeverityWarning:
		r.warnings = append(r.warnings, err)
	}
}

// AddSimpleError adds a simple error with automatic enhancement
func (r *ErrorReporter) AddSimpleError(message string, pos ast.Position, category string) {
	err := &EnhancedError{
		ID:         generateErrorID(category, pos),
		Severity:   SeverityError,
		Title:      message,
		Message:    "",
		Position:   pos,
		Category:   category,
		Suggestion: r.generateSuggestion(message, category),
	}
	r.AddError(err)
}

// HasErrors returns true if there are any errors
func (r *ErrorReporter) HasErrors() bool {
	return len(r.errors) > 0
}

// HasWarnings returns true if there are any warnings
func (r *ErrorReporter) HasWarnings() bool {
	return len(r.warnings) > 0
}

// GetErrors returns all errors
func (r *ErrorReporter) GetErrors() []*EnhancedError {
	return r.errors
}

// GetWarnings returns all warnings
func (r *ErrorReporter) GetWarnings() []*EnhancedError {
	return r.warnings
}

// ToError converts the reporter to a standard error if there are errors
func (r *ErrorReporter) ToError() error {
	if !r.HasErrors() {
		return nil
	}

	// Sort errors by position
	sort.Slice(r.errors, func(i, j int) bool {
		a, b := r.errors[i].Position, r.errors[j].Position
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})

	return &MultiErrorEnhanced{
		Errors:   r.errors,
		Warnings: r.warnings,
		Filename: r.filename,
	}
}

// buildContext creates error context around the given position
func (r *ErrorReporter) buildContext(pos ast.Position, radius int) *ErrorContext {
	if r.source == nil {
		return nil
	}

	lines := strings.Split(string(r.source), "\n")
	if pos.Line < 1 || pos.Line > len(lines) {
		return nil
	}

	start := max(1, pos.Line-radius)
	end := min(len(lines), pos.Line+radius)

	contextLines := make([]ContextLine, 0, end-start+1)
	for i := start; i <= end; i++ {
		if i > len(lines) {
			break
		}
		contextLines = append(contextLines, ContextLine{
			Number:  i,
			Content: lines[i-1], // lines are 0-indexed, but line numbers are 1-indexed
			IsError: i == pos.Line,
		})
	}

	// Calculate highlight info
	highlight := HighlightInfo{
		StartColumn: pos.Column,
		EndColumn:   pos.Column,
		Length:      1,
	}

	// Try to highlight the whole word/token if possible
	if pos.Line <= len(lines) && pos.Column > 0 {
		line := lines[pos.Line-1]
		if pos.Column <= len(line) {
			// Find word boundaries
			start := pos.Column - 1
			end := pos.Column - 1

			// Extend backwards
			for start > 0 && isWordChar(line[start-1]) {
				start--
			}

			// Extend forwards
			for end < len(line) && isWordChar(line[end]) {
				end++
			}

			if end > start {
				highlight.StartColumn = start + 1
				highlight.EndColumn = end + 1
				highlight.Length = end - start
			}
		}
	}

	return &ErrorContext{
		Lines:      contextLines,
		Highlight:  highlight,
		SourceFile: r.filename,
	}
}

// generateSuggestion creates helpful suggestions based on error patterns
func (r *ErrorReporter) generateSuggestion(message, category string) *ErrorSuggestion {
	message = strings.ToLower(message)

	switch category {
	case "yaml":
		return r.generateYAMLSuggestion(message)
	case "schema":
		return r.generateSchemaSuggestion(message)
	case "semantic":
		return r.generateSemanticSuggestion(message)
	case "validation":
		return r.generateValidationSuggestion(message)
	default:
		return &ErrorSuggestion{
			Title:       "Check the documentation",
			Description: "Refer to the Lacquer documentation for syntax examples and troubleshooting",
			DocsURL:     "https://docs.lacquer.ai",
		}
	}
}

// generateYAMLSuggestion generates suggestions for YAML parsing errors
func (r *ErrorReporter) generateYAMLSuggestion(message string) *ErrorSuggestion {
	switch {
	case strings.Contains(message, "indent"):
		return &ErrorSuggestion{
			Title:       "Fix YAML indentation",
			Description: "YAML requires consistent indentation using spaces (not tabs)",
			Examples: []string{
				"workflow:",
				"  steps:",
				"    - id: example",
				"      agent: my_agent",
			},
			DocsURL: "https://docs.lacquer.ai/syntax/yaml-basics",
		}
	case strings.Contains(message, "duplicate"):
		return &ErrorSuggestion{
			Title:       "Remove duplicate keys",
			Description: "Each key in a YAML object must be unique",
			Examples: []string{
				"# Good:",
				"agents:",
				"  assistant: { model: gpt-4 }",
				"  researcher: { model: claude-3-opus }",
			},
		}
	case strings.Contains(message, "cannot unmarshal"):
		return &ErrorSuggestion{
			Title:       "Check data type",
			Description: "The value type doesn't match what's expected",
			Examples: []string{
				"# String values need quotes:",
				"version: \"1.0\"",
				"# Numbers don't:",
				"temperature: 0.7",
			},
		}
	default:
		return &ErrorSuggestion{
			Title:       "Check YAML syntax",
			Description: "Ensure proper YAML formatting with colons, dashes, and consistent indentation",
			DocsURL:     "https://docs.lacquer.ai/syntax/yaml-basics",
		}
	}
}

// generateSchemaSuggestion generates suggestions for schema validation errors
func (r *ErrorReporter) generateSchemaSuggestion(message string) *ErrorSuggestion {
	switch {
	case strings.Contains(message, "version"):
		return &ErrorSuggestion{
			Title:       "Set the version field",
			Description: "The version field is required and must be set to \"1.0\"",
			Fixes: []SuggestedFix{
				{
					Description: "Add version field at the top",
					NewText:     "version: \"1.0\"",
				},
			},
			Examples: []string{
				"version: \"1.0\"",
				"",
				"metadata:",
				"  name: my-workflow",
			},
		}
	case strings.Contains(message, "agents"):
		return &ErrorSuggestion{
			Title:       "Check agent definitions",
			Description: "Each agent must have a model or uses field",
			Examples: []string{
				"agents:",
				"  assistant:",
				"    model: gpt-4",
				"    temperature: 0.7",
			},
			DocsURL: "https://docs.lacquer.ai/concepts/agents",
		}
	case strings.Contains(message, "steps"):
		return &ErrorSuggestion{
			Title:       "Check step definitions",
			Description: "Each step needs an id and either agent+prompt, uses, or action",
			Examples: []string{
				"steps:",
				"  - id: greeting",
				"    agent: assistant",
				"    prompt: \"Hello, world!\"",
			},
			DocsURL: "https://docs.lacquer.ai/concepts/steps",
		}
	default:
		return &ErrorSuggestion{
			Title:       "Check the schema requirements",
			Description: "Refer to the Lacquer schema documentation for required fields",
			DocsURL:     "https://docs.lacquer.ai/reference/schema",
		}
	}
}

// generateSemanticSuggestion generates suggestions for semantic validation errors
func (r *ErrorReporter) generateSemanticSuggestion(message string) *ErrorSuggestion {
	switch {
	case strings.Contains(message, "agent") && strings.Contains(message, "not found"):
		return &ErrorSuggestion{
			Title:       "Define the agent",
			Description: "The step references an agent that hasn't been defined",
			Examples: []string{
				"agents:",
				"  my_agent:",
				"    model: gpt-4",
				"",
				"workflow:",
				"  steps:",
				"    - id: step1",
				"      agent: my_agent  # Must match agent name",
			},
		}
	case strings.Contains(message, "circular"):
		return &ErrorSuggestion{
			Title:       "Remove circular dependencies",
			Description: "Steps cannot depend on each other in a circle",
			Examples: []string{
				"# Bad: step1 → step2 → step1",
				"# Good: step1 → step2 → step3",
			},
		}
	case strings.Contains(message, "variable"):
		return &ErrorSuggestion{
			Title:       "Check variable references",
			Description: "Variables must be defined before use",
			Examples: []string{
				"# Reference step outputs:",
				"prompt: \"Previous result: {{ steps.step1.output }}\"",
				"# Reference inputs:",
				"prompt: \"Topic: {{ inputs.topic }}\"",
			},
			DocsURL: "https://docs.lacquer.ai/concepts/variables",
		}
	default:
		return &ErrorSuggestion{
			Title:       "Check workflow logic",
			Description: "Review the workflow structure and dependencies",
			DocsURL:     "https://docs.lacquer.ai/concepts/workflows",
		}
	}
}

// generateValidationSuggestion generates suggestions for general validation errors
func (r *ErrorReporter) generateValidationSuggestion(message string) *ErrorSuggestion {
	return &ErrorSuggestion{
		Title:       "Check validation requirements",
		Description: "Review the error message and check against Lacquer requirements",
		DocsURL:     "https://docs.lacquer.ai/reference/validation",
	}
}

// MultiErrorEnhanced represents multiple enhanced errors
type MultiErrorEnhanced struct {
	Errors   []*EnhancedError `json:"errors"`
	Warnings []*EnhancedError `json:"warnings"`
	Filename string           `json:"filename,omitempty"`
}

// Error implements the error interface
func (e *MultiErrorEnhanced) Error() string {
	if len(e.Errors) == 0 {
		return "no errors"
	}

	var result strings.Builder

	if e.Filename != "" {
		result.WriteString(fmt.Sprintf("  parsing %s: ", e.Filename))
	}

	if len(e.Errors) == 1 {
		result.WriteString(e.Errors[0].Error())
	} else {
		result.WriteString(fmt.Sprintf("Multiple errors (%d):\n", len(e.Errors)))
		for i, err := range e.Errors {
			result.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
		}
	}

	return result.String()
}

// GetAllIssues returns both errors and warnings
func (e *MultiErrorEnhanced) GetAllIssues() []*EnhancedError {
	all := make([]*EnhancedError, 0, len(e.Errors)+len(e.Warnings))
	all = append(all, e.Errors...)
	all = append(all, e.Warnings...)
	return all
}

// Helper functions
func generateErrorID(category string, pos ast.Position) string {
	return fmt.Sprintf("%s_%d_%d", category, pos.Line, pos.Column)
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
