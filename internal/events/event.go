package events

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lacquerai/lacquer/internal/ast"
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
