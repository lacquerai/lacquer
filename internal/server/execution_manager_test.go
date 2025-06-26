package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/lacquerai/lacquer/internal/runtime"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestExecutionManager_NewManager(t *testing.T) {
	// Use a separate registry for tests to avoid conflicts
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(5, registry)

	assert.NotNil(t, manager)
	assert.Equal(t, 5, manager.maxConcurrency)
	assert.Equal(t, 0, manager.currentCount)
	assert.Equal(t, 0, manager.GetActiveExecutions())
	assert.True(t, manager.CanStartExecution())
}

func TestExecutionManager_StartExecution(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(2, registry)

	inputs := map[string]any{"test": "value"}

	status := manager.StartExecution("run-123", "workflow-test", inputs)

	assert.NotNil(t, status)
	assert.Equal(t, "run-123", status.RunID)
	assert.Equal(t, "workflow-test", status.WorkflowID)
	assert.Equal(t, "running", status.Status)
	assert.Equal(t, inputs, status.Inputs)
	assert.NotEmpty(t, status.StartTime)
	assert.Nil(t, status.EndTime)
	assert.Empty(t, status.Progress)

	assert.Equal(t, 1, manager.GetActiveExecutions())
	assert.True(t, manager.CanStartExecution())

	// Verify execution can be retrieved
	retrieved, exists := manager.GetExecution("run-123")
	assert.True(t, exists)
	assert.Equal(t, status, retrieved)
}

func TestExecutionManager_ConcurrencyLimit(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(2, registry)

	// Start first execution
	status1 := manager.StartExecution("run-1", "workflow-1", map[string]any{})
	assert.NotNil(t, status1)
	assert.True(t, manager.CanStartExecution())
	assert.Equal(t, 1, manager.GetActiveExecutions())

	// Start second execution
	status2 := manager.StartExecution("run-2", "workflow-2", map[string]any{})
	assert.NotNil(t, status2)
	assert.False(t, manager.CanStartExecution()) // Should be at capacity
	assert.Equal(t, 2, manager.GetActiveExecutions())

	// Finish first execution
	manager.FinishExecution("run-1", map[string]any{"result": "success"}, nil)
	assert.True(t, manager.CanStartExecution()) // Should have capacity again
	assert.Equal(t, 1, manager.GetActiveExecutions())

	// Check first execution status
	finished, exists := manager.GetExecution("run-1")
	assert.True(t, exists)
	assert.Equal(t, "completed", finished.Status)
	assert.NotNil(t, finished.EndTime)
	assert.Greater(t, finished.Duration, time.Duration(0))
	assert.Equal(t, map[string]any{"result": "success"}, finished.Outputs)
	assert.Empty(t, finished.Error)
}

func TestExecutionManager_FinishExecutionWithError(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(1, registry)

	status := manager.StartExecution("run-error", "workflow-error", map[string]any{})
	assert.Equal(t, "running", status.Status)

	// Finish with error
	testError := assert.AnError
	manager.FinishExecution("run-error", nil, testError)

	finished, exists := manager.GetExecution("run-error")
	assert.True(t, exists)
	assert.Equal(t, "failed", finished.Status)
	assert.Nil(t, finished.Outputs)
	assert.Equal(t, testError.Error(), finished.Error)
	assert.NotNil(t, finished.EndTime)

	assert.Equal(t, 0, manager.GetActiveExecutions())
	assert.True(t, manager.CanStartExecution())
}

func TestExecutionManager_GetExecution_NotFound(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(1, registry)

	execution, exists := manager.GetExecution("non-existent")
	assert.False(t, exists)
	assert.Nil(t, execution)
}

func TestExecutionManager_AddProgressEvent(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(1, registry)

	status := manager.StartExecution("run-progress", "workflow-progress", map[string]any{})
	assert.Empty(t, status.Progress)

	// Add progress event
	event := runtime.ExecutionEvent{
		Type:      runtime.EventStepStarted,
		Timestamp: time.Now(),
		RunID:     "run-progress",
		StepID:    "step-1",
	}

	manager.AddProgressEvent("run-progress", event)

	updated, exists := manager.GetExecution("run-progress")
	assert.True(t, exists)
	assert.Len(t, updated.Progress, 1)
	assert.Equal(t, event, updated.Progress[0])

	// Add another event
	event2 := runtime.ExecutionEvent{
		Type:      runtime.EventStepCompleted,
		Timestamp: time.Now(),
		RunID:     "run-progress",
		StepID:    "step-1",
	}

	manager.AddProgressEvent("run-progress", event2)

	updated, exists = manager.GetExecution("run-progress")
	assert.True(t, exists)
	assert.Len(t, updated.Progress, 2)
	assert.Equal(t, event2, updated.Progress[1])
}

func TestExecutionManager_AddProgressEvent_NonExistentExecution(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(1, registry)

	event := runtime.ExecutionEvent{
		Type:      runtime.EventStepStarted,
		Timestamp: time.Now(),
		RunID:     "non-existent",
		StepID:    "step-1",
	}

	// Should not panic or error when adding event to non-existent execution
	manager.AddProgressEvent("non-existent", event)
}

func TestExecutionManager_FinishExecution_NonExistent(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(1, registry)

	// Should not panic or error when finishing non-existent execution
	manager.FinishExecution("non-existent", nil, nil)

	assert.Equal(t, 0, manager.GetActiveExecutions())
}

func TestExecutionManager_MultipleExecutions(t *testing.T) {
	registry := prometheus.NewRegistry()
	manager := NewExecutionManagerWithRegistry(5, registry)

	// Start multiple executions
	for i := 0; i < 3; i++ {
		runID := fmt.Sprintf("run-%d", i)
		workflowID := fmt.Sprintf("workflow-%d", i)
		inputs := map[string]any{"index": i}

		status := manager.StartExecution(runID, workflowID, inputs)
		assert.NotNil(t, status)
		assert.Equal(t, runID, status.RunID)
		assert.Equal(t, workflowID, status.WorkflowID)
	}

	assert.Equal(t, 3, manager.GetActiveExecutions())
	assert.True(t, manager.CanStartExecution())

	// Finish executions in different order
	manager.FinishExecution("run-1", map[string]any{"result": 1}, nil)
	assert.Equal(t, 2, manager.GetActiveExecutions())

	manager.FinishExecution("run-0", map[string]any{"result": 0}, nil)
	assert.Equal(t, 1, manager.GetActiveExecutions())

	manager.FinishExecution("run-2", nil, assert.AnError)
	assert.Equal(t, 0, manager.GetActiveExecutions())

	// Verify all executions are in correct state
	exec0, exists0 := manager.GetExecution("run-0")
	assert.True(t, exists0)
	assert.Equal(t, "completed", exec0.Status)

	exec1, exists1 := manager.GetExecution("run-1")
	assert.True(t, exists1)
	assert.Equal(t, "completed", exec1.Status)

	exec2, exists2 := manager.GetExecution("run-2")
	assert.True(t, exists2)
	assert.Equal(t, "failed", exec2.Status)
}
