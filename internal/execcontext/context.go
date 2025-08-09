package execcontext

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/utils"
	"github.com/rs/zerolog"
)

// ExecutionContext contains all the state and metadata for a workflow execution
type ExecutionContext struct {
	Parent *ExecutionContext

	// Workflow information
	Workflow  *ast.Workflow
	RunID     string
	StartTime time.Time
	Cwd       string

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

	// Execution control
	Context RunContext
	Logger  zerolog.Logger

	// Thread safety
	mu sync.RWMutex
}

func (ec *ExecutionContext) Write(p []byte) (n int, err error) {
	return ec.Context.Write(p)
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

// NewExecutionContext creates a new execution context for a workflow
func NewExecutionContext(ctx RunContext, workflow *ast.Workflow, inputs map[string]interface{}, wd string) *ExecutionContext {
	runID := utils.GenerateRunID()

	workflowName := ""
	if workflow.Metadata != nil {
		workflowName = workflow.Metadata.Name
	}

	logger := zerolog.Ctx(ctx.Context).With().
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
		Cwd:         wd,
		Environment: utils.GetEnvironmentVars(),
		Metadata:    utils.BuildMetadata(workflow),
		Context:     ctx,
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

// NewChild creates a new execution context for a sub step execution
// This is used to create an execution context that is scoped to the
// sub steps and so will not pollute the parent execution context.
// The parent execution context is used to track the overall execution
// status, progress and state.
func (ec *ExecutionContext) NewChild(steps []*ast.Step) *ExecutionContext {
	return &ExecutionContext{
		Parent:      ec,
		Workflow:    ec.Workflow,
		RunID:       ec.RunID,
		StartTime:   time.Now(),
		Cwd:         ec.Cwd,
		Inputs:      ec.Inputs,
		State:       make(map[string]interface{}),
		Outputs:     make(map[string]interface{}),
		StepResults: make(map[string]*StepResult),
		Context:     ec.Context,
		Logger:      ec.Logger,
		TotalSteps:  len(steps),
		Environment: ec.Environment,
		Metadata:    ec.Metadata,
	}
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
	if ec.Parent != nil {
		return ec.Parent.GetState(key)
	}

	ec.mu.RLock()
	defer ec.mu.RUnlock()

	return ec.getNestedValue(ec.State, key)
}

// getNestedValue retrieves a value from a nested map structure using dot notation
func (ec *ExecutionContext) getNestedValue(target map[string]interface{}, key string) (interface{}, bool) {
	keys := strings.Split(key, ".")
	current := target

	// Navigate to the parent of the final key
	for _, k := range keys[:len(keys)-1] {
		if existing, exists := current[k]; exists {
			// Check if the existing value is a map
			if existingMap, ok := existing.(map[string]interface{}); ok {
				current = existingMap
			} else {
				// If it's not a map, the path doesn't exist
				return nil, false
			}
		} else {
			// Key doesn't exist in the path
			return nil, false
		}
	}

	// Get the final value
	finalKey := keys[len(keys)-1]
	value, exists := current[finalKey]
	return value, exists
}

// UpdateState updates multiple state variables
func (ec *ExecutionContext) UpdateState(updates map[string]interface{}) {
	if ec.Parent != nil {
		ec.Parent.UpdateState(updates)
		return
	}

	ec.mu.Lock()
	defer ec.mu.Unlock()

	for key, value := range updates {
		ec.setNestedValue(ec.State, key, value)
	}

	ec.Logger.Debug().
		Interface("updates", updates).
		Msg("State batch updated")
}

// setNestedValue sets a value in a nested map structure using dot notation
func (ec *ExecutionContext) setNestedValue(target map[string]interface{}, key string, value interface{}) {
	keys := strings.Split(key, ".")
	current := target

	for _, k := range keys[:len(keys)-1] {
		if existing, exists := current[k]; exists {
			if existingMap, ok := existing.(map[string]interface{}); ok {
				current = existingMap
			} else {
				newMap := make(map[string]interface{})
				current[k] = newMap
				current = newMap
			}
		} else {
			newMap := make(map[string]interface{})
			current[k] = newMap
			current = newMap
		}
	}

	finalKey := keys[len(keys)-1]
	current[finalKey] = value
}

// GetAllState returns a copy of all state variables
func (ec *ExecutionContext) GetAllState() map[string]interface{} {
	if ec.Parent != nil {
		return ec.Parent.GetAllState()
	}

	ec.mu.RLock()
	defer ec.mu.RUnlock()

	return utils.CopyMap(ec.State)
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

	return utils.CopyMap(ec.Outputs)
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
	case <-ec.Context.Context.Done():
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

	if summary.Status == ExecutionStatusCompleted || summary.Status == ExecutionStatusFailed {
		summary.EndTime = time.Now()
		summary.Duration = summary.EndTime.Sub(ec.StartTime)
	}

	for _, step := range ec.Workflow.Workflow.Steps {
		if result, exists := ec.StepResults[step.ID]; exists {
			summary.Steps = append(summary.Steps, *result)
		}
	}

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

// TokenUsage tracks token consumption for model API calls
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type RunContext struct {
	Context context.Context
	StdOut  io.Writer
	StdErr  io.Writer
}

func (rc RunContext) Write(p []byte) (n int, err error) {
	return rc.StdOut.Write(p)
}

func (rc RunContext) Printf(format string, v ...any) {
	fmt.Fprintf(rc.StdOut, format, v...)
}
