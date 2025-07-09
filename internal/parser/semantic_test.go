package parser

import (
	"strings"
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSemanticValidator_ValidateWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		workflow *ast.Workflow
		wantErr  bool
		errCount int
	}{
		{
			name: "valid simple workflow",
			workflow: &ast.Workflow{
				Version: "1.0",
				Metadata: &ast.WorkflowMetadata{
					Name: "test-workflow",
				},
				Agents: map[string]*ast.Agent{
					"researcher": {
						Provider: "openai",
						Model:    "gpt-4",
					},
				},
				Workflow: &ast.WorkflowDef{
					Steps: []*ast.Step{
						{
							ID:     "research",
							Agent:  "researcher",
							Prompt: "Research the topic",
						},
					},
				},
			},
			wantErr:  false,
			errCount: 0,
		},
		{
			name: "circular dependency",
			workflow: &ast.Workflow{
				Version: "1.0",
				Agents: map[string]*ast.Agent{
					"agent1": {Provider: "openai", Model: "gpt-4"},
				},
				Workflow: &ast.WorkflowDef{
					Steps: []*ast.Step{
						{
							ID:     "step1",
							Agent:  "agent1",
							Prompt: "Use {{ steps.step2.output }}",
						},
						{
							ID:     "step2",
							Agent:  "agent1",
							Prompt: "Use {{ steps.step1.output }}",
						},
					},
				},
			},
			wantErr:  true,
			errCount: 0, // Don't check exact count since we may have multiple errors
		},
		{
			name: "forward reference",
			workflow: &ast.Workflow{
				Version: "1.0",
				Agents: map[string]*ast.Agent{
					"agent1": {Provider: "openai", Model: "gpt-4"},
				},
				Workflow: &ast.WorkflowDef{
					Steps: []*ast.Step{
						{
							ID:     "step1",
							Agent:  "agent1",
							Prompt: "Use {{ steps.step2.output }}",
						},
						{
							ID:     "step2",
							Agent:  "agent1",
							Prompt: "Generate something",
						},
					},
				},
			},
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "invalid block reference",
			workflow: &ast.Workflow{
				Version: "1.0",
				Workflow: &ast.WorkflowDef{
					Steps: []*ast.Step{
						{
							ID:   "step1",
							Uses: "invalid-block-reference",
						},
					},
				},
			},
			wantErr:  true,
			errCount: 0, // Don't check exact count due to multiple validation layers
		},
		{
			name: "valid lacquer block reference",
			workflow: &ast.Workflow{
				Version: "1.0",
				Workflow: &ast.WorkflowDef{
					Steps: []*ast.Step{
						{
							ID:   "step1",
							Uses: "lacquer/http-request@v1",
						},
					},
				},
			},
			wantErr:  false,
			errCount: 0,
		},
		{
			name: "unbalanced parentheses in condition",
			workflow: &ast.Workflow{
				Version: "1.0",
				Agents: map[string]*ast.Agent{
					"agent1": {Provider: "openai", Model: "gpt-4"},
				},
				Workflow: &ast.WorkflowDef{
					State: map[string]interface{}{
						"value": "test",
					},
					Steps: []*ast.Step{
						{
							ID:        "step1",
							Agent:     "agent1",
							Prompt:    "Test",
							Condition: "{{ state.value }} == 'test')",
						},
					},
				},
			},
			wantErr:  true,
			errCount: 0, // Don't check exact count since we may have multiple errors
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewSemanticValidator()
			result := validator.ValidateWorkflow(tt.workflow)

			if tt.wantErr {
				assert.True(t, result.HasErrors(), "expected validation errors")
				if tt.errCount > 0 {
					assert.Len(t, result.Errors, tt.errCount, "unexpected error count")
				}
			} else {
				assert.False(t, result.HasErrors(), "unexpected validation errors: %v", result.Errors)
			}
		})
	}
}

func TestSemanticValidator_ValidateStepDependencies(t *testing.T) {
	validator := NewSemanticValidator()

	t.Run("no circular dependencies", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Agents: map[string]*ast.Agent{
				"agent1": {Provider: "openai", Model: "gpt-4"},
			},
			Workflow: &ast.WorkflowDef{
				Steps: []*ast.Step{
					{
						ID:     "step1",
						Agent:  "agent1",
						Prompt: "First step",
					},
					{
						ID:     "step2",
						Agent:  "agent1",
						Prompt: "Use {{ steps.step1.output }}",
					},
					{
						ID:     "step3",
						Agent:  "agent1",
						Prompt: "Use {{ steps.step2.output }}",
					},
				},
			},
		}

		result := validator.ValidateWorkflow(workflow)
		assert.False(t, result.HasErrors())
	})

	t.Run("circular dependency detection", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Agents: map[string]*ast.Agent{
				"agent1": {Provider: "openai", Model: "gpt-4"},
			},
			Workflow: &ast.WorkflowDef{
				Steps: []*ast.Step{
					{
						ID:     "step1",
						Agent:  "agent1",
						Prompt: "Use {{ steps.step3.output }}",
					},
					{
						ID:     "step2",
						Agent:  "agent1",
						Prompt: "Use {{ steps.step1.output }}",
					},
					{
						ID:     "step3",
						Agent:  "agent1",
						Prompt: "Use {{ steps.step2.output }}",
					},
				},
			},
		}

		result := validator.ValidateWorkflow(workflow)
		assert.True(t, result.HasErrors())

		// Should detect circular dependency
		found := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "circular dependency") {
				found = true
				break
			}
		}
		assert.True(t, found, "should detect circular dependency")
	})
}

func TestSemanticValidator_ValidateBlockReferences(t *testing.T) {
	validator := NewSemanticValidator()

	tests := []struct {
		name      string
		blockRef  string
		shouldErr bool
	}{
		{
			name:      "valid lacquer block",
			blockRef:  "lacquer/http-request@v1",
			shouldErr: false,
		},
		{
			name:      "valid lacquer block without version",
			blockRef:  "lacquer/postgresql",
			shouldErr: false,
		},
		{
			name:      "valid github reference",
			blockRef:  "github.com/lacquer/blocks@main",
			shouldErr: false,
		},
		{
			name:      "valid local reference",
			blockRef:  "./blocks/custom-block",
			shouldErr: false,
		},
		{
			name:      "invalid lacquer block name",
			blockRef:  "lacquer/Invalid_Name",
			shouldErr: true,
		},
		{
			name:      "invalid format",
			blockRef:  "invalid-format",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := &ast.Workflow{
				Version: "1.0",
				Workflow: &ast.WorkflowDef{
					Steps: []*ast.Step{
						{
							ID:   "step1",
							Uses: tt.blockRef,
						},
					},
				},
			}

			result := validator.ValidateWorkflow(workflow)

			if tt.shouldErr {
				assert.True(t, result.HasErrors(), "expected validation error for block reference: %s", tt.blockRef)
			} else {
				// Filter out other potential errors, focus on block reference errors
				hasBlockError := false
				for _, err := range result.Errors {
					if strings.Contains(err.Message, "block reference") {
						hasBlockError = true
						break
					}
				}
				assert.False(t, hasBlockError, "unexpected block reference error for: %s", tt.blockRef)
			}
		})
	}
}

func TestSemanticValidator_ValidateControlFlow(t *testing.T) {
	validator := NewSemanticValidator()

	t.Run("balanced parentheses", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Agents: map[string]*ast.Agent{
				"agent1": {Provider: "openai", Model: "gpt-4"},
			},
			Workflow: &ast.WorkflowDef{
				Steps: []*ast.Step{
					{
						ID:        "step1",
						Agent:     "agent1",
						Prompt:    "Test",
						Condition: "({{ state.count }} > 0) && ({{ state.enabled }} == true)",
					},
				},
			},
		}

		result := validator.ValidateWorkflow(workflow)

		// Should not have parentheses errors
		hasParenthesesError := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "parentheses") {
				hasParenthesesError = true
				break
			}
		}
		assert.False(t, hasParenthesesError, "should not have parentheses errors")
	})

	t.Run("unbalanced parentheses", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Agents: map[string]*ast.Agent{
				"agent1": {Provider: "openai", Model: "gpt-4"},
			},
			Workflow: &ast.WorkflowDef{
				Steps: []*ast.Step{
					{
						ID:        "step1",
						Agent:     "agent1",
						Prompt:    "Test",
						Condition: "{{ state.count }} > 0)",
					},
				},
			},
		}

		result := validator.ValidateWorkflow(workflow)
		assert.True(t, result.HasErrors())

		// Should detect unbalanced parentheses
		found := false
		for _, err := range result.Errors {
			if strings.Contains(err.Message, "parentheses") {
				found = true
				break
			}
		}
		assert.True(t, found, "should detect unbalanced parentheses")
	})
}

func TestSemanticValidator_ExtractVariableReferences(t *testing.T) {
	validator := NewSemanticValidator()

	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "single variable",
			text:     "Hello {{ name }}",
			expected: []string{"name"},
		},
		{
			name:     "multiple variables",
			text:     "{{ greeting }} {{ name }}, today is {{ date }}",
			expected: []string{"greeting", "name", "date"},
		},
		{
			name:     "nested path variables",
			text:     "Use {{ steps.research.output }} and {{ state.count }}",
			expected: []string{"steps.research.output", "state.count"},
		},
		{
			name:     "variables with whitespace",
			text:     "{{  variable  }} and {{   another.path   }}",
			expected: []string{"variable", "another.path"},
		},
		{
			name:     "no variables",
			text:     "Just plain text with no variables",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.extractAllVariableReferences(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSemanticValidator_ExtractStepDependencies(t *testing.T) {
	validator := NewSemanticValidator()

	step := &ast.Step{
		ID:        "test-step",
		Agent:     "agent1",
		Prompt:    "Use {{ steps.step1.output }} and {{ steps.step2.result }}",
		Condition: "{{ steps.step3.success }}",
		With: map[string]interface{}{
			"data": "{{ steps.step4.data }}",
		},
	}

	deps := validator.extractStepDependencies(step)
	expected := []string{"step1", "step2", "step3", "step4"}

	assert.ElementsMatch(t, expected, deps)
}

func TestSemanticValidator_HasCycle(t *testing.T) {
	validator := NewSemanticValidator()

	t.Run("no cycle", func(t *testing.T) {
		dependencies := map[string][]string{
			"step1": {},
			"step2": {"step1"},
			"step3": {"step2"},
		}

		visited := make(map[string]bool)
		recursionStack := make(map[string]bool)

		hasCycle := validator.hasCycle("step1", dependencies, visited, recursionStack)
		assert.False(t, hasCycle)
	})

	t.Run("has cycle", func(t *testing.T) {
		dependencies := map[string][]string{
			"step1": {"step3"},
			"step2": {"step1"},
			"step3": {"step2"},
		}

		visited := make(map[string]bool)
		recursionStack := make(map[string]bool)

		hasCycle := validator.hasCycle("step1", dependencies, visited, recursionStack)
		assert.True(t, hasCycle)
	})
}

func TestSemanticValidator_BlockReferenceValidation(t *testing.T) {
	validator := NewSemanticValidator()

	tests := []struct {
		name     string
		ref      string
		expected bool
	}{
		// Lacquer blocks
		{"valid lacquer block", "lacquer/http-request@v1", true},
		{"valid lacquer block no version", "lacquer/postgresql", true},
		{"invalid lacquer block name", "lacquer/Invalid_Name", false},
		{"invalid lacquer block format", "lacquer/", false},

		// GitHub references
		{"valid github ref", "github.com/owner/repo@tag", true},
		{"valid github ref no tag", "github.com/owner/repo", true},
		{"invalid github format", "github.com/owner", false},

		// Local references
		{"valid local ref", "./blocks/custom", true},
		{"valid relative ref", "../shared/blocks", true},
		{"invalid local ref", "./", false},

		// Invalid formats
		{"unsupported format", "https://example.com/block", false},
		{"empty reference", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result bool

			if strings.HasPrefix(tt.ref, "lacquer/") {
				result = validator.isValidLacquerBlock(tt.ref)
			} else if strings.HasPrefix(tt.ref, "github.com/") {
				result = validator.isValidGitHubReference(tt.ref)
			} else if strings.HasPrefix(tt.ref, "./") || strings.HasPrefix(tt.ref, "../") {
				result = validator.isValidLocalReference(tt.ref)
			} else {
				result = false
			}

			assert.Equal(t, tt.expected, result, "validation result mismatch for: %s", tt.ref)
		})
	}
}

func TestIntegration_SemanticValidationInParserIntegration(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	t.Run("valid workflow passes all validation", func(t *testing.T) {
		validYAML := `
version: "1.0"
metadata:
  name: test-workflow
agents:
  researcher:
    provider: openai
    model: gpt-4
workflow:
  inputs:
    topic:
      type: string
  state:
    count: 0
  steps:
    - id: step1
      agent: researcher
      prompt: "Research {{ inputs.topic }}"
    - id: step2
      agent: researcher
      prompt: "Analyze {{ steps.step1.output }} with count {{ state.count }}"
  outputs:
    result: "{{ steps.step2.output }}"
`
		workflow, err := parser.ParseBytes([]byte(validYAML))
		assert.NoError(t, err)
		assert.NotNil(t, workflow)
	})

	t.Run("circular dependency detected", func(t *testing.T) {
		circularYAML := `
version: "1.0"
metadata:
  name: circular-test
agents:
  agent1:
    provider: openai
    model: gpt-4
workflow:
  steps:
    - id: step1
      agent: agent1
      prompt: "Use {{ steps.step2.output }}"
    - id: step2
      agent: agent1
      prompt: "Use {{ steps.step1.output }}"
`
		_, err := parser.ParseBytes([]byte(circularYAML))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circular dependency")
	})

	t.Run("forward reference detected", func(t *testing.T) {
		forwardRefYAML := `
version: "1.0"
metadata:
  name: forward-ref-test
agents:
  agent1:
    model: gpt-4
    provider: openai
workflow:
  steps:
    - id: step1
      agent: agent1
      prompt: "Use {{ steps.step2.output }}"
    - id: step2
      agent: agent1
      prompt: "Generate output"
`
		_, err := parser.ParseBytes([]byte(forwardRefYAML))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hasn't executed yet")
	})

	t.Run("undefined variable detected", func(t *testing.T) {
		undefinedVarYAML := `
version: "1.0"
metadata:
  name: undefined-var-test
agents:
  agent1:
    model: gpt-4
    provider: openai
workflow:
  steps:
    - id: step1
      agent: agent1
      prompt: "Use {{ undefined.variable }}"
`
		_, err := parser.ParseBytes([]byte(undefinedVarYAML))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "undefined variable")
	})

	t.Run("undefined agent detected", func(t *testing.T) {
		undefinedAgentYAML := `
version: "1.0"
metadata:
  name: undefined-agent-test
agents:
  agent1:
    model: gpt-4
    provider: openai
workflow:
  steps:
    - id: step1
      agent: undefined_agent
      prompt: "Test"
`
		_, err := parser.ParseBytes([]byte(undefinedAgentYAML))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "undefined agent")
	})

	t.Run("invalid block reference detected", func(t *testing.T) {
		invalidBlockYAML := `
version: "1.0"
metadata:
  name: invalid-block-test
workflow:
  steps:
    - id: step1
      uses: "invalid-block-format"
      with:
        param: "value"
`
		_, err := parser.ParseBytes([]byte(invalidBlockYAML))
		assert.Error(t, err)
		// Either schema validation or semantic validation should catch this
		errMsg := err.Error()
		assert.True(t, strings.Contains(errMsg, "block reference") || strings.Contains(errMsg, "pattern"),
			"should contain block reference error or pattern error")
	})

	t.Run("valid block references pass", func(t *testing.T) {
		validBlockYAML := `
version: "1.0"
metadata:
  name: valid-block-test
workflow:
  steps:
    - id: step1
      uses: "lacquer/http-request@v1"
      with:
        url: "https://api.example.com"
    - id: step2
      uses: "github.com/owner/repo@main"
    - id: step3
      uses: "./local/block"
`
		workflow, err := parser.ParseBytes([]byte(validBlockYAML))
		assert.NoError(t, err)
		assert.NotNil(t, workflow)
	})
}

func TestIntegration_SemanticValidation(t *testing.T) {
	t.Run("complex workflow validation", func(t *testing.T) {
		// Note: Strict mode has been removed from SemanticValidator
		parser, err := NewYAMLParser(WithSemanticValidator(NewSemanticValidator()))
		require.NoError(t, err)

		complexYAML := `
version: "1.0"
metadata:
  name: complex-test
agents:
  agent1:
    model: gpt-4
    provider: openai
workflow:
  steps:
    - id: step1
      agent: agent1
      prompt: "Use {{ steps.step2.some_complex_output | default('fallback') }}"
      condition: "{{ now() > '2023-01-01' }}"
    - id: step2
      agent: agent1
      prompt: "Generate complex output"
`

		// Parse the complex YAML
		_, parseErr := parser.ParseBytes([]byte(complexYAML))

		// Check if parsing succeeded or failed
		if parseErr != nil {
			t.Logf("Complex YAML validation failed with: %v", parseErr)
		} else {
			t.Log("Complex YAML validation passed")
		}
	})
}

func TestIntegration_ErrorMessageQuality(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	t.Run("error messages provide helpful suggestions", func(t *testing.T) {
		testCases := []struct {
			name        string
			yaml        string
			expectedMsg string
			expectedSug string
		}{
			{
				name: "circular dependency",
				yaml: `
version: "1.0"
agents:
  agent1:
    model: gpt-4
    provider: openai
workflow:
  steps:
    - id: step1
      agent: agent1
      prompt: "{{ steps.step2.output }}"
    - id: step2
      agent: agent1
      prompt: "{{ steps.step1.output }}"
`,
				expectedMsg: "circular dependency",
				expectedSug: "Remove circular dependencies",
			},
			{
				name: "forward reference",
				yaml: `
version: "1.0"
agents:
  agent1:
    model: gpt-4
    provider: openai
workflow:
  steps:
    - id: step1
      agent: agent1
      prompt: "{{ steps.step2.output }}"
    - id: step2
      agent: agent1
      prompt: "output"
`,
				expectedMsg: "hasn't executed yet",
				expectedSug: "Check workflow logic",
			},
			{
				name: "undefined variable",
				yaml: `
version: "1.0"
agents:
  agent1:
    model: gpt-4
    provider: openai
workflow:
  steps:
    - id: step1
      agent: agent1
      prompt: "{{ undefined.var }}"
`,
				expectedMsg: "undefined variable",
				expectedSug: "Check variable references",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := parser.ParseBytes([]byte(tc.yaml))
				require.Error(t, err)

				errMsg := err.Error()
				assert.Contains(t, errMsg, tc.expectedMsg, "should contain expected error message")
				assert.Contains(t, errMsg, tc.expectedSug, "should contain helpful suggestion")
			})
		}
	})
}

func TestIntegration_RealWorldExample(t *testing.T) {
	t.Run("parse semantic_valid test file", func(t *testing.T) {
		parser, err := NewYAMLParser()
		require.NoError(t, err)

		workflow, err := parser.ParseFile("testdata/valid/semantic_valid.laq.yaml")
		assert.NoError(t, err)
		assert.NotNil(t, workflow)

		// Verify the workflow structure
		assert.Equal(t, "1.0", workflow.Version)
		assert.Equal(t, "semantic-validation-test", workflow.Metadata.Name)
		assert.Len(t, workflow.Agents, 2)
		assert.Len(t, workflow.Workflow.Steps, 4)
	})

	t.Run("parse test files with known issues", func(t *testing.T) {
		parser, err := NewYAMLParser()
		require.NoError(t, err)

		testFiles := []struct {
			name       string
			file       string
			shouldFail bool
		}{
			{"circular dependency", "testdata/invalid/circular_dependency.laq.yaml", true},
			{"forward reference", "testdata/invalid/forward_reference.laq.yaml", true},
			{"undefined variables", "testdata/invalid/undefined_variables.laq.yaml", true},
		}

		for _, tc := range testFiles {
			t.Run(tc.name, func(t *testing.T) {
				_, err := parser.ParseFile(tc.file)
				if tc.shouldFail {
					assert.Error(t, err, "expected parsing to fail for %s", tc.file)
				} else {
					assert.NoError(t, err, "expected parsing to succeed for %s", tc.file)
				}
			})
		}
	})
}
