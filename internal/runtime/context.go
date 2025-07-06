package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/rs/zerolog"
)

// ExecutionContext contains all the state and metadata for a workflow execution
type ExecutionContext struct {
	// Workflow information
	Workflow  *ast.Workflow
	RunID     string
	StartTime time.Time

	// Input parameters and state
	Inputs  map[string]interface{}
	State   map[string]interface{}
	Outputs map[string]interface{}

	// Step execution tracking
	StepResults      map[string]*StepResult
	CurrentStepIndex int
	TotalSteps       int

	// Environment and metadata
	Environment map[string]string
	Metadata    map[string]interface{}
	Matrix      map[string]interface{}

	// Execution control
	Context context.Context
	Cancel  context.CancelFunc
	Logger  zerolog.Logger

	// Thread safety
	mu sync.RWMutex
}

// StepResult represents the result of executing a single step
type StepResult struct {
	StepID     string                 `json:"step_id"`
	Status     StepStatus             `json:"status"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time"`
	Duration   time.Duration          `json:"duration"`
	Output     map[string]interface{} `json:"output"`
	Response   string                 `json:"response,omitempty"`
	Error      error                  `json:"error,omitempty"`
	TokenUsage *TokenUsage            `json:"token_usage,omitempty"`
	Retries    int                    `json:"retries"`
}

// StepStatus represents the execution status of a step
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

// TokenUsage tracks token consumption for model API calls
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewExecutionContext creates a new execution context for a workflow
func NewExecutionContext(ctx context.Context, workflow *ast.Workflow, inputs map[string]interface{}) *ExecutionContext {
	runID := generateRunID()
	execCtx, cancel := context.WithCancel(ctx)

	workflowName := ""
	if workflow.Metadata != nil {
		workflowName = workflow.Metadata.Name
	}

	logger := zerolog.Ctx(ctx).With().
		Str("workflow", workflowName).
		Str("run_id", runID).
		Logger()

	execContext := &ExecutionContext{
		Workflow:    workflow,
		RunID:       runID,
		StartTime:   time.Now(),
		Inputs:      inputs,
		State:       make(map[string]interface{}),
		Outputs:     make(map[string]interface{}),
		StepResults: make(map[string]*StepResult),
		Environment: getEnvironmentVars(),
		Metadata:    buildMetadata(workflow),
		Matrix:      make(map[string]interface{}),
		Context:     execCtx,
		Cancel:      cancel,
		Logger:      logger,
		TotalSteps:  len(workflow.Workflow.Steps),
	}

	// Initialize state with workflow defaults
	if workflow.Workflow.State != nil {
		for k, v := range workflow.Workflow.State {
			execContext.State[k] = v
		}
	}

	// Initialize step results
	for _, step := range workflow.Workflow.Steps {
		execContext.StepResults[step.ID] = &StepResult{
			StepID: step.ID,
			Status: StepStatusPending,
		}
	}

	return execContext
}

// GetInput returns an input parameter value
func (ec *ExecutionContext) GetInput(key string) (interface{}, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	value, exists := ec.Inputs[key]
	return value, exists
}

// GetState returns a state variable value
func (ec *ExecutionContext) GetState(key string) (interface{}, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	value, exists := ec.State[key]
	return value, exists
}

// SetState updates a state variable
func (ec *ExecutionContext) SetState(key string, value interface{}) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.State[key] = value
	ec.Logger.Debug().
		Str("key", key).
		Interface("value", value).
		Msg("State updated")
}

// UpdateState updates multiple state variables
func (ec *ExecutionContext) UpdateState(updates map[string]interface{}) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	for key, value := range updates {
		ec.State[key] = value
	}

	ec.Logger.Debug().
		Interface("updates", updates).
		Msg("State batch updated")
}

// GetAllState returns a copy of all state variables
func (ec *ExecutionContext) GetAllState() map[string]interface{} {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	return CopyMap(ec.State)
}

// SetWorkflowOutputs updates the workflow outputs
func (ec *ExecutionContext) SetWorkflowOutputs(outputs map[string]interface{}) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.Outputs = outputs
	ec.Logger.Debug().
		Interface("outputs", outputs).
		Msg("Workflow outputs set")
}

// GetWorkflowOutputs returns a copy of workflow outputs
func (ec *ExecutionContext) GetWorkflowOutputs() map[string]interface{} {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	return CopyMap(ec.Outputs)
}

// GetStepResult returns the result of a specific step
func (ec *ExecutionContext) GetStepResult(stepID string) (*StepResult, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	result, exists := ec.StepResults[stepID]
	return result, exists
}

// SetStepResult updates the result for a specific step
func (ec *ExecutionContext) SetStepResult(stepID string, result *StepResult) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.StepResults[stepID] = result

	ec.Logger.Debug().
		Str("step_id", stepID).
		Str("status", string(result.Status)).
		Dur("duration", result.Duration).
		Msg("Step result updated")
}

// GetEnvironment returns an environment variable value
func (ec *ExecutionContext) GetEnvironment(key string) (string, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	value, exists := ec.Environment[key]
	return value, exists
}

// GetMetadata returns a metadata value
func (ec *ExecutionContext) GetMetadata(key string) (interface{}, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	value, exists := ec.Metadata[key]
	return value, exists
}

// IncrementCurrentStep advances to the next step
func (ec *ExecutionContext) IncrementCurrentStep() {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.CurrentStepIndex++
}

// IsCompleted returns true if all steps have been executed
func (ec *ExecutionContext) IsCompleted() bool {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	return ec.CurrentStepIndex >= ec.TotalSteps
}

// IsCancelled returns true if the execution context has been cancelled
func (ec *ExecutionContext) IsCancelled() bool {
	select {
	case <-ec.Context.Done():
		return true
	default:
		return false
	}
}

// GetExecutionSummary returns a summary of the execution
func (ec *ExecutionContext) GetExecutionSummary() ExecutionSummary {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	summary := ExecutionSummary{
		RunID:     ec.RunID,
		StartTime: ec.StartTime,
		Status:    ec.getOverallStatus(),
		Steps:     make([]StepResult, 0, len(ec.StepResults)),
		Inputs:    ec.Inputs,
		State:     ec.State,
		Outputs:   ec.Outputs,
	}

	// Calculate duration if completed
	if summary.Status == ExecutionStatusCompleted || summary.Status == ExecutionStatusFailed {
		summary.EndTime = time.Now()
		summary.Duration = summary.EndTime.Sub(ec.StartTime)
	}

	// Add step results in order
	for _, step := range ec.Workflow.Workflow.Steps {
		if result, exists := ec.StepResults[step.ID]; exists {
			summary.Steps = append(summary.Steps, *result)
		}
	}

	// Calculate token usage
	for _, result := range ec.StepResults {
		if result.TokenUsage != nil {
			summary.TotalTokens += result.TokenUsage.TotalTokens
		}
	}

	return summary
}

// getOverallStatus determines the overall execution status
func (ec *ExecutionContext) getOverallStatus() ExecutionStatus {
	if ec.IsCancelled() {
		return ExecutionStatusCancelled
	}

	hasRunning := false
	hasFailed := false
	allCompleted := true

	for _, result := range ec.StepResults {
		switch result.Status {
		case StepStatusRunning:
			hasRunning = true
			allCompleted = false
		case StepStatusFailed:
			hasFailed = true
			allCompleted = false
		case StepStatusPending:
			allCompleted = false
		case StepStatusCompleted:
			// Step is completed, continue
		case StepStatusSkipped:
			// Step is skipped, continue
		default:
			allCompleted = false
		}
	}

	if hasFailed {
		return ExecutionStatusFailed
	}
	if hasRunning {
		return ExecutionStatusRunning
	}
	if allCompleted && ec.IsCompleted() {
		return ExecutionStatusCompleted
	}

	return ExecutionStatusRunning
}

// ExecutionSummary provides a summary of workflow execution
type ExecutionSummary struct {
	RunID         string                 `json:"run_id"`
	Status        ExecutionStatus        `json:"status"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       time.Time              `json:"end_time,omitempty"`
	Duration      time.Duration          `json:"duration"`
	Steps         []StepResult           `json:"steps"`
	Inputs        map[string]interface{} `json:"inputs"`
	State         map[string]interface{} `json:"state"`
	Outputs       map[string]interface{} `json:"outputs,omitempty"`
	TotalTokens   int                    `json:"total_tokens"`
	EstimatedCost float64                `json:"estimated_cost"`
}

// ExecutionStatus represents the overall execution status
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)
