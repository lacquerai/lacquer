package events

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lacquerai/lacquer/internal/ast"

	pkgEvents "github.com/lacquerai/lacquer/pkg/events"
)

func NewToolUseEvent(stepID, actionID string, toolName string, runID string, text string) pkgEvents.ExecutionEvent {
	input := text
	if input == "" {
		input = generateRandomUsageText(toolName)
	}

	return pkgEvents.ExecutionEvent{
		Type:      pkgEvents.EventStepActionStarted,
		ActionID:  actionID,
		Text:      input,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    stepID,
	}
}

func NewToolUseCompletedEvent(stepID, actionID string, toolName string, runID string) pkgEvents.ExecutionEvent {
	return pkgEvents.ExecutionEvent{
		Type:      pkgEvents.EventStepActionCompleted,
		ActionID:  actionID,
		Text:      generateRandomUsageText(toolName),
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    stepID,
	}
}

func NewToolUseFailedEvent(step *ast.Step, actionID string, toolName string, runID string) pkgEvents.ExecutionEvent {
	return pkgEvents.ExecutionEvent{
		Type:      pkgEvents.EventStepActionFailed,
		ActionID:  actionID,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    step.ID,
	}
}

func NewPromptAgentEvent(stepID, actionID string, runID string, prompt ...string) pkgEvents.ExecutionEvent {
	text := generateRandomPromptingText()
	if len(prompt) > 0 {
		text = strings.Join(prompt, "\n")
	}
	return pkgEvents.ExecutionEvent{
		Type:      pkgEvents.EventStepActionStarted,
		ActionID:  actionID,
		Text:      text,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    stepID,
	}
}

func NewAgentCompletedEvent(step *ast.Step, actionID string, runID string) pkgEvents.ExecutionEvent {
	return pkgEvents.ExecutionEvent{
		Type:      pkgEvents.EventStepActionCompleted,
		ActionID:  actionID,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    step.ID,
	}
}

func NewAgentFailedEvent(step *ast.Step, actionID string, runID string) pkgEvents.ExecutionEvent {
	return pkgEvents.ExecutionEvent{
		Type:      pkgEvents.EventStepActionFailed,
		ActionID:  actionID,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    step.ID,
	}
}

func NewGenericActionEvent(stepID, actionID string, runID string, text string) pkgEvents.ExecutionEvent {
	return pkgEvents.ExecutionEvent{
		Type:      pkgEvents.EventStepActionStarted,
		ActionID:  actionID,
		Text:      text,
		Timestamp: time.Now(),
		RunID:     runID,
		StepID:    stepID,
	}
}

func NewGenericActionCompletedEvent(stepID, actionID string, runID string) pkgEvents.ExecutionEvent {
	return pkgEvents.ExecutionEvent{
		Type:      pkgEvents.EventStepActionCompleted,
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
