package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/lacquer/lacquer/internal/ast"
	"github.com/rs/zerolog/log"
)

// ExecutionEventType represents the type of execution event
type ExecutionEventType string

const (
	EventWorkflowStarted   ExecutionEventType = "workflow_started"
	EventWorkflowCompleted ExecutionEventType = "workflow_completed"
	EventWorkflowFailed    ExecutionEventType = "workflow_failed"
	EventStepStarted       ExecutionEventType = "step_started"
	EventStepCompleted     ExecutionEventType = "step_completed"
	EventStepFailed        ExecutionEventType = "step_failed"
	EventStepSkipped       ExecutionEventType = "step_skipped"
	EventStepRetrying      ExecutionEventType = "step_retrying"
)

// ExecutionEvent represents an event during workflow execution
type ExecutionEvent struct {
	Type      ExecutionEventType `json:"type"`
	Timestamp time.Time          `json:"timestamp"`
	RunID     string             `json:"run_id"`
	StepID    string             `json:"step_id,omitempty"`
	StepIndex int                `json:"step_index,omitempty"`
	Duration  time.Duration      `json:"duration,omitempty"`
	Error     string             `json:"error,omitempty"`
	Attempt   int                `json:"attempt,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Executor is the main workflow execution engine
type Executor struct {
	templateEngine *TemplateEngine
	modelRegistry  *ModelRegistry
	config         *ExecutorConfig
}

// ExecutorConfig contains configuration for the executor
type ExecutorConfig struct {
	MaxConcurrentSteps   int           `yaml:"max_concurrent_steps"`
	DefaultTimeout       time.Duration `yaml:"default_timeout"`
	EnableRetries        bool          `yaml:"enable_retries"`
	MaxRetries           int           `yaml:"max_retries"`
	RetryDelay           time.Duration `yaml:"retry_delay"`
	EnableMetrics        bool          `yaml:"enable_metrics"`
	EnableStateSnapshots bool          `yaml:"enable_state_snapshots"`
}

// DefaultExecutorConfig returns a sensible default configuration
func DefaultExecutorConfig() *ExecutorConfig {
	return &ExecutorConfig{
		MaxConcurrentSteps:   1, // Sequential execution for MVP
		DefaultTimeout:       5 * time.Minute,
		EnableRetries:        true,
		MaxRetries:           3,
		RetryDelay:           time.Second,
		EnableMetrics:        true,
		EnableStateSnapshots: false,
	}
}

// NewExecutor creates a new workflow executor
func NewExecutor(config *ExecutorConfig) *Executor {
	if config == nil {
		config = DefaultExecutorConfig()
	}

	registry := NewModelRegistry()
	
	// Initialize and register providers
	initializeProviders(registry)

	return &Executor{
		templateEngine: NewTemplateEngine(),
		modelRegistry:  registry,
		config:         config,
	}
}

// initializeProviders initializes and registers all available model providers
func initializeProviders(registry *ModelRegistry) {
	// Register Anthropic provider if API key is available
	if apiKey := GetAnthropicAPIKeyFromEnv(); apiKey != "" {
		anthropicProvider, err := NewAnthropicProvider(nil) // Use default config
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize Anthropic provider")
		} else {
			if err := registry.RegisterProvider(anthropicProvider); err != nil {
				log.Warn().Err(err).Msg("Failed to register Anthropic provider")
			} else {
				log.Info().Msg("Anthropic provider registered successfully")
			}
		}
	}
	
	// Register Claude Code provider if CLI is available
	claudeCodeProvider, err := NewClaudeCodeProvider(nil) // Use default config
	if err != nil {
		log.Debug().Err(err).Msg("Claude Code provider not available")
	} else {
		if err := registry.RegisterProvider(claudeCodeProvider); err != nil {
			log.Warn().Err(err).Msg("Failed to register Claude Code provider")
		} else {
			log.Info().Msg("Claude Code provider registered successfully")
		}
	}
	
	// Register OpenAI provider if API key is available
	if apiKey := GetOpenAIAPIKeyFromEnv(); apiKey != "" {
		openaiProvider, err := NewOpenAIProvider(nil) // Use default config
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize OpenAI provider")
		} else {
			if err := registry.RegisterProvider(openaiProvider); err != nil {
				log.Warn().Err(err).Msg("Failed to register OpenAI provider")
			} else {
				log.Info().Msg("OpenAI provider registered successfully")
			}
		}
	}
}

// ExecuteWorkflow runs a workflow with progress events sent to the given channel
func (e *Executor) ExecuteWorkflow(ctx context.Context, execCtx *ExecutionContext, progressChan chan<- ExecutionEvent) error {
	log.Info().
		Str("workflow", getWorkflowNameFromContext(execCtx)).
		Str("run_id", execCtx.RunID).
		Int("total_steps", execCtx.TotalSteps).
		Msg("Starting workflow execution")

	// Send workflow started event
	if progressChan != nil {
		progressChan <- ExecutionEvent{
			Type:      EventWorkflowStarted,
			Timestamp: time.Now(),
			RunID:     execCtx.RunID,
		}
	}

	// Execute workflow steps sequentially
	for i, step := range execCtx.Workflow.Workflow.Steps {
		if execCtx.IsCancelled() {
			log.Info().Str("run_id", execCtx.RunID).Msg("Workflow execution cancelled")
			break
		}

		execCtx.CurrentStepIndex = i
		
		// Send step started event
		if progressChan != nil {
			progressChan <- ExecutionEvent{
				Type:      EventStepStarted,
				Timestamp: time.Now(),
				RunID:     execCtx.RunID,
				StepID:    step.ID,
				StepIndex: i + 1,
			}
		}
		
		stepStart := time.Now()
		err := e.executeStep(execCtx, step)
		stepDuration := time.Since(stepStart)

		if err != nil {
			log.Error().
				Err(err).
				Str("run_id", execCtx.RunID).
				Str("step_id", step.ID).
				Msg("Step execution failed")
			
			// Send step failed event
			if progressChan != nil {
				progressChan <- ExecutionEvent{
					Type:      EventStepFailed,
					Timestamp: time.Now(),
					RunID:     execCtx.RunID,
					StepID:    step.ID,
					StepIndex: i + 1,
					Duration:  stepDuration,
					Error:     err.Error(),
				}
			}
			
			// Mark step as failed
			result := &StepResult{
				StepID:    step.ID,
				Status:    StepStatusFailed,
				StartTime: stepStart,
				EndTime:   time.Now(),
				Duration:  stepDuration,
				Error:     err,
			}
			execCtx.SetStepResult(step.ID, result)
			
			// Send workflow failed event
			if progressChan != nil {
				progressChan <- ExecutionEvent{
					Type:      EventWorkflowFailed,
					Timestamp: time.Now(),
					RunID:     execCtx.RunID,
					Error:     err.Error(),
				}
			}
			
			return err
		} else {
			// Send step completed event
			if progressChan != nil {
				progressChan <- ExecutionEvent{
					Type:      EventStepCompleted,
					Timestamp: time.Now(),
					RunID:     execCtx.RunID,
					StepID:    step.ID,
					StepIndex: i + 1,
					Duration:  stepDuration,
				}
			}
		}
	}

	// Send workflow completed event
	if progressChan != nil {
		progressChan <- ExecutionEvent{
			Type:      EventWorkflowCompleted,
			Timestamp: time.Now(),
			RunID:     execCtx.RunID,
		}
	}

	log.Info().
		Str("run_id", execCtx.RunID).
		Dur("duration", time.Since(execCtx.StartTime)).
		Msg("Workflow execution completed successfully")

	return nil
}

// getWorkflowNameFromContext extracts workflow name from execution context
func getWorkflowNameFromContext(execCtx *ExecutionContext) string {
	if execCtx.Workflow.Metadata != nil && execCtx.Workflow.Metadata.Name != "" {
		return execCtx.Workflow.Metadata.Name
	}
	return "Untitled Workflow"
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
		
		// Process state updates for any step type
		if step.Updates != nil {
			updates := make(map[string]interface{})
			for key, value := range step.Updates {
				if strValue, ok := value.(string); ok {
					rendered, renderErr := e.templateEngine.Render(strValue, execCtx)
					if renderErr != nil {
						log.Warn().
							Err(renderErr).
							Str("step_id", step.ID).
							Str("key", key).
							Msg("Failed to render state update value")
						updates[key] = value
					} else {
						updates[key] = rendered
					}
				} else {
					updates[key] = value
				}
			}
			execCtx.UpdateState(updates)
		}
	}

	execCtx.SetStepResult(step.ID, result)
	
	// Increment current step index after processing the step
	execCtx.IncrementCurrentStep()
	
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