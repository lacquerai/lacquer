package expression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionRegistry_StringFunctions(t *testing.T) {
	fr := NewFunctionRegistry()
	execCtx := createTestExecutionContext()

	t.Run("contains function", func(t *testing.T) {
		// Test contains true
		result, err := fr.Call("contains", []interface{}{"hello world", "world"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, true, result)

		// Test contains false
		result, err = fr.Call("contains", []interface{}{"hello world", "xyz"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, false, result)

		// Test wrong number of arguments
		_, err = fr.Call("contains", []interface{}{"hello"}, execCtx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires exactly 2 arguments")
	})

	t.Run("startsWith function", func(t *testing.T) {
		// Test startsWith true
		result, err := fr.Call("startsWith", []interface{}{"hello world", "hello"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, true, result)

		// Test startsWith false
		result, err = fr.Call("startsWith", []interface{}{"hello world", "world"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, false, result)
	})

	t.Run("endsWith function", func(t *testing.T) {
		// Test endsWith true
		result, err := fr.Call("endsWith", []interface{}{"hello world", "world"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, true, result)

		// Test endsWith false
		result, err = fr.Call("endsWith", []interface{}{"hello world", "hello"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, false, result)
	})

	t.Run("format function", func(t *testing.T) {
		// Test basic formatting
		result, err := fr.Call("format", []interface{}{"Hello {0}!", "world"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "Hello world!", result)

		// Test multiple placeholders
		result, err = fr.Call("format", []interface{}{"Hello {0}, you are {1}!", "Alice", "awesome"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "Hello Alice, you are awesome!", result)

		// Test no placeholders
		result, err = fr.Call("format", []interface{}{"Hello world!"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "Hello world!", result)

		// Test no arguments
		_, err = fr.Call("format", []interface{}{}, execCtx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least 1 argument")
	})

	t.Run("join function", func(t *testing.T) {
		// Test join with custom separator
		result, err := fr.Call("join", []interface{}{"a,b,c", "|"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "a|b|c", result)

		// Test join with default separator
		result, err = fr.Call("join", []interface{}{"a,b,c"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "a,b,c", result)

		// Test join with array
		result, err = fr.Call("join", []interface{}{[]interface{}{"a", "b", "c"}, "-"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, "a-b-c", result)
	})

	t.Run("toJSON function", func(t *testing.T) {
		// Test simple object
		obj := map[string]interface{}{
			"name": "test",
			"age":  30,
		}
		result, err := fr.Call("toJSON", []interface{}{obj}, execCtx)
		require.NoError(t, err)
		assert.Contains(t, result.(string), "test")
		assert.Contains(t, result.(string), "30")

		// Test array
		arr := []interface{}{"a", "b", "c"}
		result, err = fr.Call("toJSON", []interface{}{arr}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, `["a","b","c"]`, result)

		// Test wrong number of arguments
		_, err = fr.Call("toJSON", []interface{}{}, execCtx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires exactly 1 argument")
	})

	t.Run("fromJSON function", func(t *testing.T) {
		// Test parse object
		jsonStr := `{"name":"test","age":30}`
		result, err := fr.Call("fromJSON", []interface{}{jsonStr}, execCtx)
		require.NoError(t, err)

		resultMap := result.(map[string]interface{})
		assert.Equal(t, "test", resultMap["name"])
		assert.Equal(t, float64(30), resultMap["age"]) // JSON numbers are float64

		// Test parse array
		jsonStr = `["a","b","c"]`
		result, err = fr.Call("fromJSON", []interface{}{jsonStr}, execCtx)
		require.NoError(t, err)

		resultArray := result.([]interface{})
		assert.Equal(t, 3, len(resultArray))
		assert.Equal(t, "a", resultArray[0])

		// Test invalid JSON
		_, err = fr.Call("fromJSON", []interface{}{"invalid json"}, execCtx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JSON")
	})
}

func TestFunctionRegistry_UtilityFunctions(t *testing.T) {
	fr := NewFunctionRegistry()

	t.Run("success function", func(t *testing.T) {
		// Test with no failed steps
		execCtx := createTestExecutionContext()
		result, err := fr.Call("success", []interface{}{}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, true, result)

		// Test with failed step
		execCtx.SetStepResult("step1", &StepResult{
			StepID: "step1",
			Status: StepStatusFailed,
		})
		result, err = fr.Call("success", []interface{}{}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, false, result)

		// Test wrong number of arguments
		_, err = fr.Call("success", []interface{}{"arg"}, execCtx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "takes no arguments")
	})

	t.Run("always function", func(t *testing.T) {
		execCtx := createTestExecutionContext()
		result, err := fr.Call("always", []interface{}{}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, true, result)

		// Test wrong number of arguments
		_, err = fr.Call("always", []interface{}{"arg"}, execCtx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "takes no arguments")
	})

	t.Run("cancelled function", func(t *testing.T) {
		execCtx := createTestExecutionContext()
		result, err := fr.Call("cancelled", []interface{}{}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, false, result)

		// Cancel the context
		execCtx.Cancel()
		result, err = fr.Call("cancelled", []interface{}{}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, true, result)
	})

	t.Run("failure function", func(t *testing.T) {
		// Test with no failed steps
		execCtx := createTestExecutionContext()
		result, err := fr.Call("failure", []interface{}{}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, false, result)

		// Test with failed step
		execCtx.SetStepResult("step1", &StepResult{
			StepID: "step1",
			Status: StepStatusFailed,
		})
		result, err = fr.Call("failure", []interface{}{}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, true, result)
	})
}

func TestFunctionRegistry_ContextFunctions(t *testing.T) {
	fr := NewFunctionRegistry()
	execCtx := createTestExecutionContext()

	t.Run("hashFiles function", func(t *testing.T) {
		// Test with file paths
		result, err := fr.Call("hashFiles", []interface{}{"file1.txt", "file2.txt"}, execCtx)
		require.NoError(t, err)
		assert.IsType(t, "", result)
		assert.Len(t, result.(string), 32) // MD5 hash length

		// Test consistent hashing
		result2, err := fr.Call("hashFiles", []interface{}{"file1.txt", "file2.txt"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, result, result2)

		// Test different order gives different hash
		result3, err := fr.Call("hashFiles", []interface{}{"file2.txt", "file1.txt"}, execCtx)
		require.NoError(t, err)
		assert.NotEqual(t, result, result3)

		// Test no arguments
		_, err = fr.Call("hashFiles", []interface{}{}, execCtx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least 1 argument")
	})

	t.Run("runner function", func(t *testing.T) {
		result, err := fr.Call("runner", []interface{}{}, execCtx)
		require.NoError(t, err)

		runnerInfo := result.(map[string]interface{})
		assert.Equal(t, "linux", runnerInfo["os"])
		assert.Equal(t, "x64", runnerInfo["arch"])
		assert.Equal(t, "lacquer-runner", runnerInfo["name"])

		// Test wrong number of arguments
		_, err = fr.Call("runner", []interface{}{"arg"}, execCtx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "takes no arguments")
	})

	t.Run("job function", func(t *testing.T) {
		result, err := fr.Call("job", []interface{}{}, execCtx)
		require.NoError(t, err)

		jobInfo := result.(map[string]interface{})
		assert.Equal(t, "success", jobInfo["status"])
	})

	t.Run("needs function", func(t *testing.T) {
		result, err := fr.Call("needs", []interface{}{}, execCtx)
		require.NoError(t, err)

		needs := result.(map[string]interface{})
		assert.Equal(t, 0, len(needs)) // Empty for single-job workflows
	})

}

func TestFunctionRegistry_ObjectFunctions(t *testing.T) {
	fr := NewFunctionRegistry()
	execCtx := createTestExecutionContext()

	t.Run("keys function", func(t *testing.T) {
		// Test with string map
		obj := map[string]interface{}{
			"name": "test",
			"age":  30,
		}
		result, err := fr.Call("keys", []interface{}{obj}, execCtx)
		require.NoError(t, err)

		keys := result.([]interface{})
		assert.Equal(t, 2, len(keys))
		assert.Contains(t, keys, "name")
		assert.Contains(t, keys, "age")

		// Test with empty object
		result, err = fr.Call("keys", []interface{}{map[string]interface{}{}}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, 0, len(result.([]interface{})))

		// Test with non-object
		result, err = fr.Call("keys", []interface{}{"string"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, 0, len(result.([]interface{})))
	})

	t.Run("values function", func(t *testing.T) {
		// Test with string map
		obj := map[string]interface{}{
			"name": "test",
			"age":  30,
		}
		result, err := fr.Call("values", []interface{}{obj}, execCtx)
		require.NoError(t, err)

		values := result.([]interface{})
		assert.Equal(t, 2, len(values))
		assert.Contains(t, values, "test")
		assert.Contains(t, values, 30)
	})

	t.Run("length function", func(t *testing.T) {
		// Test with string
		result, err := fr.Call("length", []interface{}{"hello"}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, int64(5), result)

		// Test with array
		result, err = fr.Call("length", []interface{}{[]interface{}{"a", "b", "c"}}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result)

		// Test with map
		obj := map[string]interface{}{
			"name": "test",
			"age":  30,
		}
		result, err = fr.Call("length", []interface{}{obj}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result)

		// Test with non-countable type
		result, err = fr.Call("length", []interface{}{42}, execCtx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), result)
	})
}

func TestFunctionRegistry_UnknownFunction(t *testing.T) {
	fr := NewFunctionRegistry()
	execCtx := createTestExecutionContext()

	_, err := fr.Call("unknownFunction", []interface{}{}, execCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown function: unknownFunction")
}

func TestFunctionRegistry_AllSupportedFunctions(t *testing.T) {
	fr := NewFunctionRegistry()
	execCtx := createTestExecutionContext()

	// List of all functions that should be supported
	supportedFunctions := []string{
		// String functions
		"contains", "startsWith", "endsWith", "format", "join", "toJSON", "fromJSON",
		// Utility functions
		"success", "always", "cancelled", "failure",
		// Context functions
		"hashFiles", "runner", "job", "needs", "matrix",
		// File functions
		"glob",
		// Object functions
		"keys", "values", "length",
	}

	// Test that all functions exist (don't error on unknown function)
	for _, funcName := range supportedFunctions {
		t.Run("function_exists_"+funcName, func(t *testing.T) {
			// We don't test the actual functionality here, just that the function exists
			// Some functions may error due to wrong arguments, but not due to being unknown
			_, err := fr.Call(funcName, []interface{}{}, execCtx)
			if err != nil {
				// The error should not be "unknown function"
				assert.NotContains(t, err.Error(), "unknown function")
			}
		})
	}
}
