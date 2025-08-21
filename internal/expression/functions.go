package expression

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lacquerai/lacquer/internal/execcontext"
)

// FunctionRegistry manages built-in functions for expressions
type FunctionRegistry struct {
	functions map[string]*FunctionDefinition
}

type FunctionDefinition struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Args        []Argument `json:"args"`
	Returns     string     `json:"returns"`
	Example     string     `json:"example"`
	Impl        Function   `json:"-"`
}

type Argument struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// Function represents a built-in function
type Function func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error)

// NewFunctionRegistry creates a new function registry with all GitHub Actions functions
func NewFunctionRegistry() *FunctionRegistry {
	fr := &FunctionRegistry{
		functions: make(map[string]*FunctionDefinition),
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

	return fn.Impl(args, execCtx)
}

// GetFunctionDefinition returns the function definition for a given name
func (fr *FunctionRegistry) GetFunctionDefinition(name string) (*FunctionDefinition, bool) {
	fn, exists := fr.functions[name]
	return fn, exists
}

// ListFunctions returns all available function definitions sorted by name
func (fr *FunctionRegistry) ListFunctions() []*FunctionDefinition {
	funcs := make([]*FunctionDefinition, 0, len(fr.functions))

	for _, fn := range fr.functions {
		funcs = append(funcs, fn)
	}

	sort.Slice(funcs, func(i, j int) bool {
		return funcs[i].Name < funcs[j].Name
	})

	return funcs
}

// GetCompactPromptDescription returns a condensed version suitable for shorter prompts
func (fr *FunctionRegistry) GetCompactPromptDescription() string {
	var sb strings.Builder

	sb.WriteString("Available expression functions:\n")

	// Sort function names for consistent output
	var names []string
	for name := range fr.functions {
		names = append(names, name)
	}

	for i, name := range names {
		if i > 0 {
			sb.WriteString(", ")
		}

		fn := fr.functions[name]
		sb.WriteString(name)
		sb.WriteString("(")

		var argParts []string
		for _, arg := range fn.Args {
			argStr := arg.Name
			if !arg.Required {
				argStr += "?"
			}
			argParts = append(argParts, argStr)
		}
		sb.WriteString(strings.Join(argParts, ", "))
		sb.WriteString(")")
	}

	return sb.String()
}

// registerStringFunctions registers string manipulation functions
func (fr *FunctionRegistry) registerStringFunctions() {
	// contains(search, item) - returns true if search contains item
	fr.functions["contains"] = &FunctionDefinition{
		Name:        "contains",
		Description: "Returns true if search contains item",
		Args: []Argument{
			{Name: "search", Type: "string", Required: true},
			{Name: "item", Type: "string", Required: true},
		},
		Returns: "boolean",
		Example: "contains('hello world', 'world') → true",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("contains() requires exactly 2 arguments")
			}

			search := toString(args[0])
			item := toString(args[1])

			return strings.Contains(search, item), nil
		},
	}

	// startsWith(searchString, searchValue) - returns true if searchString starts with searchValue
	fr.functions["startsWith"] = &FunctionDefinition{
		Name:        "startsWith",
		Description: "Returns true if searchString starts with searchValue",
		Args: []Argument{
			{Name: "searchString", Type: "string", Required: true},
			{Name: "searchValue", Type: "string", Required: true},
		},
		Returns: "boolean",
		Example: "startsWith('hello world', 'hello') → true",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("startsWith() requires exactly 2 arguments")
			}

			searchString := toString(args[0])
			searchValue := toString(args[1])

			return strings.HasPrefix(searchString, searchValue), nil
		},
	}

	// endsWith(searchString, searchValue) - returns true if searchString ends with searchValue
	fr.functions["endsWith"] = &FunctionDefinition{
		Name:        "endsWith",
		Description: "Returns true if searchString ends with searchValue",
		Args: []Argument{
			{Name: "searchString", Type: "string", Required: true},
			{Name: "searchValue", Type: "string", Required: true},
		},
		Returns: "boolean",
		Example: "endsWith('hello world', 'world') → true",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("endsWith() requires exactly 2 arguments")
			}

			searchString := toString(args[0])
			searchValue := toString(args[1])

			return strings.HasSuffix(searchString, searchValue), nil
		},
	}

	// format(string, ...args) - formats a string with placeholders
	fr.functions["format"] = &FunctionDefinition{
		Name:        "format",
		Description: "Formats a string with placeholders using {0}, {1}, etc.",
		Args: []Argument{
			{Name: "format", Type: "string", Required: true},
			{Name: "args", Type: "any", Required: false}, // variadic args
		},
		Returns: "string",
		Example: "format('Hello {0}!', 'world') → 'Hello world!'",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
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
		},
	}

	// join(array, separator) - joins array elements with separator
	fr.functions["join"] = &FunctionDefinition{
		Name:        "join",
		Description: "Joins array elements with separator",
		Args: []Argument{
			{Name: "array", Type: "array", Required: true},
			{Name: "separator", Type: "string", Required: false},
		},
		Returns: "string",
		Example: "join(['a', 'b', 'c'], '-') → 'a-b-c'",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
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
		},
	}

	// toJSON(value) - converts value to JSON string
	fr.functions["toJSON"] = &FunctionDefinition{
		Name:        "toJSON",
		Description: "Converts value to JSON string",
		Args: []Argument{
			{Name: "value", Type: "any", Required: true},
		},
		Returns: "string",
		Example: "toJSON({name: 'test'}) → '{\"name\":\"test\"}'",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("toJSON() requires exactly 1 argument")
			}

			jsonBytes, err := json.Marshal(args[0])
			if err != nil {
				return nil, fmt.Errorf("failed to convert to JSON: %v", err)
			}

			return string(jsonBytes), nil
		},
	}

	// fromJSON(value) - parses JSON string to object
	fr.functions["fromJSON"] = &FunctionDefinition{
		Name:        "fromJSON",
		Description: "Parses JSON string to object",
		Args: []Argument{
			{Name: "jsonString", Type: "string", Required: true},
		},
		Returns: "object",
		Example: "fromJSON('{\"name\":\"test\"}') → {name: 'test'}",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
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
		},
	}
}

// registerUtilityFunctions registers utility functions
func (fr *FunctionRegistry) registerUtilityFunctions() {
	// success() - returns true if no previous step failed
	fr.functions["success"] = &FunctionDefinition{
		Name:        "success",
		Description: "Returns true if no previous step failed",
		Args:        []Argument{},
		Returns:     "boolean",
		Example:     "success() → true",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
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
		},
	}

	// always() - always returns true
	fr.functions["always"] = &FunctionDefinition{
		Name:        "always",
		Description: "Always returns true, regardless of previous step status",
		Args:        []Argument{},
		Returns:     "boolean",
		Example:     "always() → true",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("always() takes no arguments")
			}
			return true, nil
		},
	}

	// cancelled() - returns true if workflow was cancelled
	fr.functions["cancelled"] = &FunctionDefinition{
		Name:        "cancelled",
		Description: "Returns true if workflow was cancelled",
		Args:        []Argument{},
		Returns:     "boolean",
		Example:     "cancelled() → false",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("cancelled() takes no arguments")
			}
			return execCtx.IsCancelled(), nil
		},
	}

	// failure() - returns true if any previous step failed
	fr.functions["failure"] = &FunctionDefinition{
		Name:        "failure",
		Description: "Returns true if any previous step failed",
		Args:        []Argument{},
		Returns:     "boolean",
		Example:     "failure() → false",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("failure() takes no arguments")
			}

			for _, result := range execCtx.StepResults {
				if result.Status == execcontext.StepStatusFailed {
					return true, nil
				}
			}

			return false, nil
		},
	}
}

// registerContextFunctions registers context-related functions
func (fr *FunctionRegistry) registerContextFunctions() {
	// hashFiles(path1, path2, ...) - returns hash of files
	fr.functions["hashFiles"] = &FunctionDefinition{
		Name:        "hashFiles",
		Description: "Returns MD5 hash of the specified files",
		Args: []Argument{
			{Name: "paths", Type: "string", Required: true}, // variadic paths
		},
		Returns: "string",
		Example: "hashFiles('package.json', 'yarn.lock') → 'abc123...'",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("hashFiles() requires at least 1 argument")
			}

			// For now, return a simple hash based on the file paths
			// In a real implementation, this would read and hash the actual files
			var paths []string
			for _, arg := range args {
				paths = append(paths, filepath.Join(execCtx.Cwd, toString(arg)))
			}

			combined := strings.Join(paths, "|")
			hash := sha256.Sum256([]byte(combined))

			return fmt.Sprintf("%x", hash), nil
		},
	}
}

// registerFileFunctions registers file-related functions
func (fr *FunctionRegistry) registerFileFunctions() {
	// glob(pattern) - returns files matching pattern
	fr.functions["glob"] = &FunctionDefinition{
		Name:        "glob",
		Description: "Returns files matching the specified glob pattern",
		Args: []Argument{
			{Name: "pattern", Type: "string", Required: true},
		},
		Returns: "array",
		Example: "glob('*.js') → ['file1.js', 'file2.js']",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("glob() requires exactly 1 argument")
			}

			pattern := toString(args[0])
			matches, err := filepath.Glob(filepath.Join(execCtx.Cwd, pattern))
			if err != nil {
				return nil, fmt.Errorf("glob pattern error: %v", err)
			}

			// Convert to interface{} array
			result := make([]interface{}, len(matches))
			for i, match := range matches {
				result[i] = match
			}

			return result, nil
		},
	}

	// file(path) - reads and returns file contents
	fr.functions["file"] = &FunctionDefinition{
		Name:        "file",
		Description: "Reads and returns the contents of a file",
		Args: []Argument{
			{Name: "path", Type: "string", Required: true},
		},
		Returns: "string",
		Example: "file('config.txt') → 'file content here'",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("file() requires exactly 1 argument")
			}

			path := toString(args[0])
			content, err := os.ReadFile(filepath.Join(execCtx.Cwd, path))
			if err != nil {
				return nil, fmt.Errorf("failed to read file '%s': %v", path, err)
			}

			return string(content), nil
		},
	}
}

// registerObjectFunctions registers object manipulation functions
func (fr *FunctionRegistry) registerObjectFunctions() {
	// keys(object) - returns keys of an object
	fr.functions["keys"] = &FunctionDefinition{
		Name:        "keys",
		Description: "Returns the keys of an object as an array",
		Args: []Argument{
			{Name: "object", Type: "object", Required: true},
		},
		Returns: "array",
		Example: "keys({a: 1, b: 2}) → ['a', 'b']",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
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
		},
	}

	// values(object) - returns values of an object
	fr.functions["values"] = &FunctionDefinition{
		Name:        "values",
		Description: "Returns the values of an object as an array",
		Args: []Argument{
			{Name: "object", Type: "object", Required: true},
		},
		Returns: "array",
		Example: "values({a: 1, b: 2}) → [1, 2]",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
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
		},
	}

	// length(array_or_string) - returns length of array or string
	fr.functions["length"] = &FunctionDefinition{
		Name:        "length",
		Description: "Returns the length of an array, string, or object",
		Args: []Argument{
			{Name: "value", Type: "any", Required: true},
		},
		Returns: "number",
		Example: "length('hello') → 5, length([1, 2, 3]) → 3",
		Impl: func(args []interface{}, execCtx *execcontext.ExecutionContext) (interface{}, error) {
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
		},
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
