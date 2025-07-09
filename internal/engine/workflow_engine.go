package engine

import (
	"fmt"
	"path/filepath"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/provider"
)

// RuntimeWorkflowEngine implements block.WorkflowEngine using runtime.Executor
type RuntimeWorkflowEngine struct {
	config        *ExecutorConfig
	modelRegistry *provider.Registry
}

// NewRuntimeWorkflowEngine creates a new workflow engine using engine.Executor
func NewRuntimeWorkflowEngine(config *ExecutorConfig, registry *provider.Registry) *RuntimeWorkflowEngine {
	return &RuntimeWorkflowEngine{
		config:        config,
		modelRegistry: registry,
	}
}

// Execute executes a workflow definition with the given inputs
func (e *RuntimeWorkflowEngine) Execute(execCtx *execcontext.ExecutionContext, workflow *ast.Workflow, inputs map[string]interface{}) (map[string]interface{}, error) {
	// Create a new executor for this workflow
	executor, err := NewExecutor(execCtx.Context, e.config, workflow, e.modelRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	// Create execution context for the child workflow
	childExecCtx := execcontext.NewExecutionContext(execCtx.Context, workflow, inputs, filepath.Dir(workflow.SourceFile))

	// Execute the workflow
	err = executor.ExecuteWorkflow(execCtx.Context, childExecCtx, nil) // nil progress channel for now
	if err != nil {
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	// Extract the outputs from the execution summary
	summary := childExecCtx.GetExecutionSummary()
	if summary.Status != "completed" {
		return nil, fmt.Errorf("workflow execution did not complete successfully: %s", summary.Status)
	}

	return summary.Outputs, nil
}
