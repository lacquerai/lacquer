package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	EventStepProgress      ExecutionEventType = "step_progress"
	EventStepCompleted     ExecutionEventType = "step_completed"
	EventStepFailed        ExecutionEventType = "step_failed"
	EventStepSkipped       ExecutionEventType = "step_skipped"
	EventStepRetrying      ExecutionEventType = "step_retrying"
)

// ExecutionEvent represents an event during workflow execution
type ExecutionEvent struct {
	Type      ExecutionEventType     `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	RunID     string                 `json:"run_id"`
	StepID    string                 `json:"step_id,omitempty"`
	StepIndex int                    `json:"step_index,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Attempt   int                    `json:"attempt,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Executor is the main workflow execution engine
type Executor struct {
	templateEngine  *TemplateEngine
	modelRegistry   *ModelRegistry
	config          *ExecutorConfig
	outputParser    *OutputParser
	schemaGenerator *SchemaGenerator
	progressChan    chan<- ExecutionEvent
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
		MaxConcurrentSteps:   3, // Enable concurrent execution with reasonable limit
		DefaultTimeout:       5 * time.Minute,
		EnableRetries:        true,
		MaxRetries:           3,
		RetryDelay:           time.Second,
		EnableMetrics:        true,
		EnableStateSnapshots: false,
	}
}

// NewExecutor creates a new workflow executor with only required providers
func NewExecutor(config *ExecutorConfig, workflow *ast.Workflow, registry *ModelRegistry) (*Executor, error) {
	if config == nil {
		config = DefaultExecutorConfig()
	}

	if registry == nil {
		registry = NewModelRegistry()
	}

	// Only initialize providers that are used in the workflow
	requiredProviders := getRequiredProviders(workflow)
	if err := initializeRequiredProviders(registry, requiredProviders); err != nil {
		return nil, fmt.Errorf("failed to initialize required providers: %w", err)
	}

	return &Executor{
		templateEngine:  NewTemplateEngine(),
		modelRegistry:   registry,
		config:          config,
		outputParser:    NewOutputParser(),
		schemaGenerator: NewSchemaGenerator(),
	}, nil
}

// ExecuteWorkflow runs a workflow with progress events sent to the given channel
func (e *Executor) ExecuteWorkflow(ctx context.Context, execCtx *ExecutionContext, progressChan chan<- ExecutionEvent) error {
	e.progressChan = progressChan
	log.Info().
		Str("workflow", getWorkflowNameFromContext(execCtx)).
		Str("run_id", execCtx.RunID).
		Int("total_steps", execCtx.TotalSteps).
		Msg("Starting workflow execution")

	// Send workflow started event
	if e.progressChan != nil {
		e.progressChan <- ExecutionEvent{
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
		if e.progressChan != nil {
			e.progressChan <- ExecutionEvent{
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
			if e.progressChan != nil {
				e.progressChan <- ExecutionEvent{
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
			if e.progressChan != nil {
				e.progressChan <- ExecutionEvent{
					Type:      EventWorkflowFailed,
					Timestamp: time.Now(),
					RunID:     execCtx.RunID,
					Error:     err.Error(),
				}
			}

			return err
		} else {
			// Send step completed event
			if e.progressChan != nil {
				e.progressChan <- ExecutionEvent{
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

	// Collect workflow outputs if defined
	if err := e.collectWorkflowOutputs(execCtx); err != nil {
		log.Error().
			Err(err).
			Str("run_id", execCtx.RunID).
			Msg("Failed to collect workflow outputs")
		return err
	}

	// Send workflow completed event
	if e.progressChan != nil {
		e.progressChan <- ExecutionEvent{
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

	// Execute workflow steps with concurrency support
	if err := e.executeStepsWithConcurrency(execCtx, workflow.Workflow.Steps); err != nil {
		log.Error().
			Err(err).
			Str("run_id", execCtx.RunID).
			Msg("Workflow execution failed")
		return &ExecutionSummary{
			RunID:    execCtx.RunID,
			Status:   ExecutionStatusFailed,
			Duration: time.Since(execCtx.StartTime),
		}, err
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
		// TODO: should we send a step skipped event?

		result.Status = StepStatusSkipped
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)
		execCtx.SetStepResult(step.ID, result)

		log.Debug().
			Str("step_id", step.ID).
			Msg("Step skipped due to condition")
		return nil
	}

	var stepOutput map[string]interface{}
	var stepResponse string
	var tokenUsage *TokenUsage
	var err error

	switch {
	case step.IsAgentStep():
		stepResponse, tokenUsage, err = e.executeAgentStep(execCtx, step)
		if err == nil {
			// Parse the agent response according to output definitions
			var parseErr error
			stepOutput, parseErr = e.outputParser.ParseStepOutput(step, stepResponse)
			if parseErr != nil {
				log.Warn().
					Err(parseErr).
					Str("step_id", step.ID).
					Msg("Failed to parse agent output, using raw response")
				// Fallback to raw response
				stepOutput = map[string]interface{}{
					"response": stepResponse,
				}
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
		for key, value := range stepOutput {
			execCtx.SetState(fmt.Sprintf("steps.%s.%s", step.ID, key), value)
		}

		// Process state updates for any step type
		if step.Updates != nil {
			updates := make(map[string]interface{})
			for key, value := range step.Updates {
				rendered, renderErr := e.renderValueRecursively(value, execCtx)
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
			}
			execCtx.UpdateState(updates)
		}
	}

	execCtx.SetStepResult(step.ID, result)

	// Increment current step index after processing the step
	execCtx.IncrementCurrentStep()

	return err
}

// executeStepsWithConcurrency executes steps with support for concurrent execution
func (e *Executor) executeStepsWithConcurrency(execCtx *ExecutionContext, steps []*ast.Step) error {
	if e.config.MaxConcurrentSteps <= 1 {
		// Fall back to sequential execution
		return e.executeStepsSequentially(execCtx, steps)
	}

	// Build dependency graph
	dependencies := e.buildDependencyGraph(steps)

	// Track step completion and execution using sync.Map for thread safety
	var completed sync.Map
	var executing sync.Map
	var stepErrors sync.Map

	// Channel to limit concurrent executions
	semaphore := make(chan struct{}, e.config.MaxConcurrentSteps)

	// Channel for step completion notifications
	stepDone := make(chan string, len(steps))

	// Execute steps
	for {
		completedCount := 0
		completed.Range(func(key, value interface{}) bool {
			completedCount++
			return true
		})
		errorCount := 0
		stepErrors.Range(func(key, value interface{}) bool {
			errorCount++
			return true
		})

		if completedCount+errorCount >= len(steps) {
			break
		}
		if execCtx.IsCancelled() {
			return execCtx.Context.Err()
		}

		// Find steps that are ready to execute (dependencies satisfied)
		readySteps := e.findReadySteps(steps, dependencies, &completed, &executing, &stepErrors)

		if len(readySteps) == 0 {
			// Check if we have any steps currently executing
			hasExecutingSteps := false
			executing.Range(func(key, value interface{}) bool {
				if value.(bool) {
					hasExecutingSteps = true
					return false // Stop iteration
				}
				return true
			})

			if !hasExecutingSteps {
				// No steps are executing and none are ready
				// Check if there are steps that can't proceed due to failed dependencies
				hasErrors := false
				stepErrors.Range(func(key, value interface{}) bool {
					hasErrors = true
					return false // Stop iteration
				})
				if hasErrors {
					// Some steps failed, so dependent steps can't proceed
					break
				}
				// Otherwise it's a deadlock
				return fmt.Errorf("workflow execution deadlocked: no steps ready and none executing")
			}

			// Wait for a step to complete
			select {
			case stepID := <-stepDone:
				// Step completed, continue to next iteration
				_ = stepID
			case <-execCtx.Context.Done():
				return execCtx.Context.Err()
			}
			continue
		}

		// Launch ready steps concurrently
		for _, step := range readySteps {
			_, isCompleted := completed.Load(step.ID)
			_, isExecuting := executing.Load(step.ID)
			if isCompleted || isExecuting {
				continue // Skip already completed or executing steps
			}

			// Mark as executing
			executing.Store(step.ID, true)

			// Acquire semaphore
			semaphore <- struct{}{}

			go func(s *ast.Step) {
				defer func() { <-semaphore }() // Release semaphore

				log.Debug().
					Str("run_id", execCtx.RunID).
					Str("step_id", s.ID).
					Msg("Executing step concurrently")

				err := e.executeStep(execCtx, s)
				if err != nil {
					stepErrors.Store(s.ID, err)
					log.Error().
						Err(err).
						Str("run_id", execCtx.RunID).
						Str("step_id", s.ID).
						Msg("Step execution failed")
				} else {
					completed.Store(s.ID, true)
				}

				// Clear executing flag
				executing.Store(s.ID, false)

				// Notify completion
				stepDone <- s.ID
			}(step)
		}
	}

	// Don't return errors for failed steps - let the workflow complete
	// The ExecutionSummary will have the correct status based on step results
	return nil
}

// executeStepsSequentially executes steps one by one (fallback for sequential execution)
func (e *Executor) executeStepsSequentially(execCtx *ExecutionContext, steps []*ast.Step) error {
	for i, step := range steps {
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
			return err
		}
	}
	return nil
}

// buildDependencyGraph analyzes steps to determine dependencies based on template variable usage
func (e *Executor) buildDependencyGraph(steps []*ast.Step) map[string][]string {
	dependencies := make(map[string][]string)

	for _, step := range steps {
		deps := e.findStepDependencies(step, steps)
		if len(deps) > 0 {
			dependencies[step.ID] = deps
		}
	}

	return dependencies
}

// findStepDependencies finds which other steps this step depends on by analyzing template variables
func (e *Executor) findStepDependencies(step *ast.Step, allSteps []*ast.Step) []string {
	var dependencies []string

	// Create a map of step IDs for quick lookup
	stepIDs := make(map[string]bool)
	for _, s := range allSteps {
		stepIDs[s.ID] = true
	}

	// Check template variables in prompt
	if step.Prompt != "" {
		deps := e.extractStepReferences(step.Prompt, stepIDs)
		dependencies = append(dependencies, deps...)
	}

	// Check template variables in condition
	if step.Condition != "" {
		deps := e.extractStepReferences(step.Condition, stepIDs)
		dependencies = append(dependencies, deps...)
	}

	// Check template variables in skip condition
	if step.SkipIf != "" {
		deps := e.extractStepReferences(step.SkipIf, stepIDs)
		dependencies = append(dependencies, deps...)
	}

	// Check template variables in updates
	if step.Updates != nil {
		for _, value := range step.Updates {
			if strValue, ok := value.(string); ok {
				deps := e.extractStepReferences(strValue, stepIDs)
				dependencies = append(dependencies, deps...)
			}
		}
	}

	// Remove duplicates
	unique := make(map[string]bool)
	var result []string
	for _, dep := range dependencies {
		if !unique[dep] {
			unique[dep] = true
			result = append(result, dep)
		}
	}

	return result
}

// extractStepReferences extracts step references like {{ steps.stepname.output }} from template strings
func (e *Executor) extractStepReferences(template string, validStepIDs map[string]bool) []string {
	var dependencies []string

	// Use strings.Contains to find patterns - simpler and more reliable
	for stepID := range validStepIDs {
		// Look for patterns like {{ steps.stepID.output }}
		pattern := "{{ steps." + stepID + "."
		if strings.Contains(template, pattern) {
			dependencies = append(dependencies, stepID)
		}

		// Also check for just {{ steps.stepID }} (without property)
		simplePattern := "{{ steps." + stepID + " }}"
		if strings.Contains(template, simplePattern) {
			// Check if not already added
			found := false
			for _, dep := range dependencies {
				if dep == stepID {
					found = true
					break
				}
			}
			if !found {
				dependencies = append(dependencies, stepID)
			}
		}
	}

	return dependencies
}

// findReadySteps finds steps that have all their dependencies satisfied
func (e *Executor) findReadySteps(steps []*ast.Step, dependencies map[string][]string, completed *sync.Map, executing *sync.Map, stepErrors *sync.Map) []*ast.Step {
	var ready []*ast.Step

	for _, step := range steps {
		_, isCompleted := completed.Load(step.ID)
		_, isExecuting := executing.Load(step.ID)
		if isCompleted || isExecuting {
			continue // Already completed or executing
		}

		// Skip failed steps
		if _, hasFailed := stepErrors.Load(step.ID); hasFailed {
			continue
		}

		// Check if all dependencies are satisfied
		deps, hasDeps := dependencies[step.ID]
		if !hasDeps {
			// No dependencies, ready to execute
			ready = append(ready, step)
			continue
		}

		allDepsSatisfied := true
		for _, dep := range deps {
			if _, isDepCompleted := completed.Load(dep); !isDepCompleted {
				allDepsSatisfied = false
				break
			}
		}

		if allDepsSatisfied {
			ready = append(ready, step)
		}
	}

	return ready
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

	// Enhance prompt with JSON schema instructions if outputs are defined
	if len(step.Outputs) > 0 {
		schema, schemaErr := e.schemaGenerator.GenerateJSONSchema(step.Outputs)
		if schemaErr != nil {
			log.Warn().
				Err(schemaErr).
				Str("step_id", step.ID).
				Msg("Failed to generate JSON schema, proceeding without schema guidance")
		} else if schema != "" {
			schemaInstructions := e.schemaGenerator.GeneratePromptInstructions(schema)
			prompt = prompt + schemaInstructions
		}
	}

	// Get model provider using the provider-aware lookup
	provider, err := e.modelRegistry.GetProviderForModel(agent.Provider, agent.Model)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get provider %s for model %s: %w", agent.Provider, agent.Model, err)
	}

	// Create model request
	request := &ModelRequest{
		Model:        agent.Model,
		Prompt:       prompt,
		SystemPrompt: agent.SystemPrompt,
		Temperature:  agent.Temperature,
		MaxTokens:    agent.MaxTokens,
		TopP:         agent.TopP,
	}

	// Execute with timeout
	timeout := e.config.DefaultTimeout
	if step.Timeout != nil {
		timeout = step.Timeout.Duration
	}

	ctx, cancel := context.WithTimeout(execCtx.Context, timeout)
	defer cancel()

	// Call the model
	response, tokenUsage, err := provider.Generate(ctx, request, e.progressChan)
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
			rendered, err := e.renderValueRecursively(value, execCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to render update value for %s: %w", key, err)
			}
			updates[key] = rendered
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

// renderValueRecursively renders template variables in nested structures
func (e *Executor) renderValueRecursively(value interface{}, execCtx *ExecutionContext) (interface{}, error) {
	switch v := value.(type) {
	case string:
		// Render template strings
		return e.templateEngine.Render(v, execCtx)
	case map[string]interface{}:
		// Recursively render nested maps
		rendered := make(map[string]interface{})
		for key, val := range v {
			renderedVal, err := e.renderValueRecursively(val, execCtx)
			if err != nil {
				return nil, err
			}
			rendered[key] = renderedVal
		}
		return rendered, nil
	case []interface{}:
		// Recursively render arrays
		rendered := make([]interface{}, len(v))
		for i, val := range v {
			renderedVal, err := e.renderValueRecursively(val, execCtx)
			if err != nil {
				return nil, err
			}
			rendered[i] = renderedVal
		}
		return rendered, nil
	default:
		// Return other types as-is (numbers, booleans, etc.)
		return value, nil
	}
}

// getKeys returns the keys of a map as a slice
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getRequiredProviders extracts the unique set of providers used in a workflow
func getRequiredProviders(workflow *ast.Workflow) map[string]bool {
	providers := make(map[string]bool)

	// Check agents for provider usage
	for _, agent := range workflow.Agents {
		if agent.Provider != "" {
			providers[agent.Provider] = true
		}
	}

	return providers
}

// initializeRequiredProviders initializes only the specified providers
func initializeRequiredProviders(registry *ModelRegistry, requiredProviders map[string]bool) error {
	for providerName := range requiredProviders {
		// Check if provider is already registered (e.g., mock providers in tests)
		if _, err := registry.GetProviderByName(providerName); err == nil {
			log.Debug().Str("provider", providerName).Msg("Provider already registered, skipping initialization")
			continue
		}

		var provider ModelProvider
		var err error

		switch providerName {
		case "anthropic":
			provider, err = NewAnthropicProvider(nil)
		case "openai":
			provider, err = NewOpenAIProvider(nil)
		case "local":
			provider, err = NewClaudeCodeProvider(nil)
		default:
			return fmt.Errorf("unknown provider: %s", providerName)
		}

		if err != nil {
			return fmt.Errorf("failed to initialize %s provider: %w", providerName, err)
		}

		if err := registry.RegisterProvider(provider); err != nil {
			return fmt.Errorf("failed to register %s provider: %w", providerName, err)
		}

		log.Info().Str("provider", providerName).Msg("Provider registered successfully")
	}

	return nil
}

// collectWorkflowOutputs collects and renders workflow-level outputs using the template engine
func (e *Executor) collectWorkflowOutputs(execCtx *ExecutionContext) error {
	workflowOutputs := execCtx.Workflow.Workflow.Outputs
	if len(workflowOutputs) == 0 {
		return nil // No outputs defined
	}

	outputs := make(map[string]interface{})

	for key, valueTemplate := range workflowOutputs {
		// Convert the template value to a string for rendering
		templateStr, ok := valueTemplate.(string)
		if !ok {
			// If it's not a string template, use the value as-is
			outputs[key] = valueTemplate
			continue
		}

		// Render the template using the template engine
		renderedValue, err := e.templateEngine.Render(templateStr, execCtx)
		if err != nil {
			log.Error().
				Err(err).
				Str("key", key).
				Str("template", templateStr).
				Msg("Failed to render workflow output template")
			return fmt.Errorf("failed to render output '%s': %w", key, err)
		}

		outputs[key] = renderedValue
	}

	// Set the rendered outputs in the execution context
	execCtx.SetWorkflowOutputs(outputs)

	log.Info().
		Str("run_id", execCtx.RunID).
		Interface("outputs", outputs).
		Msg("Workflow outputs collected successfully")

	return nil
}
