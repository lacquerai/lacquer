package block

import (
	"fmt"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/execcontext"
)

// WorkflowEngine interface for workflow execution
type WorkflowEngine interface {
	Execute(execCtx *execcontext.ExecutionContext, workflow *ast.Workflow, inputs map[string]interface{}) (map[string]interface{}, error)
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
func (e *NativeExecutor) Execute(execCtx *execcontext.ExecutionContext, block *Block, inputs map[string]interface{}) (interface{}, error) {
	mappedInputs, err := e.validateAndMapInputs(block, inputs)
	if err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	outputs, err := e.engine.Execute(execCtx, block.Workflow, mappedInputs)
	if err != nil {
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	return outputs, nil
}

// validateAndMapInputs validates inputs against block schema and applies defaults
func (e *NativeExecutor) validateAndMapInputs(block *Block, inputs map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for inputName, schema := range block.Inputs {
		value, exists := inputs[inputName]

		if !exists {
			if schema.Required {
				return nil, fmt.Errorf("required input '%s' is missing", inputName)
			}

			if schema.Default != nil {
				result[inputName] = schema.Default
			}
			continue
		}

		// TODO: Add type validation here
		// For now, accept the value as-is
		result[inputName] = value
	}

	return result, nil
}
