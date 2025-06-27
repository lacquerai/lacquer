package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/parser"
	"github.com/lacquerai/lacquer/internal/runtime"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [workflow.laq.yaml]",
	Short: "Execute a Lacquer workflow",
	Long: `Execute a Lacquer workflow locally with real-time progress reporting.

This command:
- Parses and validates the workflow syntax
- Initializes the runtime engine with configured providers
- Executes workflow steps sequentially with proper error handling
- Provides real-time progress updates and logging
- Supports graceful shutdown on interruption signals

Examples:
  laq run workflow.laq.yaml                    # Run workflow with default settings
  laq run workflow.laq.yaml --input key=value # Provide input parameters
  laq run workflow.laq.yaml --dry-run         # Validate without execution
  laq run workflow.laq.yaml --output json     # JSON output for automation
  laq run workflow.laq.yaml --save-state      # Persist state for debugging`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runWorkflow(args[0])
	},
}

var (
	// Input parameters
	inputs map[string]string

	// Execution options
	dryRun     bool
	saveState  bool
	maxRetries int
	timeout    time.Duration

	// Removed showProgress and showSteps - using single clean format
)

func init() {
	rootCmd.AddCommand(runCmd)

	// Input flags
	runCmd.Flags().StringToStringVarP(&inputs, "input", "i", map[string]string{}, "input parameters (key=value)")

	// Execution flags
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and plan without executing")
	runCmd.Flags().BoolVar(&saveState, "save-state", false, "persist execution state for debugging")
	runCmd.Flags().IntVar(&maxRetries, "max-retries", 3, "maximum number of retries for failed steps")
	runCmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "overall execution timeout")

	// Removed verbose output flags - using single clean format
}

// ExecutionResult represents the result of running a workflow
type ExecutionResult struct {
	WorkflowFile  string                 `json:"workflow_file" yaml:"workflow_file"`
	RunID         string                 `json:"run_id" yaml:"run_id"`
	Status        string                 `json:"status" yaml:"status"`
	StartTime     time.Time              `json:"start_time" yaml:"start_time"`
	EndTime       time.Time              `json:"end_time,omitempty" yaml:"end_time,omitempty"`
	Duration      time.Duration          `json:"duration" yaml:"duration"`
	StepsExecuted int                    `json:"steps_executed" yaml:"steps_executed"`
	StepsTotal    int                    `json:"steps_total" yaml:"steps_total"`
	StepResults   []StepExecutionResult  `json:"step_results,omitempty" yaml:"step_results,omitempty"`
	Inputs        map[string]interface{} `json:"inputs" yaml:"inputs"`
	Outputs       map[string]interface{} `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	FinalState    map[string]interface{} `json:"final_state,omitempty" yaml:"final_state,omitempty"`
	Error         string                 `json:"error,omitempty" yaml:"error,omitempty"`
	TokenUsage    *TokenUsageSummary     `json:"token_usage,omitempty" yaml:"token_usage,omitempty"`
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
	TotalTokens      int     `json:"total_tokens" yaml:"total_tokens"`
	PromptTokens     int     `json:"prompt_tokens" yaml:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens" yaml:"completion_tokens"`
	EstimatedCost    float64 `json:"estimated_cost" yaml:"estimated_cost"`
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
	spinner    *spinner.Spinner
	mu         sync.RWMutex
}

// ProgressTracker manages the visual progress display for all steps
type ProgressTracker struct {
	steps map[string]*StepProgressState
	mu    sync.RWMutex
	done  chan struct{}
	wg    sync.WaitGroup
}

func runWorkflow(workflowFile string) {
	startTime := time.Now()

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info().Msg("Received interrupt signal, shutting down gracefully...")
		cancel()
	}()

	// Apply timeout if specified
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Parse workflow
	yamlParser, err := parser.NewYAMLParser()
	if err != nil {
		Error(fmt.Sprintf("Failed to create parser: %v", err))
		os.Exit(1)
	}

	workflow, err := yamlParser.ParseFile(workflowFile)
	if err != nil {
		var enhancedErr *parser.MultiErrorEnhanced

		if errors.As(err, &enhancedErr) {
			result := NewValidationResult(workflowFile)
			result.CollectError(err)
			summary := ValidationSummary{
				Total:   1,
				Results: []ValidationResult{*result},
				Invalid: 1,
			}

			printValidationSummary(summary)
		} else {
			Error(fmt.Sprintf("Failed to parse workflow: %v", err))
		}

		os.Exit(1)
	}
	log.Info().
		Str("workflow", workflowFile).
		Str("version", workflow.Version).
		Int("steps", len(workflow.Workflow.Steps)).
		Msg("Workflow loaded and validated")

	// Convert inputs to proper types
	workflowInputs := make(map[string]interface{})
	for k, v := range inputs {
		workflowInputs[k] = v
	}

	for k, v := range workflow.Workflow.Inputs {
		if _, ok := workflowInputs[k]; !ok && v.Default != nil {
			workflowInputs[k] = v.Default
		}
	}

	validationResult := runtime.ValidateWorkflowInputs(workflow, workflowInputs)
	if !validationResult.Valid {
		printValidationErrors(validationResult)
		os.Exit(1)
	}

	// Show workflow info
	if !viper.GetBool("quiet") && viper.GetString("output") == "text" {
		printWorkflowInfo(workflow)
	}

	// Dry run mode
	if dryRun {
		if !viper.GetBool("quiet") {
			Success("Workflow validation completed (dry-run mode)")
		}
		return
	}

	// Create executor with configuration
	executorConfig := &runtime.ExecutorConfig{
		MaxConcurrentSteps:   3, // TODO: make this configurable
		DefaultTimeout:       5 * time.Minute,
		EnableRetries:        true,
		MaxRetries:           maxRetries,
		EnableStateSnapshots: saveState,
	}

	executor, err := runtime.NewExecutor(executorConfig, workflow, nil)
	if err != nil {
		Error(fmt.Sprintf("Failed to create executor: %v", err))
		return
	}

	// Create execution context
	execCtx := runtime.NewExecutionContext(ctx, workflow, workflowInputs)

	// Execute workflow
	result := ExecutionResult{
		WorkflowFile: workflowFile,
		RunID:        execCtx.RunID,
		Status:       "running",
		StartTime:    startTime,
		Inputs:       workflowInputs,
		StepsTotal:   len(workflow.Workflow.Steps),
	}

	// Removed verbose execution start message

	// Execute with progress reporting
	err = executeWithProgress(ctx, executor, execCtx, &result)

	// Calculate final metrics
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.FinalState = execCtx.GetAllState()
	result.Outputs = execCtx.GetWorkflowOutputs()

	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()

		log.Error().
			Err(err).
			Str("run_id", execCtx.RunID).
			Dur("duration", result.Duration).
			Msg("Workflow execution failed")
	} else {
		result.Status = "completed"

		log.Info().
			Str("run_id", execCtx.RunID).
			Dur("duration", result.Duration).
			Int("steps", result.StepsExecuted).
			Msg("Workflow execution completed successfully")
	}

	// Collect step results and token usage
	collectExecutionResults(execCtx, &result)

	// Output results
	outputResults(result)

	// Exit with appropriate code
	if result.Status == "failed" {
		os.Exit(1)
	}
}

func executeWithProgress(ctx context.Context, executor *runtime.Executor, execCtx *runtime.ExecutionContext, result *ExecutionResult) error {
	// Create a progress channel for real-time updates
	progressChan := make(chan runtime.ExecutionEvent, 100)

	// Start progress reporter if not quiet and text output
	if !viper.GetBool("quiet") && viper.GetString("output") == "text" {
		tracker := NewProgressTracker()
		go tracker.Start(progressChan, result)
		defer tracker.Stop()
	}

	// Execute the workflow
	err := executor.ExecuteWorkflow(ctx, execCtx, progressChan)

	// Close progress channel
	close(progressChan)

	return err
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		steps: make(map[string]*StepProgressState),
		done:  make(chan struct{}),
	}
}

// Start begins the progress tracking
func (pt *ProgressTracker) Start(progressChan <-chan runtime.ExecutionEvent, result *ExecutionResult) {
	pt.wg.Add(1)
	defer pt.wg.Done()

	// Process events - spinners handle their own animation
	for event := range progressChan {
		switch event.Type {
		case runtime.EventStepStarted:
			result.StepsExecuted++
			pt.startStep(event.StepID, event.StepIndex, result.StepsTotal)

		case runtime.EventStepCompleted:
			pt.completeStep(event.StepID, event.Duration)

		case runtime.EventStepFailed:
			pt.failStep(event.StepID, event.Duration, event.Error)

		case runtime.EventStepRetrying:
			pt.retryStep(event.StepID, event.Attempt)

		case runtime.EventStepProgress:
			pt.updateStepProgress(event.StepID, event.Metadata)
		}
	}
}

// Stop halts the progress tracker and ensures all spinners are stopped
func (pt *ProgressTracker) Stop() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Stop any running spinners
	for _, state := range pt.steps {
		if state.spinner != nil && state.status == "running" {
			state.spinner.Stop()
		}
	}

	close(pt.done)
	pt.wg.Wait()
}

// startStep initializes a new step in running state
func (pt *ProgressTracker) startStep(stepID string, stepIndex, totalSteps int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Create and configure spinner
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond) // Dots spinner with 100ms delay
	s.Suffix = fmt.Sprintf(" Step %d/%d: %s", stepIndex, totalSteps, accentStyle.Render(stepID))

	state := &StepProgressState{
		stepID:     stepID,
		stepIndex:  stepIndex,
		totalSteps: totalSteps,
		status:     "running",
		startTime:  time.Now(),
		spinner:    s,
	}
	pt.steps[stepID] = state

	// Start the spinner
	s.Start()
}

// updateStepProgress updates the progress of a step
func (pt *ProgressTracker) updateStepProgress(stepID string, metadata map[string]interface{}) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if state, exists := pt.steps[stepID]; exists {
		state.mu.Lock()
		state.spinner.Suffix = fmt.Sprintf(" %v", metadata["message"])
		state.mu.Unlock()
	}
}

// completeStep marks a step as completed
func (pt *ProgressTracker) completeStep(stepID string, duration time.Duration) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if state, exists := pt.steps[stepID]; exists {
		state.mu.Lock()
		state.status = "completed"
		state.endTime = time.Now()
		state.spinner.FinalMSG = SuccessIcon() + " " + fmt.Sprintf("Step %s completed (%s)\n", accentStyle.Render(stepID), formatDuration(duration))
		state.spinner.Stop()
		state.mu.Unlock()
	}
}

// failStep marks a step as failed
func (pt *ProgressTracker) failStep(stepID string, duration time.Duration, _ string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if state, exists := pt.steps[stepID]; exists {
		state.mu.Lock()
		state.status = "failed"
		state.endTime = time.Now()
		state.spinner.FinalMSG = ErrorIcon() + " " + strings.TrimSpace(state.spinner.Suffix) + "\n"
		state.spinner.Stop()
		state.mu.Unlock()
	}
}

// retryStep shows retry information
func (pt *ProgressTracker) retryStep(stepID string, attempt int) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if state, exists := pt.steps[stepID]; exists {
		// Stop current spinner and update suffix to show retry
		state.spinner.Stop()
		state.spinner.Suffix = fmt.Sprintf(" Step %d/%d: %s (retry %d)",
			state.stepIndex, state.totalSteps, stepID, attempt)
		state.spinner.Start()
	}
}

// No animation functions needed - briandowns/spinner handles animation automatically

func collectExecutionResults(execCtx *runtime.ExecutionContext, result *ExecutionResult) {
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
				EstimatedCost:    step.TokenUsage.EstimatedCost,
			}

			// Aggregate token usage
			tokenSummary.PromptTokens += step.TokenUsage.PromptTokens
			tokenSummary.CompletionTokens += step.TokenUsage.CompletionTokens
			tokenSummary.TotalTokens += step.TokenUsage.TotalTokens
			tokenSummary.EstimatedCost += step.TokenUsage.EstimatedCost
		}

		result.StepResults = append(result.StepResults, stepResult)
	}

	if tokenSummary.TotalTokens > 0 {
		result.TokenUsage = tokenSummary
	}
}

func printWorkflowInfo(workflow *ast.Workflow) {
	name := getWorkflowName(workflow)
	stepCount := len(workflow.Workflow.Steps)

	fmt.Printf("\nRunning %s (%d steps)\n\n", infoStyle.Render(name), stepCount)

}

func getWorkflowName(workflow *ast.Workflow) string {
	if workflow.Metadata != nil && workflow.Metadata.Name != "" {
		return workflow.Metadata.Name
	}
	return "Untitled Workflow"
}

func outputResults(result ExecutionResult) {
	outputFormat := viper.GetString("output")

	switch outputFormat {
	case "json":
		printJSON(result)
	case "yaml":
		printYAML(result)
	default:
		printExecutionSummary(result)
	}
}

func printExecutionSummary(result ExecutionResult) {
	if viper.GetBool("quiet") {
		return
	}

	fmt.Printf("\n")

	// Show success or failure with duration
	if result.Status == "completed" {
		fmt.Printf("%s Workflow completed %s (%s)\n", SuccessIcon(), successStyle.Render("successfully"), formatDuration(result.Duration))

	} else {
		fmt.Printf("%s Workflow failed\n\n", ErrorIcon())
		// Show error details for failures
		if result.Error != "" {
			fmt.Printf("%s\n", errorStyle.Render(result.Error))
		}
	}

	if len(result.Outputs) > 0 {
		var outputContent strings.Builder
		outputContent.WriteString("\n")
		outputContent.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render("Outputs"))
		outputContent.WriteString("\n\n")
		var i int
		for k, v := range result.Outputs {
			outputContent.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render(k))
			outputContent.WriteString(fmt.Sprintf(": %v", v))
			if i < len(result.Outputs)-1 {
				outputContent.WriteString("\n")
			}
			i++
		}
		fmt.Printf("%s\n", outputContent.String())
	}

}

func printValidationErrors(validationResult *runtime.InputValidationResult) {
	fmt.Printf("\n❌ Input validation failed:\n\n")

	for i, err := range validationResult.Errors {
		// Add spacing between errors for better readability
		if i > 0 {
			fmt.Println()
		}

		// Format field name with color/emphasis
		fmt.Printf("   Field: %s\n", err.Field)
		fmt.Printf("   Error: %s\n", err.Message)

		// Show the actual value if provided
		if err.Value != nil {
			fmt.Printf("   Value: %v\n", err.Value)
		}
	}

	fmt.Printf("\n💡 Please check your input parameters and try again.\n")
}

func formatDuration(duration time.Duration) string {
	return fmt.Sprintf("%.2fs", duration.Seconds())
}
