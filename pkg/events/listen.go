// Package events provides types and interfaces for tracking workflow execution progress.
// This package enables monitoring of workflow lifecycle events, from workflow start
// to completion, including step-level progress tracking and error handling.
//
// The core types allow for real-time progress monitoring of Lacquer workflow executions,
// providing detailed information about each step's execution state, timing, and any
// errors that occur during the process.
package events

import (
	"time"
)

// ExecutionEventType represents the type of execution event that occurred during
// workflow processing. These events provide granular visibility into the workflow
// execution lifecycle.
type ExecutionEventType string

const (
	// EventWorkflowStarted is emitted when a workflow begins execution.
	EventWorkflowStarted ExecutionEventType = "workflow_started"

	// EventWorkflowCompleted is emitted when a workflow successfully completes.
	EventWorkflowCompleted ExecutionEventType = "workflow_completed"

	// EventWorkflowFailed is emitted when a workflow fails and cannot continue.
	EventWorkflowFailed ExecutionEventType = "workflow_failed"

	// EventStepStarted is emitted when an individual step begins execution.
	EventStepStarted ExecutionEventType = "step_started"

	// EventStepProgress is emitted to provide progress updates during step execution.
	EventStepProgress ExecutionEventType = "step_progress"

	// EventStepCompleted is emitted when a step successfully completes.
	EventStepCompleted ExecutionEventType = "step_completed"

	// EventStepFailed is emitted when a step fails during execution.
	EventStepFailed ExecutionEventType = "step_failed"

	// EventStepSkipped is emitted when a step is skipped due to conditions.
	EventStepSkipped ExecutionEventType = "step_skipped"

	// EventStepRetrying is emitted when a step is being retried after failure.
	EventStepRetrying ExecutionEventType = "step_retrying"

	// EventStepActionStarted is emitted when a specific action within a step starts.
	EventStepActionStarted ExecutionEventType = "step_action_started"

	// EventStepActionCompleted is emitted when a specific action within a step completes.
	EventStepActionCompleted ExecutionEventType = "step_action_completed"

	// EventStepActionFailed is emitted when a specific action within a step fails.
	EventStepActionFailed ExecutionEventType = "step_action_failed"
)

// ExecutionEvent represents a single event that occurred during workflow execution.
// It contains detailed information about what happened, when it happened, and
// contextual metadata about the execution state.
type ExecutionEvent struct {
	// Type specifies the kind of execution event that occurred.
	Type ExecutionEventType `json:"type"`
	// Timestamp indicates when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// RunID is the unique identifier for the workflow execution run.
	RunID string `json:"run_id"`
	// StepID is the identifier of the step associated with this event (optional).
	StepID string `json:"step_id,omitempty"`
	// ActionID is the identifier of the specific action within a step (optional).
	ActionID string `json:"action_id,omitempty"`
	// StepIndex is the zero-based index of the step in the workflow (optional).
	StepIndex int `json:"step_index,omitempty"`
	// Duration represents how long the operation took (for completion events).
	Duration time.Duration `json:"duration,omitempty"`
	// Error contains the error message if the event represents a failure.
	Error string `json:"error,omitempty"`
	// Attempt indicates which retry attempt this event represents (1-based).
	Attempt int `json:"attempt,omitempty"`
	// Text provides additional descriptive information about the event.
	Text string `json:"text,omitempty"`
	// Metadata contains additional structured data specific to the event type.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Listener defines the interface for tracking workflow execution progress.
// Implementations of this interface can monitor workflow executions in real-time,
// receiving events as they occur and being notified when tracking should stop.
type Listener interface {
	// StartListening begins monitoring the provided progress channel for execution events.
	// The progressChan parameter contains the channel that will be used to send execution events.
	// This method should be called when workflow execution begins.
	StartListening(progressChan <-chan ExecutionEvent)

	// StopListening signals that progress listening should end.
	StopListening()
}

// NoopListener is a Listener implementation that performs no operations.
// It can be used as a default listener when progress tracking is not needed
// or as a fallback when other tracking mechanisms are unavailable.
type NoopListener struct{}

// StartListening implements the Listener interface but performs no operation.
// This allows the NoopListener to be used as a safe default when
// progress listening is not required.
func (n *NoopListener) StartListening(progressChan <-chan ExecutionEvent) {}

// StopListening implements the Listener interface but performs no operation.
// This method completes the Listener interface implementation for NoopListener.
func (n *NoopListener) StopListening() {}
