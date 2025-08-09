package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/block"
	"github.com/lacquerai/lacquer/internal/events"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/expression"
	"github.com/lacquerai/lacquer/internal/provider"
	"github.com/lacquerai/lacquer/internal/provider/anthropic"
	"github.com/lacquerai/lacquer/internal/provider/claudecode"
	"github.com/lacquerai/lacquer/internal/provider/openai"
	"github.com/lacquerai/lacquer/internal/runtime"
	"github.com/lacquerai/lacquer/internal/tools"
	"github.com/lacquerai/lacquer/internal/tools/mcp"
	"github.com/lacquerai/lacquer/internal/tools/script"
	"github.com/lacquerai/lacquer/internal/utils"
	pkgEvents "github.com/lacquerai/lacquer/pkg/events"
	"github.com/rs/zerolog/log"
)

const (
	JSON_OUTPUT_SCHEMA_PREFIX = "IMPORTANT: Respond in JSON using the following schema:"
)

// Executor orchestrates the execution of workflow steps, managing AI providers,
// tools, templating, and progress reporting. It handles concurrent step execution,
// retry logic, and maintains execution context throughout the workflow lifecycle.
type Executor struct {
	templateEngine *expression.TemplateEngine
	modelRegistry  *provider.Registry
	toolRegistry   *tools.Registry
	config         *ExecutorConfig
	outputParser   *OutputParser
	progressChan   chan<- pkgEvents.ExecutionEvent
	blockManager   *block.Manager
	runner         *Runner

	execCtx *execcontext.ExecutionContext
}

// ExecutorConfig defines the runtime behavior and limits for workflow execution.
// It controls concurrency, timeouts, retry policies, and observability features.
type ExecutorConfig struct {
	MaxConcurrentSteps int           `yaml:"max_concurrent_steps"`
	DefaultTimeout     time.Duration `yaml:"default_timeout"`
	EnableRetries      bool          `yaml:"enable_retries"`
	MaxRetries         int           `yaml:"max_retries"`
	RetryDelay         time.Duration `yaml:"retry_delay"`
	EnableMetrics      bool          `yaml:"enable_metrics"`
}

// DefaultExecutorConfig returns production-ready configuration values with
// moderate concurrency limits and retry policies enabled.
func DefaultExecutorConfig() *ExecutorConfig {
	return &ExecutorConfig{
		MaxConcurrentSteps: 3, // Enable concurrent execution with reasonable limit
		DefaultTimeout:     30 * time.Minute,
		EnableRetries:      true,
		MaxRetries:         3,
		RetryDelay:         time.Second,
		EnableMetrics:      true,
	}
}

// ExecutorFunc is a function that creates a new Executor instance.
type ExecutorFunc func(ctx execcontext.RunContext, config *ExecutorConfig, workflow *ast.Workflow, registry *provider.Registry, runner *Runner) (WorkflowExecutor, error)

// WorkflowExecutor is an interface that defines the methods that an executor must implement.
// This is used to allow for custom executor implementations to be used.
// In general this is only used for testing.
type WorkflowExecutor interface {
	ExecuteWorkflow(execCtx *execcontext.ExecutionContext, progressChan chan<- pkgEvents.ExecutionEvent) error
}

// NewExecutor creates a workflow executor instance with lazy initialization of
// AI providers, tool registries, and runtime dependencies. Only providers and
// tools referenced in the workflow are initialized to minimize resource usage.
// Returns an error if provider initialization or dependency resolution fails.
func NewExecutor(ctx execcontext.RunContext, config *ExecutorConfig, workflow *ast.Workflow, registry *provider.Registry, runner *Runner) (WorkflowExecutor, error) {
	if config == nil {
		config = DefaultExecutorConfig()
	}

	if registry == nil {
		registry = provider.NewRegistry(false)
	}

	// Only initialize providers that are used in the workflow
	requiredProviders := getRequiredProviders(workflow)
	if err := initializeRequiredProviders(registry, requiredProviders); err != nil {
		return nil, fmt.Errorf("failed to initialize required providers: %w", err)
	}

	cacheDir := filepath.Join(os.TempDir(), "laq-blocks")
	blockManager, err := block.NewManager(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create block manager: %w", err)
	}

	runtimeManager, err := runtime.NewManager(filepath.Join(utils.LacquerCacheDir, "runtimes"))
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime manager: %w", err)
	}

	// install any requirements so any script steps can be executed
	if workflow.Requirements != nil {
		for _, runtime := range workflow.Requirements.Runtimes {
			if runtime.Version != "" {
				_, err := runtimeManager.Get(ctx.Context, string(runtime.Name), runtime.Version)
				if err != nil {
					return nil, fmt.Errorf("failed to get runtime %s: %w", runtime.Name, err)
				}
				continue
			}

			_, err := runtimeManager.GetLatest(ctx.Context, string(runtime.Name))
			if err != nil {
				return nil, fmt.Errorf("failed to get latest runtime %s: %w", runtime.Name, err)
			}
		}
	}

	toolRegistry := tools.NewRegistry()

	if err := initializeToolProviders(toolRegistry, workflow, cacheDir); err != nil {
		return nil, fmt.Errorf("failed to initialize tool providers: %w", err)
	}

	return &Executor{
		templateEngine: expression.NewTemplateEngine(),
		modelRegistry:  registry,
		toolRegistry:   toolRegistry,
		config:         config,
		outputParser:   NewOutputParser(),
		blockManager:   blockManager,
		runner:         runner,
	}, nil
}

// ExecuteWorkflow runs the complete workflow, executing steps sequentially while
// respecting dependencies and conditional logic. Progress events are sent to the
// provided channel for real-time monitoring. Returns an error if any step fails
// or if workflow output collection encounters issues.
func (e *Executor) ExecuteWorkflow(execCtx *execcontext.ExecutionContext, progressChan chan<- pkgEvents.ExecutionEvent) error {
	e.execCtx = execCtx
	e.progressChan = progressChan
	log.Info().
		Str("workflow", getWorkflowNameFromContext(execCtx)).
		Str("run_id", execCtx.RunID).
		Int("total_steps", execCtx.TotalSteps).
		Msg("Starting workflow execution")

	if e.progressChan != nil {
		e.progressChan <- pkgEvents.ExecutionEvent{
			Type:      pkgEvents.EventWorkflowStarted,
			Timestamp: time.Now(),
			RunID:     execCtx.RunID,
		}
	}

	if err := e.executeSteps(execCtx, execCtx.Workflow.Workflow.Steps); err != nil {
		return err
	}

	if err := e.collectWorkflowOutputs(execCtx); err != nil {
		log.Error().
			Err(err).
			Str("run_id", execCtx.RunID).
			Msg("Failed to collect workflow outputs")
		return err
	}

	if e.progressChan != nil {
		e.progressChan <- pkgEvents.ExecutionEvent{
			Type:      pkgEvents.EventWorkflowCompleted,
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

func (e *Executor) executeSteps(execCtx *execcontext.ExecutionContext, steps []*ast.Step) error {
	for i, step := range steps {
		if execCtx.IsCancelled() {
			log.Info().Str("run_id", execCtx.RunID).Msg("Workflow execution cancelled")
			break
		}

		execCtx.CurrentStepIndex = i

		stepStart := time.Now()
		err := e.executeStep(execCtx, step)
		stepDuration := time.Since(stepStart)
		if err != nil {
			if err == errStepSkipped {
				log.Debug().
					Str("run_id", execCtx.RunID).
					Str("step_id", step.ID).
					Msg("Step skipped")
				continue
			}

			log.Error().
				Err(err).
				Str("run_id", execCtx.RunID).
				Str("step_id", step.ID).
				Msg("Step execution failed")

			// Send step failed event
			if e.progressChan != nil {
				e.progressChan <- pkgEvents.ExecutionEvent{
					Type:      pkgEvents.EventStepFailed,
					Timestamp: time.Now(),
					RunID:     execCtx.RunID,
					StepID:    step.ID,
					StepIndex: i + 1,
					Duration:  stepDuration,
					Error:     err.Error(),
				}
			}

			result := &execcontext.StepResult{
				StepID:    step.ID,
				Status:    execcontext.StepStatusFailed,
				StartTime: stepStart,
				EndTime:   time.Now(),
				Duration:  stepDuration,
				Error:     err,
			}
			execCtx.SetStepResult(step.ID, result)

			if e.progressChan != nil {
				e.progressChan <- pkgEvents.ExecutionEvent{
					Type:      pkgEvents.EventWorkflowFailed,
					Timestamp: time.Now(),
					RunID:     execCtx.RunID,
					Error:     err.Error(),
				}
			}

			return err
		} else if e.progressChan != nil {
			e.progressChan <- pkgEvents.ExecutionEvent{
				Type:      pkgEvents.EventStepCompleted,
				Timestamp: time.Now(),
				RunID:     execCtx.RunID,
				StepID:    step.ID,
				StepIndex: i + 1,
				Duration:  stepDuration,
			}
		}
	}

	return nil
}

// getWorkflowNameFromContext extracts workflow name from execution context
func getWorkflowNameFromContext(execCtx *execcontext.ExecutionContext) string {
	if execCtx.Workflow.Metadata != nil && execCtx.Workflow.Metadata.Name != "" {
		return execCtx.Workflow.Metadata.Name
	}
	return "Untitled Workflow"
}

var (
	errStepSkipped = fmt.Errorf("step skipped")
)

// executeStep executes a single workflow step
func (e *Executor) executeStep(execCtx *execcontext.ExecutionContext, step *ast.Step) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			log.Error().Msgf("panic in executeStep: %s\n%s", r, stack)
			err = fmt.Errorf("step execution failed: %s\n%s", r, stack)
		}
	}()

	start := time.Now()

	result := &execcontext.StepResult{
		StepID:    step.ID,
		Status:    execcontext.StepStatusRunning,
		StartTime: start,
	}
	execCtx.SetStepResult(step.ID, result)

	// Check if step should be skipped
	if shouldSkip, err := e.evaluateSkipCondition(execCtx, step); err != nil {
		return fmt.Errorf("failed to evaluate skip condition: %w", err)
	} else if shouldSkip {
		// @TODO: should we send a step skipped event?

		result.Status = execcontext.StepStatusSkipped
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)
		execCtx.SetStepResult(step.ID, result)

		log.Debug().
			Str("step_id", step.ID).
			Msg("Step skipped due to condition")
		return errStepSkipped
	}

	if e.progressChan != nil {
		e.progressChan <- pkgEvents.ExecutionEvent{
			Type:      pkgEvents.EventStepStarted,
			Timestamp: time.Now(),
			RunID:     execCtx.RunID,
			StepID:    step.ID,
			StepIndex: execCtx.CurrentStepIndex + 1,
		}
	}

	var stepResult *StepResult
	if step.IsWhileStep() {
		stepResult, err = e.executeWhileStep(execCtx, step)
	} else {
		stepResult, err = e.collectStepResults(execCtx, step)
	}
	if err != nil {
		result.Status = execcontext.StepStatusFailed
		result.Error = err
		return err
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(start)
	result.Response = stepResult.Response
	execCtx.IncrementCurrentStep()

	result.Status = execcontext.StepStatusCompleted
	result.Output = stepResult.Output

	// set the step result before the updates so that we can reference any outputs
	// of the current step in the updates
	execCtx.SetStepResult(step.ID, result)

	// now let's update the state if there are any updates, we can no reference the
	// current step if needed.
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
				continue
			}

			updates[key] = rendered
		}

		execCtx.UpdateState(updates)
	}

	return nil
}

// StepResult contains the execution result of a workflow step, including
// structured output data and the raw response from the execution.
type StepResult struct {
	Output   map[string]interface{}
	Response string
}

// NewStepResult creates a StepResult from execution output, automatically
// formatting structured data and optionally accepting a raw response string.
// The output is normalized to include both "output" and "outputs" fields.
func NewStepResult(output interface{}, raw ...string) *StepResult {
	var response string
	if len(raw) > 0 {
		response = raw[0]
	} else {
		response = expression.ValueToString(output)
	}

	stepResultOut := map[string]interface{}{
		"output": response,
	}

	switch output.(type) {
	case map[string]interface{}:
		stepResultOut["outputs"] = output
	default:
		stepResultOut["output"] = output
	}

	return &StepResult{
		Output:   stepResultOut,
		Response: response,
	}
}

// NewChildStepResult creates a StepResult for composite steps (like while loops)
// that contain nested steps. It aggregates all sub-step outputs and includes
// metadata about the number of iterations executed.
func NewChildStepResult(subExecCtx *execcontext.ExecutionContext, step *ast.Step) *StepResult {
	stepOutputs := make(map[string]interface{}, len(subExecCtx.StepResults))
	for _, subStep := range subExecCtx.StepResults {
		stepOutputs[subStep.StepID] = subStep.Output
	}

	return &StepResult{
		Output: map[string]interface{}{
			"steps":      stepOutputs,
			"iterations": subExecCtx.CurrentStepIndex,
		},
		Response: expression.ValueToString(stepOutputs),
	}
}

func (e *Executor) collectStepResults(execCtx *execcontext.ExecutionContext, step *ast.Step) (*StepResult, error) {
	switch {
	case step.IsAgentStep():
		return e.executeAgentStep(execCtx, step)
	case step.IsBlockStep():
		return e.executeBlockStep(execCtx, step)
	case step.IsScriptStep():
		return e.executeScriptStep(execCtx, step)
	case step.IsContainerStep():
		return e.executeContainerStep(execCtx, step)
	default:
		return nil, fmt.Errorf("unknown step type for step %s", step.ID)
	}
}

func (e *Executor) parseAgentOutput(step *ast.Step, response string) (*StepResult, error) {
	// if there is no output schema, return the raw response as there is nothing to parse
	if len(step.Outputs) == 0 {
		return NewStepResult(response), nil
	}

	stepOutput := e.outputParser.ParseStepOutput(step, response)
	return NewStepResult(stepOutput), nil
}

func (e *Executor) executeWhileStep(execCtx *execcontext.ExecutionContext, step *ast.Step) (*StepResult, error) {
	iterationCount := 0

	subExecCtx := execCtx.NewChild(step.Steps)
	for {
		condition, err := e.templateEngine.Render(step.While, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to render while condition: %w", err)
		}

		conditionBool := utils.SafeBool(condition)
		if !conditionBool {
			log.Debug().
				Str("step_id", step.ID).
				Int("iterations", iterationCount).
				Msg("While condition evaluated to false, exiting loop")
			break
		}

		err = e.executeSteps(subExecCtx, step.Steps)
		if err != nil {
			return nil, err
		}
	}

	return NewChildStepResult(subExecCtx, step), nil
}

// executeAgentStep executes a step that uses an AI agent
func (e *Executor) executeAgentStep(execCtx *execcontext.ExecutionContext, step *ast.Step) (*StepResult, error) {
	agent, exists := execCtx.Workflow.GetAgent(step.Agent)
	if !exists {
		return nil, fmt.Errorf("agent %s not found", step.Agent)
	}

	response, err := e.executeAgentStepWithTools(execCtx, step, agent)
	if err != nil {
		return nil, err
	}

	return e.parseAgentOutput(step, response)
}

// executeAgentStepWithTools executes an agent step with tool support
func (e *Executor) executeAgentStepWithTools(execCtx *execcontext.ExecutionContext, step *ast.Step, agent *ast.Agent) (string, error) {
	initialPrompt, err := e.buildInitialPrompt(execCtx, step)
	if err != nil {
		return "", fmt.Errorf("failed to build initial prompt: %w", err)
	}

	provider, err := e.modelRegistry.GetProviderForModel(agent.Provider, agent.Model)
	if err != nil {
		return "", fmt.Errorf("failed to get provider %s for model %s: %w", agent.Provider, agent.Model, err)
	}

	return e.executeConversationWithTools(execCtx, provider, agent, initialPrompt, step)
}

func (e *Executor) buildInitialPrompt(execCtx *execcontext.ExecutionContext, step *ast.Step) (string, error) {
	prompt, err := e.templateEngine.Render(step.Prompt, execCtx)
	if err != nil {
		return "", fmt.Errorf("failed to render prompt template: %w", err)
	}

	promptString, ok := prompt.(string)
	if !ok {
		return "", fmt.Errorf("prompt is not a string")
	}

	if step.Outputs == nil {
		return promptString, nil
	}

	promptString += "\n\n"
	promptString += JSON_OUTPUT_SCHEMA_PREFIX + "\n"

	jsonSchema, err := json.Marshal(step.Outputs)
	if err != nil {
		return promptString, fmt.Errorf("failed to marshal step outputs: %w", err)
	}
	promptString += "```json\n"
	promptString += string(jsonSchema)
	promptString += "\n```"

	return promptString, nil
}

// executeConversationWithTools handles multi-turn conversation with tool calling
func (e *Executor) executeConversationWithTools(execCtx *execcontext.ExecutionContext, pr provider.Provider, agent *ast.Agent, initialPrompt string, step *ast.Step) (string, error) {
	// @TODO: make this configurable in the step & or agent definition
	maxTurns := 10

	messages := []provider.Message{
		{
			Role: "user",
			Content: []provider.ContentBlockParamUnion{
				provider.NewTextBlock(initialPrompt),
			},
		},
	}

	// if the provider is local, don't run in a loop as these models are self contained and
	// handle all the tool calling themselves
	if _, ok := pr.(provider.LocalModelProvider); ok {
		request, err := e.createModelRequestWithTools(agent, messages, pr.GetName())
		if err != nil {
			return "", fmt.Errorf("failed to create model request: %w", err)
		}

		responseMessages, _, err := pr.Generate(provider.GenerateContext{
			StepID:  step.ID,
			RunID:   execCtx.RunID,
			Context: execCtx.Context.Context,
		}, request, e.progressChan)
		if err != nil {
			return "", fmt.Errorf("model generation failed: %w", err)
		}

		return getLastContentBlock(responseMessages), nil
	}

	for turn := 0; turn < maxTurns; turn++ {
		request, err := e.createModelRequestWithTools(agent, messages, pr.GetName())
		if err != nil {
			return "", fmt.Errorf("failed to create model request: %w", err)
		}

		actionID := fmt.Sprintf("turn-%d", turn)
		prompt := getLastContentBlock(messages)
		prompt = RemoveJSONSchema(prompt)
		e.progressChan <- events.NewPromptAgentEvent(step.ID, actionID, execCtx.RunID, prompt)

		responseMessages, _, err := pr.Generate(provider.GenerateContext{
			StepID:  step.ID,
			RunID:   execCtx.RunID,
			Context: execCtx.Context.Context,
		}, request, e.progressChan)
		if err != nil {
			e.progressChan <- events.NewAgentFailedEvent(step, actionID, execCtx.RunID)

			return "", fmt.Errorf("model generation failed: %w", err)
		}

		e.progressChan <- events.NewAgentCompletedEvent(step, actionID, execCtx.RunID)

		// Check if the response contains tool calls if there are no tool calls
		// its safe to exit with a final response from the response
		toolCalls := e.getToolCallsFromResponseMessages(responseMessages)
		if len(toolCalls) == 0 {
			return getLastContentBlock(responseMessages), nil
		}

		// Execute tool calls
		toolResults, err := e.executeToolCalls(execCtx, toolCalls, step)
		if err != nil {
			return "", fmt.Errorf("tool execution failed: %w", err)
		}

		// add the response messages and the tool results to the messages
		// the response messages are needed so that the id of the tool calls
		// can be matched to the tool results
		messages = append(messages, responseMessages...)
		messages = append(messages, toolResults...)
	}

	return "Max conversation turns reached without completion", nil
}

func (e *Executor) getToolCallsFromResponseMessages(responseMessages []provider.Message) []*provider.ToolUseBlockParam {
	var toolCalls []*provider.ToolUseBlockParam

	for _, message := range responseMessages {
		for _, content := range message.Content {
			if content.Type() == provider.ContentBlockTypeToolUse {
				toolCalls = append(toolCalls, content.OfToolUse)
			}
		}
	}

	return toolCalls
}

// createModelRequestWithTools creates a model request with tool schemas
func (e *Executor) createModelRequestWithTools(agent *ast.Agent, messages []provider.Message, providerName string) (*provider.Request, error) {
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
func (e *Executor) createLocalRequest(agent *ast.Agent, messages []provider.Message) (*provider.Request, error) {
	systemPrompt, err := e.templateEngine.Render(agent.SystemPrompt, e.execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to render system prompt: %w", err)
	}

	request := &provider.Request{
		Model:        agent.Model,
		Messages:     messages,
		SystemPrompt: fmt.Sprintf("%s", systemPrompt),
		Temperature:  agent.Temperature,
		MaxTokens:    agent.MaxTokens,
		TopP:         agent.TopP,

		// Tools are not supported for local models
		// Tools:
	}

	return request, nil
}

// createAnthropicRequestWithTools creates an Anthropic request with tools
func (e *Executor) createAnthropicRequestWithTools(agent *ast.Agent, messages []provider.Message) (*provider.Request, error) {
	systemPrompt, err := e.templateEngine.Render(agent.SystemPrompt, e.execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to render system prompt: %w", err)
	}

	request := &provider.Request{
		Model:        agent.Model,
		Messages:     messages,
		SystemPrompt: fmt.Sprintf("%s", systemPrompt),
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
func (e *Executor) createOpenAIRequestWithTools(agent *ast.Agent, messages []provider.Message) (*provider.Request, error) {
	systemPrompt, err := e.templateEngine.Render(agent.SystemPrompt, e.execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to render system prompt: %w", err)
	}

	request := &provider.Request{
		Model:        agent.Model,
		Messages:     messages,
		SystemPrompt: fmt.Sprintf("%s", systemPrompt),
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
func (e *Executor) executeToolCalls(execCtx *execcontext.ExecutionContext, toolCalls []*provider.ToolUseBlockParam, step *ast.Step) ([]provider.Message, error) {
	var results []provider.Message

	for _, toolCall := range toolCalls {
		actionID := fmt.Sprintf("tool-%s", toolCall.ID)

		toolCallMsg := provider.FormatToolCall(toolCall)
		e.progressChan <- events.NewToolUseEvent(step.ID, actionID, toolCall.Name, execCtx.RunID, toolCallMsg)

		result, err := e.toolRegistry.ExecuteTool(execCtx, toolCall.Name, toolCall.Input)
		if err != nil || result.Error != "" {
			msg := result.Error
			if err != nil {
				msg = err.Error()
			}

			log.Warn().
				Err(errors.New(msg)).
				Str("tool", toolCall.Name).
				Msg("Tool execution failed")

			isError := true
			results = append(results,
				provider.Message{
					Role: "user",
					Content: []provider.ContentBlockParamUnion{
						provider.NewToolResultBlock(toolCall.ID, msg, &isError),
					},
				},
			)
			e.progressChan <- events.NewToolUseFailedEvent(step, actionID, toolCall.Name, execCtx.RunID)
			continue
		}

		e.progressChan <- events.NewToolUseCompletedEvent(step.ID, actionID, toolCall.Name, execCtx.RunID)

		content := "Tool executed successfully"
		if outputJSON, err := json.Marshal(result.Output); err == nil {
			content = string(outputJSON)
		}

		results = append(results,
			provider.Message{
				Role: "user",
				Content: []provider.ContentBlockParamUnion{
					provider.NewToolResultBlock(toolCall.ID, content, &result.Success),
				},
			},
		)
	}

	return results, nil
}

// executeBlockStep executes a step that uses a reusable block
func (e *Executor) executeBlockStep(execCtx *execcontext.ExecutionContext, step *ast.Step) (*StepResult, error) {
	log.Debug().
		Str("step_id", step.ID).
		Str("block", step.Uses).
		Msg("Executing block step")

	blockPath := step.Uses
	if !filepath.IsAbs(blockPath) && execCtx.Workflow.SourceFile != "" {
		workflowDir := filepath.Dir(execCtx.Workflow.SourceFile)
		blockPath = filepath.Join(workflowDir, blockPath)
	}

	inputs := make(map[string]interface{})
	for key, value := range step.With {
		rendered, err := e.renderValueRecursively(value, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to render input %s: %w", key, err)
		}
		inputs[key] = rendered
	}

	result, err := e.runner.RunWorkflow(execCtx.Context, blockPath, inputs, step.ID)
	if err != nil {
		return nil, fmt.Errorf("block execution failed: %w", err)
	}

	return NewStepResult(result.Outputs), nil
}

// executeScriptStep executes a step that runs a Go script
func (e *Executor) executeScriptStep(execCtx *execcontext.ExecutionContext, step *ast.Step) (*StepResult, error) {
	log.Debug().
		Str("step_id", step.ID).
		Str("script", step.Run).
		Msg("Executing script step")

	inputs := make(map[string]interface{})
	for key, value := range step.With {
		rendered, err := e.renderValueRecursively(value, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to render input %s: %w", key, err)
		}
		inputs[key] = rendered
	}

	script, err := e.templateEngine.Render(step.Run, execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to render run string: %w", err)
	}

	tempBlock := &block.Block{
		Name:    fmt.Sprintf("script-%s", step.ID),
		Runtime: block.RuntimeBash,
		Script:  script.(string),
	}

	outputs, err := e.blockManager.ExecuteRawBlock(execCtx, tempBlock, inputs)
	if err != nil {
		return nil, fmt.Errorf("script execution failed: %w", err)
	}

	return NewStepResult(outputs), nil
}

// executeContainerStep executes a step that runs a Docker container
func (e *Executor) executeContainerStep(execCtx *execcontext.ExecutionContext, step *ast.Step) (*StepResult, error) {
	log.Debug().
		Str("step_id", step.ID).
		Str("container", step.Container).
		Msg("Executing container step")

	inputs := make(map[string]interface{})
	for key, value := range step.With {
		rendered, err := e.renderValueRecursively(value, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to render input %s: %w", key, err)
		}
		inputs[key] = rendered
	}

	tempBlock := &block.Block{
		Name:    fmt.Sprintf("container-%s", step.ID),
		Runtime: block.RuntimeDocker,
		Image:   step.Container,
		Inputs:  make(map[string]block.InputSchema),
		Outputs: make(map[string]block.OutputSchema),
		Command: step.Command,
	}

	for key := range inputs {
		tempBlock.Inputs[key] = block.InputSchema{
			Type:     "string",
			Required: false,
		}
	}

	outputs, err := e.blockManager.ExecuteRawBlock(execCtx, tempBlock, inputs)
	if err != nil {
		return nil, fmt.Errorf("container execution failed: %w", err)
	}

	return NewStepResult(outputs), nil
}

// evaluateSkipCondition evaluates whether a step should be skipped
func (e *Executor) evaluateSkipCondition(execCtx *execcontext.ExecutionContext, step *ast.Step) (bool, error) {
	if step.SkipIf == "" && step.Condition == "" {
		return false, nil
	}

	if step.SkipIf != "" {
		result, err := e.templateEngine.Render(step.SkipIf, execCtx)
		if err != nil {
			return false, err
		}
		return utils.SafeBool(result), nil
	}

	if step.Condition != "" {
		result, err := e.templateEngine.Render(step.Condition, execCtx)
		if err != nil {
			return false, err
		}
		return !utils.SafeBool(result), nil // Skip if condition is false
	}

	return false, nil
}

// renderValueRecursively renders template variables in nested structures
func (e *Executor) renderValueRecursively(value interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return e.templateEngine.Render(v, execCtx)
	case map[string]interface{}:
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

// getRequiredProviders extracts the unique set of providers used in a workflow
func getRequiredProviders(workflow *ast.Workflow) map[string]map[string]interface{} {
	providers := make(map[string]map[string]interface{})

	for _, agent := range workflow.Agents {
		if agent.Provider != "" {
			providers[agent.Provider] = agent.Config
		}
	}

	return providers
}

// initializeRequiredProviders initializes only the specified providers
func initializeRequiredProviders(registry *provider.Registry, requiredProviders map[string]map[string]interface{}) error {
	for providerName, config := range requiredProviders {
		if _, err := registry.GetProviderByName(providerName); err == nil {
			log.Debug().Str("provider", providerName).Msg("Provider already registered, skipping initialization")
			continue
		}

		var pr provider.Provider
		var err error

		switch providerName {
		case "anthropic":
			pr, err = anthropic.NewProvider(config)
		case "openai":
			pr, err = openai.NewProvider(config)
		case "local":
			pr, err = claudecode.NewProvider(config)
		default:
			return fmt.Errorf("unknown provider: %s", providerName)
		}

		if err != nil {
			return fmt.Errorf("failed to initialize %s provider: %w", providerName, err)
		}

		if err := registry.RegisterProvider(pr); err != nil {
			return fmt.Errorf("failed to register %s provider: %w", providerName, err)
		}

		log.Info().Str("provider", providerName).Msg("Provider registered successfully")
	}

	return nil
}

// initializeToolProviders initializes tool providers for the workflow
func initializeToolProviders(toolRegistry *tools.Registry, workflow *ast.Workflow, cacheDir string) error {
	scriptProvider, err := script.NewScriptToolProvider("local", cacheDir)
	if err != nil {
		return fmt.Errorf("failed to create script tool provider: %w", err)
	}

	if err := toolRegistry.RegisterProvider(scriptProvider); err != nil {
		return fmt.Errorf("failed to register script tool provider: %w", err)
	}

	mcpProvider := mcp.NewMCPToolProvider()
	if err := toolRegistry.RegisterProvider(mcpProvider); err != nil {
		return fmt.Errorf("failed to register MCP tool provider: %w", err)
	}

	// @TODO: register the workflow provider (block provider)

	for name, agent := range workflow.Agents {
		if err := toolRegistry.RegisterToolsForAgent(agent); err != nil {
			return fmt.Errorf("failed to register tools for agent %s: %w", name, err)
		}
	}

	return nil
}

// collectWorkflowOutputs collects and renders workflow-level outputs using the template engine
func (e *Executor) collectWorkflowOutputs(execCtx *execcontext.ExecutionContext) error {
	workflowOutputs := execCtx.Workflow.Workflow.Outputs
	if len(workflowOutputs) == 0 {
		return nil // No outputs defined
	}

	outputs := make(map[string]interface{})

	for key, valueTemplate := range workflowOutputs {
		renderedValue, err := e.renderValueRecursively(valueTemplate, execCtx)
		if err != nil {
			log.Error().
				Err(err).
				Str("key", key).
				Interface("template", valueTemplate).
				Msg("Failed to render workflow output template")
			return fmt.Errorf("failed to render output '%s': %w", key, err)
		}

		outputs[key] = expression.ValueToString(renderedValue)
	}

	execCtx.SetWorkflowOutputs(outputs)

	log.Info().
		Str("run_id", execCtx.RunID).
		Interface("outputs", outputs).
		Msg("Workflow outputs collected successfully")

	return nil
}

func getLastContentBlock(responseMessages []provider.Message) string {
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
		return provider.FormatToolResult(lastMessage.Content[len(lastMessage.Content)-1].OfToolResult)
	}

	return ""
}

// RemoveJSONSchema strips JSON schema instructions from AI model prompts
// by removing the "IMPORTANT:" directive and associated JSON schema blocks.
// This is used to clean prompts for display purposes while preserving the
// original prompt content.
func RemoveJSONSchema(input string) string {
	lines := strings.Split(input, "\n")

	// Find the last occurrence of the IMPORTANT line
	importantLineIndex := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], JSON_OUTPUT_SCHEMA_PREFIX) {
			importantLineIndex = i
			break
		}
	}

	// If not found, return original
	if importantLineIndex == -1 {
		return input
	}

	// Look for the pattern: code block start, JSON content, code block end
	foundCodeBlockStart := false

	for i := importantLineIndex + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if !foundCodeBlockStart && strings.HasPrefix(line, "```") {
			foundCodeBlockStart = true
			continue
		}

		if foundCodeBlockStart && line == "```" {
			// Check if this is the last non-empty content
			hasContentAfter := false
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) != "" {
					hasContentAfter = true
					break
				}
			}

			// If no content after, remove from importantLineIndex to end
			if !hasContentAfter {
				result := strings.Join(lines[:importantLineIndex], "\n")
				return strings.TrimRight(result, " \t\n")
			}
			break
		}
	}

	return input
}
