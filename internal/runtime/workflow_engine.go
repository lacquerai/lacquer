package runtime

import (
	"context"
	"fmt"

	"github.com/lacquerai/lacquer/internal/ast"
	"gopkg.in/yaml.v3"
)

// RuntimeWorkflowEngine implements block.WorkflowEngine using runtime.Executor
type RuntimeWorkflowEngine struct {
	config        *ExecutorConfig
	modelRegistry *ModelRegistry
}

// NewRuntimeWorkflowEngine creates a new workflow engine using runtime.Executor
func NewRuntimeWorkflowEngine(config *ExecutorConfig, registry *ModelRegistry) *RuntimeWorkflowEngine {
	return &RuntimeWorkflowEngine{
		config:        config,
		modelRegistry: registry,
	}
}

// Execute executes a workflow definition with the given inputs
func (e *RuntimeWorkflowEngine) Execute(ctx context.Context, workflow interface{}, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Convert the workflow interface{} to ast.Workflow
	astWorkflow, err := e.convertToASTWorkflow(workflow, inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert workflow: %w", err)
	}

	// Create a new executor for this workflow
	executor, err := NewExecutor(e.config, astWorkflow, e.modelRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	// Create execution context for the child workflow
	execCtx := NewExecutionContext(ctx, astWorkflow, inputs)

	// Execute the workflow
	err = executor.ExecuteWorkflow(ctx, execCtx, nil) // nil progress channel for now
	if err != nil {
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	// Extract the outputs from the execution summary
	summary := execCtx.GetExecutionSummary()
	if summary.Status != "completed" {
		return nil, fmt.Errorf("workflow execution did not complete successfully: %s", summary.Status)
	}

	return summary.Outputs, nil
}

// convertToASTWorkflow converts the block workflow definition to ast.Workflow
func (e *RuntimeWorkflowEngine) convertToASTWorkflow(workflow interface{}, inputs map[string]interface{}) (*ast.Workflow, error) {
	// Convert the workflow interface{} to a complete workflow structure

	workflowMap, ok := workflow.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("workflow must be a map[string]interface{}")
	}

	// Create a complete workflow structure with version
	completeWorkflow := map[string]interface{}{
		"version": "1.0",
	}

	// Extract agents from workflow to top level if they exist
	workflowCopy := make(map[string]interface{})
	for k, v := range workflowMap {
		if k == "agents" {
			// Move agents to top level
			completeWorkflow["agents"] = v
		} else {
			workflowCopy[k] = v
		}
	}

	// Add inputs to the workflow definition if they exist
	if len(inputs) > 0 {
		// Create inputs definition based on provided inputs
		inputDefs := make(map[string]interface{})
		for key, value := range inputs {
			inputDef := map[string]interface{}{
				"type": inferType(value),
			}
			inputDefs[key] = inputDef
		}

		// Add inputs to the workflow if not already defined
		if _, hasInputs := workflowCopy["inputs"]; !hasInputs {
			workflowCopy["inputs"] = inputDefs
		}
	}

	completeWorkflow["workflow"] = workflowCopy

	// Marshal the complete workflow
	completeYAML, err := yaml.Marshal(completeWorkflow)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal complete workflow: %w", err)
	}

	// Unmarshal to ast.Workflow
	var astWorkflow ast.Workflow
	err = yaml.Unmarshal(completeYAML, &astWorkflow)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal to ast.Workflow: %w", err)
	}

	return &astWorkflow, nil
}

// inferType infers the YAML type from a Go value
func inferType(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64:
		return "integer"
	case float32, float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "string" // Default fallback
	}
}
