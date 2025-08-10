// Package schema provides access to Lacquer workflow schema definitions and metadata.
// This package enables third-party applications to introspect Lacquer's capabilities,
// including the workflow schema structure, available expression syntax, built-in functions,
// and supported AI model providers.
//
// The schema information is essential for:
//   - Building workflow editors and IDEs with syntax validation
//   - Creating workflow validation tools
//   - Generating documentation for workflow syntax
//   - Discovering available AI models and providers
//   - Understanding supported expression and function syntax
//
// Example usage:
//
//	// Get the complete schema information
//	schema, err := GetSchema()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Access the JSON schema for workflow validation
//	var workflowSchema map[string]interface{}
//	json.Unmarshal(schema.Schema, &workflowSchema)
//
//	// List available AI models
//	for _, provider := range schema.ModelProviders {
//		fmt.Printf("Provider: %s\n", provider.Provider)
//		for _, model := range provider.Models {
//			fmt.Printf("  - %s\n", model)
//		}
//	}
package schema

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/expression"
	"github.com/lacquerai/lacquer/internal/provider"
	"github.com/lacquerai/lacquer/internal/provider/anthropic"
	"github.com/lacquerai/lacquer/internal/provider/openai"
)

// SchemaOutput represents the complete schema information for Lacquer workflows.
// This structure contains all the metadata needed to understand and validate
// Lacquer workflow definitions, including the JSON schema, expression syntax,
// available functions, and supported AI model providers.
type SchemaOutput struct {
	// Schema contains the JSON Schema definition for Lacquer workflow files.
	// This schema can be used to validate workflow YAML/JSON files and
	// provide autocompletion and validation in editors.
	Schema json.RawMessage `json:"schema"`
	// Expressions lists all available expression types and their syntax definitions.
	// This includes template expressions, conditional expressions, and data
	// transformation expressions that can be used within workflow definitions.
	Expressions []expression.ExpressionDef `json:"expressions"`
	// Functions contains all built-in functions available for use in expressions.
	// These functions provide data manipulation, string processing, mathematical
	// operations, and other utility functions within workflow expressions.
	Functions []*expression.FunctionDefinition `json:"functions"`
	// ModelProviders lists all available AI model providers and their supported models.
	// This information is essential for knowing which AI models can be referenced
	// in workflow agent configurations.
	ModelProviders []Model `json:"model_providers"`
}

// Model represents an AI model provider and its available models.
// This structure provides information about which AI models are accessible
// for use in workflow agent steps.
//
// The model information includes both the provider identifier and the list
// of model IDs that can be referenced in workflow definitions.
type Model struct {
	// Provider is the name/identifier of the AI model provider.
	// Common providers include "openai", "anthropic", and "local".
	Provider string `json:"provider"`

	// Models is a list of model identifiers available from this provider.
	// These IDs can be used directly in workflow agent configurations
	// to specify which model should handle the agent's tasks.
	//
	// Examples: ["gpt-4", "gpt-3.5-turbo"] for OpenAI,
	// ["claude-3-opus", "claude-3-sonnet"] for Anthropic.
	Models []string `json:"models"`
}

// GetSchema retrieves the complete schema information for Lacquer workflows.
// This function compiles and returns all metadata necessary for understanding
// and working with Lacquer workflow definitions.
//
// The function performs the following operations:
//   - Generates the base JSON schema for workflow validation
//   - Initializes all supported AI model providers
//   - Queries each provider for their available models
//   - Compiles expression and function definitions
//   - Returns the complete schema metadata
//
// This is the primary entry point for accessing Lacquer's schema information
// and is typically used by development tools, validators, and documentation
// generators.
//
// Returns:
//   - *SchemaOutput: Complete schema information including JSON schema,
//     expressions, functions, and available AI models
//   - error: Any error that occurred during schema generation or provider
//     initialization
//
// Errors can occur due to:
//   - Schema generation failures
//   - AI provider initialization issues
//   - Network failures when querying model availability
//   - Provider authentication or configuration problems
//
// Example:
//
//	schema, err := GetSchema()
//	if err != nil {
//		return fmt.Errorf("failed to get schema: %w", err)
//	}
//
//	// Use the schema for validation
//	var workflowSchema map[string]interface{}
//	if err := json.Unmarshal(schema.Schema, &workflowSchema); err != nil {
//		return fmt.Errorf("failed to parse schema: %w", err)
//	}
//
//	// Check available models
//	fmt.Printf("Available models: %d providers\n", len(schema.ModelProviders))
//	for _, provider := range schema.ModelProviders {
//		fmt.Printf("  %s: %d models\n", provider.Provider, len(provider.Models))
//	}
//
//	// Access expression definitions
//	fmt.Printf("Supported expressions: %d types\n", len(schema.Expressions))
//	fmt.Printf("Built-in functions: %d functions\n", len(schema.Functions))
func GetSchema() (*SchemaOutput, error) {
	schemaBytes, err := ast.NewSchema()
	if err != nil {
		return nil, fmt.Errorf("error creating base schema: %w", err)
	}

	openaiProvider, err := openai.NewProvider(nil)
	if err != nil {
		return nil, fmt.Errorf("error creating OpenAI provider: %w", err)
	}
	anthropicProvider, err := anthropic.NewProvider(nil)
	if err != nil {
		return nil, fmt.Errorf("error creating Anthropic provider: %w", err)
	}

	modelProviders := []provider.Provider{
		openaiProvider,
		anthropicProvider,
	}

	availableModels := []Model{}
	for _, modelProvider := range modelProviders {
		models, err := modelProvider.ListModels(context.Background())
		if err != nil {
			return nil, fmt.Errorf("error listing models for %s: %w", modelProvider.GetName(), err)
		}

		modelNames := []string{}
		for _, model := range models {
			modelNames = append(modelNames, model.ID)
		}

		availableModels = append(availableModels, Model{
			Provider: modelProvider.GetName(),
			Models:   modelNames,
		})
	}

	return &SchemaOutput{
		Schema:         json.RawMessage(schemaBytes),
		Expressions:    expression.ExpressionDefs,
		Functions:      expression.FunctionDefs,
		ModelProviders: availableModels,
	}, nil
}
