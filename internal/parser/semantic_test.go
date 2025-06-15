package parser

import (
	"strings"
	"testing"

	"github.com/lacquer/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
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
			name: "undefined variable reference",
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
							Prompt: "Use {{ undefined.variable }}",
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

func TestSemanticValidator_ValidateVariableReferences(t *testing.T) {
	validator := NewSemanticValidator()

	t.Run("valid variable references", func(t *testing.T) {
		workflow := &ast.Workflow{
			Version: "1.0",
			Metadata: &ast.WorkflowMetadata{
				Name: "test-workflow",
			},
			Agents: map[string]*ast.Agent{
				"agent1": {Provider: "openai", Model: "gpt-4"},
			},
			Workflow: &ast.WorkflowDef{
				Inputs: map[string]*ast.InputParam{
					"topic": {Type: "string"},
				},
				State: map[string]interface{}{
					"count": 0,
				},
				Steps: []*ast.Step{
					{
						ID:     "step1",
						Agent:  "agent1",
						Prompt: "Research {{ inputs.topic }} with {{ metadata.name }} and {{ state.count }}",
					},
					{
						ID:     "step2",
						Agent:  "agent1",
						Prompt: "Use {{ steps.step1.output }} and {{ env.API_KEY }}",
					},
				},
			},
		}

		result := validator.ValidateWorkflow(workflow)
		assert.False(t, result.HasErrors(), "should not have validation errors for valid variables")
	})

	t.Run("undefined variable references", func(t *testing.T) {
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
						Prompt: "Use {{ undefined.variable }} and {{ inputs.missing }}",
					},
				},
			},
		}

		result := validator.ValidateWorkflow(workflow)
		assert.True(t, result.HasErrors())

		// Should find undefined variable references
		errorMessages := make([]string, len(result.Errors))
		for i, err := range result.Errors {
			errorMessages[i] = err.Message
		}

		assert.Contains(t, strings.Join(errorMessages, " "), "undefined.variable")
		assert.Contains(t, strings.Join(errorMessages, " "), "inputs.missing")
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
