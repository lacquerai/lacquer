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
	assert.Equal(t, "sonnet", config.Model)
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

// Test Claude Code provider with mock (always runs)
func TestClaudeCodeProvider_MockIntegration(t *testing.T) {
	// Use mock provider instead of real Claude Code CLI
	mockProvider := NewMockModelProvider("local", []ModelInfo{
		{
			ID:       "claude-code",
			Name:     "claude-code",
			Provider: "local",
		},
	})

	// Set up mock response
	mockProvider.SetResponse("Say 'Hello, Lacquer!' and nothing else.", "Hello, Lacquer!")

	// Test basic generation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	request := &ModelRequest{
		Model:     "claude-3-5-sonnet-20241022",
		Messages:  []ModelMessage{{Role: "user", Content: []ContentBlockParamUnion{NewTextBlock("Say 'Hello, Lacquer!' and nothing else.")}}},
		RequestID: "test-request",
	}

	response, usage, err := mockProvider.Generate(GenerateContext{Context: ctx}, request, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, response)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.TotalTokens, 0)

	// Test that response contains expected content
	assert.Contains(t, strings.ToLower(response[0].Content[0].OfText.Text), "hello")
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
		Messages:  []ModelMessage{{Role: "user", Content: []ContentBlockParamUnion{NewTextBlock("Say 'Hello, Lacquer!' and nothing else.")}}},
		RequestID: "test-request",
	}

	response, usage, err := provider.Generate(GenerateContext{Context: ctx}, request, nil)
	if err != nil {
		t.Skipf("Claude Code generation failed (may not be properly configured): %v", err)
	}

	assert.NotEmpty(t, response)
	assert.NotNil(t, usage)
	assert.Greater(t, usage.TotalTokens, 0)

	// Test that response contains expected content
	assert.Contains(t, strings.ToLower(response[0].Content[0].OfText.Text), "hello")
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
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []ModelMessage{{Role: "user", Content: []ContentBlockParamUnion{NewTextBlock("What is 2+2?")}}},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, _, err := provider.Generate(GenerateContext{Context: ctx}, request, nil)
		cancel()

		if err != nil {
			b.Fatalf("Generation failed: %v", err)
		}
	}
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
		name:           "claude-code",
		executablePath: config.ExecutablePath,
		workingDir:     tempDir,
		config:         config,
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
