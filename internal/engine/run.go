package engine

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/parser"
	"github.com/lacquerai/lacquer/internal/style"
	pkgEvents "github.com/lacquerai/lacquer/pkg/events"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// ExecutionResult represents the result of running a workflow
type ExecutionResult struct {
	WorkflowFile string                 `json:"workflow_file" yaml:"workflow_file"`
	RunID        string                 `json:"run_id" yaml:"run_id"`
	Status       string                 `json:"status" yaml:"status"`
	StartTime    time.Time              `json:"start_time" yaml:"start_time"`
	EndTime      time.Time              `json:"end_time,omitempty" yaml:"end_time,omitempty"`
	Duration     time.Duration          `json:"duration" yaml:"duration"`
	StepsTotal   int                    `json:"steps_total" yaml:"steps_total"`
	StepResults  []StepExecutionResult  `json:"step_results,omitempty" yaml:"step_results,omitempty"`
	Inputs       map[string]interface{} `json:"inputs" yaml:"inputs"`
	Outputs      map[string]interface{} `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	FinalState   map[string]interface{} `json:"final_state,omitempty" yaml:"final_state,omitempty"`
	Error        string                 `json:"error,omitempty" yaml:"error,omitempty"`
	TokenUsage   *TokenUsageSummary     `json:"token_usage,omitempty" yaml:"token_usage,omitempty"`
}

// StepExecutionResult represents the result of executing a single step
type StepExecutionResult struct {
	StepID     string                 `json:"step_id" yaml:"step_id"`
	Status     string                 `json:"status" yaml:"status"`
	StartTime  time.Time              `json:"start_time" yaml:"start_time"`
	EndTime    time.Time              `json:"end_time,omitempty" yaml:"end_time,omitempty"`
	Duration   time.Duration          `json:"duration" yaml:"duration"`
	Output     map[string]interface{} `json:"output,omitempty" yaml:"output,omitempty"`
	Response   string                 `json:"response,omitempty" yaml:"response,omitempty"`
	Error      string                 `json:"error,omitempty" yaml:"error,omitempty"`
	Retries    int                    `json:"retries" yaml:"retries"`
	TokenUsage *TokenUsage            `json:"token_usage,omitempty" yaml:"token_usage,omitempty"`
}

// TokenUsageSummary represents aggregated token usage across all steps
type TokenUsageSummary struct {
	TotalTokens      int `json:"total_tokens" yaml:"total_tokens"`
	PromptTokens     int `json:"prompt_tokens" yaml:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens" yaml:"completion_tokens"`
}

// TokenUsage represents token usage for a single step
type TokenUsage struct {
	PromptTokens     int     `json:"prompt_tokens" yaml:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens" yaml:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens" yaml:"total_tokens"`
	EstimatedCost    float64 `json:"estimated_cost" yaml:"estimated_cost"`
}

// StepProgressState tracks the visual state of each step for spinner display
type StepProgressState struct {
	stepID     string
	stepIndex  int
	totalSteps int
	status     string // "running", "completed", "failed"
	startTime  time.Time
	endTime    time.Time
	title      string
	spinner    style.Spinner
	actions    ActionStates
	mu         sync.RWMutex
}

func (s *StepProgressState) String() string {
	return fmt.Sprintf(" %s\n%s", s.title, s.actions.String())
}

type ActionStates []ActionState

func (as *ActionStates) Add(action ActionState) {
	newActions := make(ActionStates, len(*as)+1)
	// make sure the previous actions are all marked as success if they are not already
	for i, a := range *as {
		if !strings.HasPrefix(a.text, style.SuccessIcon()) && !strings.HasPrefix(a.text, style.ErrorIcon()) {
			a.text = style.SuccessIcon() + " " + strings.TrimSpace(strings.ReplaceAll(a.text, "...", ""))
		}

		newActions[i] = a
	}

	newActions[len(*as)] = action
	*as = newActions
}

func (as ActionStates) String() string {
	var text strings.Builder
	for _, action := range as {
		text.WriteString(fmt.Sprintf("   %s", action.text))
		text.WriteString("\n")
	}
	return text.String()
}

type ActionState struct {
	id   string
	text string
}

type Runner struct {
	progressListener pkgEvents.Listener
}

func NewRunner(progressListener pkgEvents.Listener) *Runner {
	return &Runner{
		progressListener: progressListener,
	}
}

func (r *Runner) SetProgressListener(listener pkgEvents.Listener) {
	r.progressListener = listener
}

func (r *Runner) RunWorkflowRaw(execCtx *execcontext.ExecutionContext, workflow *ast.Workflow, startTime time.Time, prefix ...string) (*ExecutionResult, error) {
	executorConfig := &ExecutorConfig{
		MaxConcurrentSteps: 3,
		DefaultTimeout:     5 * time.Minute,
		EnableRetries:      true,
	}
	executor, err := NewExecutor(execCtx.Context, executorConfig, workflow, nil, r)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	result := ExecutionResult{
		WorkflowFile: workflow.SourceFile,
		RunID:        execCtx.RunID,
		Status:       "running",
		StartTime:    startTime,
		Inputs:       execCtx.Inputs,
		StepsTotal:   len(workflow.Workflow.Steps),
	}

	err = r.executeWithProgress(executor, execCtx, &result)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()

		log.Error().
			Err(err).
			Str("run_id", execCtx.RunID).
			Dur("duration", result.Duration).
			Msg("Workflow execution failed")

		return nil, err
	} else {
		result.Status = "completed"
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.FinalState = execCtx.GetAllState()
		result.Outputs = execCtx.GetWorkflowOutputs()

		log.Info().
			Str("run_id", execCtx.RunID).
			Dur("duration", result.Duration).
			Msg("Workflow execution completed successfully")
	}

	collectExecutionResults(execCtx, &result)

	return &result, nil
}

func (r *Runner) RunWorkflow(ctx execcontext.RunContext, workflowFile string, inputs map[string]interface{}, prefix ...string) (*ExecutionResult, error) {
	startTime := time.Now()

	yamlParser, err := parser.NewYAMLParser()
	if err != nil {
		style.Error(ctx, fmt.Sprintf("Failed to create parser: %v", err))
		return nil, err
	}

	workflow, err := yamlParser.ParseFile(workflowFile)
	if err != nil {
		return nil, err
	}

	// add prefix to step IDs if provided
	// this is used to identify the step in the progress tracker if the workflow is run
	// in a nested workflow
	// if prefix != nil {
	// 	prefixStr := strings.Join(prefix, ".")
	// 	for i, step := range workflow.Workflow.Steps {
	// 		workflow.Workflow.Steps[i].ID = fmt.Sprintf("%s.%s", prefixStr, step.ID)
	// 	}
	// }

	log.Info().
		Str("version", workflow.Version).
		Int("steps", len(workflow.Workflow.Steps)).
		Msg("Workflow loaded and validated")

	workflowInputs := make(map[string]interface{})
	for k, v := range inputs {
		workflowInputs[k] = v
	}

	for k, v := range workflow.Inputs {
		if _, ok := workflowInputs[k]; !ok && v.Default != nil {
			workflowInputs[k] = v.Default
		}
	}

	validationResult := ValidateWorkflowInputs(workflow, workflowInputs)
	if !validationResult.Valid {
		return nil, validationResult
	}

	// Show workflow info
	if !viper.GetBool("quiet") && viper.GetString("output") == "text" {
		printWorkflowInfo(ctx, workflow)
	}

	// Create executor with configuration
	wd := filepath.Dir(workflow.SourceFile)
	execCtx := execcontext.NewExecutionContext(ctx, workflow, workflowInputs, wd)
	if v, ok := r.progressListener.(*CLIProgressTracker); ok {
		v.totalSteps = len(workflow.Workflow.Steps)
	}

	return r.RunWorkflowRaw(execCtx, workflow, startTime, prefix...)
}

func (r *Runner) executeWithProgress(executor *Executor, execCtx *execcontext.ExecutionContext, result *ExecutionResult) error {
	// Create a progress channel for real-time updates
	progressChan := make(chan pkgEvents.ExecutionEvent, 100)
	defer close(progressChan)

	if r.progressListener != nil {
		go r.progressListener.StartListening(progressChan)
	}

	err := executor.ExecuteWorkflow(execCtx, progressChan)
	return err
}

func (r *Runner) Close() {
	if r.progressListener != nil {
		r.progressListener.StopListening()
	}
}

// ProgressTracker manages the visual progress display for all steps
type CLIProgressTracker struct {
	steps      map[string]*StepProgressState
	mu         sync.RWMutex
	writer     io.Writer
	totalSteps int
	done       chan struct{}
	prefix     string
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(writer io.Writer, prefix string, totalSteps int) *CLIProgressTracker {
	return &CLIProgressTracker{
		steps:      make(map[string]*StepProgressState),
		writer:     writer,
		totalSteps: totalSteps,
		done:       make(chan struct{}),
		prefix:     prefix,
	}
}

// Start begins the progress tracking
func (pt *CLIProgressTracker) StartListening(progressChan <-chan pkgEvents.ExecutionEvent) {
	// Process events - spinners handle their own animation
	for event := range progressChan {
		switch event.Type {
		case pkgEvents.EventStepStarted:
			pt.startStep(event.StepID, event.StepIndex, pt.totalSteps)
		case pkgEvents.EventStepCompleted:
			pt.completeStep(event.StepID, event.Duration)

		case pkgEvents.EventStepFailed:
			pt.failStep(event.StepID, event.Duration, event.Error)

		case pkgEvents.EventStepRetrying:
			pt.retryStep(event.StepID, event.Attempt)

		case pkgEvents.EventStepProgress:
			pt.updateStepProgress(event.StepID, event.ActionID, event.Text)

		case pkgEvents.EventStepActionStarted:
			pt.createActionSpinner(event.StepID, event.ActionID, event.Text)

		case pkgEvents.EventStepActionCompleted:
			pt.completeActionSpinner(event.StepID, event.ActionID)

		case pkgEvents.EventStepActionFailed:
			pt.failActionSpinner(event.StepID, event.ActionID)
		}
	}
}

// Stop halts the progress tracker and ensures all spinners are stopped
func (pt *CLIProgressTracker) StopListening() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Stop any running spinners
	for _, state := range pt.steps {
		if state.spinner != nil && state.status == "running" {
			state.spinner.Stop()
		}
	}

	close(pt.done)
}

// startStep initializes a new step in running state
func (pt *CLIProgressTracker) startStep(stepID string, stepIndex, totalSteps int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Create and configure spinner
	s := style.NewSpinner(pt.writer)
	title := fmt.Sprintf(" Running step %s (%d/%d)", style.AccentStyle.Render(stepID), stepIndex, totalSteps)
	s.SetSuffix(title)

	state := &StepProgressState{
		stepID:     stepID,
		stepIndex:  stepIndex,
		totalSteps: totalSteps,
		title:      title,
		status:     "running",
		startTime:  time.Now(),
		spinner:    s,
		actions:    []ActionState{},
	}
	pt.steps[stepID] = state

	// Start the spinner
	s.Start()
}

// updateStepProgress updates the progress of a step
func (pt *CLIProgressTracker) updateStepProgress(stepID string, actionID string, text string) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if state, exists := pt.steps[stepID]; exists {
		state.mu.Lock()
		state.spinner.SetSuffix(fmt.Sprintf(" %s", text))
		state.mu.Unlock()
	}
}

func (pt *CLIProgressTracker) createActionSpinner(stepID string, actionID string, text string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if state, exists := pt.steps[stepID]; exists {
		state.mu.Lock()

		state.actions.Add(ActionState{
			id:   actionID,
			text: text,
		})

		state.spinner.SetSuffix(state.String())
		state.mu.Unlock()
	}
}

func (pt *CLIProgressTracker) completeActionSpinner(stepID string, actionID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if state, exists := pt.steps[stepID]; exists {
		state.mu.Lock()
		for i, action := range state.actions {
			if action.id == actionID {
				state.actions[i].text = style.SuccessIcon() + " " + strings.TrimSpace(strings.ReplaceAll(action.text, "...", ""))
				break
			}
		}

		state.spinner.SetSuffix(state.String())
		state.mu.Unlock()
	}
}

func (pt *CLIProgressTracker) failActionSpinner(stepID string, actionID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if state, exists := pt.steps[stepID]; exists {
		state.mu.Lock()
		for i, action := range state.actions {
			if action.id == actionID {
				state.actions[i].text = style.ErrorIcon() + " " + strings.TrimSpace(strings.ReplaceAll(action.text, "...", ""))
				break
			}
		}

		state.spinner.SetSuffix(state.String())
		state.mu.Unlock()
	}
}

// completeStep marks a step as completed
func (pt *CLIProgressTracker) completeStep(stepID string, duration time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if state, exists := pt.steps[stepID]; exists {
		state.mu.Lock()
		state.status = "completed"
		state.endTime = time.Now()
		state.spinner.SetFinalMSG(style.SuccessIcon() + state.String())
		state.spinner.Stop()
		state.mu.Unlock()
	}
}

// failStep marks a step as failed
func (pt *CLIProgressTracker) failStep(stepID string, duration time.Duration, _ string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if state, exists := pt.steps[stepID]; exists {
		state.mu.Lock()
		state.status = "failed"
		state.endTime = time.Now()
		state.spinner.SetFinalMSG(style.ErrorIcon() + " " + state.String())
		state.spinner.Stop()
		state.mu.Unlock()
	}
}

// retryStep shows retry information
func (pt *CLIProgressTracker) retryStep(stepID string, attempt int) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if state, exists := pt.steps[stepID]; exists {
		// Stop current spinner and update suffix to show retry
		state.spinner.Stop()
		state.spinner.SetSuffix(fmt.Sprintf(" Step %d/%d: %s (retry %d)", state.stepIndex, state.totalSteps, stepID, attempt))
		state.spinner.Start()
	}
}

// No animation functions needed - briandowns/spinner handles animation automatically

func collectExecutionResults(execCtx *execcontext.ExecutionContext, result *ExecutionResult) {
	summary := execCtx.GetExecutionSummary()

	// Convert step results
	result.StepResults = make([]StepExecutionResult, 0, len(summary.Steps))
	tokenSummary := &TokenUsageSummary{}

	for _, step := range summary.Steps {
		stepResult := StepExecutionResult{
			StepID:    step.StepID,
			Status:    string(step.Status),
			StartTime: step.StartTime,
			EndTime:   step.EndTime,
			Duration:  step.Duration,
			Output:    step.Output,
			Response:  step.Response,
			Retries:   step.Retries,
		}

		if step.Error != nil {
			stepResult.Error = step.Error.Error()
		}

		if step.TokenUsage != nil {
			stepResult.TokenUsage = &TokenUsage{
				PromptTokens:     step.TokenUsage.PromptTokens,
				CompletionTokens: step.TokenUsage.CompletionTokens,
				TotalTokens:      step.TokenUsage.TotalTokens,
			}

			// Aggregate token usage
			tokenSummary.PromptTokens += step.TokenUsage.PromptTokens
			tokenSummary.CompletionTokens += step.TokenUsage.CompletionTokens
			tokenSummary.TotalTokens += step.TokenUsage.TotalTokens
		}

		result.StepResults = append(result.StepResults, stepResult)
	}

	if tokenSummary.TotalTokens > 0 {
		result.TokenUsage = tokenSummary
	}
}

func printWorkflowInfo(w io.Writer, workflow *ast.Workflow) {
	name := getWorkflowName(workflow)
	stepCount := len(workflow.Workflow.Steps)

	fmt.Fprintf(w, "\nRunning %s workflow (%d steps)\n\n", style.InfoStyle.Render(name), stepCount)

}

func getWorkflowName(workflow *ast.Workflow) string {
	if workflow.Metadata != nil && workflow.Metadata.Name != "" {
		return workflow.Metadata.Name
	}
	return "Untitled Workflow"
}
