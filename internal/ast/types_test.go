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

func TestWorkflow_Validate(t *testing.T) {
	testCases := []struct {
		name        string
		workflow    *Workflow
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid workflow",
			workflow: &Workflow{
				Version: "1.0",
				Workflow: &WorkflowDef{
					Steps: []*Step{
						{
							ID:     "test",
							Agent:  "agent",
							Prompt: "test",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Invalid version",
			workflow: &Workflow{
				Version: "2.0",
			},
			expectError: true,
			errorMsg:    "unsupported version",
		},
		{
			name: "Missing workflow",
			workflow: &Workflow{
				Version: "1.0",
			},
			expectError: true,
			errorMsg:    "workflow definition is required",
		},
		{
			name: "Empty steps",
			workflow: &Workflow{
				Version: "1.0",
				Workflow: &WorkflowDef{
					Steps: []*Step{},
				},
			},
			expectError: true,
			errorMsg:    "at least one step",
		},
		{
			name: "Duplicate step IDs",
			workflow: &Workflow{
				Version: "1.0",
				Workflow: &WorkflowDef{
					Steps: []*Step{
						{ID: "test", Agent: "agent", Prompt: "test"},
						{ID: "test", Agent: "agent", Prompt: "test"},
					},
				},
			},
			expectError: true,
			errorMsg:    "duplicate step ID",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.workflow.Validate()

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStep_Validation(t *testing.T) {
	testCases := []struct {
		name        string
		step        *Step
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid agent step",
			step: &Step{
				ID:     "test",
				Agent:  "agent",
				Prompt: "Hello",
			},
			expectError: false,
		},
		{
			name: "Valid block step",
			step: &Step{
				ID:   "test",
				Uses: "lacquer/web-search@v1",
			},
			expectError: false,
		},
		{
			name: "Valid action step",
			step: &Step{
				ID:     "test",
				Action: "human_input",
			},
			expectError: false,
		},
		{
			name: "Missing ID",
			step: &Step{
				Agent:  "agent",
				Prompt: "Hello",
			},
			expectError: true,
			errorMsg:    "step ID is required",
		},
		{
			name: "No execution method",
			step: &Step{
				ID: "test",
			},
			expectError: true,
			errorMsg:    "must specify either",
		},
		{
			name: "Multiple execution methods",
			step: &Step{
				ID:     "test",
				Agent:  "agent",
				Prompt: "Hello",
				Uses:   "lacquer/test@v1",
			},
			expectError: true,
			errorMsg:    "cannot specify multiple",
		},
		{
			name: "Invalid action",
			step: &Step{
				ID:     "test",
				Action: "invalid_action",
			},
			expectError: true,
			errorMsg:    "invalid action",
		},
		{
			name: "Update state without updates",
			step: &Step{
				ID:     "test",
				Action: "update_state",
			},
			expectError: true,
			errorMsg:    "requires updates field",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.step.Validate()

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStep_TypeDetection(t *testing.T) {
	testCases := []struct {
		name     string
		step     *Step
		stepType string
		isAgent  bool
		isBlock  bool
		isAction bool
	}{
		{
			name: "Agent step",
			step: &Step{
				Agent:  "test_agent",
				Prompt: "Hello",
			},
			stepType: "agent",
			isAgent:  true,
			isBlock:  false,
			isAction: false,
		},
		{
			name: "Block step",
			step: &Step{
				Uses: "lacquer/test@v1",
			},
			stepType: "block",
			isAgent:  false,
			isBlock:  true,
			isAction: false,
		},
		{
			name: "Action step",
			step: &Step{
				Action: "human_input",
			},
			stepType: "action",
			isAgent:  false,
			isBlock:  false,
			isAction: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.stepType, tc.step.GetStepType())
			assert.Equal(t, tc.isAgent, tc.step.IsAgentStep())
			assert.Equal(t, tc.isBlock, tc.step.IsBlockStep())
			assert.Equal(t, tc.isAction, tc.step.IsActionStep())
		})
	}
}

func TestAgent_Validation(t *testing.T) {
	testCases := []struct {
		name        string
		agent       *Agent
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid custom agent",
			agent: &Agent{
				Model: "gpt-4",
			},
			expectError: false,
		},
		{
			name: "Valid pre-built agent",
			agent: &Agent{
				Uses: "lacquer/researcher@v1",
			},
			expectError: false,
		},
		{
			name: "No model or uses",
			agent: &Agent{
				Temperature: &[]float64{0.7}[0],
			},
			expectError: true,
			errorMsg:    "must specify either model or uses",
		},
		{
			name: "Both model and uses",
			agent: &Agent{
				Model: "gpt-4",
				Uses:  "lacquer/researcher@v1",
			},
			expectError: true,
			errorMsg:    "cannot specify both",
		},
		{
			name: "Invalid model",
			agent: &Agent{
				Model: "invalid-model",
			},
			expectError: true,
			errorMsg:    "unsupported model",
		},
		{
			name: "Invalid temperature",
			agent: &Agent{
				Model:       "gpt-4",
				Temperature: &[]float64{3.0}[0],
			},
			expectError: true,
			errorMsg:    "temperature must be between",
		},
		{
			name: "Invalid top_p",
			agent: &Agent{
				Model: "gpt-4",
				TopP:  &[]float64{1.5}[0],
			},
			expectError: true,
			errorMsg:    "top_p must be between",
		},
		{
			name: "Invalid max_tokens",
			agent: &Agent{
				Model:     "gpt-4",
				MaxTokens: &[]int{0}[0],
			},
			expectError: true,
			errorMsg:    "max_tokens must be positive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.agent.Validate()

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
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
				MCPServer: "enterprise-crm",
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
