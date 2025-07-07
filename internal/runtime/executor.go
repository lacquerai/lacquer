package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/block"
	"github.com/lacquerai/lacquer/internal/style"
	"github.com/rs/zerolog/log"
)

// ExecutionEventType represents the type of execution event
type ExecutionEventType string

const (
	EventWorkflowStarted     ExecutionEventType = "workflow_started"
	EventWorkflowCompleted   ExecutionEventType = "workflow_completed"
	EventWorkflowFailed      ExecutionEventType = "workflow_failed"
	EventStepStarted         ExecutionEventType = "step_started"
	EventStepProgress        ExecutionEventType = "step_progress"
	EventStepCompleted       ExecutionEventType = "step_completed"
	EventStepFailed          ExecutionEventType = "step_failed"
	EventStepSkipped         ExecutionEventType = "step_skipped"
	EventStepRetrying        ExecutionEventType = "step_retrying"
	EventStepActionStarted   ExecutionEventType = "step_action_started"
	EventStepActionCompleted ExecutionEventType = "step_action_completed"
	EventStepActionFailed    ExecutionEventType = "step_action_failed"
)

// ExecutionEvent represents an event during workflow execution
type ExecutionEvent struct {
	Type      ExecutionEventType     `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	RunID     string                 `json:"run_id"`
	StepID    string                 `json:"step_id,omitempty"`
	ActionID  string                 `json:"action_id,omitempty"`
	StepIndex int                    `json:"step_index,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Attempt   int                    `json:"attempt,omitempty"`
	Text      string                 `json:"text,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func NewToolUseEvent(stepID, actionID string, toolName string, runID string, text string) ExecutionEvent {
	input := text
	if input == "" {
		input = generateRandomUsageText(toolName)
	}

	return ExecutionEvent{
		Type:      EventStepActionStarted,
		ActionID:  actionID,
		Text:      input,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    stepID,
	}
}

func NewToolUseCompletedEvent(stepID, actionID string, toolName string, runID string) ExecutionEvent {
	return ExecutionEvent{
		Type:      EventStepActionCompleted,
		ActionID:  actionID,
		Text:      generateRandomUsageText(toolName),
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    stepID,
	}
}

func NewToolUseFailedEvent(step *ast.Step, actionID string, toolName string, runID string) ExecutionEvent {
	return ExecutionEvent{
		Type:      EventStepActionFailed,
		ActionID:  actionID,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    step.ID,
	}
}

func NewPromptAgentEvent(stepID, actionID string, runID string, prompt ...string) ExecutionEvent {
	text := generateRandomPromptingText()
	if len(prompt) > 0 {
		text = strings.Join(prompt, "\n")
		if len(text) > 200 {
			text = text[:200] + "..."
		}
	}
	return ExecutionEvent{
		Type:      EventStepActionStarted,
		ActionID:  actionID,
		Text:      text,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    stepID,
	}
}

func NewAgentCompletedEvent(step *ast.Step, actionID string, runID string) ExecutionEvent {
	return ExecutionEvent{
		Type:      EventStepActionCompleted,
		ActionID:  actionID,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    step.ID,
	}
}

func NewAgentFailedEvent(step *ast.Step, actionID string, runID string) ExecutionEvent {
	return ExecutionEvent{
		Type:      EventStepActionFailed,
		ActionID:  actionID,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    step.ID,
	}
}

func NewGenericActionEvent(stepID, actionID string, runID string, text string) ExecutionEvent {
	return ExecutionEvent{
		Type:      EventStepActionStarted,
		ActionID:  actionID,
		Text:      text,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    stepID,
	}
}

func NewGenericActionCompletedEvent(stepID, actionID string, runID string) ExecutionEvent {
	return ExecutionEvent{
		Type:      EventStepActionCompleted,
		ActionID:  actionID,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    stepID,
	}
}

// Executor is the main workflow execution engine
type Executor struct {
	templateEngine  *TemplateEngine
	modelRegistry   *ModelRegistry
	toolRegistry    *ToolRegistry
	config          *ExecutorConfig
	outputParser    *OutputParser
	schemaGenerator *SchemaGenerator
	progressChan    chan<- ExecutionEvent
	blockManager    *block.Manager
	goExecutor      block.Executor
	dockerExecutor  block.Executor
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
		DefaultTimeout:       30 * time.Minute,
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
		registry = NewModelRegistry(false)
	}

	// Only initialize providers that are used in the workflow
	requiredProviders := getRequiredProviders(workflow)
	if err := initializeRequiredProviders(registry, requiredProviders); err != nil {
		return nil, fmt.Errorf("failed to initialize required providers: %w", err)
	}

	// Create block manager with temporary cache directory
	cacheDir := filepath.Join(os.TempDir(), "laq-blocks")
	blockManager, err := block.NewManager(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create block manager: %w", err)
	}

	// Register native executor with workflow engine
	workflowEngine := NewRuntimeWorkflowEngine(config, registry)
	blockManager.RegisterNativeExecutor(workflowEngine)

	// Create Go executor for script execution
	goExecutor, err := block.NewGoExecutor(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create Go executor: %w", err)
	}

	// Create Docker executor for container execution
	dockerExecutor := block.NewDockerExecutor()

	// Create tool registry
	toolRegistry := NewToolRegistry()

	// Initialize tool providers for workflow
	if err := initializeToolProviders(toolRegistry, workflow, cacheDir); err != nil {
		return nil, fmt.Errorf("failed to initialize tool providers: %w", err)
	}

	return &Executor{
		templateEngine:  NewTemplateEngine(),
		modelRegistry:   registry,
		toolRegistry:    toolRegistry,
		config:          config,
		outputParser:    NewOutputParser(),
		schemaGenerator: NewSchemaGenerator(),
		blockManager:    blockManager,
		goExecutor:      goExecutor,
		dockerExecutor:  dockerExecutor,
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

// executeStep executes a single workflow step
func (e *Executor) executeStep(execCtx *ExecutionContext, step *ast.Step) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			log.Error().Msgf("panic in executeStep: %s\n%s", r, stack)
			err = fmt.Errorf("step execution failed: %s\n%s", r, stack)
		}
	}()

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

	case step.IsScriptStep():
		stepOutput, err = e.executeScriptStep(execCtx, step)

	case step.IsContainerStep():
		stepOutput, err = e.executeContainerStep(execCtx, step)

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

	return e.executeAgentStepWithTools(execCtx, step, agent)
}

// executeAgentStepWithTools executes an agent step with tool support
func (e *Executor) executeAgentStepWithTools(execCtx *ExecutionContext, step *ast.Step, agent *ast.Agent) (string, *TokenUsage, error) {
	prompt, err := e.templateEngine.Render(step.Prompt, execCtx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to render prompt template: %w", err)
	}

	provider, err := e.modelRegistry.GetProviderForModel(agent.Provider, agent.Model)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get provider %s for model %s: %w", agent.Provider, agent.Model, err)
	}

	timeout := e.config.DefaultTimeout
	if step.Timeout != nil {
		timeout = step.Timeout.Duration
	}

	ctx, cancel := context.WithTimeout(execCtx.Context, timeout)
	defer cancel()

	return e.executeConversationWithTools(ctx, provider, agent, prompt, execCtx, step)
}

// executeConversationWithTools handles multi-turn conversation with tool calling
func (e *Executor) executeConversationWithTools(ctx context.Context, provider ModelProvider, agent *ast.Agent, initialPrompt string, execCtx *ExecutionContext, step *ast.Step) (string, *TokenUsage, error) {
	totalTokenUsage := &TokenUsage{}

	// @TODO: make this configurable in the step & or agent definition
	maxTurns := 10

	messages := []ModelMessage{
		{
			Role: "user",
			Content: []ContentBlockParamUnion{
				NewTextBlock(initialPrompt),
			},
		},
	}

	// if the provider is local, don't run in a loop as these models are self contained and
	// handle all the tool calling themselves
	if _, ok := provider.(LocalModelProvider); ok {
		request, err := e.createModelRequestWithTools(agent, messages, provider.GetName())
		if err != nil {
			return "", totalTokenUsage, fmt.Errorf("failed to create model request: %w", err)
		}

		responseMessages, tokenUsage, err := provider.Generate(GenerateContext{
			StepID:  step.ID,
			RunID:   execCtx.RunID,
			Context: ctx,
		}, request, e.progressChan)
		if err != nil {
			return "", tokenUsage, fmt.Errorf("model generation failed: %w", err)
		}

		return getLastContentBlock(responseMessages), tokenUsage, nil
	}

	for turn := 0; turn < maxTurns; turn++ {
		// Create request with tools
		request, err := e.createModelRequestWithTools(agent, messages, provider.GetName())
		if err != nil {
			return "", totalTokenUsage, fmt.Errorf("failed to create model request: %w", err)
		}

		actionID := fmt.Sprintf("turn-%d", turn)
		prompt := getLastContentBlock(messages)
		e.progressChan <- NewPromptAgentEvent(step.ID, actionID, execCtx.RunID, prompt)

		responseMessages, tokenUsage, err := provider.Generate(GenerateContext{
			StepID:  step.ID,
			RunID:   execCtx.RunID,
			Context: ctx,
		}, request, e.progressChan)
		if err != nil {
			e.progressChan <- NewAgentFailedEvent(step, actionID, execCtx.RunID)

			return "", totalTokenUsage, fmt.Errorf("model generation failed: %w", err)
		}

		e.progressChan <- NewAgentCompletedEvent(step, actionID, execCtx.RunID)

		// Accumulate token usage
		if tokenUsage != nil {
			totalTokenUsage.PromptTokens += tokenUsage.PromptTokens
			totalTokenUsage.CompletionTokens += tokenUsage.CompletionTokens
			totalTokenUsage.TotalTokens += tokenUsage.TotalTokens
		}

		// Check if the response contains tool calls
		toolCalls := e.getToolCallsFromResponseMessages(responseMessages)
		if len(toolCalls) == 0 {
			return getLastContentBlock(responseMessages), totalTokenUsage, nil
		}

		// Execute tool calls
		toolResults, err := e.executeToolCalls(ctx, toolCalls, execCtx, step)
		if err != nil {
			return "", totalTokenUsage, fmt.Errorf("tool execution failed: %w", err)
		}

		// add the response messages and the tool results to the messages
		// the response messages are needed so that the id of the tool calls
		// can be matched to the tool results
		messages = append(messages, responseMessages...)
		messages = append(messages, toolResults...)
	}

	return "Max conversation turns reached without completion", totalTokenUsage, nil
}

func (e *Executor) getToolCallsFromResponseMessages(responseMessages []ModelMessage) []*ToolUseBlockParam {
	var toolCalls []*ToolUseBlockParam

	for _, message := range responseMessages {
		for _, content := range message.Content {
			if content.Type() == ContentBlockTypeToolUse {
				toolCalls = append(toolCalls, content.OfToolUse)
			}
		}
	}

	return toolCalls
}

// createModelRequestWithTools creates a model request with tool schemas
func (e *Executor) createModelRequestWithTools(agent *ast.Agent, messages []ModelMessage, providerName string) (*ModelRequest, error) {
	// Create request based on provider type
	switch providerName {
	case "anthropic":
		return e.createAnthropicRequestWithTools(agent, messages)
	case "openai":
		return e.createOpenAIRequestWithTools(agent, messages)
	case "local":
		// local provider does not support tool calling
		return e.createLocalRequest(agent, messages)
	default:
		return nil, fmt.Errorf("unsupported provider for tool calling: %s", providerName)
	}
}

// createLocalRequest creates a local request with tools
func (e *Executor) createLocalRequest(agent *ast.Agent, messages []ModelMessage) (*ModelRequest, error) {
	request := &ModelRequest{
		Model:        agent.Model,
		Messages:     messages,
		SystemPrompt: agent.SystemPrompt,
		Temperature:  agent.Temperature,
		MaxTokens:    agent.MaxTokens,
		TopP:         agent.TopP,

		// Tools are not supported for local models
		// Tools:
	}

	return request, nil
}

// createAnthropicRequestWithTools creates an Anthropic request with tools
func (e *Executor) createAnthropicRequestWithTools(agent *ast.Agent, messages []ModelMessage) (*ModelRequest, error) {
	request := &ModelRequest{
		Model:        agent.Model,
		Messages:     messages,
		SystemPrompt: agent.SystemPrompt,
		Temperature:  agent.Temperature,
		MaxTokens:    agent.MaxTokens,
		TopP:         agent.TopP,
		Tools:        e.toolRegistry.GetToolsForAgent(agent.Name),
		Metadata: map[string]interface{}{
			"provider_type": "anthropic",
		},
	}

	return request, nil
}

// createOpenAIRequestWithTools creates an OpenAI request with tools
func (e *Executor) createOpenAIRequestWithTools(agent *ast.Agent, messages []ModelMessage) (*ModelRequest, error) {
	request := &ModelRequest{
		Model:        agent.Model,
		Messages:     messages,
		SystemPrompt: agent.SystemPrompt,
		Temperature:  agent.Temperature,
		MaxTokens:    agent.MaxTokens,
		TopP:         agent.TopP,
		Tools:        e.toolRegistry.GetToolsForAgent(agent.Name),
		Metadata: map[string]interface{}{
			"provider_type": "openai",
		},
	}

	return request, nil
}

// executeToolCalls executes the tool calls and returns results
func (e *Executor) executeToolCalls(ctx context.Context, toolCalls []*ToolUseBlockParam, execCtx *ExecutionContext, step *ast.Step) ([]ModelMessage, error) {
	var results []ModelMessage

	for _, toolCall := range toolCalls {
		actionID := fmt.Sprintf("tool-%s", toolCall.ID)

		toolCallMsg := formatToolCall(toolCall)
		e.progressChan <- NewToolUseEvent(step.ID, actionID, toolCall.Name, execCtx.RunID, toolCallMsg)

		toolExecCtx := &ToolExecutionContext{
			WorkflowID: execCtx.RunID,
			StepID:     step.ID,
			AgentID:    step.Agent,
			RunID:      execCtx.RunID,
			Context:    ctx,
			Timeout:    e.config.DefaultTimeout,
		}

		// Execute the tool
		result, err := e.toolRegistry.ExecuteTool(ctx, toolCall.Name, toolCall.Input, toolExecCtx)
		if err != nil {
			log.Warn().
				Err(err).
				Str("tool", toolCall.Name).
				Msg("Tool execution failed")

			// Add error result
			isError := true
			results = append(results,
				ModelMessage{
					Role: "user",
					Content: []ContentBlockParamUnion{
						NewToolResultBlock(toolCall.ID, err.Error(), &isError),
					},
				},
			)
			e.progressChan <- NewToolUseFailedEvent(step, actionID, toolCall.Name, execCtx.RunID)
			continue
		}

		e.progressChan <- NewToolUseCompletedEvent(step.ID, actionID, toolCall.Name, execCtx.RunID)

		content := "Tool executed successfully"
		if outputJSON, err := json.Marshal(result.Output); err == nil {
			content = string(outputJSON)
		}

		results = append(results,
			ModelMessage{
				Role: "user",
				Content: []ContentBlockParamUnion{
					NewToolResultBlock(toolCall.ID, content, &result.Success),
				},
			},
		)
	}

	return results, nil
}

// executeBlockStep executes a step that uses a reusable block
func (e *Executor) executeBlockStep(execCtx *ExecutionContext, step *ast.Step) (map[string]interface{}, error) {
	log.Debug().
		Str("step_id", step.ID).
		Str("block", step.Uses).
		Msg("Executing block step")

	// Resolve block path (handle relative paths)
	blockPath := step.Uses
	if !filepath.IsAbs(blockPath) && execCtx.Workflow.SourceFile != "" {
		// Resolve relative to workflow file location
		workflowDir := filepath.Dir(execCtx.Workflow.SourceFile)
		blockPath = filepath.Join(workflowDir, blockPath)
	}

	// Prepare inputs from step.With
	inputs := make(map[string]interface{})
	for key, value := range step.With {
		rendered, err := e.renderValueRecursively(value, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to render input %s: %w", key, err)
		}
		inputs[key] = rendered
	}

	// Execute block using block manager
	outputs, err := e.blockManager.ExecuteBlock(
		execCtx.Context,
		blockPath,
		inputs,
		execCtx.RunID,
		step.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("block execution failed: %w", err)
	}

	return map[string]interface{}{
		"outputs": outputs,
	}, nil
}

// executeScriptStep executes a step that runs a Go script
func (e *Executor) executeScriptStep(execCtx *ExecutionContext, step *ast.Step) (map[string]interface{}, error) {
	log.Debug().
		Str("step_id", step.ID).
		Str("script", step.Script).
		Msg("Executing script step")

	// Prepare inputs from step.With
	inputs := make(map[string]interface{})
	for key, value := range step.With {
		rendered, err := e.renderValueRecursively(value, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to render input %s: %w", key, err)
		}
		inputs[key] = rendered
	}

	// Get script content - either from file or inline
	scriptContent := step.Script
	if strings.HasPrefix(step.Script, "./") || strings.HasPrefix(step.Script, "/") {
		// It's a file path, read the content
		contentBytes, err := os.ReadFile(step.Script)
		if err != nil {
			return nil, fmt.Errorf("failed to read script file %s: %w", step.Script, err)
		}
		scriptContent = string(contentBytes)
	}

	// Create a temporary block configuration for Go script execution
	tempBlock := &block.Block{
		Name:    fmt.Sprintf("script-%s", step.ID),
		Runtime: block.RuntimeGo,
		Script:  scriptContent,
		Inputs:  make(map[string]block.InputSchema),
		Outputs: make(map[string]block.OutputSchema),
	}

	// Validate the block before execution
	if err := e.goExecutor.Validate(tempBlock); err != nil {
		return nil, fmt.Errorf("script validation failed: %w", err)
	}

	// Define dynamic inputs based on step.With
	for key := range inputs {
		tempBlock.Inputs[key] = block.InputSchema{
			Type:     "string", // Default type
			Required: false,
		}
	}

	// Create workspace directory
	workspace := filepath.Join(os.TempDir(), fmt.Sprintf("laq-script-%s", step.ID))
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	// Create execution context for the block
	blockExecCtx := &block.ExecutionContext{
		WorkflowID: execCtx.RunID,
		StepID:     step.ID,
		Workspace:  workspace,
		Timeout:    e.config.DefaultTimeout,
		Context:    execCtx.Context,
	}

	// Execute using Go executor
	outputs, err := e.goExecutor.Execute(execCtx.Context, tempBlock, inputs, blockExecCtx)
	if err != nil {
		return nil, fmt.Errorf("script execution failed: %w", err)
	}

	return outputs, nil
}

// executeContainerStep executes a step that runs a Docker container
func (e *Executor) executeContainerStep(execCtx *ExecutionContext, step *ast.Step) (map[string]interface{}, error) {
	log.Debug().
		Str("step_id", step.ID).
		Str("container", step.Container).
		Msg("Executing container step")

	// Prepare inputs from step.With
	inputs := make(map[string]interface{})
	for key, value := range step.With {
		rendered, err := e.renderValueRecursively(value, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to render input %s: %w", key, err)
		}
		inputs[key] = rendered
	}

	// Create a temporary block configuration for Docker container execution
	tempBlock := &block.Block{
		Name:    fmt.Sprintf("container-%s", step.ID),
		Runtime: block.RuntimeDocker,
		Image:   step.Container,
		Inputs:  make(map[string]block.InputSchema),
		Outputs: make(map[string]block.OutputSchema),
		Command: []string{"sh", "-c", `
			# Extract the inputs from LACQUER_INPUTS environment variable
			if echo "$LACQUER_INPUTS" | grep -q '"message"'; then
				echo '{"outputs": {"message": "processed"}}'
			elif echo "$LACQUER_INPUTS" | grep -q '"text"'; then
				echo '{"outputs": {"result": "processed"}}'
			else
				echo '{"outputs": {"status": "completed"}}'
			fi
		`},
	}

	// Validate the block before execution
	if err := e.dockerExecutor.Validate(tempBlock); err != nil {
		return nil, fmt.Errorf("container validation failed: %w", err)
	}

	// Define dynamic inputs based on step.With
	for key := range inputs {
		tempBlock.Inputs[key] = block.InputSchema{
			Type:     "string", // Default type
			Required: false,
		}
	}

	// Create workspace directory
	workspace := filepath.Join(os.TempDir(), fmt.Sprintf("laq-container-%s", step.ID))
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	// Create execution context for the block
	blockExecCtx := &block.ExecutionContext{
		WorkflowID: execCtx.RunID,
		StepID:     step.ID,
		Workspace:  workspace,
		Timeout:    e.config.DefaultTimeout,
		Context:    execCtx.Context,
	}

	// Execute using Docker executor
	outputs, err := e.dockerExecutor.Execute(execCtx.Context, tempBlock, inputs, blockExecCtx)
	if err != nil {
		return nil, fmt.Errorf("container execution failed: %w", err)
	}

	return outputs, nil
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
	default:
		return nil, fmt.Errorf("unknown action: %s", step.Action)
	}
}

// evaluateSkipCondition evaluates whether a step should be skipped
func (e *Executor) evaluateSkipCondition(execCtx *ExecutionContext, step *ast.Step) (bool, error) {
	if step.SkipIf == "" && step.Condition == "" {
		return false, nil
	}

	if step.SkipIf != "" {
		result, err := e.templateEngine.Render(step.SkipIf, execCtx)
		if err != nil {
			return false, err
		}
		return SafeBool(result), nil
	}

	if step.Condition != "" {
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

// initializeToolProviders initializes tool providers for the workflow
func initializeToolProviders(toolRegistry *ToolRegistry, workflow *ast.Workflow, cacheDir string) error {
	scriptProvider, err := NewScriptToolProvider("local", cacheDir)
	if err != nil {
		return fmt.Errorf("failed to create script tool provider: %w", err)
	}

	if err := toolRegistry.RegisterProvider(scriptProvider); err != nil {
		return fmt.Errorf("failed to register script tool provider: %w", err)
	}

	// Register MCP tool provider
	mcpProvider := NewMCPToolProvider()
	if err := toolRegistry.RegisterProvider(mcpProvider); err != nil {
		return fmt.Errorf("failed to register MCP tool provider: %w", err)
	}

	// TODO: register the workflow provider (block provider)

	for name, agent := range workflow.Agents {
		if err := toolRegistry.RegisterToolsForAgent(agent); err != nil {
			return fmt.Errorf("failed to register tools for agent %s: %w", name, err)
		}
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

func getLastContentBlock(responseMessages []ModelMessage) string {
	if len(responseMessages) == 0 {
		return ""
	}

	lastMessage := responseMessages[len(responseMessages)-1]
	if len(lastMessage.Content) == 0 {
		return ""
	}

	if lastMessage.Content[len(lastMessage.Content)-1].OfText != nil {
		return lastMessage.Content[len(lastMessage.Content)-1].OfText.Text
	}

	if lastMessage.Content[len(lastMessage.Content)-1].OfToolResult != nil {
		return formatToolResult(lastMessage.Content[len(lastMessage.Content)-1].OfToolResult)
	}

	return ""
}

func generateRandomPromptingText() string {
	promptingTexts := []string{
		"Pondering the mysteries of the universe...",
		"Neurons firing at maximum capacity...",
		"Channeling digital wisdom...",
		"Consulting the AI crystal ball...",
		"Launching thoughts into cyberspace...",
		"Juggling ones and zeros...",
		"️Casting computational spells...",
		"Painting with pixels of possibility...",
		"Aiming for the perfect response...",
		"Summoning stellar insights...",
		"Rolling the dice of creativity...",
		"Conducting experiments in thought...",
		"Composing a symphony of words...",
		"Surfing waves of information...",
		"Performing mental acrobatics...",
		"Igniting sparks of brilliance...",
		"Chasing rainbows of logic...",
		"Brewing the perfect response...",
		"Hovering over the solution...",
		"Taming wild thoughts...",
		"Dreaming in binary...",
		"Sketching ideas in the digital ether...",
	}

	return promptingTexts[rand.Intn(len(promptingTexts))]
}

func generateRandomUsageText(rawTool string) string {
	toolName := lipgloss.NewStyle().Foreground(lipgloss.Color("#42A5F5")).Bold(true).Render(rawTool)

	usageTexts := []string{
		fmt.Sprintf("Wielding the mighty %s tool...", toolName),
		fmt.Sprintf("Summoning the power of %s tool...", toolName),
		fmt.Sprintf("Channeling the ancient art of %s tool...", toolName),
		fmt.Sprintf("Invoking %s tool from the depths of cyberspace...", toolName),
		fmt.Sprintf("Whispering sweet commands to %s tool...", toolName),
		fmt.Sprintf("Convincing %s tool to do the heavy lifting...", toolName),
		fmt.Sprintf("Politely asking %s tool to work its magic...", toolName),
		fmt.Sprintf("Giving %s tool a gentle nudge...", toolName),
		fmt.Sprintf("Waking up %s tool from its digital slumber...", toolName),
		fmt.Sprintf("Feeding %s tool some tasty data...", toolName),
		fmt.Sprintf("Cranking the %s tool machine to eleven...", toolName),
		fmt.Sprintf("Letting %s tool stretch its computational legs...", toolName),
	}

	return usageTexts[rand.Intn(len(usageTexts))]
}

func formatToolCall(toolCall *ToolUseBlockParam) string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Using tool %s ", style.InfoStyle.Render(toolCall.Name)))
	var input map[string]interface{}
	err := json.Unmarshal(toolCall.Input, &input)
	if err != nil {
		return sb.String()
	}

	sb.WriteString(formatInputs(input))

	return sb.String()
}

func formatToolResult(toolResult *ToolResultBlockParam) string {
	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("Tool result: %s", toolResult.Content))

	return sb.String()
}

func formatInputs(inputs map[string]interface{}) string {
	sb := strings.Builder{}
	if len(inputs) > 0 {
		sb.WriteString("(")
		var i int
		for key, value := range inputs {
			sb.WriteString(fmt.Sprintf("%s: %v", style.MutedStyle.Render(key), style.MutedStyle.Render(fmt.Sprintf("%v", value))))
			if i != len(inputs)-1 {
				sb.WriteString("; ")
			}
			i++
		}

		sb.WriteString(")")
	}

	return sb.String()
}
