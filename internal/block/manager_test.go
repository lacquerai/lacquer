package block

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBlockManager(t *testing.T) {
	// Create temporary cache directory
	tmpDir, err := os.MkdirTemp("", "laq-block-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create block manager
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Test loading a block that doesn't exist
	ctx := context.Background()
	_, err = manager.LoadBlock(ctx, "./nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent block")
	}

	// Test loading the data transformer example
	examplePath := "../../../examples/blocks/data-transformer"
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Skip("Example blocks not available")
	}

	block, err := manager.LoadBlock(ctx, examplePath)
	if err != nil {
		t.Fatalf("Failed to load example block: %v", err)
	}

	// Verify block metadata
	if block.Name != "data-transformer" {
		t.Errorf("Expected block name 'data-transformer', got '%s'", block.Name)
	}
	if block.Runtime != RuntimeGo {
		t.Errorf("Expected runtime 'go', got '%s'", block.Runtime)
	}

	// Test input validation
	inputs := map[string]interface{}{
		"numbers":   []interface{}{1.0, 2.0, 3.0, 4.0, 5.0},
		"operation": "sum",
	}

	err = manager.validateInputs(block, inputs)
	if err != nil {
		t.Errorf("Input validation failed: %v", err)
	}

	// Test invalid input
	invalidInputs := map[string]interface{}{
		"numbers":   "not an array",
		"operation": "sum",
	}

	err = manager.validateInputs(block, invalidInputs)
	if err == nil {
		t.Error("Expected validation error for invalid input type")
	}
}

func TestFileLoader(t *testing.T) {
	loader := NewFileLoader()
	ctx := context.Background()

	// Test loading nonexistent block
	_, err := loader.Load(ctx, "./nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent block")
	}

	// Create a temporary block for testing
	tmpDir, err := os.MkdirTemp("", "laq-test-block-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple block config
	blockConfig := `
name: test-block
runtime: go
description: Test block

inputs:
  value:
    type: string
    required: true

script: |
  package main
  import "fmt"
  func main() {
    fmt.Println("test")
  }

outputs:
  result:
    type: string
`

	configPath := filepath.Join(tmpDir, "block.laq.yaml")
	if err := os.WriteFile(configPath, []byte(blockConfig), 0644); err != nil {
		t.Fatalf("Failed to write block config: %v", err)
	}

	// Load the block
	block, err := loader.Load(ctx, tmpDir)
	if err != nil {
		t.Fatalf("Failed to load test block: %v", err)
	}

	// Verify block properties
	if block.Name != "test-block" {
		t.Errorf("Expected name 'test-block', got '%s'", block.Name)
	}
	if block.Runtime != RuntimeGo {
		t.Errorf("Expected runtime 'go', got '%s'", block.Runtime)
	}

	// Test caching
	cached, ok := loader.GetFromCache(tmpDir)
	if !ok {
		t.Error("Block should be cached")
	}
	if cached.Name != block.Name {
		t.Error("Cached block should match loaded block")
	}

	// Test cache invalidation on file change
	time.Sleep(10 * time.Millisecond) // Ensure different mod time
	newConfig := blockConfig + "\n# Updated"
	if err := os.WriteFile(configPath, []byte(newConfig), 0644); err != nil {
		t.Fatalf("Failed to update block config: %v", err)
	}

	// Cache should be invalidated
	_, ok = loader.getFromCache(configPath)
	if ok {
		t.Error("Cache should be invalidated after file change")
	}
}

func TestValidateType(t *testing.T) {
	tests := []struct {
		name         string
		value        interface{}
		expectedType string
		shouldError  bool
	}{
		{"string valid", "hello", "string", false},
		{"string invalid", 123, "string", true},
		{"number valid int", 42, "number", false},
		{"number valid float", 3.14, "number", false},
		{"number invalid", "not a number", "number", true},
		{"boolean valid", true, "boolean", false},
		{"boolean invalid", "true", "boolean", true},
		{"array valid", []interface{}{1, 2, 3}, "array", false},
		{"array invalid", "not array", "array", true},
		{"object valid", map[string]interface{}{"key": "value"}, "object", false},
		{"object invalid", []interface{}{}, "object", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateType("test", tt.value, tt.expectedType)
			if tt.shouldError && err == nil {
				t.Error("Expected validation error")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidateEnum(t *testing.T) {
	enum := []string{"option1", "option2", "option3"}

	// Valid enum value
	err := validateEnum("test", "option1", enum)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Invalid enum value
	err = validateEnum("test", "invalid", enum)
	if err == nil {
		t.Error("Expected enum validation error")
	}

	// Non-string value
	err = validateEnum("test", 123, enum)
	if err == nil {
		t.Error("Expected type error for non-string enum value")
	}
}
