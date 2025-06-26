package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/block"
	"github.com/rs/zerolog/log"
)

// ScriptToolProvider implements the ToolProvider interface for script-based tools
type ScriptToolProvider struct {
	name       string
	scripts    map[string]*ScriptTool
	goExecutor block.Executor
	cacheDir   string
	mu         sync.RWMutex
}

// ScriptTool represents a script-based tool
type ScriptTool struct {
	Name        string
	Description string
	ScriptPath  string
	ScriptType  string // "go", "python", "bash", etc.
	Content     string // Inline script content
	Parameters  []ToolParameter
}

// NewScriptToolProvider creates a new script tool provider
func NewScriptToolProvider(name string, cacheDir string) (*ScriptToolProvider, error) {
	if name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "laq-script-tools")
	}

	// Create cache directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create Go executor for Go script execution
	goExecutor, err := block.NewGoExecutor(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create Go executor: %w", err)
	}

	return &ScriptToolProvider{
		name:       name,
		scripts:    make(map[string]*ScriptTool),
		goExecutor: goExecutor,
		cacheDir:   cacheDir,
	}, nil
}

// GetName returns the provider name
func (stp *ScriptToolProvider) GetName() string {
	return stp.name
}

// GetType returns the tool type this provider handles
func (stp *ScriptToolProvider) GetType() ToolType {
	return ToolTypeScript
}

// AddScript adds a script tool to the provider
func (stp *ScriptToolProvider) AddScript(tool *ScriptTool) error {
	stp.mu.Lock()
	defer stp.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("script tool name is required")
	}

	if tool.ScriptPath == "" && tool.Content == "" {
		return fmt.Errorf("script tool must have either script path or content")
	}

	// Validate script content if provided
	if tool.Content != "" {
		if err := stp.validateScriptContent(tool); err != nil {
			return fmt.Errorf("script validation failed: %w", err)
		}
	}

	stp.scripts[tool.Name] = tool
	log.Info().
		Str("provider", stp.name).
		Str("tool", tool.Name).
		Str("type", tool.ScriptType).
		Msg("Added script tool")

	return nil
}

// validateScriptContent performs basic validation on script content
func (stp *ScriptToolProvider) validateScriptContent(tool *ScriptTool) error {
	switch tool.ScriptType {
	case "go":
		// Basic validation for Go scripts
		if !strings.Contains(tool.Content, "package main") {
			return fmt.Errorf("Go script must have 'package main'")
		}
		if !strings.Contains(tool.Content, "func main()") {
			return fmt.Errorf("Go script must have 'func main()'")
		}
	case "python":
		// Basic validation for Python scripts
		if strings.TrimSpace(tool.Content) == "" {
			return fmt.Errorf("Python script cannot be empty")
		}
	case "bash":
		// Basic validation for Bash scripts
		if strings.TrimSpace(tool.Content) == "" {
			return fmt.Errorf("Bash script cannot be empty")
		}
	default:
		return fmt.Errorf("unsupported script type: %s", tool.ScriptType)
	}
	return nil
}

// DiscoverTools discovers available script tools
func (stp *ScriptToolProvider) DiscoverTools(ctx context.Context) ([]ToolDefinition, error) {
	stp.mu.RLock()
	defer stp.mu.RUnlock()

	var tools []ToolDefinition

	for _, scriptTool := range stp.scripts {
		toolDef := ToolDefinition{
			Name:        scriptTool.Name,
			Description: scriptTool.Description,
			Type:        ToolTypeScript,
			Parameters:  scriptTool.Parameters,
			Source:      scriptTool.ScriptPath,
			ProviderID:  stp.name,
			Config: map[string]interface{}{
				"script_type": scriptTool.ScriptType,
				"script_path": scriptTool.ScriptPath,
				"has_content": scriptTool.Content != "",
			},
		}

		tools = append(tools, toolDef)
	}

	log.Info().
		Str("provider", stp.name).
		Int("tool_count", len(tools)).
		Msg("Discovered script tools")

	return tools, nil
}

// ExecuteTool executes a script tool
func (stp *ScriptToolProvider) ExecuteTool(ctx context.Context, toolName string, parameters map[string]interface{}, execCtx *ToolExecutionContext) (*ToolResult, error) {
	stp.mu.RLock()
	defer stp.mu.RUnlock()

	scriptTool, exists := stp.scripts[toolName]
	if !exists {
		return nil, fmt.Errorf("script tool %s not found", toolName)
	}

	startTime := time.Now()

	// Execute based on script type
	var output map[string]interface{}
	var err error

	switch scriptTool.ScriptType {
	case "go":
		output, err = stp.executeGoScript(ctx, scriptTool, parameters, execCtx)
	case "python":
		output, err = stp.executePythonScript(ctx, scriptTool, parameters, execCtx)
	case "bash":
		output, err = stp.executeBashScript(ctx, scriptTool, parameters, execCtx)
	default:
		err = fmt.Errorf("unsupported script type: %s", scriptTool.ScriptType)
	}

	duration := time.Since(startTime)

	if err != nil {
		return &ToolResult{
			ToolName: toolName,
			Success:  false,
			Error:    err.Error(),
			Duration: duration,
			Metadata: map[string]interface{}{
				"provider":    stp.name,
				"script_type": scriptTool.ScriptType,
				"tool_type":   string(ToolTypeScript),
			},
		}, nil
	}

	return &ToolResult{
		ToolName: toolName,
		Success:  true,
		Output:   output,
		Duration: duration,
		Metadata: map[string]interface{}{
			"provider":    stp.name,
			"script_type": scriptTool.ScriptType,
			"tool_type":   string(ToolTypeScript),
		},
	}, nil
}

// executeGoScript executes a Go script using the existing block executor
func (stp *ScriptToolProvider) executeGoScript(ctx context.Context, scriptTool *ScriptTool, parameters map[string]interface{}, execCtx *ToolExecutionContext) (map[string]interface{}, error) {
	// Get script content
	scriptContent := scriptTool.Content
	if scriptContent == "" && scriptTool.ScriptPath != "" {
		contentBytes, err := os.ReadFile(scriptTool.ScriptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read script file: %w", err)
		}
		scriptContent = string(contentBytes)
	}

	// Create a temporary block for execution
	tempBlock := &block.Block{
		Name:    fmt.Sprintf("tool-%s", scriptTool.Name),
		Runtime: block.RuntimeGo,
		Script:  scriptContent,
		Inputs:  make(map[string]block.InputSchema),
		Outputs: make(map[string]block.OutputSchema),
	}

	// Define dynamic inputs based on parameters
	for key := range parameters {
		tempBlock.Inputs[key] = block.InputSchema{
			Type:     "string", // Default type
			Required: false,
		}
	}

	// Create workspace directory
	workspace := filepath.Join(stp.cacheDir, fmt.Sprintf("tool-%s-%s", scriptTool.Name, execCtx.StepID))
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}
	defer os.RemoveAll(workspace) // Clean up

	// Create execution context for the block
	blockExecCtx := &block.ExecutionContext{
		WorkflowID: execCtx.WorkflowID,
		StepID:     execCtx.StepID,
		Workspace:  workspace,
		Timeout:    execCtx.Timeout,
		Context:    execCtx.Context,
	}

	// Execute using Go executor
	return stp.goExecutor.Execute(ctx, tempBlock, parameters, blockExecCtx)
}

// executePythonScript executes a Python script
func (stp *ScriptToolProvider) executePythonScript(ctx context.Context, scriptTool *ScriptTool, parameters map[string]interface{}, execCtx *ToolExecutionContext) (map[string]interface{}, error) {
	// For MVP, implement basic Python script execution
	// In a full implementation, this would use a proper Python executor similar to the Go executor

	// Get script content
	scriptContent := scriptTool.Content
	if scriptContent == "" && scriptTool.ScriptPath != "" {
		contentBytes, err := os.ReadFile(scriptTool.ScriptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read script file: %w", err)
		}
		scriptContent = string(contentBytes)
	}

	return stp.executeScriptWithCommand(ctx, "python3", scriptContent, parameters, execCtx)
}

// executeBashScript executes a Bash script
func (stp *ScriptToolProvider) executeBashScript(ctx context.Context, scriptTool *ScriptTool, parameters map[string]interface{}, execCtx *ToolExecutionContext) (map[string]interface{}, error) {
	// Get script content
	scriptContent := scriptTool.Content
	if scriptContent == "" && scriptTool.ScriptPath != "" {
		contentBytes, err := os.ReadFile(scriptTool.ScriptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read script file: %w", err)
		}
		scriptContent = string(contentBytes)
	}

	return stp.executeScriptWithCommand(ctx, "bash", scriptContent, parameters, execCtx)
}

// executeScriptWithCommand executes a script using the specified command
func (stp *ScriptToolProvider) executeScriptWithCommand(ctx context.Context, command string, scriptContent string, parameters map[string]interface{}, execCtx *ToolExecutionContext) (map[string]interface{}, error) {
	// Create temporary script file
	workspace := filepath.Join(stp.cacheDir, fmt.Sprintf("tool-%s", execCtx.StepID))
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}
	defer os.RemoveAll(workspace) // Clean up

	scriptFile := filepath.Join(workspace, "script")
	if err := os.WriteFile(scriptFile, []byte(scriptContent), 0755); err != nil {
		return nil, fmt.Errorf("failed to write script file: %w", err)
	}

	// Create input JSON file
	inputFile := filepath.Join(workspace, "input.json")
	inputData := map[string]interface{}{
		"inputs": parameters,
		"env":    make(map[string]string),
		"context": map[string]interface{}{
			"workflow_id": execCtx.WorkflowID,
			"step_id":     execCtx.StepID,
			"agent_id":    execCtx.AgentID,
			"workspace":   workspace,
		},
	}

	inputBytes, err := encodeJSON(inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to encode input data: %w", err)
	}

	if err := os.WriteFile(inputFile, inputBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write input file: %w", err)
	}

	// Execute script with timeout
	toolCtx := execCtx.Context
	if execCtx.Timeout > 0 {
		var cancel context.CancelFunc
		toolCtx, cancel = context.WithTimeout(execCtx.Context, execCtx.Timeout)
		defer cancel()
	}

	// Run the script
	output, err := stp.runScript(toolCtx, command, scriptFile, inputFile)
	if err != nil {
		return nil, err
	}

	return output, nil
}

// runScript runs a script and captures its output
func (stp *ScriptToolProvider) runScript(ctx context.Context, command string, scriptFile string, inputFile string) (map[string]interface{}, error) {
	// Import the execution functions from block package
	cmd := fmt.Sprintf("%s %s < %s", command, scriptFile, inputFile)

	// For now, return a placeholder implementation
	// In a full implementation, this would properly execute the script and parse JSON output
	return map[string]interface{}{
		"message": "Script execution placeholder - implement proper script execution",
		"command": cmd,
	}, nil
}

// ValidateTool validates that a tool can be executed
func (stp *ScriptToolProvider) ValidateTool(toolDef *ToolDefinition) error {
	if toolDef.Type != ToolTypeScript {
		return fmt.Errorf("tool type must be script for script provider")
	}

	if toolDef.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	// Check if we have this tool
	stp.mu.RLock()
	defer stp.mu.RUnlock()

	if _, exists := stp.scripts[toolDef.Name]; !exists {
		return fmt.Errorf("script tool %s not found in provider", toolDef.Name)
	}

	return nil
}

// Close cleans up resources
func (stp *ScriptToolProvider) Close() error {
	stp.mu.Lock()
	defer stp.mu.Unlock()

	// Clean up cache directory if needed
	// For now, we'll keep the cache for performance

	log.Info().Str("provider", stp.name).Msg("Script tool provider closed")
	return nil
}

// RemoveScript removes a script tool from the provider
func (stp *ScriptToolProvider) RemoveScript(toolName string) error {
	stp.mu.Lock()
	defer stp.mu.Unlock()

	if _, exists := stp.scripts[toolName]; !exists {
		return fmt.Errorf("script tool %s not found", toolName)
	}

	delete(stp.scripts, toolName)
	log.Info().
		Str("provider", stp.name).
		Str("tool", toolName).
		Msg("Removed script tool")

	return nil
}

// ListScripts returns the names of all script tools
func (stp *ScriptToolProvider) ListScripts() []string {
	stp.mu.RLock()
	defer stp.mu.RUnlock()

	names := make([]string, 0, len(stp.scripts))
	for name := range stp.scripts {
		names = append(names, name)
	}
	return names
}

// encodeJSON encodes data to JSON bytes
func encodeJSON(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}
