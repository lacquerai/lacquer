package ast

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestWorkflow_Basic(t *testing.T) {
	workflow := &Workflow{
		Version: "1.0",
		Metadata: &WorkflowMetadata{
			Name:        "test-workflow",
			Description: "A test workflow",
		},
		Agents: map[string]*Agent{
			"test_agent": {
				Model:       "gpt-4",
				Temperature: &[]float64{0.7}[0],
			},
		},
		Workflow: &WorkflowDef{
			Steps: []*Step{
				{
					ID:     "test_step",
					Agent:  "test_agent",
					Prompt: "Hello world",
				},
			},
		},
	}

	// Test basic functionality
	assert.Equal(t, "1.0", workflow.Version)
	assert.Equal(t, "test-workflow", workflow.Metadata.Name)

	agent, exists := workflow.GetAgent("test_agent")
	assert.True(t, exists)
	assert.Equal(t, "gpt-4", agent.Model)

	step, exists := workflow.GetStep("test_step")
	assert.True(t, exists)
	assert.Equal(t, "test_step", step.ID)
}

func TestStep_TypeDetection(t *testing.T) {
	testCases := []struct {
		name        string
		step        *Step
		stepType    string
		isAgent     bool
		isBlock     bool
		isAction    bool
		isScript    bool
		isContainer bool
	}{
		{
			name: "Agent step",
			step: &Step{
				Agent:  "test_agent",
				Prompt: "Hello",
			},
			stepType:    "agent",
			isAgent:     true,
			isBlock:     false,
			isAction:    false,
			isScript:    false,
			isContainer: false,
		},
		{
			name: "Block step",
			step: &Step{
				Uses: "lacquer/test@v1",
			},
			stepType:    "block",
			isAgent:     false,
			isBlock:     true,
			isAction:    false,
			isScript:    false,
			isContainer: false,
		},
		{
			name: "Action step",
			step: &Step{
				Action: "human_input",
			},
			stepType:    "action",
			isAgent:     false,
			isBlock:     false,
			isAction:    true,
			isScript:    false,
			isContainer: false,
		},
		{
			name: "Script step",
			step: &Step{
				Script: "./scripts/analyzer.go",
			},
			stepType:    "script",
			isAgent:     false,
			isBlock:     false,
			isAction:    false,
			isScript:    true,
			isContainer: false,
		},
		{
			name: "Container step",
			step: &Step{
				Container: "alpine:latest",
			},
			stepType:    "container",
			isAgent:     false,
			isBlock:     false,
			isAction:    false,
			isScript:    false,
			isContainer: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.stepType, tc.step.GetStepType())
			assert.Equal(t, tc.isAgent, tc.step.IsAgentStep())
			assert.Equal(t, tc.isBlock, tc.step.IsBlockStep())
			assert.Equal(t, tc.isAction, tc.step.IsActionStep())
			assert.Equal(t, tc.isScript, tc.step.IsScriptStep())
			assert.Equal(t, tc.isContainer, tc.step.IsContainerStep())
		})
	}
}

func TestDuration_Marshaling(t *testing.T) {
	testCases := []struct {
		name     string
		duration Duration
		yamlStr  string
		jsonStr  string
	}{
		{
			name:     "Seconds",
			duration: Duration{30 * time.Second},
			yamlStr:  "30s",
			jsonStr:  `"30s"`,
		},
		{
			name:     "Minutes",
			duration: Duration{5 * time.Minute},
			yamlStr:  "5m0s",
			jsonStr:  `"5m0s"`,
		},
		{
			name:     "Complex duration",
			duration: Duration{2*time.Hour + 30*time.Minute + 15*time.Second},
			yamlStr:  "2h30m15s",
			jsonStr:  `"2h30m15s"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test YAML marshaling
			yamlData, err := yaml.Marshal(tc.duration)
			require.NoError(t, err)
			assert.Equal(t, tc.yamlStr+"\n", string(yamlData))

			// Test YAML unmarshaling
			var yamlDuration Duration
			err = yaml.Unmarshal(yamlData, &yamlDuration)
			require.NoError(t, err)
			assert.Equal(t, tc.duration.Duration, yamlDuration.Duration)

			// Test JSON marshaling
			jsonData, err := json.Marshal(tc.duration)
			require.NoError(t, err)
			assert.Equal(t, tc.jsonStr, string(jsonData))

			// Test JSON unmarshaling
			var jsonDuration Duration
			err = json.Unmarshal(jsonData, &jsonDuration)
			require.NoError(t, err)
			assert.Equal(t, tc.duration.Duration, jsonDuration.Duration)
		})
	}
}

func TestDuration_InvalidFormat(t *testing.T) {
	testCases := []struct {
		name     string
		yamlData string
		jsonData string
	}{
		{
			name:     "invalid",
			yamlData: "invalid",
			jsonData: `"invalid"`,
		},
		{
			name:     "number without unit",
			yamlData: "30",
			jsonData: `"30"`,
		},
		{
			name:     "invalid unit",
			yamlData: "30x",
			jsonData: `"30x"`,
		},
		{
			name:     "empty string",
			yamlData: `""`,
			jsonData: `""`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var duration Duration

			// Test YAML unmarshaling
			err := yaml.Unmarshal([]byte(tc.yamlData), &duration)
			assert.Error(t, err)

			// Test JSON unmarshaling
			err = json.Unmarshal([]byte(tc.jsonData), &duration)
			assert.Error(t, err)
		})
	}
}

func TestInputParam_UnmarshalYAML(t *testing.T) {
	testCases := []struct {
		name     string
		yamlStr  string
		expected *InputParam
	}{
		{
			name:    "Shorthand string",
			yamlStr: "string",
			expected: &InputParam{
				Type:     "string",
				Required: true,
			},
		},
		{
			name:    "Shorthand integer",
			yamlStr: "integer",
			expected: &InputParam{
				Type:     "integer",
				Required: true,
			},
		},
		{
			name: "Full object",
			yamlStr: `
type: string
description: Test parameter
required: false
default: "test"
`,
			expected: &InputParam{
				Type:        "string",
				Description: "Test parameter",
				Required:    false,
				Default:     "test",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var param InputParam
			err := yaml.Unmarshal([]byte(tc.yamlStr), &param)
			require.NoError(t, err)

			assert.Equal(t, tc.expected.Type, param.Type)
			assert.Equal(t, tc.expected.Required, param.Required)
			assert.Equal(t, tc.expected.Description, param.Description)
			assert.Equal(t, tc.expected.Default, param.Default)
		})
	}
}

func TestTool_TypeDetection(t *testing.T) {
	testCases := []struct {
		name       string
		tool       *Tool
		toolType   string
		isOfficial bool
		isScript   bool
		isMCP      bool
	}{
		{
			name: "Official tool",
			tool: &Tool{
				Uses: "lacquer/web-search@v1",
			},
			toolType:   "official",
			isOfficial: true,
			isScript:   false,
			isMCP:      false,
		},
		{
			name: "Script tool",
			tool: &Tool{
				Script: "./tools/custom.py",
			},
			toolType:   "script",
			isOfficial: false,
			isScript:   true,
			isMCP:      false,
		},
		{
			name: "MCP tool",
			tool: &Tool{
				MCPServer: &MCPServerConfig{
					Type:    "local",
					Command: "enterprise-crm",
				},
			},
			toolType:   "mcp",
			isOfficial: false,
			isScript:   false,
			isMCP:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.toolType, tc.tool.GetToolType())
			assert.Equal(t, tc.isOfficial, tc.tool.IsOfficialTool())
			assert.Equal(t, tc.isScript, tc.tool.IsScript())
			assert.Equal(t, tc.isMCP, tc.tool.IsMCPTool())
		})
	}
}

func TestWorkflow_HelperMethods(t *testing.T) {
	workflow := &Workflow{
		Agents: map[string]*Agent{
			"agent1": {Model: "gpt-4"},
			"agent2": {Model: "claude-3-opus"},
		},
		Workflow: &WorkflowDef{
			Inputs: map[string]*InputParam{
				"param1": {Type: "string"},
				"param2": {Type: "integer"},
			},
			Steps: []*Step{
				{ID: "step1", Agent: "agent1", Prompt: "test1"},
				{ID: "step2", Agent: "agent2", Prompt: "test2"},
			},
		},
	}

	// Test agent retrieval
	agent, exists := workflow.GetAgent("agent1")
	assert.True(t, exists)
	assert.Equal(t, "gpt-4", agent.Model)

	_, exists = workflow.GetAgent("nonexistent")
	assert.False(t, exists)

	// Test step retrieval
	step, exists := workflow.GetStep("step1")
	assert.True(t, exists)
	assert.Equal(t, "step1", step.ID)

	_, exists = workflow.GetStep("nonexistent")
	assert.False(t, exists)

	// Test input parameter retrieval
	param, exists := workflow.GetInputParam("param1")
	assert.True(t, exists)
	assert.Equal(t, "string", param.Type)

	// Test list methods
	agentNames := workflow.ListAgents()
	assert.Contains(t, agentNames, "agent1")
	assert.Contains(t, agentNames, "agent2")

	stepIDs := workflow.ListStepIDs()
	assert.Equal(t, []string{"step1", "step2"}, stepIDs)
}

func TestStep_YAMLUnmarshalingWithNewFields(t *testing.T) {
	testCases := []struct {
		name     string
		yamlStr  string
		expected *Step
	}{
		{
			name: "Script step from file",
			yamlStr: `
id: analyze
run: ./scripts/analyzer.go
with:
  text: "some text"
  mode: "analyze"
`,
			expected: &Step{
				ID:  "analyze",
				Run: "./scripts/analyzer.go",
				With: map[string]interface{}{
					"text": "some text",
					"mode": "analyze",
				},
			},
		},
		{
			name: "Script step inline",
			yamlStr: `
id: process
run: |
  package main
  import "fmt"
  func main() {
    fmt.Println("Hello")
  }
with:
  input: "data"
`,
			expected: &Step{
				ID: "process",
				Run: `package main
import "fmt"
func main() {
  fmt.Println("Hello")
}
`,
				With: map[string]interface{}{
					"input": "data",
				},
			},
		},
		{
			name: "Container with local Dockerfile",
			yamlStr: `
id: build_and_run
container: ./docker/analyzer
with:
  text: "analyze this"
`,
			expected: &Step{
				ID:        "build_and_run",
				Container: "./docker/analyzer",
				With: map[string]interface{}{
					"text": "analyze this",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var step Step
			err := yaml.Unmarshal([]byte(tc.yamlStr), &step)
			require.NoError(t, err)

			assert.Equal(t, tc.expected.ID, step.ID)
			assert.Equal(t, tc.expected.Run, step.Run)
			assert.Equal(t, tc.expected.Container, step.Container)
			assert.Equal(t, tc.expected.With, step.With)
		})
	}
}
