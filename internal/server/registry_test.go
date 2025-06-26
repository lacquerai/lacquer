package server

import (
	"fmt"
	"testing"

	"github.com/lacquerai/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
)

func TestWorkflowRegistry_NewRegistry(t *testing.T) {
	registry := NewWorkflowRegistry()
	
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.workflows)
	assert.Equal(t, 0, registry.Count())
	assert.Empty(t, registry.List())
}

func TestWorkflowRegistry_Register(t *testing.T) {
	registry := NewWorkflowRegistry()
	
	workflow := &ast.Workflow{
		Version: "1.0",
		Metadata: &ast.WorkflowMetadata{
			Name: "test-workflow",
		},
	}
	
	registry.Register("test-id", workflow)
	
	assert.Equal(t, 1, registry.Count())
	assert.Contains(t, registry.List(), "test-id")
	
	retrieved, exists := registry.Get("test-id")
	assert.True(t, exists)
	assert.Equal(t, workflow, retrieved)
}

func TestWorkflowRegistry_Get_NotFound(t *testing.T) {
	registry := NewWorkflowRegistry()
	
	workflow, exists := registry.Get("non-existent")
	assert.False(t, exists)
	assert.Nil(t, workflow)
}

func TestWorkflowRegistry_Multiple_Workflows(t *testing.T) {
	registry := NewWorkflowRegistry()
	
	workflow1 := &ast.Workflow{Version: "1.0", Metadata: &ast.WorkflowMetadata{Name: "workflow1"}}
	workflow2 := &ast.Workflow{Version: "1.1", Metadata: &ast.WorkflowMetadata{Name: "workflow2"}}
	workflow3 := &ast.Workflow{Version: "2.0", Metadata: &ast.WorkflowMetadata{Name: "workflow3"}}
	
	registry.Register("id1", workflow1)
	registry.Register("id2", workflow2)
	registry.Register("id3", workflow3)
	
	assert.Equal(t, 3, registry.Count())
	
	ids := registry.List()
	assert.Len(t, ids, 3)
	assert.Contains(t, ids, "id1")
	assert.Contains(t, ids, "id2")
	assert.Contains(t, ids, "id3")
	
	// Test individual retrieval
	retrieved1, exists1 := registry.Get("id1")
	assert.True(t, exists1)
	assert.Equal(t, workflow1, retrieved1)
	
	retrieved2, exists2 := registry.Get("id2")
	assert.True(t, exists2)
	assert.Equal(t, workflow2, retrieved2)
	
	retrieved3, exists3 := registry.Get("id3")
	assert.True(t, exists3)
	assert.Equal(t, workflow3, retrieved3)
}

func TestWorkflowRegistry_Overwrite(t *testing.T) {
	registry := NewWorkflowRegistry()
	
	workflow1 := &ast.Workflow{Version: "1.0", Metadata: &ast.WorkflowMetadata{Name: "original"}}
	workflow2 := &ast.Workflow{Version: "2.0", Metadata: &ast.WorkflowMetadata{Name: "updated"}}
	
	registry.Register("test-id", workflow1)
	assert.Equal(t, 1, registry.Count())
	
	// Overwrite with same ID
	registry.Register("test-id", workflow2)
	assert.Equal(t, 1, registry.Count()) // Count should remain the same
	
	retrieved, exists := registry.Get("test-id")
	assert.True(t, exists)
	assert.Equal(t, workflow2, retrieved) // Should be the updated workflow
	assert.Equal(t, "updated", retrieved.Metadata.Name)
}

func TestWorkflowRegistry_Concurrent_Access(t *testing.T) {
	registry := NewWorkflowRegistry()
	
	// Test concurrent reads and writes
	done := make(chan bool)
	
	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			workflow := &ast.Workflow{
				Version: "1.0",
				Metadata: &ast.WorkflowMetadata{Name: fmt.Sprintf("workflow-%d", i)},
			}
			registry.Register(fmt.Sprintf("id-%d", i), workflow)
		}
		done <- true
	}()
	
	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			registry.List()
			registry.Count()
			registry.Get(fmt.Sprintf("id-%d", i%10))
		}
		done <- true
	}()
	
	// Wait for both goroutines to complete
	<-done
	<-done
	
	assert.Equal(t, 100, registry.Count())
}