package engine

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/provider"
	pkgEvents "github.com/lacquerai/lacquer/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWorkflowExecutor is a mock implementation of WorkflowExecutor for testing
type mockWorkflowExecutor struct {
	events []pkgEvents.ExecutionEvent
}

func (m *mockWorkflowExecutor) ExecuteWorkflow(execCtx *execcontext.ExecutionContext, progressChan chan<- pkgEvents.ExecutionEvent) error {
	if progressChan != nil && len(m.events) > 0 {
		for _, event := range m.events {
			progressChan <- event
		}
	}

	return nil
}

// mockExecutorFunc creates a new mock executor
func mockExecutorFunc(events []pkgEvents.ExecutionEvent) ExecutorFunc {
	return func(ctx execcontext.RunContext, config *ExecutorConfig, workflow *ast.Workflow, registry *provider.Registry, runner *Runner) (WorkflowExecutor, error) {
		return &mockWorkflowExecutor{events: events}, nil
	}
}

func TestRunWorkflow_Success(t *testing.T) {
	runner := NewRunner(nil, WithExecutorFunc(mockExecutorFunc([]pkgEvents.ExecutionEvent{
		{
			Type:      pkgEvents.EventStepStarted,
			StepID:    "test_step",
			StepIndex: 1,
		},
		{
			Type:     pkgEvents.EventStepCompleted,
			StepID:   "test_step",
			Duration: 10 * time.Millisecond,
		},
	})))
	defer runner.Close()

	ctx := execcontext.RunContext{
		Context: context.Background(),
		StdOut:  os.Stdout,
		StdErr:  os.Stderr,
	}
	workflowFile := filepath.Join("testdata", "basic_workflow.laq.yml")
	inputs := map[string]interface{}{
		"name": "World",
	}

	result, err := runner.RunWorkflow(ctx, workflowFile, inputs)

	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, workflowFile, result.WorkflowFile)
	assert.Equal(t, "World", result.Inputs["name"])
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.Equal(t, 1, result.StepsTotal)
}

func TestRunWorkflow_WithProgressTracker(t *testing.T) {
	t.Setenv("LACQUER_TEST", "true")

	out := &bytes.Buffer{}
	progressTracker := NewProgressTracker(out, "", 1)
	runner := NewRunner(progressTracker, WithExecutorFunc(mockExecutorFunc([]pkgEvents.ExecutionEvent{
		{
			Type:      pkgEvents.EventStepStarted,
			StepID:    "test_step",
			StepIndex: 1,
		},
		{
			Type:     pkgEvents.EventStepCompleted,
			StepID:   "test_step",
			Duration: 10 * time.Millisecond,
		},
	})))
	defer runner.Close()

	ctx := execcontext.RunContext{
		Context: context.Background(),
		StdOut:  out,
		StdErr:  out,
	}
	workflowFile := filepath.Join("testdata", "basic_workflow.laq.yml")
	inputs := map[string]interface{}{
		"name": "World",
	}

	result, err := runner.RunWorkflow(ctx, workflowFile, inputs)

	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	time.Sleep(10 * time.Millisecond)

	snaps.MatchSnapshot(t, out.String())
}
