package block

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileLoaderDetailed(t *testing.T) {
	loader := NewFileLoader()
	ctx := context.Background()

	t.Run("LoadValidBlock", func(t *testing.T) {
		blockDir := createTestBlock(t, "test-block", RuntimeGo)
		defer os.RemoveAll(blockDir)

		block, err := loader.Load(ctx, blockDir)
		if err != nil {
			t.Fatalf("Failed to load block: %v", err)
		}

		if block.Name != "test-block" {
			t.Errorf("Expected name 'test-block', got '%s'", block.Name)
		}

		if block.Runtime != RuntimeGo {
			t.Errorf("Expected runtime 'go', got '%s'", block.Runtime)
		}

		if block.Path != blockDir {
			t.Errorf("Expected path '%s', got '%s'", blockDir, block.Path)
		}
	})

	t.Run("CachingBehavior", func(t *testing.T) {
		blockDir := createTestBlock(t, "cache-test", RuntimeNative)
		defer os.RemoveAll(blockDir)

		// First load
		block1, err := loader.Load(ctx, blockDir)
		if err != nil {
			t.Fatalf("Failed to load block: %v", err)
		}

		// Second load should come from cache
		block2, err := loader.Load(ctx, blockDir)
		if err != nil {
			t.Fatalf("Failed to load cached block: %v", err)
		}

		// Should be the same instance (from cache)
		if block1 != block2 {
			t.Error("Expected cached block to be the same instance")
		}

		// Verify cache hit
		cached, ok := loader.GetFromCache(blockDir)
		if !ok {
			t.Error("Block should be in cache")
		}

		if cached.Name != block1.Name {
			t.Error("Cached block should match original")
		}
	})

	t.Run("CacheInvalidation", func(t *testing.T) {
		blockDir := createTestBlock(t, "invalidation-test", RuntimeGo)
		defer os.RemoveAll(blockDir)

		// Load block
		_, err := loader.Load(ctx, blockDir)
		if err != nil {
			t.Fatalf("Failed to load block: %v", err)
		}

		// Verify it's cached
		_, ok := loader.GetFromCache(blockDir)
		if !ok {
			t.Error("Block should be cached")
		}

		// Wait a bit to ensure different modification time
		time.Sleep(10 * time.Millisecond)

		// Modify the block file
		configPath := filepath.Join(blockDir, "block.laq.yaml")
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		// Add a comment to modify the file
		modifiedData := string(data) + "\n# Modified"
		if err := os.WriteFile(configPath, []byte(modifiedData), 0644); err != nil {
			t.Fatalf("Failed to write modified config: %v", err)
		}

		// Cache should be invalidated on next access
		_, ok = loader.getFromCache(configPath)
		if ok {
			t.Error("Cache should be invalidated after file modification")
		}

		// Loading again should work and update cache
		newBlock, err := loader.Load(ctx, blockDir)
		if err != nil {
			t.Fatalf("Failed to load modified block: %v", err)
		}

		if newBlock.Name != "invalidation-test" {
			t.Errorf("Block should still have correct name after reload")
		}
	})

	t.Run("ManualCacheInvalidation", func(t *testing.T) {
		blockDir := createTestBlock(t, "manual-invalidation", RuntimeDocker)
		defer os.RemoveAll(blockDir)

		// Load and cache block
		_, err := loader.Load(ctx, blockDir)
		if err != nil {
			t.Fatalf("Failed to load block: %v", err)
		}

		// Verify cached
		_, ok := loader.GetFromCache(blockDir)
		if !ok {
			t.Error("Block should be cached")
		}

		// Manually invalidate
		loader.InvalidateCache(blockDir)

		// Should no longer be cached
		_, ok = loader.GetFromCache(blockDir)
		if ok {
			t.Error("Block should not be cached after manual invalidation")
		}
	})

	t.Run("ErrorCases", func(t *testing.T) {
		// Non-existent directory
		_, err := loader.Load(ctx, "/non/existent/path")
		if err == nil {
			t.Error("Expected error for non-existent path")
		}

		// File instead of directory
		tempFile, err := os.CreateTemp("", "not-a-dir-*")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tempFile.Close()
		defer os.Remove(tempFile.Name())

		_, err = loader.Load(ctx, tempFile.Name())
		if err == nil {
			t.Error("Expected error when path is a file, not directory")
		}

		// Directory without block.laq.yaml
		emptyDir, err := os.MkdirTemp("", "empty-dir-*")
		if err != nil {
			t.Fatalf("Failed to create empty dir: %v", err)
		}
		defer os.RemoveAll(emptyDir)

		_, err = loader.Load(ctx, emptyDir)
		if err == nil {
			t.Error("Expected error for directory without block.laq.yaml")
		}
	})

	t.Run("InvalidYAML", func(t *testing.T) {
		blockDir, err := os.MkdirTemp("", "invalid-yaml-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(blockDir)

		// Write invalid YAML
		configPath := filepath.Join(blockDir, "block.laq.yaml")
		invalidYAML := "invalid: yaml: content: [\nno closing bracket"
		if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
			t.Fatalf("Failed to write invalid YAML: %v", err)
		}

		_, err = loader.Load(ctx, blockDir)
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}
	})

	t.Run("ValidationErrors", func(t *testing.T) {
		tests := []struct {
			name   string
			config string
		}{
			{
				name: "MissingName",
				config: `runtime: go
script: |
  package main
  func main() {}`,
			},
			{
				name: "InvalidRuntime",
				config: `name: test-block
runtime: invalid_runtime`,
			},
			{
				name: "GoBlockMissingScript",
				config: `name: test-block
runtime: go`,
			},
			{
				name: "DockerBlockMissingImage",
				config: `name: test-block
runtime: docker`,
			},
			{
				name: "InvalidInputType",
				config: `name: test-block
runtime: go
script: |
  package main
  func main() {}
inputs:
  test_input:
    type: invalid_type`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				blockDir, err := os.MkdirTemp("", "validation-test-*")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				defer os.RemoveAll(blockDir)

				configPath := filepath.Join(blockDir, "block.laq.yaml")
				if err := os.WriteFile(configPath, []byte(tt.config), 0644); err != nil {
					t.Fatalf("Failed to write config: %v", err)
				}

				_, err = loader.Load(ctx, blockDir)
				if err == nil {
					t.Errorf("Expected validation error for %s", tt.name)
				}
			})
		}
	})

	t.Run("RelativeAndAbsolutePaths", func(t *testing.T) {
		blockDir := createTestBlock(t, "path-test", RuntimeNative)
		defer os.RemoveAll(blockDir)

		// Test with absolute path
		absPath, err := filepath.Abs(blockDir)
		if err != nil {
			t.Fatalf("Failed to get absolute path: %v", err)
		}

		block1, err := loader.Load(ctx, absPath)
		if err != nil {
			t.Fatalf("Failed to load with absolute path: %v", err)
		}

		// Test with relative path (if possible)
		if filepath.IsAbs(blockDir) {
			// blockDir is already absolute, so we can't test relative
			t.Log("Skipping relative path test (blockDir is absolute)")
		} else {
			block2, err := loader.Load(ctx, blockDir)
			if err != nil {
				t.Fatalf("Failed to load with relative path: %v", err)
			}

			// Both should resolve to the same absolute path
			if block1.Path != block2.Path {
				t.Errorf("Absolute and relative paths should resolve to same path: %s vs %s", 
					block1.Path, block2.Path)
			}
		}
	})
}

func TestValidateSchema(t *testing.T) {
	tests := []struct {
		name       string
		fieldName  string
		schemaType string
		kind       string
		wantErr    bool
	}{
		{"ValidString", "test", "string", "input", false},
		{"ValidNumber", "test", "number", "input", false},
		{"ValidInteger", "test", "integer", "input", false},
		{"ValidBoolean", "test", "boolean", "input", false},
		{"ValidArray", "test", "array", "input", false},
		{"ValidObject", "test", "object", "input", false},
		{"EmptyType", "test", "", "input", true},
		{"InvalidType", "test", "invalid", "input", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchema(tt.fieldName, tt.schemaType, tt.kind)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSchema() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function to create test blocks
func createTestBlock(t *testing.T, name string, runtime RuntimeType) string {
	blockDir, err := os.MkdirTemp("", fmt.Sprintf("%s-*", name))
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	var config string
	switch runtime {
	case RuntimeNative:
		config = fmt.Sprintf(`name: %s
runtime: native
description: Test native block

inputs:
  text:
    type: string
    required: true

workflow:
  agents:
    test_agent:
      model: gpt-4
  steps:
    - id: process
      agent: test_agent
      prompt: "Process: {{ inputs.text }}"

outputs:
  result:
    type: string`, name)

	case RuntimeGo:
		config = fmt.Sprintf(`name: %s
runtime: go
description: Test Go block

inputs:
  input:
    type: string
    required: true

script: |
  package main
  import (
    "encoding/json"
    "os"
  )
  
  func main() {
    var input map[string]interface{}
    json.NewDecoder(os.Stdin).Decode(&input)
    
    output := map[string]interface{}{
      "outputs": map[string]interface{}{
        "result": "processed",
      },
    }
    
    json.NewEncoder(os.Stdout).Encode(output)
  }

outputs:
  result:
    type: string`, name)

	case RuntimeDocker:
		config = fmt.Sprintf(`name: %s
runtime: docker
image: alpine:latest
description: Test Docker block

inputs:
  input:
    type: string
    required: true

command: ["echo", "{\"outputs\": {\"result\": \"docker-processed\"}}"]

outputs:
  result:
    type: string`, name)
	}

	configPath := filepath.Join(blockDir, "block.laq.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to write block config: %v", err)
	}

	return blockDir
}