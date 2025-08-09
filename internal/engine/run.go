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

// ExecutionResult contains the complete outcome of workflow execution including
// timing, status, step results, and resource usage metrics.
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

// StepExecutionResult contains the execution outcome for an individual workflow step
// including its output, timing, retry information, and token usage.
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

// TokenUsageSummary aggregates token consumption metrics across all workflow steps.
type TokenUsageSummary struct {
	TotalTokens      int `json:"total_tokens" yaml:"total_tokens"`
	PromptTokens     int `json:"prompt_tokens" yaml:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens" yaml:"completion_tokens"`
}

// TokenUsage tracks token consumption and estimated cost for a single step execution.
type TokenUsage struct {
	PromptTokens     int     `json:"prompt_tokens" yaml:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens" yaml:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens" yaml:"total_tokens"`
	EstimatedCost    float64 `json:"estimated_cost" yaml:"estimated_cost"`
}

// StepProgressState manages the visual display state for a workflow step,
// including its spinner animation and nested action states.
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

// String returns a formatted representation of the step progress state
// including its title and action states.
func (s *StepProgressState) String() string {
	return fmt.Sprintf(" %s\n%s", s.title, s.actions.String())
}

// ActionStates is a collection of action states within a workflow step.
type ActionStates []ActionState

// Add appends a new action state and marks all previous actions as completed
// if they haven't already been marked with success or error icons.
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

// wrapLine splits text into multiple lines respecting word boundaries.
// Falls back to hard breaks if no suitable word boundary is found.
func wrapLine(line string, maxWidth int) []string {
	if len(line) <= maxWidth {
		return []string{line}
	}

	var wrapped []string
	for len(line) > maxWidth {
		// Try to find a space to break at
		breakPoint := maxWidth
		for breakPoint > 0 && line[breakPoint] != ' ' {
			breakPoint--
		}

		// If no space found within reasonable distance, just cut at maxWidth
		if breakPoint <= maxWidth/2 {
			breakPoint = maxWidth
		}

		wrapped = append(wrapped, strings.TrimSpace(line[:breakPoint]))
		line = strings.TrimSpace(line[breakPoint:])
	}

	if len(line) > 0 {
		wrapped = append(wrapped, line)
	}

	return wrapped
}

// String formats all action states with proper indentation and line wrapping.
func (as ActionStates) String() string {
	var text strings.Builder
	for _, action := range as {
		lines := strings.Split(action.text, "\n")
		indentation := "   "
		successIndentation := ""

		for i, line := range lines {
			// Check for success icon to determine additional indentation
			if i == 0 && strings.HasPrefix(line, style.SuccessIcon()) {
				successIndentation = "  "
			}

			// Calculate the maximum width for this line considering indentation
			totalIndent := indentation
			if i > 0 {
				totalIndent += successIndentation
			}
			maxWidth := 100 - len(totalIndent)
			if maxWidth < 20 {
				maxWidth = 20 // Minimum reasonable width
			}

			// For the first line, handle icon prefixes
			lineContent := line
			iconPrefix := ""
			if strings.HasPrefix(line, style.SuccessIcon()) {
				iconPrefix = style.SuccessIcon() + " "
				lineContent = strings.TrimPrefix(line, iconPrefix)
			} else if strings.HasPrefix(line, style.ErrorIcon()) {
				iconPrefix = style.ErrorIcon() + " "
				lineContent = strings.TrimPrefix(line, iconPrefix)
			}

			// Wrap the line content if needed
			wrappedLines := wrapLine(lineContent, maxWidth)

			// Write the lines with proper indentation
			for j, wrappedLine := range wrappedLines {
				if i == 0 && j == 0 {
					// First line of first text line
					text.WriteString(fmt.Sprintf("%s%s%s", indentation, iconPrefix, wrappedLine))
				} else {
					// Continuation lines or subsequent original lines
					text.WriteString(fmt.Sprintf("\n%s%s%s", indentation, successIndentation, wrappedLine))
				}
			}
		}
		text.WriteString("\n")
	}
	return text.String()
}

// ActionState represents a single action within a workflow step,
// tracking its identifier and display text.
type ActionState struct {
	id   string
	text string
}

// Runner orchestrates workflow execution with progress tracking capabilities.
type Runner struct {
	progressListener pkgEvents.Listener
	newExecutor      ExecutorFunc
}

// RunnerOption is a function that can be used to configure a Runner.
type RunnerOption func(*Runner)

// WithExecutorFunc sets the function that creates a new Executor instance.
// This allows for custom Executor implementations to be used.
// In general this is only used for testing.
func WithExecutorFunc(newExecutor ExecutorFunc) RunnerOption {
	return func(r *Runner) {
		r.newExecutor = newExecutor
	}
}

// NewRunner creates a workflow runner with the specified progress listener.
func NewRunner(progressListener pkgEvents.Listener, options ...RunnerOption) *Runner {
	r := &Runner{
		progressListener: progressListener,
	}

	for _, option := range options {
		option(r)
	}

	return r
}

// SetProgressListener updates the progress listener for workflow execution events.
func (r *Runner) SetProgressListener(listener pkgEvents.Listener) {
	r.progressListener = listener
}

// RunWorkflowRaw executes a parsed workflow with the provided execution context.
// Returns detailed execution results including step outcomes and resource usage.
func (r *Runner) RunWorkflowRaw(execCtx *execcontext.ExecutionContext, workflow *ast.Workflow, startTime time.Time, prefix ...string) (*ExecutionResult, error) {
	// If no executor function is set, use the default implementation
	if r.newExecutor == nil {
		r.newExecutor = NewExecutor
	}

	executorConfig := &ExecutorConfig{
		MaxConcurrentSteps: 3,
		DefaultTimeout:     5 * time.Minute,
		EnableRetries:      true,
	}
	executor, err := r.newExecutor(execCtx.Context, executorConfig, workflow, nil, r)
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

// RunWorkflow parses and executes a workflow file with the given inputs.
// Handles input validation, default value assignment, and progress tracking.
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

// executeWithProgress runs the workflow executor while sending progress events to registered listeners.
func (r *Runner) executeWithProgress(executor WorkflowExecutor, execCtx *execcontext.ExecutionContext, _ *ExecutionResult) error {
	progressChan := make(chan pkgEvents.ExecutionEvent, 100)

	if r.progressListener != nil {
		go r.progressListener.StartListening(progressChan)
	}

	err := executor.ExecuteWorkflow(execCtx, progressChan)
	close(progressChan)

	if r.progressListener != nil {
		r.progressListener.StopListening()
	}

	return err
}

// CLIProgressTracker manages visual progress display for workflow execution,
// coordinating step-level spinners and action states.
type CLIProgressTracker struct {
	steps          map[string]*StepProgressState
	mu             sync.RWMutex
	writer         io.Writer
	totalSteps     int
	done           bool
	prefix         string
	spinnerManager *style.SpinnerManager
}

// NewProgressTracker creates a progress tracker for displaying workflow execution status.
func NewProgressTracker(writer io.Writer, prefix string, totalSteps int) *CLIProgressTracker {
	return &CLIProgressTracker{
		steps:          make(map[string]*StepProgressState),
		writer:         writer,
		totalSteps:     totalSteps,
		done:           false,
		prefix:         prefix,
		spinnerManager: style.NewSpinnerManager(writer),
	}
}

// StartListening processes execution events and updates the visual progress display.
func (pt *CLIProgressTracker) StartListening(progressChan <-chan pkgEvents.ExecutionEvent) {
	pt.mu.Lock()
	pt.done = false
	pt.mu.Unlock()

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

// StopListening halts progress tracking and stops all active spinners.
func (pt *CLIProgressTracker) StopListening() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Stop any running spinners
	for _, state := range pt.steps {
		if state.spinner != nil && state.status == "running" {
			state.spinner.Stop()
		}
	}

	pt.done = true
}

// HasCompleted checks if the progress tracker has completed.
func (pt *CLIProgressTracker) HasCompleted() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.done
}

// startStep creates and starts a spinner for a new workflow step.
func (pt *CLIProgressTracker) startStep(stepID string, stepIndex, totalSteps int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Create and configure spinner
	s := pt.spinnerManager.Start()
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

// updateStepProgress updates the display text for an active step's spinner.
func (pt *CLIProgressTracker) updateStepProgress(stepID string, _ string, text string) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if state, exists := pt.steps[stepID]; exists {
		state.mu.Lock()
		state.spinner.SetSuffix(fmt.Sprintf(" %s", text))
		state.mu.Unlock()
	}
}

// createActionSpinner adds a new action to a step's progress display.
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

// completeActionSpinner marks an action as completed with a success icon.
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

// failActionSpinner marks an action as failed with an error icon.
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

// completeStep finalizes a step's display with a success indicator and stops its spinner.
func (pt *CLIProgressTracker) completeStep(stepID string, _ time.Duration) {
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

// failStep finalizes a step's display with an error indicator and stops its spinner.
func (pt *CLIProgressTracker) failStep(stepID string, _ time.Duration, _ string) {
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

// retryStep updates the step display to show retry attempt information.
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

// collectExecutionResults aggregates step execution data and token usage
// statistics into the final execution result.
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

// printWorkflowInfo displays workflow metadata including name and step count.
func printWorkflowInfo(w io.Writer, workflow *ast.Workflow) {
	name := getWorkflowName(workflow)
	stepCount := len(workflow.Workflow.Steps)

	_, _ = fmt.Fprintf(w, "\nRunning %s workflow (%d steps)\n\n", style.InfoStyle.Render(name), stepCount)
}

// getWorkflowName extracts the workflow name from metadata or returns a default.
func getWorkflowName(workflow *ast.Workflow) string {
	if workflow.Metadata != nil && workflow.Metadata.Name != "" {
		return workflow.Metadata.Name
	}
	return "Untitled Workflow"
}
