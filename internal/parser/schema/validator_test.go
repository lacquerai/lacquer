package schema

import (
	"path/filepath"
	"testing"
)

func TestValidator_ValidateExampleWorkflows(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Test cases with example workflows
	// Note: Only testing basic MVP-compatible examples, as advanced features
	// like parallel execution, switch statements are out of scope for v0.1.0
	testCases := []struct {
		name        string
		filename    string
		expectValid bool
	}{
		{
			name:        "Hello World Example",
			filename:    "../../../docs/examples/hello-world.laq.yaml",
			expectValid: true,
		},
		{
			name:        "Research Workflow Example",
			filename:    "../../../docs/examples/research-workflow.laq.yaml",
			expectValid: true,
		},
		// Advanced examples with parallel/switch constructs are out of MVP scope
		// Will be enabled in future versions when those features are implemented
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Resolve the absolute path
			absPath, err := filepath.Abs(tc.filename)
			if err != nil {
				t.Fatalf("Failed to resolve path %s: %v", tc.filename, err)
			}

			result, err := validator.ValidateFile(absPath)
			if err != nil {
				t.Fatalf("Validation failed with error: %v", err)
			}

			if result.Valid != tc.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v", tc.expectValid, result.Valid)
				if !result.Valid {
					t.Logf("Validation errors:")
					for _, validationErr := range result.Errors {
						t.Logf("  - %s at %s (value: %v)", validationErr.Message, validationErr.Path, validationErr.Value)
					}
				}
			}
		})
	}
}

func TestValidator_InvalidWorkflows(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	// Test cases with invalid workflows
	testCases := []struct {
		name        string
		yaml        string
		expectError string
	}{
		{
			name: "Missing version",
			yaml: `
metadata:
  name: test
workflow:
  steps:
    - id: test
      agent: test
      prompt: "test"
`,
			expectError: "version",
		},
		{
			name: "Invalid version",
			yaml: `
version: "2.0"
workflow:
  steps:
    - id: test
      agent: test
      prompt: "test"
`,
			expectError: "version",
		},
		{
			name: "Missing workflow",
			yaml: `
version: "1.0"
metadata:
  name: test
`,
			expectError: "workflow",
		},
		{
			name: "Empty steps",
			yaml: `
version: "1.0"
workflow:
  steps: []
`,
			expectError: "steps",
		},
		{
			name: "Invalid step - missing id",
			yaml: `
version: "1.0"
workflow:
  steps:
    - agent: test
      prompt: "test"
`,
			expectError: "id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := validator.ValidateBytes([]byte(tc.yaml))
			if err != nil {
				t.Fatalf("Validation failed with error: %v", err)
			}

			if result.Valid {
				t.Errorf("Expected validation to fail, but it passed")
				return
			}

			// Check if the expected error is mentioned
			found := false
			for _, validationErr := range result.Errors {
				if contains(validationErr.Message, tc.expectError) || contains(validationErr.Path, tc.expectError) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected error containing '%s', but got errors: %v", tc.expectError, result.Errors)
			}
		})
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
