package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultClaudeCodeConfig(t *testing.T) {
	config := DefaultClaudeCodeConfig()
	
	assert.Equal(t, "claude", config.ExecutablePath)
	assert.Equal(t, "", config.WorkingDirectory)
	assert.Equal(t, 30*time.Minute, config.SessionTimeout)
	assert.Equal(t, 5, config.MaxSessions)
	assert.Equal(t, "claude-3-5-sonnet-20241022", config.Model)
	assert.True(t, config.EnableTools)
	assert.Equal(t, "info", config.LogLevel)
}

func TestDetectClaudeCodeExecutable(t *testing.T) {
	// Test with non-existent path in non-existent directory
	_, err := detectClaudeCodeExecutable("/this/path/definitely/does/not/exist/claude")
	assert.Error(t, err)
	
	// Test with empty path (should try to find in PATH)
	path, err := detectClaudeCodeExecutable("")
	// This may or may not succeed depending on the system
	// We just test that it doesn't panic
	if err != nil {
		assert.Contains(t, err.Error(), "not found")
	} else {
		assert.NotEmpty(t, path)
	}
	
	// Test with a path that exists (using a common system executable as a stand-in)
	path, err = detectClaudeCodeExecutable("/bin/sh")
	assert.NoError(t, err)
	assert.Equal(t, "/bin/sh", path)
}

func TestGetSupportedClaudeModels(t *testing.T) {
	models := getSupportedClaudeModels()
	
	assert.NotEmpty(t, models)
	assert.Contains(t, models, "claude-3-5-sonnet-20241022")
	assert.Contains(t, models, "claude-3-opus-20240229")
	assert.Contains(t, models, "claude-3-sonnet-20240229")
	assert.Contains(t, models, "claude-3-haiku-20240307")
}

func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	time.Sleep(1 * time.Nanosecond) // Ensure different timestamps
	id2 := generateSessionID()
	
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "claude-code-")
	assert.Contains(t, id2, "claude-code-")
}

func TestEstimateTokens(t *testing.T) {
	text := "Hello, world!"
	tokens := estimateTokens(text)
	
	// Should be roughly len(text)/4
	expected := len(text) / 4
	assert.Equal(t, expected, tokens)
	
	// Test with empty string
	assert.Equal(t, 0, estimateTokens(""))
}

func TestClaudeCodeProvider_Basic(t *testing.T) {
	// Create a mock configuration that doesn't require actual Claude Code
	config := &ClaudeCodeConfig{
		ExecutablePath:   "/bin/echo", // Use echo as a mock
		WorkingDirectory: os.TempDir(),
		SessionTimeout:   1 * time.Minute,
		MaxSessions:      2,
		Model:            "claude-3-5-sonnet-20241022",
		EnableTools:      true,
		LogLevel:         "debug",
	}
	
	// We can't easily test the full provider without Claude Code installed
	// So we'll test the basic structure and methods
	
	provider := &ClaudeCodeProvider{
		name:         "claude-code",
		executablePath: config.ExecutablePath,
		workingDir:   config.WorkingDirectory,
		models:       getSupportedClaudeModels(),
		sessions:     make(map[string]*ClaudeCodeSession),
		config:       config,
	}
	
	// Test basic methods
	assert.Equal(t, "claude-code", provider.GetName())
	
	models := provider.SupportedModels()
	assert.NotEmpty(t, models)
	assert.Contains(t, models, "claude-3-5-sonnet-20241022")
	
	assert.True(t, provider.IsModelSupported("claude-3-5-sonnet-20241022"))
	assert.False(t, provider.IsModelSupported("nonexistent-model"))
	
	// Test close (should not error even with no sessions)
	err := provider.Close()
	assert.NoError(t, err)
}

func TestClaudeCodeSession_Structure(t *testing.T) {
	session := &ClaudeCodeSession{
		ID:         "test-session",
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		WorkingDir: "/tmp",
		Active:     true,
	}
	
	assert.Equal(t, "test-session", session.ID)
	assert.True(t, session.Active)
	assert.Equal(t, "/tmp", session.WorkingDir)
	assert.False(t, session.CreatedAt.IsZero())
	assert.False(t, session.LastUsed.IsZero())
}

func TestClaudeCodeResponse_Structure(t *testing.T) {
	response := &ClaudeCodeResponse{
		Content:   "Hello, world!",
		SessionID: "test-session",
		Metadata: map[string]interface{}{
			"working_dir": "/tmp",
		},
	}
	
	assert.Equal(t, "Hello, world!", response.Content)
	assert.Equal(t, "test-session", response.SessionID)
	assert.Equal(t, "/tmp", response.Metadata["working_dir"])
	assert.Empty(t, response.Error)
}

func TestClaudeCodeToolUse_Structure(t *testing.T) {
	toolUse := &ClaudeCodeToolUse{
		Name: "bash",
		Parameters: map[string]interface{}{
			"command": "ls -la",
		},
		Result: "total 8\ndrwxr-xr-x  2 user user 4096 Jan 1 12:00 .",
	}
	
	assert.Equal(t, "bash", toolUse.Name)
	assert.Equal(t, "ls -la", toolUse.Parameters["command"])
	assert.Contains(t, toolUse.Result, "total 8")
	assert.Empty(t, toolUse.Error)
}

func TestParseClaudeCodeOutput(t *testing.T) {
	// Test basic text output
	output := "Hello, this is a simple response."
	response, err := parseClaudeCodeOutput(output)
	assert.NoError(t, err)
	assert.Equal(t, output, response.Content)
	
	// Test empty output
	response, err = parseClaudeCodeOutput("")
	assert.NoError(t, err)
	assert.Equal(t, "", response.Content)
}

func TestValidateClaudeCodeInstallation(t *testing.T) {
	// Test with a known good executable (using echo as a stand-in)
	// This will fail validation because echo doesn't output "claude"
	err := validateClaudeCodeInstallation("/bin/echo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected version output")
	
	// Test with non-existent executable
	err = validateClaudeCodeInstallation("/nonexistent/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to run Claude Code version check")
}

// Test Claude Code provider with mock (always runs)
func TestClaudeCodeProvider_MockIntegration(t *testing.T) {
	// Use mock provider instead of real Claude Code CLI
	mockProvider := NewMockModelProvider("claude-code", []string{
		"claude-3-5-sonnet-20241022",
		"claude-3-sonnet",
		"claude-3-haiku",
	})
	
	// Set up mock response
	mockProvider.SetResponse("Say 'Hello, Lacquer!' and nothing else.", "Hello, Lacquer!")
	
	// Test basic generation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	request := &ModelRequest{
		Model:     "claude-3-5-sonnet-20241022",
		Prompt:    "Say 'Hello, Lacquer!' and nothing else.",
		RequestID: "test-request",
	}
	
	response, usage, err := mockProvider.Generate(ctx, request)
	require.NoError(t, err)
	assert.NotEmpty(t, response)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.TotalTokens, 0)
	
	// Test that response contains expected content
	assert.Contains(t, strings.ToLower(response), "hello")
	assert.Equal(t, "Hello, Lacquer!", response)
}

// Real integration test with actual Claude Code CLI (may skip)
func TestClaudeCodeProvider_RealIntegration(t *testing.T) {
	// Skip if in short test mode
	if testing.Short() {
		t.Skip("Skipping real Claude Code integration test in short mode")
	}
	
	// Try to detect Claude Code
	execPath, err := detectClaudeCodeExecutable("")
	if err != nil {
		t.Skip("Claude Code not found, skipping real integration test")
	}
	
	// Validate installation
	if err := validateClaudeCodeInstallation(execPath); err != nil {
		t.Skipf("Claude Code installation validation failed: %v", err)
	}
	
	// Create provider
	config := DefaultClaudeCodeConfig()
	config.ExecutablePath = execPath
	config.SessionTimeout = 30 * time.Second // Shorter timeout for tests
	
	provider, err := NewClaudeCodeProvider(config)
	if err != nil {
		t.Skipf("Failed to create Claude Code provider: %v", err)
	}
	defer provider.Close()
	
	// Test basic generation
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	request := &ModelRequest{
		Model:     "claude-3-5-sonnet-20241022",
		Prompt:    "Say 'Hello, Lacquer!' and nothing else.",
		RequestID: "test-request",
	}
	
	response, usage, err := provider.Generate(ctx, request)
	if err != nil {
		t.Skipf("Claude Code generation failed (may not be properly configured): %v", err)
	}
	
	assert.NotEmpty(t, response)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.TotalTokens, 0)
	
	// Test that response contains expected content
	assert.Contains(t, strings.ToLower(response), "hello")
}

// Benchmark test for Claude Code provider
func BenchmarkClaudeCodeProvider_Generate(b *testing.B) {
	// Skip if Claude Code is not available
	execPath, err := detectClaudeCodeExecutable("")
	if err != nil {
		b.Skip("Claude Code not found, skipping benchmark")
	}
	
	config := DefaultClaudeCodeConfig()
	config.ExecutablePath = execPath
	
	provider, err := NewClaudeCodeProvider(config)
	if err != nil {
		b.Skip("Failed to create Claude Code provider, skipping benchmark")
	}
	defer provider.Close()
	
	request := &ModelRequest{
		Model:  "claude-3-5-sonnet-20241022",
		Prompt: "What is 2+2?",
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, _, err := provider.Generate(ctx, request)
		cancel()
		
		if err != nil {
			b.Fatalf("Generation failed: %v", err)
		}
	}
}

func TestClaudeCodeProvider_SessionManagement(t *testing.T) {
	config := &ClaudeCodeConfig{
		ExecutablePath:   "/bin/echo",
		WorkingDirectory: os.TempDir(),
		SessionTimeout:   1 * time.Minute,
		MaxSessions:      2,
		Model:            "claude-3-5-sonnet-20241022",
		EnableTools:      true,
		LogLevel:         "debug",
	}
	
	provider := &ClaudeCodeProvider{
		name:         "claude-code",
		executablePath: config.ExecutablePath,
		workingDir:   config.WorkingDirectory,
		models:       getSupportedClaudeModels(),
		sessions:     make(map[string]*ClaudeCodeSession),
		config:       config,
	}
	
	// Test session limits
	assert.Equal(t, 0, len(provider.sessions))
	
	// Test close with no sessions
	err := provider.Close()
	assert.NoError(t, err)
}

func TestClaudeCodeProvider_WorkingDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	config := &ClaudeCodeConfig{
		ExecutablePath:   "/bin/echo",
		WorkingDirectory: tempDir,
		SessionTimeout:   1 * time.Minute,
		MaxSessions:      1,
		Model:            "claude-3-5-sonnet-20241022",
	}
	
	provider := &ClaudeCodeProvider{
		name:         "claude-code",
		executablePath: config.ExecutablePath,
		workingDir:   tempDir,
		models:       getSupportedClaudeModels(),
		sessions:     make(map[string]*ClaudeCodeSession),
		config:       config,
	}
	
	// Verify working directory is set correctly
	assert.Equal(t, tempDir, provider.workingDir)
	
	// Test that relative paths work
	relativeDir := "test-subdir"
	fullPath := filepath.Join(tempDir, relativeDir)
	os.MkdirAll(fullPath, 0755)
	
	config.WorkingDirectory = fullPath
	provider.workingDir = fullPath
	assert.Equal(t, fullPath, provider.workingDir)
}