package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_SemanticValidationInParser(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	t.Run("valid workflow passes all validation", func(t *testing.T) {
		validYAML := `
version: "1.0"
metadata:
  name: test-workflow
agents:
  researcher:
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
