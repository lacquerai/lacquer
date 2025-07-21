// Package engine provides a public API for executing Lacquer workflows programmatically.
// This package allows third-party applications to integrate Lacquer workflow execution
// capabilities directly into their codebase.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/lacquerai/lacquer/internal/engine"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/pkg/events"

	"github.com/rs/zerolog/log"
)

func init() {
	log.Logger.Output(io.Discard)
}

// Option represents a functional option for configuring workflow execution.
// Options allow customization of the workflow runner behavior, such as
// adding progress listeners or modifying execution parameters.
//
// Options follow the functional options pattern, allowing for flexible
// and extensible configuration of the workflow execution engine.
type Option func(*engine.Runner)

// WithProgressListener creates an Option that configures a progress listener
// for monitoring workflow execution events in real-time.
//
// The provided listener will receive execution events throughout the workflow
// lifecycle, including workflow start/completion, step progress, errors, and
// retry attempts. This enables real-time monitoring and logging of workflow
// execution progress.
//
// Parameters:
//   - listener: An implementation of events.Listener that will receive execution events
//
// Returns:
//   - Option: A functional option that can be passed to RunWorkflow
//
// Example:
//
//	type MyListener struct{}
//
//	func (l *MyListener) StartListening(progressChan <-chan events.ExecutionEvent) {
//		for event := range progressChan {
//			fmt.Printf("Event: %s at %s\n", event.Type, event.Timestamp)
//		}
//	}
//
//	func (l *MyListener) StopListening() {
//		fmt.Println("Progress tracking stopped")
//	}
//
//	listener := &MyListener{}
//	outputs, err := RunWorkflow("workflow.laq.yml", inputs, WithProgressListener(listener))
func WithProgressListener(listener events.Listener) Option {
	return func(r *engine.Runner) {
		r.SetProgressListener(listener)
	}
}

// RunWorkflow executes a Lacquer workflow from a YAML definition file with the
// provided inputs and configuration options.
//
// This is the primary entry point for executing Lacquer workflows programmatically.
// The function loads the workflow definition, validates it, and executes all steps
// according to their dependencies and conditions.
//
// The workflow execution includes:
//   - Input validation and type checking
//   - Step dependency resolution and ordering
//   - Conditional execution based on step conditions
//   - Error handling and retry logic
//   - State management across steps
//   - Output collection and transformation
//
// Parameters:
//   - ctx: Context for the workflow execution
//   - workflowFile: Path to the Lacquer workflow YAML file (.laq.yml or .laq.yaml)
//   - inputs: Map of input values that will be available to the workflow steps.
//     Keys should match the input names defined in the workflow's inputs section.
//   - outputs: Pointer to a struct that will be populated with the workflow outputs.
//   - options: Variadic functional options for configuring execution behavior
//
// Returns:
//   - error: Any error that occurred during workflow loading, validation, or execution
//
// Errors can occur due to:
//   - Invalid workflow file path or format
//   - Workflow validation failures (syntax, dependencies, etc.)
//   - Missing or invalid input values
//   - Step execution failures
//   - Runtime errors in scripts or agents
//   - Network or external service failures
//
// Example:
//
//	// Basic workflow execution
//	inputs := map[string]interface{}{
//		"text": "Process this content",
//		"threshold": 0.8,
//	}

//	outputs := &struct {
//		ProcessedData string `json:"processed_data"`
//	}{}
//
// err := RunWorkflow(context.Background(), "data-processing.laq.yml", inputs, outputs)
//
//	if err != nil {
//		return fmt.Errorf("workflow failed: %w", err)
//	}
//
// result := outputs["processed_data"]
//
// // With progress monitoring
// listener := &MyProgressListener{}
// err = RunWorkflow(
//
//	context.Background(),
//	"complex-workflow.laq.yml",
//	inputs,
//	outputs,
//	WithProgressListener(listener)
//
// )
func RunWorkflow(ctx context.Context, workflowFile string, inputs map[string]interface{}, outputs any, options ...Option) error {
	runner := engine.NewRunner(nil)

	for _, option := range options {
		option(runner)
	}

	result, err := runner.RunWorkflow(execcontext.RunContext{
		Context: ctx,
		StdOut:  io.Discard,
		StdErr:  io.Discard,
	}, workflowFile, inputs)
	if err != nil {
		return err
	}

	jsonOutput, err := json.Marshal(result.Outputs)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsonOutput, outputs); err != nil {
		return fmt.Errorf("failed to unmarshal outputs %s: %w", string(jsonOutput), err)
	}

	return nil
}
