package runtime

import (
	"testing"
	"time"

	"github.com/lacquer/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
)

func TestGenerateRunID(t *testing.T) {
	id1 := generateRunID()
	id2 := generateRunID()

	// IDs should be unique
	assert.NotEqual(t, id1, id2)

	// IDs should have correct prefix
	assert.Contains(t, id1, "run_")
	assert.Contains(t, id2, "run_")

	// IDs should be reasonable length
	assert.True(t, len(id1) > 4)
	assert.True(t, len(id2) > 4)
}

func TestGetEnvironmentVars(t *testing.T) {
	env := getEnvironmentVars()

	// Should contain at least some environment variables
	assert.NotEmpty(t, env)

	// Should be able to access PATH (present on all systems)
	_, hasPath := env["PATH"]
	assert.True(t, hasPath)
}

func TestBuildMetadata(t *testing.T) {
	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name:        "test-workflow",
			Description: "A test workflow",
			Author:      "Alice",
			Version:     "1.0.0",
			Tags:        []string{"test", "example"},
		},
		SourceFile: "test.laq.yaml",
	}

	metadata := buildMetadata(workflow)

	assert.Equal(t, "test-workflow", metadata["name"])
	assert.Equal(t, "A test workflow", metadata["description"])
	assert.Equal(t, "Alice", metadata["author"])
	assert.Equal(t, "1.0.0", metadata["version"])
	assert.Equal(t, []string{"test", "example"}, metadata["tags"])
	assert.Equal(t, "1.0", metadata["workflow_version"])
	assert.Equal(t, "test.laq.yaml", metadata["source_file"])
}

func TestBuildMetadata_NilMetadata(t *testing.T) {
	workflow := &ast.Workflow{
		Version:    "1.0",
		Metadata:   nil,
		SourceFile: "test.laq.yaml",
	}

	metadata := buildMetadata(workflow)

	assert.Equal(t, "1.0", metadata["workflow_version"])
	assert.Equal(t, "test.laq.yaml", metadata["source_file"])
}

func TestSafeString(t *testing.T) {
	assert.Equal(t, "hello", SafeString("hello"))
	assert.Equal(t, "", SafeString(nil))
	assert.Equal(t, "", SafeString(42))
	assert.Equal(t, "", SafeString(true))
}

func TestSafeInt(t *testing.T) {
	assert.Equal(t, 42, SafeInt(42))
	assert.Equal(t, 42, SafeInt(int64(42)))
	assert.Equal(t, 42, SafeInt(42.7))
	assert.Equal(t, 42, SafeInt("42"))
	assert.Equal(t, 0, SafeInt("invalid"))
	assert.Equal(t, 0, SafeInt(nil))
	assert.Equal(t, 0, SafeInt(true))
}

func TestSafeBool(t *testing.T) {
	assert.Equal(t, true, SafeBool(true))
	assert.Equal(t, false, SafeBool(false))
	assert.Equal(t, true, SafeBool("true"))
	assert.Equal(t, true, SafeBool("1"))
	assert.Equal(t, true, SafeBool("yes"))
	assert.Equal(t, false, SafeBool("false"))
	assert.Equal(t, true, SafeBool(1))
	assert.Equal(t, false, SafeBool(0))
	assert.Equal(t, true, SafeBool(1.5))
	assert.Equal(t, false, SafeBool(0.0))
	assert.Equal(t, false, SafeBool(nil))
}

func TestMergeMap(t *testing.T) {
	dst := map[string]interface{}{
		"a": 1,
		"b": 2,
	}

	src := map[string]interface{}{
		"b": 3,
		"c": 4,
	}

	MergeMap(dst, src)

	assert.Equal(t, 1, dst["a"])
	assert.Equal(t, 3, dst["b"]) // Overwritten
	assert.Equal(t, 4, dst["c"])
}

func TestCopyMap(t *testing.T) {
	original := map[string]interface{}{
		"string": "value",
		"number": 42,
		"nested": map[string]interface{}{
			"inner": "value",
		},
		"array": []interface{}{1, 2, 3},
	}

	copied := CopyMap(original)

	// Should be equal but not same reference
	assert.Equal(t, original, copied)
	assert.NotSame(t, &original, &copied)

	// Nested maps should also be copied
	originalNested := original["nested"].(map[string]interface{})
	copiedNested := copied["nested"].(map[string]interface{})
	assert.NotSame(t, &originalNested, &copiedNested)

	originalArray := original["array"].([]interface{})
	copiedArray := copied["array"].([]interface{})
	assert.NotSame(t, &originalArray, &copiedArray)

	// Modifying copy shouldn't affect original
	copied["string"] = "modified"
	assert.Equal(t, "value", original["string"])
	assert.Equal(t, "modified", copied["string"])
}

func TestGetMapValue(t *testing.T) {
	m := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"value": "found",
			},
			"simple": "direct",
		},
		"direct": "value",
	}

	// Test direct access
	value, exists := GetMapValue(m, "direct")
	assert.True(t, exists)
	assert.Equal(t, "value", value)

	// Test nested access
	value, exists = GetMapValue(m, "level1.simple")
	assert.True(t, exists)
	assert.Equal(t, "direct", value)

	// Test deep nested access
	value, exists = GetMapValue(m, "level1.level2.value")
	assert.True(t, exists)
	assert.Equal(t, "found", value)

	// Test missing path
	_, exists = GetMapValue(m, "missing")
	assert.False(t, exists)

	// Test missing nested path
	_, exists = GetMapValue(m, "level1.missing")
	assert.False(t, exists)

	// Test empty path
	_, exists = GetMapValue(m, "")
	assert.False(t, exists)
}

func TestSetMapValue(t *testing.T) {
	m := make(map[string]interface{})

	// Test direct set
	SetMapValue(m, "direct", "value")
	assert.Equal(t, "value", m["direct"])

	// Test nested set (creating structure)
	SetMapValue(m, "level1.level2.value", "nested")
	value, exists := GetMapValue(m, "level1.level2.value")
	assert.True(t, exists)
	assert.Equal(t, "nested", value)

	// Test overwriting existing
	SetMapValue(m, "direct", "new_value")
	assert.Equal(t, "new_value", m["direct"])

	// Test empty path (should not panic)
	SetMapValue(m, "", "ignored")
}

func TestSplitPath(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, splitPath("a.b.c"))
	assert.Equal(t, []string{"single"}, splitPath("single"))
	assert.Equal(t, []string{"a", "b"}, splitPath("a.b"))
	assert.Nil(t, splitPath(""))
	assert.Equal(t, []string{"a"}, splitPath("a."))
	assert.Equal(t, []string{"b"}, splitPath(".b"))
}

func TestIsValidVariableName(t *testing.T) {
	// Valid names
	assert.True(t, IsValidVariableName("valid"))
	assert.True(t, IsValidVariableName("_underscore"))
	assert.True(t, IsValidVariableName("CamelCase"))
	assert.True(t, IsValidVariableName("snake_case"))
	assert.True(t, IsValidVariableName("with123numbers"))
	assert.True(t, IsValidVariableName("a"))
	assert.True(t, IsValidVariableName("_"))

	// Invalid names
	assert.False(t, IsValidVariableName(""))
	assert.False(t, IsValidVariableName("123invalid"))
	assert.False(t, IsValidVariableName("with-dash"))
	assert.False(t, IsValidVariableName("with space"))
	assert.False(t, IsValidVariableName("with.dot"))
	assert.False(t, IsValidVariableName("with@symbol"))
}

func TestFormatDuration(t *testing.T) {
	assert.Equal(t, "500ns", FormatDuration(500*time.Nanosecond))
	assert.Equal(t, "150ms", FormatDuration(150*time.Millisecond))
	assert.Equal(t, "1.5s", FormatDuration(1500*time.Millisecond))
	assert.Equal(t, "2m30s", FormatDuration(150*time.Second))
	assert.Equal(t, "1h0m0s", FormatDuration(time.Hour))
}
