package parser

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lacquerai/lacquer/internal/ast"
	"gopkg.in/yaml.v3"
)

// Parser interface defines the contract for workflow parsing
type Parser interface {
	ParseFile(filename string) (*ast.Workflow, error)
	ParseBytes(data []byte) (*ast.Workflow, error)
	ParseReader(r io.Reader) (*ast.Workflow, error)
}

// YAMLParser implements the Parser interface using go-yaml/v3
type YAMLParser struct {
	semanticValidator *SemanticValidator
}

// ParserOption configures the YAML parser
type ParserOption func(*YAMLParser)

// WithSemanticValidator sets a custom semantic validator
func WithSemanticValidator(validator *SemanticValidator) ParserOption {
	return func(p *YAMLParser) {
		p.semanticValidator = validator
	}
}

// NewYAMLParser creates a new YAML parser with the given options
func NewYAMLParser(opts ...ParserOption) (*YAMLParser, error) {
	parser := &YAMLParser{}

	// Apply options
	for _, opt := range opts {
		opt(parser)
	}

	// Create semantic validator
	if parser.semanticValidator == nil {
		parser.semanticValidator = NewSemanticValidator()
	}

	return parser, nil
}

// ParseFile parses a workflow file
func (p *YAMLParser) ParseFile(filename string) (*ast.Workflow, error) {
	reporter := NewErrorReporter(nil, filename)

	// Validate file extension
	if !isValidWorkflowFile(filename) {
		reporter.AddError(&EnhancedError{
			ID:       "file_ext_invalid",
			Severity: SeverityError,
			Title:    "Invalid file extension",
			Message:  fmt.Sprintf("Expected .laq.yaml or .laq.yml, got %s", filepath.Ext(filename)),
			Position: ast.Position{Line: 1, Column: 1, File: filename},
			Category: "file",
			Suggestion: &ErrorSuggestion{
				Title:       "Use correct file extension",
				Description: "Lacquer workflow files must have .laq.yaml or .laq.yml extension",
				Examples:    []string{"my-workflow.laq.yaml", "pipeline.laq.yml"},
				DocsURL:     "https://docs.lacquer.ai/concepts/files",
			},
		})
		return nil, reporter.ToError()
	}

	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		reporter.AddError(&EnhancedError{
			ID:       "file_read_error",
			Severity: SeverityError,
			Title:    "Cannot read file",
			Message:  err.Error(),
			Position: ast.Position{Line: 1, Column: 1, File: filename},
			Category: "file",
			Suggestion: &ErrorSuggestion{
				Title:       "Check file permissions",
				Description: "Ensure the file exists and is readable",
			},
		})
		return nil, reporter.ToError()
	}

	// Update reporter with source data
	reporter.source = data

	// Validate file size (prevent DoS)
	if len(data) > 10*1024*1024 { // 10MB limit
		reporter.AddError(&EnhancedError{
			ID:       "file_too_large",
			Severity: SeverityError,
			Title:    "File too large",
			Message:  fmt.Sprintf("File size %d bytes exceeds maximum of 10MB", len(data)),
			Position: ast.Position{Line: 1, Column: 1, File: filename},
			Category: "file",
			Suggestion: &ErrorSuggestion{
				Title:       "Reduce file size",
				Description: "Large workflow files can cause performance issues. Consider splitting into smaller workflows.",
			},
		})
		return nil, reporter.ToError()
	}

	// Parse the workflow
	workflow, err := p.ParseBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filename, err)
	}

	// Set source file
	workflow.SourceFile = filename
	workflow.Position.File = filename

	return workflow, nil
}

// ParseBytes parses workflow data from bytes
func (p *YAMLParser) ParseBytes(data []byte) (*ast.Workflow, error) {
	reporter := NewErrorReporter(data, "")

	if len(data) == 0 {
		reporter.AddError(&EnhancedError{
			ID:       "empty_file",
			Severity: SeverityError,
			Title:    "Empty workflow file",
			Message:  "Workflow file contains no content",
			Position: ast.Position{Line: 1, Column: 1},
			Category: "yaml",
			Suggestion: &ErrorSuggestion{
				Title:       "Add basic workflow structure",
				Description: "Lacquer workflows require at minimum a version and workflow section",
				Examples: []string{
					"version: \"1.0\"",
					"",
					"workflow:",
					"  steps:",
					"    - id: hello",
					"      prompt: \"Hello, world!\"",
				},
				DocsURL: "https://docs.lacquer.ai/getting-started",
			},
		})
		return nil, reporter.ToError()
	}

	// First, try to parse using yaml.Node to get position information
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, p.enhanceYAMLError(err, data, reporter)
	}

	// Parse into workflow struct
	var workflow ast.Workflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, p.enhanceYAMLError(err, data, reporter)
	}

	// Set position information from the root node
	if node.Line > 0 {
		workflow.Position = ast.Position{
			Line:   node.Line,
			Column: node.Column,
		}
	}

	// Perform semantic validation
	if p.semanticValidator != nil {
		if err := p.validateSemanticsEnhanced(&workflow, reporter); err != nil {
			return nil, err
		}
	}

	return &workflow, nil
}

// ParseReader parses workflow data from a reader
func (p *YAMLParser) ParseReader(r io.Reader) (*ast.Workflow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	return p.ParseBytes(data)
}

// validateSemantics performs semantic validation on the parsed workflow
func (p *YAMLParser) validateSemantics(workflow *ast.Workflow) error {
	result := p.semanticValidator.ValidateWorkflow(workflow)
	if result.HasErrors() {
		var multiErr MultiError
		for _, validationErr := range result.Errors {
			parseErr := &ParseError{
				Message:    validationErr.Message,
				Position:   ast.Position{Line: 1, Column: 1}, // TODO: Extract position from validation error path
				Suggestion: generateSemanticSuggestion(validationErr.Message),
			}
			multiErr.Add(parseErr)
		}
		return multiErr.ToError()
	}
	return nil
}

// generateSemanticSuggestion provides suggestions for semantic validation errors
func generateSemanticSuggestion(errorMessage string) string {
	message := strings.ToLower(errorMessage)

	switch {
	case strings.Contains(message, "circular dependency"):
		return "Remove the circular reference by reordering steps or using intermediate variables"
	case strings.Contains(message, "forward reference"):
		return "Steps can only reference outputs from previous steps. Reorder your steps or use workflow state"
	case strings.Contains(message, "undefined variable"):
		return "Check that the variable is defined in inputs, state, or previous step outputs"
	case strings.Contains(message, "block reference"):
		return "Use format: lacquer/block-name@version, github.com/owner/repo@tag, or ./local/path"
	case strings.Contains(message, "parentheses"):
		return "Ensure all opening parentheses have matching closing parentheses"
	case strings.Contains(message, "agent"):
		return "Ensure the agent is defined in the agents section before using it in steps"
	default:
		return "Check the workflow structure and refer to the Lacquer documentation"
	}
}

// extractPositionFromPath attempts to find position from JSON path
func extractPositionFromPath(path string, source []byte) ast.Position {
	if path == "" || path == "/" {
		return ast.Position{Line: 1, Column: 1}
	}

	// Parse the YAML into a tree structure that preserves line numbers
	var node yaml.Node
	if err := yaml.Unmarshal(source, &node); err != nil {
		return ast.Position{Line: 1, Column: 1}
	}

	// Navigate through the path to find the target node
	pos := findNodeByPath(&node, path)
	if pos.Line > 0 {
		return pos
	}

	// Fallback to the simple method if YAML parsing fails
	return extractPositionFromPathSimple(path, source)
}

// findNodeByPath navigates through a YAML node tree using a path
func findNodeByPath(node *yaml.Node, path string) ast.Position {
	if path == "" || path == "/" {
		return ast.Position{Line: node.Line, Column: node.Column}
	}

	// Convert different path formats to a standard format
	pathParts := parsePath(path)

	// Start from the document node (should be the first node)
	current := node
	if current.Kind == yaml.DocumentNode && len(current.Content) > 0 {
		current = current.Content[0]
	}

	// Navigate through each path part
	for _, part := range pathParts {
		if current == nil {
			break
		}

		switch current.Kind {
		case yaml.MappingNode:
			// Look for the key in the mapping
			found := false
			for i := 0; i < len(current.Content); i += 2 {
				key := current.Content[i]
				value := current.Content[i+1]

				if key.Value == part {
					current = value
					found = true
					break
				}
			}
			if !found {
				return ast.Position{Line: current.Line, Column: current.Column}
			}

		case yaml.SequenceNode:
			// Parse the part as an array index
			index := 0
			if _, err := fmt.Sscanf(part, "%d", &index); err != nil {
				return ast.Position{Line: current.Line, Column: current.Column}
			}

			if index >= 0 && index < len(current.Content) {
				current = current.Content[index]
			} else {
				return ast.Position{Line: current.Line, Column: current.Column}
			}

		default:
			// For scalar nodes or unknown types, return current position
			return ast.Position{Line: current.Line, Column: current.Column}
		}
	}

	if current != nil {
		return ast.Position{Line: current.Line, Column: current.Column}
	}

	return ast.Position{Line: 1, Column: 1}
}

// parsePath converts different path formats to a uniform slice of parts
func parsePath(path string) []string {
	// Handle JSON schema style paths like "/workflow/steps/0/agent"
	if strings.HasPrefix(path, "/") {
		return strings.Split(strings.TrimPrefix(path, "/"), "/")
	}

	// Handle dot notation with square brackets like "workflow.steps[0].agent"
	// Replace [index] with .index format first
	re := regexp.MustCompile(`\[(\d+)\]`)
	normalized := re.ReplaceAllString(path, ".$1")

	// Split by dots
	return strings.Split(normalized, ".")
}

// extractPositionFromPathSimple is the fallback simple implementation
func extractPositionFromPathSimple(path string, source []byte) ast.Position {
	if path == "" || path == "/" {
		return ast.Position{Line: 1, Column: 1}
	}

	// Remove leading slash and split path
	pathParts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	// Try to find the field in the YAML
	lines := strings.Split(string(source), "\n")
	for lineNum, line := range lines {
		for _, part := range pathParts {
			if strings.Contains(line, part+":") {
				return ast.Position{
					Line:   lineNum + 1,
					Column: strings.Index(line, part) + 1,
				}
			}
		}
	}

	return ast.Position{Line: 1, Column: 1}
}

// isValidWorkflowFile checks if the filename has a valid extension
func isValidWorkflowFile(filename string) bool {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filepath.Base(filename), ext)

	// Check for .laq.yaml extension
	if ext == ".yaml" && strings.HasSuffix(base, ".laq") {
		return true
	}

	// Also accept .yml variant
	if ext == ".yml" && strings.HasSuffix(base, ".laq") {
		return true
	}

	return false
}

// ParseWorkflowNode parses a workflow from a YAML node (for advanced use cases)
func (p *YAMLParser) ParseWorkflowNode(node *yaml.Node) (*ast.Workflow, error) {
	var workflow ast.Workflow
	if err := node.Decode(&workflow); err != nil {
		return nil, fmt.Errorf("failed to decode workflow: %w", err)
	}

	workflow.Position = ast.Position{
		Line:   node.Line,
		Column: node.Column,
	}

	return &workflow, nil
}

// GetSupportedExtensions returns the list of supported file extensions
func GetSupportedExtensions() []string {
	return []string{".laq.yaml", ".laq.yml"}
}

// enhanceYAMLError converts a YAML error to an enhanced error
func (p *YAMLParser) enhanceYAMLError(err error, data []byte, reporter *ErrorReporter) error {
	switch yamlErr := err.(type) {
	case *yaml.TypeError:
		if len(yamlErr.Errors) > 0 {
			for _, errMsg := range yamlErr.Errors {
				pos := extractPositionFromMessage(errMsg, data)
				reporter.AddError(&EnhancedError{
					ID:         generateErrorID("yaml_type", pos),
					Severity:   SeverityError,
					Title:      "YAML type error",
					Message:    errMsg,
					Position:   pos,
					Category:   "yaml",
					Suggestion: reporter.generateYAMLSuggestion(errMsg),
				})
			}
		}
	default:
		pos := extractPositionFromMessage(err.Error(), data)
		reporter.AddError(&EnhancedError{
			ID:         generateErrorID("yaml_parse", pos),
			Severity:   SeverityError,
			Title:      "YAML parsing error",
			Message:    err.Error(),
			Position:   pos,
			Category:   "yaml",
			Suggestion: reporter.generateYAMLSuggestion(err.Error()),
		})
	}

	return reporter.ToError()
}

// validateSemanticsEnhanced performs semantic validation with enhanced error reporting
func (p *YAMLParser) validateSemanticsEnhanced(workflow *ast.Workflow, reporter *ErrorReporter) error {
	result := p.semanticValidator.ValidateWorkflow(workflow)
	if result.HasErrors() {
		for _, validationErr := range result.Errors {
			// Try to extract position from the validation error context
			pos := ast.Position{Line: 1, Column: 1}
			if validationErr.Path != "" {
				pos = extractPositionFromPath(validationErr.Path, reporter.source)
			}

			reporter.AddError(&EnhancedError{
				ID:         generateErrorID("semantic", pos),
				Severity:   SeverityError,
				Title:      "Semantic validation error",
				Message:    validationErr.Message,
				Position:   pos,
				Category:   "semantic",
				Suggestion: reporter.generateSemanticSuggestion(validationErr.Message),
			})
		}
		return reporter.ToError()
	}

	return nil
}
