package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/events"
	"github.com/lacquerai/lacquer/internal/parser"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

// Config holds the server configuration
type Config struct {
	Host            string
	Port            int
	Concurrency     int
	Timeout         time.Duration
	EnableMetrics   bool
	EnableCORS      bool
	WorkflowFiles   []string
	WorkflowDir     string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// DefaultConfig returns a default server configuration
func DefaultConfig() *Config {
	return &Config{
		Host:            "localhost",
		Port:            8080,
		Concurrency:     5,
		Timeout:         30 * time.Minute,
		EnableMetrics:   true,
		EnableCORS:      true,
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}
}

// WorkflowRegistry holds validated workflows
type WorkflowRegistry struct {
	workflows map[string]*ast.Workflow
	mu        sync.RWMutex
}

// NewWorkflowRegistry creates a new workflow registry
func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{
		workflows: make(map[string]*ast.Workflow),
	}
}

// Register adds a workflow to the registry
func (r *WorkflowRegistry) Register(id string, workflow *ast.Workflow) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workflows[id] = workflow
}

// Get retrieves a workflow by ID
func (r *WorkflowRegistry) Get(id string) (*ast.Workflow, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	workflow, exists := r.workflows[id]
	return workflow, exists
}

// List returns all workflow IDs
func (r *WorkflowRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.workflows))
	for id := range r.workflows {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of registered workflows
func (r *WorkflowRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.workflows)
}

// ExecutionStatus represents the status of a workflow execution
type ExecutionStatus struct {
	RunID      string                  `json:"run_id"`
	WorkflowID string                  `json:"workflow_id"`
	Status     string                  `json:"status"`
	StartTime  time.Time               `json:"start_time"`
	EndTime    *time.Time              `json:"end_time,omitempty"`
	Duration   time.Duration           `json:"duration"`
	Inputs     map[string]any          `json:"inputs"`
	Outputs    map[string]any          `json:"outputs,omitempty"`
	Error      string                  `json:"error,omitempty"`
	Progress   []events.ExecutionEvent `json:"progress,omitempty"`

	// WebSocket connections for streaming
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex

	// Context for cancelling the execution
	// @TODO handle cancelling the execution
	cancel context.CancelFunc
}

// ExecutionManager handles concurrent workflow executions
type ExecutionManager struct {
	executions     map[string]*ExecutionStatus
	maxConcurrency int
	currentCount   int
	mu             sync.RWMutex

	// Metrics
	totalExecutions   prometheus.Counter
	activeExecutions  prometheus.Gauge
	executionDuration prometheus.HistogramVec
	executionStatus   prometheus.CounterVec
}

// NewExecutionManager creates a new execution manager
func NewExecutionManager(maxConcurrency int) *ExecutionManager {
	return NewExecutionManagerWithRegistry(maxConcurrency, prometheus.DefaultRegisterer)
}

// NewExecutionManagerWithRegistry creates a new execution manager with a custom registry
func NewExecutionManagerWithRegistry(maxConcurrency int, registerer prometheus.Registerer) *ExecutionManager {
	em := &ExecutionManager{
		executions:     make(map[string]*ExecutionStatus),
		maxConcurrency: maxConcurrency,

		// Initialize Prometheus metrics
		totalExecutions: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "lacquer_executions_total",
			Help: "Total number of workflow executions started",
		}),
		activeExecutions: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "lacquer_executions_active",
			Help: "Number of currently active workflow executions",
		}),
		executionDuration: *prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "lacquer_execution_duration_seconds",
			Help: "Workflow execution duration in seconds",
		}, []string{"workflow_id", "status"}),
		executionStatus: *prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "lacquer_execution_status_total",
			Help: "Total executions by status",
		}, []string{"workflow_id", "status"}),
	}

	// Register metrics with the provided registerer
	if registerer != nil {
		registerer.MustRegister(em.totalExecutions)
		registerer.MustRegister(em.activeExecutions)
		registerer.MustRegister(em.executionDuration)
		registerer.MustRegister(em.executionStatus)
	}

	return em
}

// CanStartExecution checks if a new execution can be started
func (em *ExecutionManager) CanStartExecution() bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.currentCount < em.maxConcurrency
}

// StartExecution starts tracking a new execution
func (em *ExecutionManager) StartExecution(runID, workflowID string, cancel context.CancelFunc, inputs map[string]any) *ExecutionStatus {
	em.mu.Lock()
	defer em.mu.Unlock()

	status := &ExecutionStatus{
		RunID:      runID,
		WorkflowID: workflowID,
		Status:     "running",
		StartTime:  time.Now(),
		Inputs:     inputs,
		Progress:   make([]events.ExecutionEvent, 0),
		clients:    make(map[*websocket.Conn]bool),
		cancel:     cancel,
	}

	em.executions[runID] = status
	em.currentCount++

	// Update metrics
	em.totalExecutions.Inc()
	em.activeExecutions.Inc()

	return status
}

// FinishExecution marks an execution as finished
func (em *ExecutionManager) FinishExecution(runID string, outputs map[string]any, err error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	status, exists := em.executions[runID]
	if !exists {
		return
	}

	now := time.Now()
	status.EndTime = &now
	status.Duration = now.Sub(status.StartTime)
	status.Outputs = outputs

	if err != nil {
		status.Status = "failed"
		status.Error = err.Error()
	} else {
		status.Status = "completed"
	}

	em.currentCount--

	// Update metrics
	em.activeExecutions.Dec()
	em.executionDuration.WithLabelValues(status.WorkflowID, status.Status).Observe(status.Duration.Seconds())
	em.executionStatus.WithLabelValues(status.WorkflowID, status.Status).Inc()

	// Close WebSocket clients
	status.clientsMu.Lock()
	for client := range status.clients {
		client.Close()
	}
	status.clientsMu.Unlock()
}

// GetExecution retrieves an execution status
func (em *ExecutionManager) GetExecution(runID string) (*ExecutionStatus, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()
	status, exists := em.executions[runID]
	return status, exists
}

// AddProgressEvent adds a progress event to an execution
func (em *ExecutionManager) AddProgressEvent(runID string, event events.ExecutionEvent) {
	em.mu.RLock()
	status, exists := em.executions[runID]
	em.mu.RUnlock()

	if !exists {
		return
	}

	em.mu.Lock()
	status.Progress = append(status.Progress, event)
	em.mu.Unlock()

	// Broadcast to WebSocket clients
	status.clientsMu.RLock()
	defer status.clientsMu.RUnlock()

	eventJSON, _ := json.Marshal(event)
	for client := range status.clients {
		client.WriteMessage(websocket.TextMessage, eventJSON)
	}
}

// GetActiveExecutions returns the number of active executions
func (em *ExecutionManager) GetActiveExecutions() int {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.currentCount
}

// Server represents the Lacquer HTTP server
type Server struct {
	config   *Config
	registry *WorkflowRegistry
	manager  *ExecutionManager
	server   *http.Server
	upgrader websocket.Upgrader
}

// New creates a new Lacquer server
func New(config *Config) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	registry := NewWorkflowRegistry()

	server := &Server{
		config:   config,
		registry: registry,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return config.EnableCORS // Allow all origins if CORS enabled
			},
		},
	}

	return server, nil
}

// initializeManager initializes the execution manager if not already set
func (s *Server) initializeManager() {
	if s.manager == nil {
		s.manager = NewExecutionManager(s.config.Concurrency)
	}
}

// LoadWorkflows loads and validates workflows from the configuration
func (s *Server) LoadWorkflows() error {
	// Collect workflow files
	workflowFiles := s.config.WorkflowFiles
	if s.config.WorkflowDir != "" {
		dirFiles, err := s.findWorkflowFiles(s.config.WorkflowDir)
		if err != nil {
			return fmt.Errorf("failed to scan workflow directory: %w", err)
		}
		workflowFiles = append(workflowFiles, dirFiles...)
	}

	if len(workflowFiles) == 0 {
		return fmt.Errorf("no workflow files specified")
	}

	// Parse and validate workflows
	yamlParser, err := parser.NewYAMLParser()
	if err != nil {
		return fmt.Errorf("failed to create parser: %w", err)
	}

	log.Info().Msg("Loading and validating workflows...")
	for _, file := range workflowFiles {
		workflow, err := yamlParser.ParseFile(file)
		if err != nil {
			return fmt.Errorf("failed to parse workflow %s: %w", file, err)
		}

		workflowID := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)), ".laq")
		s.registry.Register(workflowID, workflow)

		log.Info().
			Str("workflow_id", workflowID).
			Str("file", file).
			Str("version", workflow.Version).
			Msg("Workflow loaded")
	}

	if s.registry.Count() == 0 {
		return fmt.Errorf("no valid workflows loaded")
	}

	return nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Initialize manager if not set
	s.initializeManager()

	// Setup routes
	router := mux.NewRouter()

	// Apply CORS middleware to all routes if enabled
	if s.config.EnableCORS {
		router.Use(s.corsMiddleware)
	}

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()
	api.Use(s.loggingMiddleware)

	// Workflow endpoints
	api.HandleFunc("/workflows", s.listWorkflows).Methods("GET")
	api.HandleFunc("/workflows/{id}/execute", s.executeWorkflow).Methods("POST")
	api.HandleFunc("/workflows/{id}/stream", s.streamWorkflow).Methods("GET")

	// Execution endpoints
	api.HandleFunc("/executions/{runId}", s.getExecution).Methods("GET")

	// Handle OPTIONS for CORS preflight
	if s.config.EnableCORS {
		api.Methods("OPTIONS").HandlerFunc(s.handleOptions)
	}

	// Metrics endpoint
	if s.config.EnableMetrics {
		router.Handle("/metrics", promhttp.Handler())
	}

	// Health check
	router.HandleFunc("/health", s.healthCheck)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	log.Info().
		Str("addr", addr).
		Int("workflows", s.registry.Count()).
		Int("concurrency", s.config.Concurrency).
		Bool("metrics", s.config.EnableMetrics).
		Msg("Starting Lacquer server")

	// Start server
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	return nil
}

// Stop stops the HTTP server gracefully
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	log.Info().Msg("Shutting down server...")
	return s.server.Shutdown(ctx)
}

// StartWithGracefulShutdown starts the server and handles graceful shutdown
func (s *Server) StartWithGracefulShutdown() error {
	if err := s.Start(); err != nil {
		return err
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info().Msg("Received shutdown signal")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer shutdownCancel()

		if err := s.Stop(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Server shutdown error")
		}

		cancel()
	}()

	// Wait for shutdown
	<-ctx.Done()
	log.Info().Msg("Server shutdown complete")
	return nil
}

// GetAddr returns the server address
func (s *Server) GetAddr() string {
	if s.server != nil && s.config.Port == 0 {
		// If port was 0, get the actual assigned port
		return s.server.Addr
	}
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// GetWorkflowCount returns the number of loaded workflows
func (s *Server) GetWorkflowCount() int {
	return s.registry.Count()
}

// findWorkflowFiles finds workflow files in a directory
func (s *Server) findWorkflowFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".laq.yaml") || strings.HasSuffix(path, ".laq.yml")) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// getWorkflowName extracts workflow name from metadata
func (s *Server) getWorkflowName(workflow *ast.Workflow) string {
	if workflow.Metadata != nil && workflow.Metadata.Name != "" {
		return workflow.Metadata.Name
	}
	return "Untitled Workflow"
}

// handleOptions handles CORS preflight requests
func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	// CORS headers are already set by middleware
	w.WriteHeader(http.StatusOK)
}
