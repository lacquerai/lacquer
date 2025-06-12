package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"time"

	"github.com/lacquer/lacquer/internal/ast"
)

// generateRunID creates a unique identifier for a workflow execution
func generateRunID() string {
	// Generate a random 8-byte ID
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return "run_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return "run_" + hex.EncodeToString(bytes)
}

// getEnvironmentVars returns a map of environment variables
func getEnvironmentVars() map[string]string {
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		if idx := findIndex(kv, '='); idx != -1 {
			key := kv[:idx]
			value := kv[idx+1:]
			env[key] = value
		}
	}
	return env
}

// buildMetadata creates metadata map from workflow
func buildMetadata(workflow *ast.Workflow) map[string]interface{} {
	metadata := make(map[string]interface{})
	
	if workflow.Metadata != nil {
		metadata["name"] = workflow.Metadata.Name
		metadata["description"] = workflow.Metadata.Description
		metadata["author"] = workflow.Metadata.Author
		metadata["version"] = workflow.Metadata.Version
		metadata["tags"] = workflow.Metadata.Tags
	}
	
	metadata["workflow_version"] = workflow.Version
	metadata["source_file"] = workflow.SourceFile
	
	return metadata
}

// findIndex finds the first occurrence of a byte in a string
func findIndex(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// SafeString safely converts an interface to string
func SafeString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// SafeInt safely converts an interface to int
func SafeInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return 0
}

// SafeBool safely converts an interface to bool
func SafeBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "1" || val == "yes"
	case int:
		return val != 0
	case float64:
		return val != 0
	}
	return false
}

// MergeMap merges source map into destination map
func MergeMap(dst, src map[string]interface{}) {
	for k, v := range src {
		dst[k] = v
	}
}

// CopyMap creates a deep copy of a map
func CopyMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{})
	for k, v := range src {
		dst[k] = copyValue(v)
	}
	return dst
}

// copyValue creates a copy of a value, handling nested maps and slices
func copyValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return CopyMap(val)
	case []interface{}:
		copy := make([]interface{}, len(val))
		for i, item := range val {
			copy[i] = copyValue(item)
		}
		return copy
	default:
		return v
	}
}

// GetMapValue safely gets a value from a nested map using dot notation
func GetMapValue(m map[string]interface{}, path string) (interface{}, bool) {
	if path == "" {
		return nil, false
	}
	
	keys := splitPath(path)
	current := interface{}(m)
	
	for _, key := range keys {
		switch val := current.(type) {
		case map[string]interface{}:
			if next, exists := val[key]; exists {
				current = next
			} else {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	
	return current, true
}

// SetMapValue safely sets a value in a nested map using dot notation
func SetMapValue(m map[string]interface{}, path string, value interface{}) {
	if path == "" {
		return
	}
	
	keys := splitPath(path)
	current := m
	
	// Navigate to the parent of the target key
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]
		
		if next, exists := current[key]; exists {
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				// Create new map if existing value is not a map
				current[key] = make(map[string]interface{})
				current = current[key].(map[string]interface{})
			}
		} else {
			// Create new map for missing key
			current[key] = make(map[string]interface{})
			current = current[key].(map[string]interface{})
		}
	}
	
	// Set the final value
	current[keys[len(keys)-1]] = value
}

// splitPath splits a dot-notation path into individual keys
func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	
	var keys []string
	current := ""
	
	for i, char := range path {
		if char == '.' {
			if current != "" {
				keys = append(keys, current)
				current = ""
			}
		} else {
			current += string(char)
		}
		
		// Add the last key
		if i == len(path)-1 && current != "" {
			keys = append(keys, current)
		}
	}
	
	return keys
}

// IsValidVariableName checks if a string is a valid variable name
func IsValidVariableName(name string) bool {
	if name == "" {
		return false
	}
	
	// Must start with letter or underscore
	first := name[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return false
	}
	
	// Rest can be letters, digits, or underscores
	for i := 1; i < len(name); i++ {
		char := name[i]
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || 
			 (char >= '0' && char <= '9') || char == '_') {
			return false
		}
	}
	
	return true
}

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return d.String()
	}
	if d < time.Second {
		return d.Truncate(time.Millisecond).String()
	}
	if d < time.Minute {
		return d.Truncate(10 * time.Millisecond).String()
	}
	return d.Truncate(time.Second).String()
}