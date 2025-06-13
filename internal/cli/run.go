package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lacquer/lacquer/internal/ast"
	"github.com/lacquer/lacquer/internal/parser"
	"github.com/lacquer/lacquer/internal/runtime"
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
	
	// Output options
	showProgress bool
	showSteps    bool
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
	
	// Output flags
	runCmd.Flags().BoolVar(&showProgress, "progress", true, "show real-time progress")
	runCmd.Flags().BoolVar(&showSteps, "show-steps", false, "show detailed step information")
}

// ExecutionResult represents the result of running a workflow
type ExecutionResult struct {
	WorkflowFile    string                   `json:"workflow_file" yaml:"workflow_file"`
	RunID          string                   `json:"run_id" yaml:"run_id"`
	Status         string                   `json:"status" yaml:"status"`
	StartTime      time.Time                `json:"start_time" yaml:"start_time"`
	EndTime        time.Time                `json:"end_time,omitempty" yaml:"end_time,omitempty"`
	Duration       time.Duration            `json:"duration" yaml:"duration"`
	StepsExecuted  int                      `json:"steps_executed" yaml:"steps_executed"`
	StepsTotal     int                      `json:"steps_total" yaml:"steps_total"`
	StepResults    []StepExecutionResult    `json:"step_results,omitempty" yaml:"step_results,omitempty"`
	Inputs         map[string]interface{}   `json:"inputs" yaml:"inputs"`
	FinalState     map[string]interface{}   `json:"final_state,omitempty" yaml:"final_state,omitempty"`
	Error          string                   `json:"error,omitempty" yaml:"error,omitempty"`
	TokenUsage     *TokenUsageSummary       `json:"token_usage,omitempty" yaml:"token_usage,omitempty"`
}

// StepExecutionResult represents the result of executing a single step
type StepExecutionResult struct {
	StepID      string                 `json:"step_id" yaml:"step_id"`
	Status      string                 `json:"status" yaml:"status"`
	StartTime   time.Time              `json:"start_time" yaml:"start_time"`
	EndTime     time.Time              `json:"end_time,omitempty" yaml:"end_time,omitempty"`
	Duration    time.Duration          `json:"duration" yaml:"duration"`
	Output      map[string]interface{} `json:"output,omitempty" yaml:"output,omitempty"`
	Response    string                 `json:"response,omitempty" yaml:"response,omitempty"`
	Error       string                 `json:"error,omitempty" yaml:"error,omitempty"`
	Retries     int                    `json:"retries" yaml:"retries"`
	TokenUsage  *TokenUsage            `json:"token_usage,omitempty" yaml:"token_usage,omitempty"`
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
		Error(fmt.Sprintf("Failed to parse workflow: %v", err))
		os.Exit(1)
	}

	// Validate workflow
	if err := workflow.Validate(); err != nil {
		Error(fmt.Sprintf("Workflow validation failed: %v", err))
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

	// Show workflow info
	if !viper.GetBool("quiet") && viper.GetString("output") == "text" {
		printWorkflowInfo(workflow, workflowInputs)
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
		MaxConcurrentSteps: 1, // Sequential execution for MVP
		DefaultTimeout:     5 * time.Minute,
		EnableRetries:      true,
		MaxRetries:         maxRetries,
		EnableStateSnapshots: saveState,
	}
	
	executor := runtime.NewExecutor(executorConfig)

	// Create execution context
	execCtx := runtime.NewExecutionContext(ctx, workflow, workflowInputs)
	
	// Execute workflow
	result := ExecutionResult{
		WorkflowFile: workflowFile,
		RunID:       execCtx.RunID,
		Status:      "running",
		StartTime:   startTime,
		Inputs:      workflowInputs,
		StepsTotal:  len(workflow.Workflow.Steps),
	}

	if !viper.GetBool("quiet") && viper.GetString("output") == "text" {
		fmt.Printf("\n🚀 Starting workflow execution (Run ID: %s)\n\n", execCtx.RunID)
	}

	// Execute with progress reporting
	err = executeWithProgress(ctx, executor, execCtx, &result)
	
	// Calculate final metrics
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.FinalState = execCtx.GetAllState()

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
	
	// Start progress reporter if enabled
	if showProgress && !viper.GetBool("quiet") && viper.GetString("output") == "text" {
		go progressReporter(progressChan, result)
	}

	// Execute the workflow
	err := executor.ExecuteWorkflow(ctx, execCtx, progressChan)
	
	// Close progress channel
	close(progressChan)
	
	return err
}

func progressReporter(progressChan <-chan runtime.ExecutionEvent, result *ExecutionResult) {
	for event := range progressChan {
		switch event.Type {
		case runtime.EventStepStarted:
			result.StepsExecuted++
			if showSteps {
				fmt.Printf("  📝 Step %d/%d: %s\n", result.StepsExecuted, result.StepsTotal, event.StepID)
			} else {
				fmt.Printf("  ▶️  Step %d/%d\n", result.StepsExecuted, result.StepsTotal)
			}
			
		case runtime.EventStepCompleted:
			if showSteps {
				fmt.Printf("  ✅ Completed: %s (%.2fs)\n", event.StepID, event.Duration.Seconds())
			}
			
		case runtime.EventStepFailed:
			if showSteps {
				fmt.Printf("  ❌ Failed: %s - %s\n", event.StepID, event.Error)
			}
			
		case runtime.EventStepRetrying:
			if showSteps {
				fmt.Printf("  🔄 Retrying: %s (attempt %d)\n", event.StepID, event.Attempt)
			}
		}
	}
}

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

func printWorkflowInfo(workflow *ast.Workflow, inputs map[string]interface{}) {
	fmt.Printf("📋 Workflow: %s\n", getWorkflowName(workflow))
	if workflow.Metadata != nil && workflow.Metadata.Description != "" {
		fmt.Printf("📝 Description: %s\n", workflow.Metadata.Description)
	}
	fmt.Printf("🔢 Steps: %d\n", len(workflow.Workflow.Steps))
	
	if len(inputs) > 0 {
		fmt.Printf("📥 Inputs:\n")
		for k, v := range inputs {
			fmt.Printf("  %s = %v\n", k, v)
		}
	}
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
	
	if result.Status == "completed" {
		Success(fmt.Sprintf("Workflow completed successfully in %v", result.Duration))
	} else {
		Error(fmt.Sprintf("Workflow failed after %v: %s", result.Duration, result.Error))
	}

	// Print execution stats
	fmt.Printf("\n📊 Execution Summary:\n")
	fmt.Printf("  Run ID: %s\n", result.RunID)
	fmt.Printf("  Duration: %v\n", result.Duration)
	fmt.Printf("  Steps: %d/%d executed\n", result.StepsExecuted, result.StepsTotal)
	
	if result.TokenUsage != nil {
		fmt.Printf("  Tokens: %d total (%.4f estimated cost)\n", 
			result.TokenUsage.TotalTokens, result.TokenUsage.EstimatedCost)
	}

	// Show step details if verbose or if there were failures
	if viper.GetBool("verbose") || result.Status == "failed" {
		fmt.Printf("\n📋 Step Details:\n")
		headers := []string{"Step", "Status", "Duration", "Tokens", "Cost"}
		rows := make([][]string, len(result.StepResults))
		
		for i, step := range result.StepResults {
			status := "✅"
			if step.Status == "failed" {
				status = "❌"
			} else if step.Status == "skipped" {
				status = "⏭️"
			}
			
			tokens := "-"
			cost := "-"
			if step.TokenUsage != nil {
				tokens = fmt.Sprintf("%d", step.TokenUsage.TotalTokens)
				cost = fmt.Sprintf("$%.4f", step.TokenUsage.EstimatedCost)
			}
			
			rows[i] = []string{
				step.StepID,
				status,
				step.Duration.String(),
				tokens,
				cost,
			}
		}
		
		printTable(headers, rows)
	}

	// Show final state if verbose and not empty
	if viper.GetBool("verbose") && len(result.FinalState) > 0 {
		fmt.Printf("\n🏁 Final State:\n")
		for k, v := range result.FinalState {
			if k == "_last_saved" {
				continue // Skip internal state
			}
			fmt.Printf("  %s = %v\n", k, v)
		}
	}
}