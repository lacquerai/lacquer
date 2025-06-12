package runtime

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ClaudeCodeProvider implements the ModelProvider interface using Claude Code CLI
type ClaudeCodeProvider struct {
	name         string
	executablePath string
	workingDir   string
	models       []string
	sessions     map[string]*ClaudeCodeSession
	sessionMutex sync.RWMutex
	config       *ClaudeCodeConfig
}

// ClaudeCodeConfig contains configuration for Claude Code provider
type ClaudeCodeConfig struct {
	ExecutablePath   string        `yaml:"executable_path"`
	WorkingDirectory string        `yaml:"working_directory"`
	SessionTimeout   time.Duration `yaml:"session_timeout"`
	MaxSessions      int           `yaml:"max_sessions"`
	Model            string        `yaml:"model"`
	EnableTools      bool          `yaml:"enable_tools"`
	LogLevel         string        `yaml:"log_level"`
}

// ClaudeCodeSession represents an active Claude Code session
type ClaudeCodeSession struct {
	ID          string
	Process     *exec.Cmd
	Stdin       io.WriteCloser
	Stdout      io.ReadCloser
	Stderr      io.ReadCloser
	Scanner     *bufio.Scanner
	ErrorScanner *bufio.Scanner
	CreatedAt   time.Time
	LastUsed    time.Time
	WorkingDir  string
	Mutex       sync.Mutex
	Active      bool
}

// ClaudeCodeResponse represents a response from Claude Code
type ClaudeCodeResponse struct {
	Content   string                 `json:"content"`
	ToolUses  []ClaudeCodeToolUse    `json:"tool_uses,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Error     string                 `json:"error,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
}

// ClaudeCodeToolUse represents a tool usage in Claude Code
type ClaudeCodeToolUse struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
	Result     string                 `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// Default configuration for Claude Code
func DefaultClaudeCodeConfig() *ClaudeCodeConfig {
	return &ClaudeCodeConfig{
		ExecutablePath:   "claude", // Assumes claude is in PATH
		WorkingDirectory: "",       // Use current directory
		SessionTimeout:   30 * time.Minute,
		MaxSessions:      5,
		Model:            "claude-3-5-sonnet-20241022", // Default to latest
		EnableTools:      true,
		LogLevel:         "info",
	}
}

// NewClaudeCodeProvider creates a new Claude Code model provider
func NewClaudeCodeProvider(config *ClaudeCodeConfig) (*ClaudeCodeProvider, error) {
	if config == nil {
		config = DefaultClaudeCodeConfig()
	}

	// Detect Claude Code executable
	execPath, err := detectClaudeCodeExecutable(config.ExecutablePath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect Claude Code executable: %w", err)
	}

	// Set working directory
	workingDir := config.WorkingDirectory
	if workingDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workingDir = wd
		} else {
			workingDir = "."
		}
	}

	provider := &ClaudeCodeProvider{
		name:           "claude-code",
		executablePath: execPath,
		workingDir:     workingDir,
		models:         getSupportedClaudeModels(),
		sessions:       make(map[string]*ClaudeCodeSession),
		config:         config,
	}

	log.Info().
		Str("executable", execPath).
		Str("working_dir", workingDir).
		Str("model", config.Model).
		Msg("Claude Code provider initialized")

	return provider, nil
}

// Generate generates a response using Claude Code
func (p *ClaudeCodeProvider) Generate(ctx context.Context, request *ModelRequest) (string, *TokenUsage, error) {
	// Get or create session
	session, err := p.getOrCreateSession(ctx, request.RequestID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get Claude Code session: %w", err)
	}

	// Update last used time
	session.LastUsed = time.Now()

	// Send request to Claude Code
	response, err := p.sendRequest(ctx, session, request)
	if err != nil {
		// If session failed, try to recreate it once
		log.Warn().Err(err).Str("session_id", session.ID).Msg("Session failed, attempting to recreate")
		
		p.closeSession(session.ID)
		session, err = p.getOrCreateSession(ctx, request.RequestID)
		if err != nil {
			return "", nil, fmt.Errorf("failed to recreate Claude Code session: %w", err)
		}
		
		response, err = p.sendRequest(ctx, session, request)
		if err != nil {
			return "", nil, fmt.Errorf("failed to send request to Claude Code: %w", err)
		}
	}

	// Parse response and extract content
	content := response.Content
	if response.Error != "" {
		return "", nil, fmt.Errorf("Claude Code error: %s", response.Error)
	}

	// Estimate token usage (Claude Code doesn't provide exact counts)
	tokenUsage := &TokenUsage{
		PromptTokens:     estimateTokens(request.Prompt + request.SystemPrompt),
		CompletionTokens: estimateTokens(content),
		TotalTokens:      estimateTokens(request.Prompt + request.SystemPrompt + content),
		EstimatedCost:    0.0, // Claude Code is free
	}

	return content, tokenUsage, nil
}

// GetName returns the provider name
func (p *ClaudeCodeProvider) GetName() string {
	return p.name
}

// SupportedModels returns the list of supported models
func (p *ClaudeCodeProvider) SupportedModels() []string {
	return p.models
}

// IsModelSupported checks if a model is supported
func (p *ClaudeCodeProvider) IsModelSupported(model string) bool {
	for _, supported := range p.models {
		if supported == model {
			return true
		}
	}
	return false
}

// Close closes all sessions and cleans up resources
func (p *ClaudeCodeProvider) Close() error {
	p.sessionMutex.Lock()
	defer p.sessionMutex.Unlock()

	var lastErr error
	for sessionID := range p.sessions {
		if err := p.closeSessionUnsafe(sessionID); err != nil {
			lastErr = err
		}
	}

	log.Info().Msg("Claude Code provider closed")
	return lastErr
}

// getOrCreateSession gets an existing session or creates a new one
func (p *ClaudeCodeProvider) getOrCreateSession(ctx context.Context, requestID string) (*ClaudeCodeSession, error) {
	sessionID := requestID
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	p.sessionMutex.Lock()
	defer p.sessionMutex.Unlock()

	// Check if session exists and is active
	if session, exists := p.sessions[sessionID]; exists && session.Active {
		return session, nil
	}

	// Clean up old sessions if we're at the limit
	if len(p.sessions) >= p.config.MaxSessions {
		p.cleanupOldestSessionUnsafe()
	}

	// Create new session
	session, err := p.createSessionUnsafe(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	p.sessions[sessionID] = session
	return session, nil
}

// createSessionUnsafe creates a new Claude Code session (caller must hold lock)
func (p *ClaudeCodeProvider) createSessionUnsafe(ctx context.Context, sessionID string) (*ClaudeCodeSession, error) {
	// Build command arguments
	args := []string{
		"--headless",
		"--no-input-history",
		"--model", p.config.Model,
	}

	if p.config.LogLevel != "" {
		args = append(args, "--log-level", p.config.LogLevel)
	}

	// Create command
	cmd := exec.CommandContext(ctx, p.executablePath, args...)
	cmd.Dir = p.workingDir

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start Claude Code process: %w", err)
	}

	session := &ClaudeCodeSession{
		ID:           sessionID,
		Process:      cmd,
		Stdin:        stdin,
		Stdout:       stdout,
		Stderr:       stderr,
		Scanner:      bufio.NewScanner(stdout),
		ErrorScanner: bufio.NewScanner(stderr),
		CreatedAt:    time.Now(),
		LastUsed:     time.Now(),
		WorkingDir:   p.workingDir,
		Active:       true,
	}

	log.Debug().
		Str("session_id", sessionID).
		Str("working_dir", p.workingDir).
		Msg("Created new Claude Code session")

	return session, nil
}

// sendRequest sends a request to Claude Code session
func (p *ClaudeCodeProvider) sendRequest(ctx context.Context, session *ClaudeCodeSession, request *ModelRequest) (*ClaudeCodeResponse, error) {
	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	if !session.Active {
		return nil, fmt.Errorf("session %s is not active", session.ID)
	}

	// Build the prompt with system prompt if provided
	fullPrompt := request.Prompt
	if request.SystemPrompt != "" {
		fullPrompt = fmt.Sprintf("System: %s\n\nUser: %s", request.SystemPrompt, request.Prompt)
	}

	// Send the prompt
	_, err := session.Stdin.Write([]byte(fullPrompt + "\n"))
	if err != nil {
		session.Active = false
		return nil, fmt.Errorf("failed to write to Claude Code stdin: %w", err)
	}

	// Read response with timeout
	responseChan := make(chan *ClaudeCodeResponse, 1)
	errorChan := make(chan error, 1)

	go func() {
		response, err := p.readResponse(session)
		if err != nil {
			errorChan <- err
		} else {
			responseChan <- response
		}
	}()

	// Wait for response or timeout
	select {
	case response := <-responseChan:
		return response, nil
	case err := <-errorChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("request timeout: %w", ctx.Err())
	}
}

// readResponse reads a response from Claude Code session
func (p *ClaudeCodeProvider) readResponse(session *ClaudeCodeSession) (*ClaudeCodeResponse, error) {
	var responseLines []string
	var inResponse bool

	// Claude Code outputs responses in a specific format
	// We need to parse the output to extract the actual response
	for session.Scanner.Scan() {
		line := session.Scanner.Text()
		
		// Skip empty lines and prompts
		if line == "" || strings.HasPrefix(line, "> ") {
			continue
		}

		// Look for response start/end markers or content
		if strings.Contains(line, "Assistant:") || inResponse {
			inResponse = true
			
			// Clean up the line
			cleaned := strings.TrimSpace(line)
			if strings.HasPrefix(cleaned, "Assistant:") {
				cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, "Assistant:"))
			}
			
			if cleaned != "" {
				responseLines = append(responseLines, cleaned)
			}
			
			// Check for natural ending
			if strings.HasSuffix(line, ".") || strings.HasSuffix(line, "!") || strings.HasSuffix(line, "?") {
				// Look ahead to see if there's more content
				// For now, we'll assume single responses
				break
			}
		}
	}

	if err := session.Scanner.Err(); err != nil {
		session.Active = false
		return nil, fmt.Errorf("error reading from Claude Code stdout: %w", err)
	}

	if len(responseLines) == 0 {
		return nil, fmt.Errorf("no response received from Claude Code")
	}

	response := &ClaudeCodeResponse{
		Content:   strings.Join(responseLines, "\n"),
		SessionID: session.ID,
		Metadata: map[string]interface{}{
			"working_dir": session.WorkingDir,
			"session_id": session.ID,
		},
	}

	return response, nil
}

// closeSession closes a specific session
func (p *ClaudeCodeProvider) closeSession(sessionID string) error {
	p.sessionMutex.Lock()
	defer p.sessionMutex.Unlock()
	return p.closeSessionUnsafe(sessionID)
}

// closeSessionUnsafe closes a session (caller must hold lock)
func (p *ClaudeCodeProvider) closeSessionUnsafe(sessionID string) error {
	session, exists := p.sessions[sessionID]
	if !exists {
		return nil
	}

	session.Active = false

	// Close pipes
	if session.Stdin != nil {
		session.Stdin.Close()
	}
	if session.Stdout != nil {
		session.Stdout.Close()
	}
	if session.Stderr != nil {
		session.Stderr.Close()
	}

	// Terminate process
	if session.Process != nil {
		session.Process.Process.Kill()
		session.Process.Wait()
	}

	delete(p.sessions, sessionID)

	log.Debug().Str("session_id", sessionID).Msg("Closed Claude Code session")
	return nil
}

// cleanupOldestSessionUnsafe removes the oldest session (caller must hold lock)
func (p *ClaudeCodeProvider) cleanupOldestSessionUnsafe() {
	var oldestID string
	var oldestTime time.Time

	for id, session := range p.sessions {
		if oldestID == "" || session.LastUsed.Before(oldestTime) {
			oldestID = id
			oldestTime = session.LastUsed
		}
	}

	if oldestID != "" {
		p.closeSessionUnsafe(oldestID)
	}
}

// Helper functions

// detectClaudeCodeExecutable detects the Claude Code executable path
func detectClaudeCodeExecutable(configPath string) (string, error) {
	// Try configured path first
	if configPath != "" {
		// If it's an absolute path, check if it exists
		if filepath.IsAbs(configPath) {
			if _, err := os.Stat(configPath); err == nil {
				return configPath, nil
			} else {
				// If user specified an absolute path that doesn't exist, fail immediately
				return "", fmt.Errorf("specified Claude Code executable not found: %s", configPath)
			}
		} else {
			// If it's a relative path or command name, look in PATH
			if path, err := exec.LookPath(configPath); err == nil {
				return path, nil
			} else {
				// If user specified a command that doesn't exist, fail immediately
				return "", fmt.Errorf("specified Claude Code executable not found in PATH: %s", configPath)
			}
		}
	}

	// Try common names
	candidates := []string{"claude", "claude-code", "claude_code"}
	
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}

	// Try common installation paths
	commonPaths := []string{
		"/usr/local/bin/claude",
		"/opt/claude/bin/claude",
		filepath.Join(os.Getenv("HOME"), ".local/bin/claude"),
		filepath.Join(os.Getenv("HOME"), "bin/claude"),
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("Claude Code executable not found. Please install Claude Code CLI or set executable_path in configuration")
}

// getSupportedClaudeModels returns the list of supported Claude models
func getSupportedClaudeModels() []string {
	return []string{
		"claude-3-5-sonnet-20241022",
		"claude-3-5-sonnet-20240620",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("claude-code-%d", time.Now().UnixNano())
}

// estimateTokens provides a rough token count estimate
func estimateTokens(text string) int {
	// Rough estimation: ~4 characters per token for English text
	return len(text) / 4
}

// validateClaudeCodeInstallation validates that Claude Code is properly installed
func validateClaudeCodeInstallation(execPath string) error {
	// Try to run claude --version
	cmd := exec.Command(execPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run Claude Code version check: %w", err)
	}

	// Check if output contains expected version information
	outputStr := string(output)
	if !strings.Contains(strings.ToLower(outputStr), "claude") {
		return fmt.Errorf("unexpected version output from Claude Code: %s", outputStr)
	}

	return nil
}

// parseClaudeCodeOutput parses output from Claude Code to extract structured information
func parseClaudeCodeOutput(output string) (*ClaudeCodeResponse, error) {
	// Claude Code may output tool usage, code blocks, or plain text
	// This is a basic parser that can be enhanced based on actual output format
	
	response := &ClaudeCodeResponse{
		Content: output,
		Metadata: make(map[string]interface{}),
	}

	// Look for tool usage patterns
	toolPattern := regexp.MustCompile(`(?s)<function_calls>(.*?)</function_calls>`)
	toolMatches := toolPattern.FindAllStringSubmatch(output, -1)
	
	for _, match := range toolMatches {
		if len(match) > 1 {
			// Parse tool usage
			toolUse := &ClaudeCodeToolUse{
				Name: "unknown",
				Parameters: make(map[string]interface{}),
			}
			
			// Basic extraction - this would need to be more sophisticated
			// for actual Claude Code tool parsing
			toolContent := match[1]
			if strings.Contains(toolContent, "bash") {
				toolUse.Name = "bash"
			} else if strings.Contains(toolContent, "read") {
				toolUse.Name = "read"
			} else if strings.Contains(toolContent, "write") {
				toolUse.Name = "write"
			}
			
			response.ToolUses = append(response.ToolUses, *toolUse)
		}
	}

	return response, nil
}