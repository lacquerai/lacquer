package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

// Validator validates Lacquer workflows against the JSON Schema
type Validator struct {
	schema *jsonschema.Schema
}

// ValidationError represents a validation error with context
type ValidationError struct {
	Message string      `json:"message"`
	Path    string      `json:"path"`
	Value   interface{} `json:"value,omitempty"`
}

// ValidationResult contains the results of workflow validation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// NewValidator creates a new schema validator
func NewValidator() (*Validator, error) {
	// Get the schema file path relative to this package
	_, currentFile, _, _ := runtime.Caller(0)
	schemaPath := filepath.Join(filepath.Dir(currentFile), "schema.json")

	// Read and compile the schema
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	// Parse the schema
	var schemaDoc interface{}
	if err := json.Unmarshal(schemaData, &schemaDoc); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	// Compile the schema
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://schemas.lacquer.ai/v1.0/workflow.json", strings.NewReader(string(schemaData))); err != nil {
		return nil, fmt.Errorf("failed to add schema resource: %w", err)
	}
	schema, err := compiler.Compile("https://schemas.lacquer.ai/v1.0/workflow.json")
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	return &Validator{schema: schema}, nil
}

// ValidateFile validates a workflow file
func (v *Validator) ValidateFile(filename string) (*ValidationResult, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	return v.ValidateBytes(data)
}

// ValidateBytes validates workflow data
func (v *Validator) ValidateBytes(data []byte) (*ValidationResult, error) {
	// Parse YAML to interface{}
	var workflow interface{}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Message: fmt.Sprintf("YAML parsing error: %v", err),
				Path:    "root",
			}},
		}, nil
	}

	// Validate against schema
	err := v.schema.Validate(workflow)
	if err == nil {
		return &ValidationResult{Valid: true}, nil
	}

	// Convert validation errors
	var validationErrors []ValidationError
	if validationErr, ok := err.(*jsonschema.ValidationError); ok {
		validationErrors = v.convertValidationErrors(validationErr)
	} else {
		validationErrors = []ValidationError{{
			Message: err.Error(),
			Path:    "root",
		}}
	}

	return &ValidationResult{
		Valid:  false,
		Errors: validationErrors,
	}, nil
}

// convertValidationErrors converts jsonschema validation errors to our format
func (v *Validator) convertValidationErrors(err *jsonschema.ValidationError) []ValidationError {
	var errors []ValidationError

	// Add the main error
	errors = append(errors, ValidationError{
		Message: err.Message,
		Path:    err.InstanceLocation,
		Value:   nil, // We'll skip the value for now to avoid interface issues
	})

	// Add any sub-errors recursively
	for _, subErr := range err.Causes {
		errors = append(errors, v.convertValidationErrors(subErr)...)
	}

	return errors
}

// ValidateWorkflowStruct validates a workflow structure for semantic correctness
func (v *Validator) ValidateWorkflowStruct(workflow interface{}) error {
	// TODO: Implement semantic validation
	// - Check agent references exist
	// - Validate step dependencies don't create cycles
	// - Check output references point to valid steps
	// - Validate variable interpolation syntax
	return nil
}
