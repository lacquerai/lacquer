package expression

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lacquerai/lacquer/internal/execcontext"
)

// FunctionRegistry manages built-in functions for expressions
type FunctionRegistry struct {
	functions map[string]Function
}

// Function represents a built-in function
type Function func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error)

// NewFunctionRegistry creates a new function registry with all GitHub Actions functions
func NewFunctionRegistry() *FunctionRegistry {
	fr := &FunctionRegistry{
		functions: make(map[string]Function),
	}

	// Register all GitHub Actions functions
	fr.registerStringFunctions()
	fr.registerUtilityFunctions()
	fr.registerContextFunctions()
	fr.registerFileFunctions()
	fr.registerObjectFunctions()

	return fr
}

// Call invokes a function with the given arguments
func (fr *FunctionRegistry) Call(name string, args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
	fn, exists := fr.functions[name]
	if !exists {
		return nil, fmt.Errorf("unknown function: %s", name)
	}

	return fn(args, execCtx)
}

// registerStringFunctions registers string manipulation functions
func (fr *FunctionRegistry) registerStringFunctions() {
	// contains(search, item) - returns true if search contains item
	fr.functions["contains"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("contains() requires exactly 2 arguments")
		}

		search := toString(args[0])
		item := toString(args[1])

		return strings.Contains(search, item), nil
	}

	// startsWith(searchString, searchValue) - returns true if searchString starts with searchValue
	fr.functions["startsWith"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("startsWith() requires exactly 2 arguments")
		}

		searchString := toString(args[0])
		searchValue := toString(args[1])

		return strings.HasPrefix(searchString, searchValue), nil
	}

	// endsWith(searchString, searchValue) - returns true if searchString ends with searchValue
	fr.functions["endsWith"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("endsWith() requires exactly 2 arguments")
		}

		searchString := toString(args[0])
		searchValue := toString(args[1])

		return strings.HasSuffix(searchString, searchValue), nil
	}

	// format(string, ...args) - formats a string with placeholders
	fr.functions["format"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("format() requires at least 1 argument")
		}

		format := toString(args[0])

		// Simple placeholder replacement {0}, {1}, etc.
		result := format
		for i, arg := range args[1:] {
			placeholder := fmt.Sprintf("{%d}", i)
			result = strings.ReplaceAll(result, placeholder, toString(arg))
		}

		return result, nil
	}

	// join(array, separator) - joins array elements with separator
	fr.functions["join"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("join() requires 1 or 2 arguments")
		}

		// Convert first argument to array
		arr := toArray(args[0])
		separator := ","
		if len(args) == 2 {
			separator = toString(args[1])
		}

		var strArray []string
		for _, item := range arr {
			strArray = append(strArray, toString(item))
		}

		return strings.Join(strArray, separator), nil
	}

	// toJSON(value) - converts value to JSON string
	fr.functions["toJSON"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("toJSON() requires exactly 1 argument")
		}

		jsonBytes, err := json.Marshal(args[0])
		if err != nil {
			return nil, fmt.Errorf("failed to convert to JSON: %v", err)
		}

		return string(jsonBytes), nil
	}

	// fromJSON(value) - parses JSON string to object
	fr.functions["fromJSON"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("fromJSON() requires exactly 1 argument")
		}

		jsonStr := toString(args[0])
		var result interface{}

		err := json.Unmarshal([]byte(jsonStr), &result)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %v", err)
		}

		return result, nil
	}
}

// registerUtilityFunctions registers utility functions
func (fr *FunctionRegistry) registerUtilityFunctions() {
	// success() - returns true if no previous step failed
	fr.functions["success"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("success() takes no arguments")
		}

		// Check if any previous step failed
		for _, result := range execCtx.StepResults {
			if result.Status == execcontext.StepStatusFailed {
				return false, nil
			}
		}

		return true, nil
	}

	// always() - always returns true
	fr.functions["always"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("always() takes no arguments")
		}
		return true, nil
	}

	// cancelled() - returns true if workflow was cancelled
	fr.functions["cancelled"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("cancelled() takes no arguments")
		}
		return execCtx.IsCancelled(), nil
	}

	// failure() - returns true if any previous step failed
	fr.functions["failure"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("failure() takes no arguments")
		}

		for _, result := range execCtx.StepResults {
			if result.Status == execcontext.StepStatusFailed {
				return true, nil
			}
		}

		return false, nil
	}
}

// registerContextFunctions registers context-related functions
func (fr *FunctionRegistry) registerContextFunctions() {
	// hashFiles(path1, path2, ...) - returns hash of files
	fr.functions["hashFiles"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("hashFiles() requires at least 1 argument")
		}

		// For now, return a simple hash based on the file paths
		// In a real implementation, this would read and hash the actual files
		var paths []string
		for _, arg := range args {
			paths = append(paths, toString(arg))
		}

		combined := strings.Join(paths, "|")
		hash := md5.Sum([]byte(combined))

		return fmt.Sprintf("%x", hash), nil
	}

	// runner() - returns information about the runner
	fr.functions["runner"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("runner() takes no arguments")
		}

		return map[string]interface{}{
			"os":   "linux",
			"arch": "x64",
			"name": "lacquer-runner",
		}, nil
	}

	// job() - returns information about the current job
	fr.functions["job"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("job() takes no arguments")
		}

		return map[string]interface{}{
			"status": "success",
		}, nil
	}

	// needs() - returns outputs from previous jobs (for multi-job workflows)
	fr.functions["needs"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("needs() takes no arguments")
		}

		// For now, return empty object - this would be populated in multi-job workflows
		return map[string]interface{}{}, nil
	}

	// matrix() - returns matrix strategy values
	fr.functions["matrix"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 0 {
			return nil, fmt.Errorf("matrix() takes no arguments")
		}

		// Return matrix values if available in context
		if execCtx.Matrix != nil {
			return execCtx.Matrix, nil
		}

		return map[string]interface{}{}, nil
	}
}

// registerFileFunctions registers file-related functions
func (fr *FunctionRegistry) registerFileFunctions() {
	// glob(pattern) - returns files matching pattern
	fr.functions["glob"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("glob() requires exactly 1 argument")
		}

		pattern := toString(args[0])
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("glob pattern error: %v", err)
		}

		// Convert to interface{} array
		result := make([]interface{}, len(matches))
		for i, match := range matches {
			result[i] = match
		}

		return result, nil
	}
}

// registerObjectFunctions registers object manipulation functions
func (fr *FunctionRegistry) registerObjectFunctions() {
	// keys(object) - returns keys of an object
	fr.functions["keys"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("keys() requires exactly 1 argument")
		}

		obj := args[0]
		switch v := obj.(type) {
		case map[string]interface{}:
			keys := make([]interface{}, 0, len(v))
			for key := range v {
				keys = append(keys, key)
			}
			return keys, nil
		case map[interface{}]interface{}:
			keys := make([]interface{}, 0, len(v))
			for key := range v {
				keys = append(keys, toString(key))
			}
			return keys, nil
		default:
			return []interface{}{}, nil
		}
	}

	// values(object) - returns values of an object
	fr.functions["values"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("values() requires exactly 1 argument")
		}

		obj := args[0]
		switch v := obj.(type) {
		case map[string]interface{}:
			values := make([]interface{}, 0, len(v))
			for _, value := range v {
				values = append(values, value)
			}
			return values, nil
		case map[interface{}]interface{}:
			values := make([]interface{}, 0, len(v))
			for _, value := range v {
				values = append(values, value)
			}
			return values, nil
		default:
			return []interface{}{}, nil
		}
	}

	// length(array_or_string) - returns length of array or string
	fr.functions["length"] = func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("length() requires exactly 1 argument")
		}

		value := args[0]
		switch v := value.(type) {
		case string:
			return int64(len(v)), nil
		case []interface{}:
			return int64(len(v)), nil
		case map[string]interface{}:
			return int64(len(v)), nil
		case map[interface{}]interface{}:
			return int64(len(v)), nil
		default:
			return int64(0), nil
		}
	}
}

// Helper functions for type conversion

func toString(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []interface{}:
		// Convert arrays to comma-separated strings
		strs := make([]string, len(val))
		for i, item := range val {
			strs[i] = toString(item)
		}
		return strings.Join(strs, ", ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toArray(v interface{}) []interface{} {
	switch val := v.(type) {
	case []interface{}:
		return val
	case string:
		// Split string by comma for simple array conversion
		parts := strings.Split(val, ",")
		result := make([]interface{}, len(parts))
		for i, part := range parts {
			result[i] = strings.TrimSpace(part)
		}
		return result
	default:
		return []interface{}{v}
	}
}

// Additional helper functions for GitHub Actions compatibility

// validateRegex validates a regex pattern
func validateRegex(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(pattern)
}

// matchesPattern checks if a string matches a glob pattern
func matchesPattern(text, pattern string) bool {
	matched, err := filepath.Match(pattern, text)
	if err != nil {
		return false
	}
	return matched
}
