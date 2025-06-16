package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lacquer/lacquer/internal/runtime"
)

func main() {
	// Example 1: Default streaming behavior with console logging
	fmt.Println("=== Example 1: Default Streaming ===")
	config1 := runtime.DefaultClaudeCodeConfig()
	// EnableStreaming and ShowProgress are true by default
	
	provider1, err := runtime.NewClaudeCodeProvider(config1)
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		return
	}
	defer provider1.Close()

	request := &runtime.ModelRequest{
		Prompt: "Hello, can you help me write a simple Python function?",
		Model:  "sonnet",
	}

	ctx := context.Background()
	
	// This will use streaming with default progress logging to console
	response1, usage1, err := provider1.Generate(ctx, request)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Response: %s\n", response1)
		fmt.Printf("Token usage: %+v\n", usage1)
	}

	fmt.Println("\n=== Example 2: Custom Progress Callback ===")
	
	// Example 2: Custom progress callback
	customCallback := func(status string, details map[string]interface{}) {
		timestamp := time.Now().Format("15:04:05")
		
		switch status {
		case "Initializing Claude Code session":
			if model, ok := details["model"].(string); ok {
				fmt.Printf("[%s] 🔧 Setting up session with model: %s\n", timestamp, model)
			}
		case "Generating response":
			if preview, ok := details["content_preview"].(string); ok {
				fmt.Printf("[%s] 💭 Thinking: %s\n", timestamp, preview)
			}
		case "Using tool":
			if toolName, ok := details["tool_name"].(string); ok {
				fmt.Printf("[%s] 🔨 Using tool: %s\n", timestamp, toolName)
			}
		case "Tool completed":
			if preview, ok := details["tool_result_preview"].(string); ok {
				fmt.Printf("[%s] ✅ Tool result: %s\n", timestamp, preview)
			}
		case "Completed":
			if duration, ok := details["duration_ms"].(int); ok {
				fmt.Printf("[%s] 🎉 Completed in %dms\n", timestamp, duration)
			}
		case "Error":
			fmt.Printf("[%s] ❌ Error occurred\n", timestamp)
		}
	}

	provider2, err := runtime.NewClaudeCodeProviderWithCallback(nil, customCallback)
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		return
	}
	defer provider2.Close()

	// This will use streaming with custom progress callback
	response2, usage2, err := provider2.Generate(ctx, request)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Response: %s\n", response2)
		fmt.Printf("Token usage: %+v\n", usage2)
	}

	fmt.Println("\n=== Example 3: Streaming Disabled ===")
	
	// Example 3: Disable streaming
	config3 := runtime.DefaultClaudeCodeConfig()
	config3.EnableStreaming = false
	config3.ShowProgress = false
	
	provider3, err := runtime.NewClaudeCodeProvider(config3)
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		return
	}
	defer provider3.Close()

	// This will not use streaming or progress callbacks
	response3, usage3, err := provider3.Generate(ctx, request)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Response: %s\n", response3)
		fmt.Printf("Token usage: %+v\n", usage3)
	}
}