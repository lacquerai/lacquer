package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewYAMLParser(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.validator)
	assert.True(t, parser.strict)
}

func TestNewYAMLParser_WithOptions(t *testing.T) {
	parser, err := NewYAMLParser(WithStrict(false))
	require.NoError(t, err)
	assert.False(t, parser.strict)
}

func TestYAMLParser_ParseFile_ValidFiles(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	testCases := []struct {
		name     string
		filename string
	}{
		{
			name:     "Minimal workflow",
			filename: "testdata/valid/minimal.laq.yaml",
		},
		{
			name:     "Hello world example",
			filename: "../../docs/examples/hello-world.laq.yaml",
		},
		{
			name:     "Research workflow example",
			filename: "testdata/valid/semantic_valid.laq.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get absolute path
			absPath, err := filepath.Abs(tc.filename)
			require.NoError(t, err)

			workflow, err := parser.ParseFile(absPath)
			require.NoError(t, err)
			
			assert.NotNil(t, workflow)
			assert.Equal(t, "1.0", workflow.Version)
			assert.Equal(t, absPath, workflow.SourceFile)
			assert.NotEmpty(t, workflow.Workflow)
		})
	}
}

func TestYAMLParser_ParseFile_InvalidExtension(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	_, err = parser.ParseFile("test.yaml") // Should be .laq.yaml
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid file extension")
}

func TestYAMLParser_ParseFile_FileNotFound(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	_, err = parser.ParseFile("nonexistent.laq.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot read file")
}

func TestYAMLParser_ParseBytes_Valid(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	validYAML := `
version: "1.0"
metadata:
  name: test-workflow
agents:
  test_agent:
    model: gpt-4
workflow:
  steps:
    - id: test_step
      agent: test_agent
      prompt: "Hello world"
`

	workflow, err := parser.ParseBytes([]byte(validYAML))
	require.NoError(t, err)
	
	assert.NotNil(t, workflow)
	assert.Equal(t, "1.0", workflow.Version)
	assert.NotNil(t, workflow.Metadata)
	assert.NotNil(t, workflow.Agents)
	assert.NotNil(t, workflow.Workflow)
}

func TestYAMLParser_ParseBytes_Empty(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	_, err = parser.ParseBytes([]byte{})
	assert.Error(t, err)
	
	// Check that it's an enhanced error
	_, ok := err.(*MultiErrorEnhanced)
	require.True(t, ok)
	assert.Contains(t, err.Error(), "Empty workflow file")
}

func TestYAMLParser_ParseBytes_SyntaxError(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	invalidYAML := `
version: "1.0"
workflow:
  steps:
    - id: test
      prompt: "Unclosed quote
`

	_, err = parser.ParseBytes([]byte(invalidYAML))
	assert.Error(t, err)
	
	// Should wrap the YAML error with additional context
	assert.Contains(t, err.Error(), "YAML parsing error")
}

func TestYAMLParser_ParseBytes_ValidationError(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	invalidYAML := `
version: "1.0"
workflow:
  steps: []  # Empty steps should fail validation
`

	_, err = parser.ParseBytes([]byte(invalidYAML))
	assert.Error(t, err)
	
	// Should be a validation error about minimum items
	assert.Contains(t, err.Error(), "minimum")
}

func TestYAMLParser_ValidateOnly(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	validYAML := `
version: "1.0"
workflow:
  steps:
    - id: test
      agent: test_agent
      prompt: "Hello"
`

	err = parser.ValidateOnly([]byte(validYAML))
	assert.NoError(t, err)
}

func TestYAMLParser_ValidateOnly_Invalid(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	invalidYAML := `
version: "2.0"  # Invalid version
workflow:
  steps:
    - id: test
      agent: test_agent
      prompt: "Hello"
`

	err = parser.ValidateOnly([]byte(invalidYAML))
	assert.Error(t, err)
}

func TestYAMLParser_ParseReader(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	validYAML := `
version: "1.0"
agents:
  test_agent:
    model: "gpt-4"
workflow:
  steps:
    - id: test
      agent: test_agent
      prompt: "Hello"
`

	reader := strings.NewReader(validYAML)
	workflow, err := parser.ParseReader(reader)
	require.NoError(t, err)
	
	assert.NotNil(t, workflow)
	assert.Equal(t, "1.0", workflow.Version)
}

func TestIsValidWorkflowFile(t *testing.T) {
	testCases := []struct {
		filename string
		expected bool
	}{
		{"workflow.laq.yaml", true},
		{"workflow.laq.yml", true},
		{"test.laq.yaml", true},
		{"workflow.yaml", false},
		{"workflow.yml", false},
		{"workflow.laq.txt", false},
		{"workflow.txt", false},
		{".laq.yaml", true},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			result := isValidWorkflowFile(tc.filename)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetSupportedExtensions(t *testing.T) {
	extensions := GetSupportedExtensions()
	assert.Contains(t, extensions, ".laq.yaml")
	assert.Contains(t, extensions, ".laq.yml")
}

func TestYAMLParser_LargeFile(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	// Create a file that's too large (over 10MB)
	largeData := make([]byte, 11*1024*1024)
	for i := range largeData {
		largeData[i] = 'a'
	}

	// Write to a temporary file
	tmpFile := filepath.Join(t.TempDir(), "large.laq.yaml")
	err = os.WriteFile(tmpFile, largeData, 0644)
	require.NoError(t, err)

	_, err = parser.ParseFile(tmpFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "File too large")
}

func TestYAMLParser_SetStrict(t *testing.T) {
	parser, err := NewYAMLParser()
	require.NoError(t, err)

	assert.True(t, parser.strict)
	
	parser.SetStrict(false)
	assert.False(t, parser.strict)
	
	parser.SetStrict(true)
	assert.True(t, parser.strict)
}

// Benchmark tests
func BenchmarkYAMLParser_ParseBytes(b *testing.B) {
	parser, err := NewYAMLParser()
	require.NoError(b, err)

	validYAML := `
version: "1.0"
metadata:
  name: benchmark-test
agents:
  test_agent:
    model: gpt-4
    temperature: 0.7
workflow:
  inputs:
    topic:
      type: string
  steps:
    - id: step1
      agent: test_agent
      prompt: "Process {{ inputs.topic }}"
    - id: step2
      agent: test_agent
      prompt: "Continue with {{ steps.step1.output }}"
  outputs:
    result: "{{ steps.step2.output }}"
`

	data := []byte(validYAML)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.ParseBytes(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkYAMLParser_ValidateOnly(b *testing.B) {
	parser, err := NewYAMLParser()
	require.NoError(b, err)

	validYAML := `
version: "1.0"
workflow:
  steps:
    - id: test
      agent: test_agent
      prompt: "Hello"
`

	data := []byte(validYAML)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := parser.ValidateOnly(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}