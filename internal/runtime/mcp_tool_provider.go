package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// MCPToolProvider implements the ToolProvider interface for MCP servers
type MCPToolProvider struct {
	name      string
	client    *MCPClient
	tools     map[string]*ToolDefinition
	mu        sync.RWMutex
	connected bool
}

// NewMCPToolProvider creates a new MCP tool provider
func NewMCPToolProvider(name string, config *MCPConfig) (*MCPToolProvider, error) {
	if name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	client, err := NewMCPClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	return &MCPToolProvider{
		name:   name,
		client: client,
		tools:  make(map[string]*ToolDefinition),
	}, nil
}

// GetName returns the provider name
func (mtp *MCPToolProvider) GetName() string {
	return mtp.name
}

// GetType returns the tool type this provider handles
func (mtp *MCPToolProvider) GetType() ToolType {
	return ToolTypeMCP
}

// DiscoverTools discovers available tools from the MCP server
func (mtp *MCPToolProvider) DiscoverTools(ctx context.Context) ([]ToolDefinition, error) {
	mtp.mu.Lock()
	defer mtp.mu.Unlock()

	// Connect to the MCP server if not already connected
	if !mtp.connected {
		if err := mtp.client.Connect(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
		}
		mtp.connected = true
	}

	// List tools from the MCP server
	mcpTools, err := mtp.client.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from MCP server: %w", err)
	}

	var tools []ToolDefinition
	mtp.tools = make(map[string]*ToolDefinition) // Reset tools cache

	for _, mcpTool := range mcpTools {
		toolDef, err := mtp.convertMCPToolToDefinition(mcpTool)
		if err != nil {
			log.Warn().
				Err(err).
				Str("tool_name", mcpTool.Name).
				Msg("Failed to convert MCP tool to definition")
			continue
		}

		tools = append(tools, *toolDef)
		mtp.tools[toolDef.Name] = toolDef
	}

	log.Info().
		Str("provider", mtp.name).
		Int("tool_count", len(tools)).
		Msg("Discovered tools from MCP server")

	return tools, nil
}

// convertMCPToolToDefinition converts an MCP tool to a ToolDefinition
func (mtp *MCPToolProvider) convertMCPToolToDefinition(mcpTool MCPTool) (*ToolDefinition, error) {
	// Extract parameters from the MCP tool schema
	parameters, err := mtp.parseToolParameters(mcpTool.Schema)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tool parameters: %w", err)
	}

	toolDef := &ToolDefinition{
		Name:        mcpTool.Name,
		Description: mcpTool.Description,
		Type:        ToolTypeMCP,
		Parameters:  parameters,
		Source:      mtp.client.serverURL,
		ProviderID:  mtp.name,
		Config: map[string]interface{}{
			"mcp_tool_name": mcpTool.Name,
			"schema":        mcpTool.Schema,
		},
	}

	return toolDef, nil
}

// parseToolParameters extracts tool parameters from MCP schema
func (mtp *MCPToolProvider) parseToolParameters(schema map[string]interface{}) ([]ToolParameter, error) {
	var parameters []ToolParameter

	// Handle JSON Schema format
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		// If no properties, return empty parameters
		return parameters, nil
	}

	// Get required fields
	requiredFields := make(map[string]bool)
	if required, ok := schema["required"].([]interface{}); ok {
		for _, field := range required {
			if fieldName, ok := field.(string); ok {
				requiredFields[fieldName] = true
			}
		}
	}

	// Parse each property
	for propName, propSchema := range properties {
		param, err := mtp.parseToolParameter(propName, propSchema, requiredFields[propName])
		if err != nil {
			log.Warn().
				Err(err).
				Str("property", propName).
				Msg("Failed to parse tool parameter")
			continue
		}
		parameters = append(parameters, *param)
	}

	return parameters, nil
}

// parseToolParameter parses a single tool parameter from schema
func (mtp *MCPToolProvider) parseToolParameter(name string, schema interface{}, required bool) (*ToolParameter, error) {
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid parameter schema for %s", name)
	}

	param := &ToolParameter{
		Name:     name,
		Required: required,
	}

	// Extract type
	if typeVal, ok := schemaMap["type"].(string); ok {
		param.Type = typeVal
	} else {
		param.Type = "string" // Default type
	}

	// Extract description
	if desc, ok := schemaMap["description"].(string); ok {
		param.Description = desc
	}

	// Extract default value
	if defaultVal, ok := schemaMap["default"]; ok {
		param.Default = defaultVal
	}

	// Extract enum values
	if enumVal, ok := schemaMap["enum"].([]interface{}); ok {
		var enumStrings []string
		for _, val := range enumVal {
			if strVal, ok := val.(string); ok {
				enumStrings = append(enumStrings, strVal)
			}
		}
		param.Enum = enumStrings
	}

	// Extract properties for object types
	if properties, ok := schemaMap["properties"].(map[string]interface{}); ok {
		param.Properties = properties
	}

	return param, nil
}

// ExecuteTool executes a tool via the MCP server
func (mtp *MCPToolProvider) ExecuteTool(ctx context.Context, toolName string, parameters map[string]interface{}, execCtx *ToolExecutionContext) (*ToolResult, error) {
	mtp.mu.RLock()
	defer mtp.mu.RUnlock()

	if !mtp.connected {
		return nil, fmt.Errorf("MCP provider not connected")
	}

	// Get tool definition for validation
	toolDef, exists := mtp.tools[toolName]
	if !exists {
		return nil, fmt.Errorf("tool %s not found in MCP provider", toolName)
	}

	// Validate parameters
	if err := mtp.validateToolParameters(toolDef, parameters); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	startTime := time.Now()

	// Create context with timeout
	toolCtx := execCtx.Context
	if execCtx.Timeout > 0 {
		var cancel context.CancelFunc
		toolCtx, cancel = context.WithTimeout(execCtx.Context, execCtx.Timeout)
		defer cancel()
	}

	// Call the tool via MCP client
	result, err := mtp.client.CallTool(toolCtx, toolName, parameters)
	duration := time.Since(startTime)

	if err != nil {
		return &ToolResult{
			ToolName: toolName,
			Success:  false,
			Error:    err.Error(),
			Duration: duration,
			Metadata: map[string]interface{}{
				"provider":    mtp.name,
				"provider_id": mtp.name,
				"tool_type":   string(ToolTypeMCP),
			},
		}, nil
	}

	// Handle MCP tool result error
	if result.Error != "" {
		return &ToolResult{
			ToolName: toolName,
			Success:  false,
			Error:    result.Error,
			Duration: duration,
			Metadata: map[string]interface{}{
				"provider":    mtp.name,
				"provider_id": mtp.name,
				"tool_type":   string(ToolTypeMCP),
			},
		}, nil
	}

	// Convert output to map[string]interface{}
	output := make(map[string]interface{})
	if result.Output != nil {
		if outputMap, ok := result.Output.(map[string]interface{}); ok {
			output = outputMap
		} else {
			// If output is not a map, put it under a generic key
			output["result"] = result.Output
		}
	}

	// Merge metadata if available
	metadata := map[string]interface{}{
		"provider":    mtp.name,
		"provider_id": mtp.name,
		"tool_type":   string(ToolTypeMCP),
	}
	for key, value := range result.Metadata {
		metadata[key] = value
	}

	return &ToolResult{
		ToolName: toolName,
		Success:  true,
		Output:   output,
		Duration: duration,
		Metadata: metadata,
	}, nil
}

// validateToolParameters validates tool parameters against the tool definition
func (mtp *MCPToolProvider) validateToolParameters(toolDef *ToolDefinition, parameters map[string]interface{}) error {
	// Check required parameters
	for _, param := range toolDef.Parameters {
		if param.Required {
			if _, exists := parameters[param.Name]; !exists {
				return fmt.Errorf("required parameter %s is missing", param.Name)
			}
		}
	}

	// Validate parameter types (basic validation)
	for paramName, paramValue := range parameters {
		// Find parameter definition
		var paramDef *ToolParameter
		for _, p := range toolDef.Parameters {
			if p.Name == paramName {
				paramDef = &p
				break
			}
		}

		if paramDef == nil {
			// Unknown parameter - log warning but don't fail
			log.Warn().
				Str("tool", toolDef.Name).
				Str("parameter", paramName).
				Msg("Unknown parameter provided to tool")
			continue
		}

		// Basic type validation
		if err := mtp.validateParameterType(paramName, paramValue, paramDef.Type); err != nil {
			return err
		}

		// Validate enum values
		if len(paramDef.Enum) > 0 {
			if err := mtp.validateParameterEnum(paramName, paramValue, paramDef.Enum); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateParameterType validates parameter type
func (mtp *MCPToolProvider) validateParameterType(paramName string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter %s must be a string", paramName)
		}
	case "number", "integer":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// Valid numeric type
		default:
			return fmt.Errorf("parameter %s must be a number", paramName)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("parameter %s must be a boolean", paramName)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("parameter %s must be an array", paramName)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("parameter %s must be an object", paramName)
		}
	}
	return nil
}

// validateParameterEnum validates parameter against enum values
func (mtp *MCPToolProvider) validateParameterEnum(paramName string, value interface{}, enumValues []string) error {
	strValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("parameter %s must be a string for enum validation", paramName)
	}

	for _, enumValue := range enumValues {
		if strValue == enumValue {
			return nil
		}
	}

	return fmt.Errorf("parameter %s value '%s' is not one of allowed values: %s", 
		paramName, strValue, strings.Join(enumValues, ", "))
}

// ValidateTool validates that a tool can be executed
func (mtp *MCPToolProvider) ValidateTool(toolDef *ToolDefinition) error {
	if toolDef.Type != ToolTypeMCP {
		return fmt.Errorf("tool type must be MCP for MCP provider")
	}

	if toolDef.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	// Validate that we have this tool
	mtp.mu.RLock()
	defer mtp.mu.RUnlock()

	if _, exists := mtp.tools[toolDef.Name]; !exists {
		return fmt.Errorf("tool %s not found in MCP provider", toolDef.Name)
	}

	return nil
}

// Close cleans up resources and disconnects from the MCP server
func (mtp *MCPToolProvider) Close() error {
	mtp.mu.Lock()
	defer mtp.mu.Unlock()

	if mtp.connected {
		if err := mtp.client.Disconnect(); err != nil {
			log.Warn().Err(err).Str("provider", mtp.name).Msg("Error disconnecting from MCP server")
		}
		mtp.connected = false
	}

	return nil
}

// HealthCheck performs a health check on the MCP server
func (mtp *MCPToolProvider) HealthCheck(ctx context.Context) error {
	mtp.mu.RLock()
	defer mtp.mu.RUnlock()

	if !mtp.connected {
		return fmt.Errorf("MCP provider not connected")
	}

	return mtp.client.HealthCheck(ctx)
}

// IsConnected returns whether the provider is connected to the MCP server
func (mtp *MCPToolProvider) IsConnected() bool {
	mtp.mu.RLock()
	defer mtp.mu.RUnlock()
	return mtp.connected
}

// GetServerURL returns the MCP server URL
func (mtp *MCPToolProvider) GetServerURL() string {
	return mtp.client.serverURL
}