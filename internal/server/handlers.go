package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/engine"
	"github.com/lacquerai/lacquer/internal/execcontext"
	pkgEvents "github.com/lacquerai/lacquer/pkg/events"
	"github.com/rs/zerolog/log"
)

// HTTP Handlers

// listWorkflows returns all available workflows
func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows := make(map[string]any)

	for _, id := range s.registry.List() {
		workflow, _ := s.registry.Get(id)
		workflows[id] = map[string]any{
			"version": workflow.Version,
			"name":    s.getWorkflowName(workflow),
			"description": func() string {
				if workflow.Metadata != nil {
					return workflow.Metadata.Description
				}
				return ""
			}(),
			"steps": len(workflow.Workflow.Steps),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"workflows": workflows,
	})
}

// executeWorkflow starts a workflow execution
func (s *Server) executeWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workflowID := vars["id"]

	// Get workflow
	workflow, exists := s.registry.Get(workflowID)
	if !exists {
		http.Error(w, fmt.Sprintf("Workflow '%s' not found", workflowID), http.StatusNotFound)
		return
	}

	// Check capacity
	if !s.manager.CanStartExecution() {
		http.Error(w, "Server at capacity, try again later", http.StatusServiceUnavailable)
		return
	}

	// Parse request body for inputs
	var req struct {
		Inputs map[string]any `json:"inputs"`
	}

	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
	}

	if req.Inputs == nil {
		req.Inputs = make(map[string]any)
	}

	// Validate inputs against workflow definition
	validationResult := engine.ValidateWorkflowInputs(workflow, req.Inputs)
	if !validationResult.Valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(formatValidationErrors(validationResult))
		return
	}

	// Use the processed inputs (with defaults applied and type conversions)
	processedInputs := validationResult.ProcessedInputs

	// use background context as hanging off the request context
	// will cause the context to be cancelled when the request is finished.
	ctx, cancel := context.WithCancel(context.Background())

	runCtx := execcontext.RunContext{
		Context: ctx,
		StdOut:  io.Discard,
		StdErr:  io.Discard,
	}
	execCtx := execcontext.NewExecutionContext(runCtx, workflow, processedInputs, workflow.SourceFile)
	runID := execCtx.RunID

	// Start execution tracking
	status := s.manager.StartExecution(runID, workflowID, cancel, processedInputs)

	// Return execution info immediately
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"run_id":      runID,
		"workflow_id": workflowID,
		"status":      "running",
		"started_at":  status.StartTime,
	})

	// Execute workflow asynchronously
	go s.executeWorkflowAsync(ctx, workflow, execCtx, runID, workflowID)
}

// executeWorkflowAsync executes a workflow in the background
func (s *Server) executeWorkflowAsync(ctx context.Context, workflow *ast.Workflow, execCtx *execcontext.ExecutionContext, runID, workflowID string) {
	runner := engine.NewRunner(s.manager)
	result, err := runner.RunWorkflowRaw(execCtx, workflow, time.Now())
	defer runner.Close()
	var outputs map[string]any
	if err == nil {
		outputs = result.Outputs
	}

	// Finish execution
	s.manager.FinishExecution(runID, outputs, err)

	log.Info().
		Str("run_id", runID).
		Str("workflow_id", workflowID).
		Err(err).
		Msg("Workflow execution completed")
}

// getExecution returns the status of a specific execution
func (s *Server) getExecution(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	runID := vars["runId"]

	status, exists := s.manager.GetExecution(runID)
	if !exists {
		http.Error(w, fmt.Sprintf("Execution '%s' not found", runID), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// streamWorkflow provides WebSocket streaming for workflow execution
func (s *Server) streamWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	_ = vars["id"] // workflowID not used in this function

	// Get run ID from query params
	runID := r.URL.Query().Get("run_id")
	if runID == "" {
		http.Error(w, "run_id query parameter required", http.StatusBadRequest)
		return
	}

	// Check if execution exists
	status, exists := s.manager.GetExecution(runID)
	if !exists {
		http.Error(w, fmt.Sprintf("Execution '%s' not found", runID), http.StatusNotFound)
		return
	}

	// Upgrade to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}
	defer conn.Close()

	// Register client
	status.clientsMu.Lock()
	status.clients[conn] = true
	status.clientsMu.Unlock()

	// Send existing progress events
	for _, event := range status.Progress {
		eventJSON, _ := json.Marshal(event)
		conn.WriteMessage(websocket.TextMessage, eventJSON)
	}

	// Send final status if execution is complete
	if status.Status != "running" {
		finalEvent := pkgEvents.ExecutionEvent{
			Type:      pkgEvents.EventWorkflowCompleted,
			Timestamp: time.Now(),
			RunID:     runID,
		}
		if status.Status == "failed" {
			finalEvent.Type = pkgEvents.EventWorkflowFailed
			finalEvent.Error = status.Error
		}
		eventJSON, _ := json.Marshal(finalEvent)
		conn.WriteMessage(websocket.TextMessage, eventJSON)
	}

	// Keep connection alive until execution is done or client disconnects
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}

		status, exists := s.manager.GetExecution(runID)
		if !exists || status.Status != "running" {
			break
		}
	}

	// Unregister client
	status.clientsMu.Lock()
	delete(status.clients, conn)
	status.clientsMu.Unlock()
}

// healthCheck returns server health status
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":            "healthy",
		"workflows_loaded":  s.registry.Count(),
		"active_executions": s.manager.GetActiveExecutions(),
		"timestamp":         time.Now(),
	})
}

// formatValidationErrors formats validation errors for HTTP response
func formatValidationErrors(result *engine.InputValidationResult) map[string]any {
	response := map[string]any{
		"error":   "Input validation failed",
		"details": make([]map[string]any, len(result.Errors)),
	}

	for i, err := range result.Errors {
		response["details"].([]map[string]any)[i] = map[string]any{
			"field":   err.Field,
			"message": err.Message,
		}
		if err.Value != nil {
			response["details"].([]map[string]any)[i]["value"] = err.Value
		}
	}

	return response
}
