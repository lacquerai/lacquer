package script

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/block"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/runtime"
	"github.com/lacquerai/lacquer/internal/schema"
	"github.com/lacquerai/lacquer/internal/tools"
)

type ScriptType string

const (
	ScriptTypeGo   ScriptType = "go"
	ScriptTypeBash ScriptType = "bash"
	ScriptTypeNode ScriptType = "node"
)

// ScriptTool represents a script-based tool
type ScriptTool struct {
	Name        string
	Description string
	ScriptPath  string
	Content     string
	ScriptType  ScriptType
	Version     string
	Parameters  schema.JSON
}

// ScriptToolProvider implements the ToolProvider interface for script-based tools
type ScriptToolProvider struct {
	name           string
	tools          map[string]*ScriptTool
	bashExecutor   *block.BashExecutor
	runtimeManager *runtime.Manager
	cacheDir       string
	mu             sync.RWMutex
}

// NewScriptToolProvider creates a new script tool provider
func NewScriptToolProvider(name string, cacheDir string, runtimeManager *runtime.Manager) (*ScriptToolProvider, error) {
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

	bashExecutor, err := block.NewBashExecutor(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create bash executor: %w", err)
	}

	return &ScriptToolProvider{
		name:           name,
		tools:          make(map[string]*ScriptTool),
		bashExecutor:   bashExecutor,
		runtimeManager: runtimeManager,
		cacheDir:       cacheDir,
	}, nil
}

func (stp *ScriptToolProvider) GetType() ast.ToolType {
	return ast.ToolTypeScript
}

// GetName returns the provider name
func (stp *ScriptToolProvider) GetName() string {
	return stp.name
}

// AddToolDefinition adds a tool to the provider
func (stp *ScriptToolProvider) AddToolDefinition(tool *ast.Tool) ([]tools.Tool, error) {
	stp.mu.Lock()
	defer stp.mu.Unlock()
	if _, exists := stp.tools[tool.Name]; exists {
		// tool already exists, skip
		return nil, fmt.Errorf("tool %s already exists", tool.Name)
	}

	var scriptType ScriptType
	switch strings.Split(tool.Runtime, "-")[0] {
	case "go":
		scriptType = ScriptTypeGo
	case "bash":
		scriptType = ScriptTypeBash
	case "node":
		scriptType = ScriptTypeNode
	default:
		scriptType = ScriptTypeGo
	}

	var scriptPath string
	var content string

	// Check if the script field contains a file path or inline content
	if strings.Contains(tool.Script, "\n") {
		content = tool.Script
	} else {
		scriptPath = tool.Script
		contentBytes, err := os.ReadFile(scriptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read script file: %w", err)
		}
		content = string(contentBytes)
	}

	// Convert the tool to a ScriptTool
	scriptTool := &ScriptTool{
		Name:        tool.Name,
		Description: tool.Description,
		ScriptPath:  tool.Script,
		Content:     string(content),
		ScriptType:  scriptType,
		Version:     tool.Version,
		Parameters:  tool.Parameters,
	}

	_, err := stp.runtimeManager.Get(context.Background(), string(scriptType), scriptTool.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime %s: %w", scriptTool.Version, err)
	}

	stp.tools[tool.Name] = scriptTool
	return []tools.Tool{
		{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		},
	}, nil
}

// ExecuteTool executes a script tool
func (stp *ScriptToolProvider) ExecuteTool(execCtx *execcontext.ExecutionContext, toolName string, parameters json.RawMessage) (*tools.Result, error) {
	stp.mu.RLock()
	defer stp.mu.RUnlock()

	scriptTool, exists := stp.tools[toolName]
	if !exists {
		return nil, fmt.Errorf("script tool %s not found", toolName)
	}

	startTime := time.Now()

	// Execute based on script type
	var output map[string]interface{}
	var err error

	output, err = stp.executeBashScript(execCtx, scriptTool, parameters)

	duration := time.Since(startTime)

	if err != nil {
		return &tools.Result{
			ToolName: toolName,
			Success:  false,
			Error:    err.Error(),
			Duration: duration,
		}, nil
	}

	return &tools.Result{
		ToolName: toolName,
		Success:  true,
		Output:   output,
		Duration: duration,
	}, nil
}

func (stp *ScriptToolProvider) executeBashScript(execCtx *execcontext.ExecutionContext, scriptTool *ScriptTool, parameters json.RawMessage) (map[string]interface{}, error) {
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
		Runtime: block.RuntimeBash,
		Script:  scriptContent,
		Inputs:  make(map[string]block.InputSchema),
		Outputs: make(map[string]block.OutputSchema),
	}

	// Execute using Go executor
	return stp.bashExecutor.ExecuteRaw(execCtx, tempBlock, parameters)
}

// executeScriptWithCommand executes a script using the specified command
func (stp *ScriptToolProvider) executeScriptWithCommand(ctx context.Context, command string, scriptContent string, parameters json.RawMessage, execCtx *tools.ExecutionContext) (map[string]interface{}, error) {
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
	inputData := map[string]interface{}{
		"inputs": parameters,
		"context": map[string]interface{}{
			"workflow_id": execCtx.WorkflowID,
			"step_id":     execCtx.StepID,
			"agent_id":    execCtx.AgentID,
			"workspace":   workspace,
		},
	}

	inputBytes, err := json.Marshal(inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to encode input data: %w", err)
	}

	// Execute script with timeout
	toolCtx := execCtx.Context
	if execCtx.Timeout > 0 {
		var cancel context.CancelFunc
		toolCtx, cancel = context.WithTimeout(execCtx.Context, execCtx.Timeout)
		defer cancel()
	}

	// Run the script
	output, err := stp.runScript(toolCtx, command, scriptFile, inputBytes)
	if err != nil {
		return nil, err
	}

	return output, nil
}

// runScript runs a script and captures its output
func (stp *ScriptToolProvider) runScript(ctx context.Context, command string, scriptFile string, inputBytes []byte) (map[string]interface{}, error) {
	cmd := exec.CommandContext(ctx, command, scriptFile)
	cmd.Env = []string{
		"LACQUER_INPUTS=" + string(inputBytes),
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmdOutput, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var output map[string]interface{}
	if err := json.Unmarshal(cmdOutput, &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal output: %w", err)
	}

	return output, nil
}

// Close cleans up resources
func (stp *ScriptToolProvider) Close() error {
	// TODO: Implement
	// close any running scripts
	// purge any cached scripts if exists
	return nil
}
