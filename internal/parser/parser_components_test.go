package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParserComponents_SchemaValidation tests the JSON schema validation component
func TestParserComponents_SchemaValidation(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	testCases := []struct {
		name        string
		yaml        string
		expectValid bool
		description string
	}{
		{
			name: "Valid minimal workflow",
			yaml: `
version: "1.0"
agents:
  test_agent:
    model: "gpt-4"
workflow:
  steps:
    - id: test
      agent: test_agent
      prompt: "Hello"
`,
			expectValid: true,
			description: "Minimal valid workflow should pass schema validation",
		},
		{
			name: "Missing version field",
			yaml: `
agents:
  test_agent:
    model: "gpt-4"
workflow:
  steps:
    - id: test
      agent: test_agent
      prompt: "Hello"
`,
			expectValid: false,
			description: "Missing version should fail schema validation",
		},
		{
			name: "Invalid version value",
			yaml: `
version: "2.0"
agents:
  test_agent:
    model: "gpt-4"
workflow:
  steps:
    - id: test
      agent: test_agent
      prompt: "Hello"
`,
			expectValid: false,
			description: "Invalid version should fail schema validation",
		},
		{
			name: "Empty steps array",
			yaml: `
version: "1.0"
workflow:
  steps: []
`,
			expectValid: false,
			description: "Empty steps should fail schema validation",
		},
		{
			name: "Invalid agent configuration",
			yaml: `
version: "1.0"
agents:
  bad_agent:
    temperature: 5.0  # Too high
    max_tokens: -100  # Negative
workflow:
  steps:
    - id: test
      agent: bad_agent
      prompt: "Hello"
`,
			expectValid: false,
			description: "Invalid agent parameters should fail schema validation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(tc.yaml))
			
			if tc.expectValid {
				assert.NoError(t, err, "Expected valid YAML to pass: %s", tc.description)
			} else {
				assert.Error(t, err, "Expected invalid YAML to fail: %s", tc.description)
			}
		})
	}
}

// TestParserComponents_ASTValidation tests the AST validation component
func TestParserComponents_ASTValidation(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	testCases := []struct {
		name        string
		yaml        string
		expectValid bool
		description string
	}{
		{
			name: "Valid agent references",
			yaml: `
version: "1.0"
agents:
  helper:
    model: "gpt-4"
  analyst:
    model: "claude-3-sonnet"
workflow:
  steps:
    - id: step1
      agent: helper
      prompt: "Help me"
    - id: step2
      agent: analyst
      prompt: "Analyze this"
`,
			expectValid: true,
			description: "Valid agent references should pass AST validation",
		},
		{
			name: "Duplicate step IDs",
			yaml: `
version: "1.0"
agents:
  test_agent:
    model: "gpt-4"
workflow:
  steps:
    - id: duplicate
      agent: test_agent
      prompt: "First"
    - id: duplicate
      agent: test_agent
      prompt: "Second"
`,
			expectValid: false,
			description: "Duplicate step IDs should fail AST validation",
		},
		{
			name: "Invalid step structure",
			yaml: `
version: "1.0"
agents:
  test_agent:
    model: "gpt-4"
workflow:
  steps:
    - id: missing_execution
      # Missing agent, uses, or action
`,
			expectValid: false,
			description: "Steps without execution method should fail AST validation",
		},
		{
			name: "Valid block references",
			yaml: `
version: "1.0"
workflow:
  steps:
    - id: lacquer_block
      uses: "lacquer/http-request@v1"
      with:
        url: "https://example.com"
    - id: github_block
      uses: "github.com/owner/repo@v1.0"
      with:
        config: "test"
    - id: local_block
      uses: "./custom/block"
      with:
        data: "test"
`,
			expectValid: true,
			description: "Valid block references should pass AST validation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(tc.yaml))
			
			if tc.expectValid {
				assert.NoError(t, err, "Expected valid YAML to pass: %s", tc.description)
			} else {
				assert.Error(t, err, "Expected invalid YAML to fail: %s", tc.description)
			}
		})
	}
}

// TestParserComponents_SemanticValidation tests the semantic validation component
func TestParserComponents_SemanticValidation(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	testCases := []struct {
		name        string
		yaml        string
		expectValid bool
		description string
	}{
		{
			name: "Step with variable reference",
			yaml: `
version: "1.0"
agents:
  test_agent:
    model: "gpt-4"
workflow:
  steps:
    - id: step1
      agent: test_agent
      prompt: "First step"
    - id: step2
      agent: test_agent
      prompt: "Second step references: {{ steps.step1.output }}"
`,
			expectValid: true, // Variable syntax is parsed but not fully validated yet
			description: "Steps with variable references currently parse successfully",
		},
		{
			name: "Undefined agent reference",
			yaml: `
version: "1.0"
agents:
  existing_agent:
    model: "gpt-4"
workflow:
  steps:
    - id: step1
      agent: nonexistent_agent
      prompt: "This should fail"
`,
			expectValid: false,
			description: "Undefined agent references should fail semantic validation",
		},
		{
			name: "Circular dependency",
			yaml: `
version: "1.0"
agents:
  test_agent:
    model: "gpt-4"
workflow:
  steps:
    - id: step1
      agent: test_agent
      prompt: "Depends on step2: {{ steps.step2.output }}"
    - id: step2
      agent: test_agent
      prompt: "Depends on step1: {{ steps.step1.output }}"
`,
			expectValid: false,
			description: "Circular dependencies should fail semantic validation",
		},
		{
			name: "Forward reference",
			yaml: `
version: "1.0"
agents:
  test_agent:
    model: "gpt-4"
workflow:
  steps:
    - id: step1
      agent: test_agent
      prompt: "References future step: {{ steps.step2.output }}"
    - id: step2
      agent: test_agent
      prompt: "Later step"
`,
			expectValid: false,
			description: "Forward references should fail semantic validation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(tc.yaml))
			
			if tc.expectValid {
				assert.NoError(t, err, "Expected valid YAML to pass: %s", tc.description)
			} else {
				assert.Error(t, err, "Expected invalid YAML to fail: %s", tc.description)
			}
		})
	}
}

// TestParserComponents_ErrorReporting tests the enhanced error reporting system
func TestParserComponents_ErrorReporting(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	testCases := []struct {
		name             string
		yaml             string
		expectedInError  []string
		description      string
	}{
		{
			name: "Enhanced error with context",
			yaml: `
version: "1.0"
workflow:
  steps:
    - id: test
      agent: missing_agent
      prompt: "Hello"
`,
			expectedInError:  []string{"undefined agent", "missing_agent"},
			description:      "Should provide enhanced error with context for undefined agent",
		},
		{
			name: "Position information",
			yaml: `
version: "1.0"
agents:
  test_agent:
    model: "gpt-4"
workflow:
  steps:
    - id: test
      prompt: "Missing agent field"
`,
			expectedInError:  []string{"agent", "prompt"},
			description:      "Should provide position information for validation errors",
		},
		{
			name: "Multiple errors",
			yaml: `
version: "1.0"
workflow:
  steps:
    - id: step1
      agent: missing1
      prompt: "First error"
    - id: step2
      agent: missing2
      prompt: "Second error"
`,
			expectedInError:  []string{"missing1", "missing2", "undefined agent"},
			description:      "Should report multiple errors with context",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(tc.yaml))
			require.Error(t, err, "Expected parsing to fail: %s", tc.description)
			
			errStr := err.Error()
			for _, expected := range tc.expectedInError {
				assert.Contains(t, errStr, expected, 
					"Expected error to contain '%s' for %s", expected, tc.description)
			}
			
			// Check if it's an enhanced error
			if enhancedErr, ok := err.(*MultiErrorEnhanced); ok {
				issues := enhancedErr.GetAllIssues()
				assert.NotEmpty(t, issues, "Enhanced error should have issues")
				
				// Check that issues have position information
				for _, issue := range issues {
					assert.Greater(t, issue.Position.Line, 0, "Issue should have line number")
					assert.Greater(t, issue.Position.Column, 0, "Issue should have column number")
				}
			}
		})
	}
}

// TestParserComponents_Performance tests parser performance with various inputs
func TestParserComponents_Performance(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	// Test parsing speed with different file sizes
	testCases := []struct {
		name     string
		yaml     string
		maxMs    int64
	}{
		{
			name: "Small workflow",
			yaml: `
version: "1.0"
agents:
  agent:
    model: "gpt-4"
workflow:
  steps:
    - id: step
      agent: agent
      prompt: "test"
`,
			maxMs: 10,
		},
		{
			name: "Medium workflow with multiple agents",
			yaml: generateMediumWorkflow(),
			maxMs: 50,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Warm up
			_, _ = parser.ParseBytes([]byte(tc.yaml))
			
			// Measure performance
			start := timeNow()
			_, parseErr := parser.ParseBytes([]byte(tc.yaml))
			duration := timeSince(start)
			
			// Allow errors for invalid syntax, but measure time anyway
			_ = parseErr // Ignore parse errors for performance testing
			durationMs := duration.Nanoseconds() / 1000000
			assert.LessOrEqual(t, durationMs, tc.maxMs,
				"Parsing should complete within %dms, took %dms", tc.maxMs, durationMs)
		})
	}
}

// Helper function to generate a medium-sized workflow for performance testing
func generateMediumWorkflow() string {
	return `
version: "1.0"
metadata:
  name: "medium-workflow"
agents:
  gpt4:
    model: "gpt-4"
    temperature: 0.7
  claude:
    model: "claude-3-sonnet"
  gemini:
    model: "gemini-pro"
workflow:
  inputs:
    topic:
      type: "string"
  state:
    counter: 0
  steps:
    - id: step1
      agent: gpt4
      prompt: "Step 1"
    - id: step2
      agent: claude
      prompt: "Step 2"
    - id: step3
      agent: gemini
      prompt: "Step 3"
    - id: step4
      uses: "lacquer/processor@v1"
      with:
        data: "test"
    - id: step5
      agent: gpt4
      prompt: "Step 5"
    - id: step6
      action: "update_state"
      updates:
        counter: 1
    - id: step7
      agent: claude
      prompt: "Step 7"
    - id: step8
      agent: gemini
      prompt: "Step 8"
  outputs:
    result: "completed"
`
}