package parser

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lacquer/lacquer/internal/ast"
	"github.com/lacquer/lacquer/internal/parser/schema"
	"gopkg.in/yaml.v3"
)

// Parser interface defines the contract for workflow parsing
type Parser interface {
	ParseFile(filename string) (*ast.Workflow, error)
	ParseBytes(data []byte) (*ast.Workflow, error)
	ParseReader(r io.Reader) (*ast.Workflow, error)
	ValidateOnly(data []byte) error
	SetStrict(strict bool)
}

// YAMLParser implements the Parser interface using go-yaml/v3
type YAMLParser struct {
	validator *schema.Validator
	strict    bool
}

// ParserOption configures the YAML parser
type ParserOption func(*YAMLParser)

// WithStrict enables strict parsing mode
func WithStrict(strict bool) ParserOption {
	return func(p *YAMLParser) {
		p.strict = strict
	}
}

// WithValidator sets a custom schema validator
func WithValidator(validator *schema.Validator) ParserOption {
	return func(p *YAMLParser) {
		p.validator = validator
	}
}

// NewYAMLParser creates a new YAML parser with the given options
func NewYAMLParser(opts ...ParserOption) (*YAMLParser, error) {
	parser := &YAMLParser{
		strict: true, // Default to strict mode
	}
	
	// Apply options
	for _, opt := range opts {
		opt(parser)
	}
	
	// Create default validator if none provided
	if parser.validator == nil {
		validator, err := schema.NewValidator()
		if err != nil {
			return nil, fmt.Errorf("failed to create schema validator: %w", err)
		}
		parser.validator = validator
	}
	
	return parser, nil
}

// SetStrict enables or disables strict parsing mode
func (p *YAMLParser) SetStrict(strict bool) {
	p.strict = strict
}

// ParseFile parses a workflow file
func (p *YAMLParser) ParseFile(filename string) (*ast.Workflow, error) {
	// Validate file extension
	if !isValidWorkflowFile(filename) {
		return nil, fmt.Errorf("invalid file extension: expected .laq.yaml, got %s", filepath.Ext(filename))
	}
	
	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	
	// Validate file size (prevent DoS)
	if len(data) > 10*1024*1024 { // 10MB limit
		return nil, fmt.Errorf("file too large: %d bytes (max 10MB)", len(data))
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
	if len(data) == 0 {
		return nil, &ParseError{
			Message:  "empty workflow file",
			Position: ast.Position{Line: 1, Column: 1},
			Suggestion: "Add a basic workflow structure with version and workflow fields",
		}
	}
	
	// First, try to parse using yaml.Node to get position information
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, WrapYAMLError(err, data, "")
	}
	
	// Parse into workflow struct
	var workflow ast.Workflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, WrapYAMLError(err, data, "")
	}
	
	// Set position information from the root node
	if node.Line > 0 {
		workflow.Position = ast.Position{
			Line:   node.Line,
			Column: node.Column,
		}
	}
	
	// Validate against schema if validator is available
	if p.validator != nil {
		if err := p.validateWorkflow(data); err != nil {
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

// ValidateOnly validates workflow data without parsing
func (p *YAMLParser) ValidateOnly(data []byte) error {
	if p.validator == nil {
		return fmt.Errorf("no validator configured")
	}
	
	return p.validateWorkflow(data)
}

// validateWorkflow validates the workflow against the schema
func (p *YAMLParser) validateWorkflow(data []byte) error {
	result, err := p.validator.ValidateBytes(data)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	
	if !result.Valid {
		// Convert validation errors to our error format
		var multiErr MultiError
		for _, validationErr := range result.Errors {
			parseErr := &ParseError{
				Message:    validationErr.Message,
				Position:   extractPositionFromPath(validationErr.Path, data),
				Suggestion: generateSuggestion(validationErr.Message),
			}
			multiErr.Add(parseErr)
		}
		
		return multiErr.ToError()
	}
	
	return nil
}

// extractPositionFromPath attempts to find position from JSON path
func extractPositionFromPath(path string, source []byte) ast.Position {
	// This is a simplified implementation
	// A more sophisticated version would parse the JSON path and map it to YAML positions
	
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