package block

import (
	"context"
	"fmt"
)

// WorkflowEngine interface for workflow execution
type WorkflowEngine interface {
	Execute(ctx context.Context, workflow interface{}, inputs map[string]interface{}) (map[string]interface{}, error)
}

// NativeExecutor executes native Lacquer blocks (YAML workflows)
type NativeExecutor struct {
	engine WorkflowEngine
}

// NewNativeExecutor creates a new native block executor
func NewNativeExecutor(engine WorkflowEngine) *NativeExecutor {
	return &NativeExecutor{
		engine: engine,
	}
}

// Validate checks if the executor can handle the given block
func (e *NativeExecutor) Validate(block *Block) error {
	if block.Runtime != RuntimeNative {
		return fmt.Errorf("invalid runtime for native executor: %s", block.Runtime)
	}
	if block.Workflow == nil {
		return fmt.Errorf("native block missing workflow definition")
	}
	return nil
}

// Execute runs a native block
func (e *NativeExecutor) Execute(ctx context.Context, block *Block, inputs map[string]interface{}, execCtx *ExecutionContext) (map[string]interface{}, error) {
	// 1. Validate and map inputs according to block schema
	mappedInputs, err := e.validateAndMapInputs(block, inputs)
	if err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// 2. Execute the workflow using the engine
	outputs, err := e.engine.Execute(ctx, block.Workflow, mappedInputs)
	if err != nil {
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	return outputs, nil
}

// validateAndMapInputs validates inputs against block schema and applies defaults
func (e *NativeExecutor) validateAndMapInputs(block *Block, inputs map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Process each defined input schema
	for inputName, schema := range block.Inputs {
		value, exists := inputs[inputName]

		if !exists {
			// Check if input is required
			if schema.Required {
				return nil, fmt.Errorf("required input '%s' is missing", inputName)
			}

			// Apply default value if available
			if schema.Default != nil {
				result[inputName] = schema.Default
			}
			// If no default and not required, skip this input
			continue
		}

		// TODO: Add type validation here
		// For now, accept the value as-is
		result[inputName] = value
	}

	// For context isolation, we only pass inputs that are explicitly defined
	// in the block's input schema. This prevents parent workflow state from
	// leaking into child workflows.

	return result, nil
}
