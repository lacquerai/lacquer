package block

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBlockSystemIntegration(t *testing.T) {
	// Create temporary cache directory
	tmpDir, err := os.MkdirTemp("", "laq-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create block manager
	manager, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	t.Run("DataTransformerBlock", func(t *testing.T) {
		testDataTransformerBlock(t, manager, ctx)
	})

	t.Run("SimpleCalculatorBlock", func(t *testing.T) {
		testSimpleCalculatorBlock(t, manager, ctx)
	})

	t.Run("InvalidBlocks", func(t *testing.T) {
		testInvalidBlocks(t, manager, ctx)
	})
}

func testDataTransformerBlock(t *testing.T, manager *Manager, ctx context.Context) {
	// Create a temporary data transformer block
	blockDir := createDataTransformerBlock(t)
	defer os.RemoveAll(blockDir)

	// Test sum operation
	t.Run("SumOperation", func(t *testing.T) {
		inputs := map[string]interface{}{
			"numbers":   []interface{}{10.0, 20.0, 30.0, 40.0, 50.0},
			"operation": "sum",
			"precision": 2,
		}

		outputs, err := manager.ExecuteBlock(ctx, blockDir, inputs, "test-workflow", "test-step")
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		// Verify result
		result, ok := outputs["result"]
		if !ok {
			t.Fatal("Expected 'result' in outputs")
		}

		if result != 150.0 {
			t.Errorf("Expected result 150.0, got %v", result)
		}

		count, ok := outputs["count"]
		if !ok {
			t.Fatal("Expected 'count' in outputs")
		}

		if count != 5.0 { // JSON numbers are float64
			t.Errorf("Expected count 5, got %v", count)
		}
	})

	// Test average operation
	t.Run("AverageOperation", func(t *testing.T) {
		inputs := map[string]interface{}{
			"numbers":   []interface{}{10.0, 20.0, 30.0},
			"operation": "average",
			"precision": 2,
		}

		outputs, err := manager.ExecuteBlock(ctx, blockDir, inputs, "test-workflow", "test-step")
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		result, ok := outputs["result"]
		if !ok {
			t.Fatal("Expected 'result' in outputs")
		}

		if result != 20.0 {
			t.Errorf("Expected result 20.0, got %v", result)
		}
	})

	// Test sort operation
	t.Run("SortOperation", func(t *testing.T) {
		inputs := map[string]interface{}{
			"numbers":   []interface{}{5.0, 1.0, 9.0, 3.0, 7.0},
			"operation": "sort_desc",
			"precision": 2,
		}

		outputs, err := manager.ExecuteBlock(ctx, blockDir, inputs, "test-workflow", "test-step")
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		sortedValues, ok := outputs["sorted_values"]
		if !ok {
			t.Fatal("Expected 'sorted_values' in outputs")
		}

		sorted, ok := sortedValues.([]interface{})
		if !ok {
			t.Fatalf("Expected sorted_values to be array, got %T", sortedValues)
		}

		expected := []float64{9.0, 7.0, 5.0, 3.0, 1.0}
		if len(sorted) != len(expected) {
			t.Fatalf("Expected %d values, got %d", len(expected), len(sorted))
		}

		for i, val := range sorted {
			if val != expected[i] {
				t.Errorf("Expected sorted[%d] = %v, got %v", i, expected[i], val)
			}
		}
	})

	// Test invalid operation
	t.Run("InvalidOperation", func(t *testing.T) {
		inputs := map[string]interface{}{
			"numbers":   []interface{}{1.0, 2.0},
			"operation": "invalid_op",
		}

		_, err := manager.ExecuteBlock(ctx, blockDir, inputs, "test-workflow", "test-step")
		if err == nil {
			t.Error("Expected error for invalid operation")
		}
	})
}

func testSimpleCalculatorBlock(t *testing.T, manager *Manager, ctx context.Context) {
	// Skip if Docker is not available
	dockerExecutor := NewDockerExecutor()
	if err := dockerExecutor.checkDockerAvailable(); err != nil {
		t.Skip("Docker not available, skipping Docker block tests")
	}

	// Create a temporary calculator block
	blockDir := createSimpleCalculatorBlock(t)
	defer os.RemoveAll(blockDir)

	testCases := []struct {
		name      string
		a, b      float64
		operation string
		expected  float64
	}{
		{"Addition", 5.0, 3.0, "add", 8.0},
		{"Subtraction", 10.0, 4.0, "subtract", 6.0},
		{"Multiplication", 6.0, 7.0, "multiply", 42.0},
		{"Division", 15.0, 3.0, "divide", 5.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := map[string]interface{}{
				"a":         tc.a,
				"b":         tc.b,
				"operation": tc.operation,
			}

			outputs, err := manager.ExecuteBlock(ctx, blockDir, inputs, "test-workflow", "test-step")
			if err != nil {
				t.Fatalf("Execution failed: %v", err)
			}

			result, ok := outputs["result"]
			if !ok {
				t.Fatal("Expected 'result' in outputs")
			}

			if result != tc.expected {
				t.Errorf("Expected result %v, got %v", tc.expected, result)
			}

			operation, ok := outputs["operation_performed"]
			if !ok {
				t.Fatal("Expected 'operation_performed' in outputs")
			}

			opStr, ok := operation.(string)
			if !ok {
				t.Fatalf("Expected operation_performed to be string, got %T", operation)
			}

			if opStr == "" {
				t.Error("Expected non-empty operation description")
			}

			t.Logf("Operation: %s", opStr)
		})
	}

	// Test division by zero
	t.Run("DivisionByZero", func(t *testing.T) {
		inputs := map[string]interface{}{
			"a":         5.0,
			"b":         0.0,
			"operation": "divide",
		}

		_, err := manager.ExecuteBlock(ctx, blockDir, inputs, "test-workflow", "test-step")
		if err == nil {
			t.Error("Expected error for division by zero")
		}
	})
}

func testInvalidBlocks(t *testing.T, manager *Manager, ctx context.Context) {
	// Test loading non-existent block
	t.Run("NonExistentBlock", func(t *testing.T) {
		_, err := manager.LoadBlock(ctx, "./non-existent-block")
		if err == nil {
			t.Error("Expected error for non-existent block")
		}
	})

	// Test invalid YAML
	t.Run("InvalidYAML", func(t *testing.T) {
		blockDir := createInvalidYAMLBlock(t)
		defer os.RemoveAll(blockDir)

		_, err := manager.LoadBlock(ctx, blockDir)
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}
	})

	// Test missing required fields
	t.Run("MissingRequiredFields", func(t *testing.T) {
		blockDir := createInvalidBlock(t)
		defer os.RemoveAll(blockDir)

		_, err := manager.LoadBlock(ctx, blockDir)
		if err == nil {
			t.Error("Expected error for missing required fields")
		}
	})

	// Test input validation
	t.Run("InputValidation", func(t *testing.T) {
		blockDir := createDataTransformerBlock(t)
		defer os.RemoveAll(blockDir)

		// Missing required input
		inputs := map[string]interface{}{
			"operation": "sum",
			// missing "numbers"
		}

		_, err := manager.ExecuteBlock(ctx, blockDir, inputs, "test-workflow", "test-step")
		if err == nil {
			t.Error("Expected error for missing required input")
		}

		// Invalid type
		inputs = map[string]interface{}{
			"numbers":   "not an array",
			"operation": "sum",
		}

		_, err = manager.ExecuteBlock(ctx, blockDir, inputs, "test-workflow", "test-step")
		if err == nil {
			t.Error("Expected error for invalid input type")
		}

		// Invalid enum value
		inputs = map[string]interface{}{
			"numbers":   []interface{}{1.0, 2.0},
			"operation": "invalid_operation",
		}

		_, err = manager.ExecuteBlock(ctx, blockDir, inputs, "test-workflow", "test-step")
		if err == nil {
			t.Error("Expected error for invalid enum value")
		}
	})
}

// Helper functions to create test blocks

func createDataTransformerBlock(t *testing.T) string {
	blockDir, err := os.MkdirTemp("", "data-transformer-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	blockConfig := `name: data-transformer
runtime: go
description: Transforms numerical data using various mathematical operations

inputs:
  numbers:
    type: array
    items: number
    description: Array of numbers to process
    required: true
  operation:
    type: string
    description: Mathematical operation to perform
    enum: ["sum", "average", "min", "max", "sort_asc", "sort_desc"]
    required: true
  precision:
    type: integer
    description: Number of decimal places for results
    default: 2

script: |
  package main
  
  import (
    "encoding/json"
    "fmt"
    "math"
    "os"
    "sort"
  )
  
  type Input struct {
    Numbers   []float64 ` + "`json:\"numbers\"`" + `
    Operation string    ` + "`json:\"operation\"`" + `
    Precision int       ` + "`json:\"precision\"`" + `
  }
  
  type Output struct {
    Result       *float64      ` + "`json:\"result,omitempty\"`" + `
    Count        int           ` + "`json:\"count\"`" + `
    SortedValues []float64     ` + "`json:\"sorted_values,omitempty\"`" + `
  }
  
  type ExecutionInput struct {
    Inputs Input ` + "`json:\"inputs\"`" + `
  }
  
  func roundToPlaces(val float64, places int) float64 {
    multiplier := math.Pow(10, float64(places))
    return math.Round(val*multiplier) / multiplier
  }
  
  func main() {
    var execInput ExecutionInput
    if err := json.NewDecoder(os.Stdin).Decode(&execInput); err != nil {
      fmt.Fprintf(os.Stderr, ` + "`{\"error\": {\"message\": \"Invalid input JSON: %s\"}}`" + `, err.Error())
      os.Exit(1)
    }
    
    input := execInput.Inputs
    
    if len(input.Numbers) == 0 {
      fmt.Fprintf(os.Stderr, ` + "`{\"error\": {\"message\": \"Numbers array cannot be empty\"}}`" + `)
      os.Exit(1)
    }
    
    var output Output
    output.Count = len(input.Numbers)
    
    switch input.Operation {
    case "sum":
      sum := 0.0
      for _, n := range input.Numbers {
        sum += n
      }
      rounded := roundToPlaces(sum, input.Precision)
      output.Result = &rounded
      
    case "average":
      sum := 0.0
      for _, n := range input.Numbers {
        sum += n
      }
      avg := sum / float64(len(input.Numbers))
      rounded := roundToPlaces(avg, input.Precision)
      output.Result = &rounded
      
    case "min":
      min := input.Numbers[0]
      for _, n := range input.Numbers[1:] {
        if n < min {
          min = n
        }
      }
      rounded := roundToPlaces(min, input.Precision)
      output.Result = &rounded
      
    case "max":
      max := input.Numbers[0]
      for _, n := range input.Numbers[1:] {
        if n > max {
          max = n
        }
      }
      rounded := roundToPlaces(max, input.Precision)
      output.Result = &rounded
      
    case "sort_asc":
      sorted := make([]float64, len(input.Numbers))
      copy(sorted, input.Numbers)
      sort.Float64s(sorted)
      output.SortedValues = sorted
      
    case "sort_desc":
      sorted := make([]float64, len(input.Numbers))
      copy(sorted, input.Numbers)
      sort.Sort(sort.Reverse(sort.Float64Slice(sorted)))
      output.SortedValues = sorted
      
    default:
      fmt.Fprintf(os.Stderr, ` + "`{\"error\": {\"message\": \"Unknown operation: %s\"}}`" + `, input.Operation)
      os.Exit(1)
    }
    
    json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
      "outputs": output,
    })
  }

outputs:
  result:
    type: number
    description: The computed result
  count:
    type: integer
    description: Number of input values processed
  sorted_values:
    type: array
    description: Sorted values (for sort operations only)`

	configPath := filepath.Join(blockDir, "block.laq.yaml")
	if err := os.WriteFile(configPath, []byte(blockConfig), 0644); err != nil {
		t.Fatalf("Failed to write block config: %v", err)
	}

	return blockDir
}

func createSimpleCalculatorBlock(t *testing.T) string {
	blockDir, err := os.MkdirTemp("", "simple-calculator-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	blockConfig := `name: simple-calculator
runtime: docker
image: python:3.11-alpine
description: Simple calculator using Python to demonstrate multi-language JSON protocol

inputs:
  a:
    type: number
    description: First operand
    required: true
  b:
    type: number
    description: Second operand
    required: true
  operation:
    type: string
    description: Mathematical operation
    enum: ["add", "subtract", "multiply", "divide"]
    required: true

command: ["python3", "-c", "
import json
import sys
import math

# Read input
try:
    input_data = json.load(sys.stdin)
    inputs = input_data['inputs']
    a = inputs['a']
    b = inputs['b'] 
    operation = inputs['operation']
except Exception as e:
    error = {'error': {'message': f'Invalid input: {str(e)}'}}
    json.dump(error, sys.stderr)
    sys.exit(1)

# Calculate result
try:
    if operation == 'add':
        result = a + b
    elif operation == 'subtract':
        result = a - b
    elif operation == 'multiply':
        result = a * b
    elif operation == 'divide':
        if b == 0:
            raise ValueError('Division by zero')
        result = a / b
    else:
        raise ValueError(f'Unknown operation: {operation}')
        
    # Return output
    output = {
        'outputs': {
            'result': result,
            'operation_performed': f'{a} {operation} {b} = {result}'
        }
    }
    json.dump(output, sys.stdout)
    
except Exception as e:
    error = {'error': {'message': str(e)}}
    json.dump(error, sys.stderr)
    sys.exit(1)
"]

outputs:
  result:
    type: number
    description: The calculated result
  operation_performed:
    type: string
    description: Human-readable description of the operation`

	configPath := filepath.Join(blockDir, "block.laq.yaml")
	if err := os.WriteFile(configPath, []byte(blockConfig), 0644); err != nil {
		t.Fatalf("Failed to write block config: %v", err)
	}

	return blockDir
}

func createInvalidYAMLBlock(t *testing.T) string {
	blockDir, err := os.MkdirTemp("", "invalid-yaml-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	invalidYAML := `name: invalid-block
runtime: go
invalid yaml structure [
  - missing closing bracket`

	configPath := filepath.Join(blockDir, "block.laq.yaml")
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write invalid YAML: %v", err)
	}

	return blockDir
}

func createInvalidBlock(t *testing.T) string {
	blockDir, err := os.MkdirTemp("", "invalid-block-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	invalidConfig := `# Missing required name field
runtime: go
script: |
  package main
  func main() {}`

	configPath := filepath.Join(blockDir, "block.laq.yaml")
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	return blockDir
}
