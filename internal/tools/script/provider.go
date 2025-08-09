package script

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/lacquerai/lacquer/internal/block"
	"github.com/lacquerai/lacquer/internal/execcontext"
	"github.com/lacquerai/lacquer/internal/schema"
	"github.com/lacquerai/lacquer/internal/tools"
)

// ScriptTool represents a script-based tool
type ScriptTool struct {
	Name        string
	Description string
	ScriptPath  string
	Content     string
	Version     string
	Parameters  schema.JSON
}

// ScriptToolProvider implements the ToolProvider interface for script-based tools
type ScriptToolProvider struct {
	name         string
	tools        map[string]*ScriptTool
	bashExecutor *block.BashExecutor
	cacheDir     string
	mu           sync.RWMutex
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

	bashExecutor, err := block.NewBashExecutor(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create bash executor: %w", err)
	}

	return &ScriptToolProvider{
		name:         name,
		tools:        make(map[string]*ScriptTool),
		bashExecutor: bashExecutor,
		cacheDir:     cacheDir,
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

	scriptTool := &ScriptTool{
		Name:        tool.Name,
		Description: tool.Description,
		ScriptPath:  tool.Script,
		Content:     tool.Script,
		Parameters:  tool.Parameters,
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
	var output interface{}
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

func (stp *ScriptToolProvider) executeBashScript(execCtx *execcontext.ExecutionContext, scriptTool *ScriptTool, parameters json.RawMessage) (interface{}, error) {
	scriptContent := scriptTool.Content
	if scriptContent == "" && scriptTool.ScriptPath != "" {
		contentBytes, err := os.ReadFile(scriptTool.ScriptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read script file: %w", err)
		}
		scriptContent = string(contentBytes)
	}

	tempBlock := &block.Block{
		Name:    fmt.Sprintf("tool-%s", scriptTool.Name),
		Runtime: block.RuntimeBash,
		Script:  scriptContent,
		Inputs:  make(map[string]block.InputSchema),
		Outputs: make(map[string]block.OutputSchema),
	}

	return stp.bashExecutor.ExecuteRaw(execCtx, tempBlock, parameters)
}

// Close cleans up resources
func (stp *ScriptToolProvider) Close() error {
	// @TODO: Implement
	// close any running scripts
	// purge any cached scripts if exists
	return nil
}
