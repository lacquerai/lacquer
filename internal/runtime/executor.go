package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/lacquer/lacquer/internal/ast"
	"github.com/rs/zerolog/log"
)

// Executor is the main workflow execution engine
type Executor struct {
	templateEngine *TemplateEngine
	modelRegistry  *ModelRegistry
	config         *ExecutorConfig
}

// ExecutorConfig contains configuration for the executor
type ExecutorConfig struct {
	MaxConcurrentSteps int           `yaml:"max_concurrent_steps"`
	DefaultTimeout     time.Duration `yaml:"default_timeout"`
	EnableRetries      bool          `yaml:"enable_retries"`
	MaxRetries         int           `yaml:"max_retries"`
	RetryDelay         time.Duration `yaml:"retry_delay"`
	EnableMetrics      bool          `yaml:"enable_metrics"`
}

// DefaultExecutorConfig returns a sensible default configuration
func DefaultExecutorConfig() *ExecutorConfig {
	return &ExecutorConfig{
		MaxConcurrentSteps: 1, // Sequential execution for MVP
		DefaultTimeout:     5 * time.Minute,
		EnableRetries:      true,
		MaxRetries:         3,
		RetryDelay:         time.Second,
		EnableMetrics:      true,
	}
}

// NewExecutor creates a new workflow executor
func NewExecutor(config *ExecutorConfig) *Executor {
	if config == nil {
		config = DefaultExecutorConfig()
	}

	return &Executor{
		templateEngine: NewTemplateEngine(),
		modelRegistry:  NewModelRegistry(),
		config:         config,
	}
}

// Execute runs a workflow with the given inputs
func (e *Executor) Execute(ctx context.Context, workflow *ast.Workflow, inputs map[string]interface{}) (*ExecutionSummary, error) {
	// Create execution context
	execCtx := NewExecutionContext(ctx, workflow, inputs)
	
	log.Info().
		Str("workflow", workflow.Metadata.Name).
		Str("run_id", execCtx.RunID).
		Int("total_steps", execCtx.TotalSteps).
		Msg("Starting workflow execution")

	// Validate inputs
	if err := e.validateInputs(workflow, inputs); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Execute workflow steps sequentially
	for i, step := range workflow.Workflow.Steps {
		if execCtx.IsCancelled() {
			log.Info().Str("run_id", execCtx.RunID).Msg("Workflow execution cancelled")
			break
		}

		execCtx.CurrentStepIndex = i
		
		log.Debug().
			Str("run_id", execCtx.RunID).
			Str("step_id", step.ID).
			Int("step_index", i+1).
			Int("total_steps", execCtx.TotalSteps).
			Msg("Executing step")

		if err := e.executeStep(execCtx, step); err != nil {
			log.Error().
				Err(err).
				Str("run_id", execCtx.RunID).
				Str("step_id", step.ID).
				Msg("Step execution failed")
			
			// Mark step as failed
			result := &StepResult{
				StepID:    step.ID,
				Status:    StepStatusFailed,
				StartTime: time.Now(),
				EndTime:   time.Now(),
				Error:     err,
			}
			execCtx.SetStepResult(step.ID, result)
			
			// Stop execution on error for MVP (no error recovery yet)
			break
		}
	}

	summary := execCtx.GetExecutionSummary()
	
	log.Info().
		Str("run_id", execCtx.RunID).
		Str("status", string(summary.Status)).
		Dur("duration", summary.Duration).
		Int("total_tokens", summary.TotalTokens).
		Float64("estimated_cost", summary.EstimatedCost).
		Msg("Workflow execution completed")

	return &summary, nil
}

// executeStep executes a single workflow step
func (e *Executor) executeStep(execCtx *ExecutionContext, step *ast.Step) error {
	start := time.Now()
	
	// Mark step as running
	result := &StepResult{
		StepID:    step.ID,
		Status:    StepStatusRunning,
		StartTime: start,
	}
	execCtx.SetStepResult(step.ID, result)

	// Check if step should be skipped
	if shouldSkip, err := e.evaluateSkipCondition(execCtx, step); err != nil {
		return fmt.Errorf("failed to evaluate skip condition: %w", err)
	} else if shouldSkip {
		result.Status = StepStatusSkipped
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)
		execCtx.SetStepResult(step.ID, result)
		
		log.Debug().
			Str("step_id", step.ID).
			Msg("Step skipped due to condition")
		return nil
	}

	// Execute based on step type
	var stepOutput map[string]interface{}
	var stepResponse string
	var tokenUsage *TokenUsage
	var err error

	switch {
	case step.IsAgentStep():
		stepResponse, tokenUsage, err = e.executeAgentStep(execCtx, step)
		if err == nil {
			stepOutput = map[string]interface{}{
				"response": stepResponse,
			}
		}
		
	case step.IsBlockStep():
		stepOutput, err = e.executeBlockStep(execCtx, step)
		
	case step.IsActionStep():
		stepOutput, err = e.executeActionStep(execCtx, step)
		
	default:
		err = fmt.Errorf("unknown step type for step %s", step.ID)
	}

	// Update step result
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(start)
	result.TokenUsage = tokenUsage
	result.Response = stepResponse

	if err != nil {
		result.Status = StepStatusFailed
		result.Error = err
	} else {
		result.Status = StepStatusCompleted
		result.Output = stepOutput
		
		// Store step outputs for template access
		if stepOutput != nil {
			for key, value := range stepOutput {
				execCtx.SetState(fmt.Sprintf("steps.%s.%s", step.ID, key), value)
			}
		}
	}

	execCtx.SetStepResult(step.ID, result)
	return err
}

// executeAgentStep executes a step that uses an AI agent
func (e *Executor) executeAgentStep(execCtx *ExecutionContext, step *ast.Step) (string, *TokenUsage, error) {
	// Get the agent configuration
	agent, exists := execCtx.Workflow.GetAgent(step.Agent)
	if !exists {
		return "", nil, fmt.Errorf("agent %s not found", step.Agent)
	}

	// Render the prompt template
	prompt, err := e.templateEngine.Render(step.Prompt, execCtx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to render prompt template: %w", err)
	}

	// Get model provider
	provider, err := e.modelRegistry.GetProvider(agent.Model)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get model provider for %s: %w", agent.Model, err)
	}

	// Create model request
	request := &ModelRequest{
		Model:       agent.Model,
		Prompt:      prompt,
		SystemPrompt: agent.SystemPrompt,
		Temperature: agent.Temperature,
		MaxTokens:   agent.MaxTokens,
		TopP:        agent.TopP,
	}

	// Execute with timeout
	timeout := e.config.DefaultTimeout
	if step.Timeout != nil {
		timeout = step.Timeout.Duration
	}

	ctx, cancel := context.WithTimeout(execCtx.Context, timeout)
	defer cancel()

	// Call the model
	response, tokenUsage, err := provider.Generate(ctx, request)
	if err != nil {
		return "", nil, fmt.Errorf("model generation failed: %w", err)
	}

	return response, tokenUsage, nil
}

// executeBlockStep executes a step that uses a reusable block
func (e *Executor) executeBlockStep(execCtx *ExecutionContext, step *ast.Step) (map[string]interface{}, error) {
	// For MVP, we'll implement basic block support
	// This is a placeholder that will be expanded in the block system task
	
	log.Debug().
		Str("step_id", step.ID).
		Str("block", step.Uses).
		Msg("Block step execution (placeholder)")

	// Return mock output for now
	return map[string]interface{}{
		"block_output": fmt.Sprintf("Output from block %s", step.Uses),
	}, nil
}

// executeActionStep executes a step that performs a system action
func (e *Executor) executeActionStep(execCtx *ExecutionContext, step *ast.Step) (map[string]interface{}, error) {
	switch step.Action {
	case "update_state":
		// Render update values using templates
		updates := make(map[string]interface{})
		for key, value := range step.Updates {
			if strValue, ok := value.(string); ok {
				rendered, err := e.templateEngine.Render(strValue, execCtx)
				if err != nil {
					return nil, fmt.Errorf("failed to render update value for %s: %w", key, err)
				}
				updates[key] = rendered
			} else {
				updates[key] = value
			}
		}
		
		execCtx.UpdateState(updates)
		
		return map[string]interface{}{
			"updated_keys": getKeys(updates),
		}, nil
		
	case "human_input":
		// For MVP, this is a placeholder
		// In a full implementation, this would pause execution and wait for human input
		log.Info().
			Str("step_id", step.ID).
			Msg("Human input requested (placeholder)")
		
		return map[string]interface{}{
			"human_input": "Mock human input response",
		}, nil
		
	default:
		return nil, fmt.Errorf("unknown action: %s", step.Action)
	}
}

// evaluateSkipCondition evaluates whether a step should be skipped
func (e *Executor) evaluateSkipCondition(execCtx *ExecutionContext, step *ast.Step) (bool, error) {
	if step.SkipIf == "" && step.Condition == "" {
		return false, nil
	}

	// For MVP, implement basic condition evaluation
	// This would be expanded to full expression evaluation later
	
	if step.SkipIf != "" {
		// Simple template-based condition evaluation
		result, err := e.templateEngine.Render(step.SkipIf, execCtx)
		if err != nil {
			return false, err
		}
		return SafeBool(result), nil
	}

	if step.Condition != "" {
		// Condition must be true to execute
		result, err := e.templateEngine.Render(step.Condition, execCtx)
		if err != nil {
			return false, err
		}
		return !SafeBool(result), nil // Skip if condition is false
	}

	return false, nil
}

// validateInputs validates that required inputs are provided
func (e *Executor) validateInputs(workflow *ast.Workflow, inputs map[string]interface{}) error {
	if workflow.Workflow.Inputs == nil {
		return nil
	}

	for name, param := range workflow.Workflow.Inputs {
		if param.Required {
			if _, exists := inputs[name]; !exists {
				if param.Default == nil {
					return fmt.Errorf("required input %s is missing", name)
				}
				// Set default value
				inputs[name] = param.Default
			}
		}
	}

	return nil
}

// getKeys returns the keys of a map as a slice
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}