package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lacquer/lacquer/internal/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultStateConfig(t *testing.T) {
	config := DefaultStateConfig()

	assert.True(t, config.PersistenceEnabled)
	assert.Equal(t, "file", config.StorageBackend)
	assert.Equal(t, ".lacquer/state", config.StoragePath)
	assert.Equal(t, 5*time.Minute, config.SnapshotInterval)
	assert.Equal(t, 10, config.MaxSnapshots)
	assert.False(t, config.CompressSnapshots)
}

func TestNewStateManager(t *testing.T) {
	// Test with default config
	manager, err := NewStateManager(nil)
	assert.NoError(t, err)
	assert.NotNil(t, manager)
	defer manager.Close()

	// Test with memory backend
	config := &StateConfig{
		PersistenceEnabled: true,
		StorageBackend:     "memory",
	}
	manager, err = NewStateManager(config)
	assert.NoError(t, err)
	assert.NotNil(t, manager)
	defer manager.Close()

	// Test with file backend
	tempDir := t.TempDir()
	config = &StateConfig{
		PersistenceEnabled: true,
		StorageBackend:     "file",
		StoragePath:        tempDir,
	}
	manager, err = NewStateManager(config)
	assert.NoError(t, err)
	assert.NotNil(t, manager)
	defer manager.Close()

	// Test with unsupported backend
	config = &StateConfig{
		StorageBackend: "unsupported",
	}
	_, err = NewStateManager(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported storage backend")
}

func TestStateManager_SaveAndLoadState(t *testing.T) {
	config := &StateConfig{
		PersistenceEnabled: true,
		StorageBackend:     "memory",
	}
	manager, err := NewStateManager(config)
	require.NoError(t, err)
	defer manager.Close()

	runID := "test-run-123"
	state := map[string]interface{}{
		"counter": 42,
		"name":    "test",
		"nested": map[string]interface{}{
			"value": "deep",
		},
	}

	// Save state
	err = manager.SaveState(runID, state)
	assert.NoError(t, err)

	// Load state
	loadedState, err := manager.LoadState(runID)
	assert.NoError(t, err)
	assert.Equal(t, 42, loadedState["counter"])
	assert.Equal(t, "test", loadedState["name"])
	assert.NotNil(t, loadedState["_last_saved"])

	// Verify nested values
	nested, ok := loadedState["nested"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "deep", nested["value"])
}

func TestStateManager_DeleteState(t *testing.T) {
	config := &StateConfig{
		PersistenceEnabled: true,
		StorageBackend:     "memory",
	}
	manager, err := NewStateManager(config)
	require.NoError(t, err)
	defer manager.Close()

	runID := "test-run-456"
	state := map[string]interface{}{
		"data": "value",
	}

	// Save state
	err = manager.SaveState(runID, state)
	assert.NoError(t, err)

	// Delete state
	err = manager.DeleteState(runID)
	assert.NoError(t, err)

	// Try to load deleted state
	loadedState, err := manager.LoadState(runID)
	assert.NoError(t, err)
	assert.Empty(t, loadedState)
}

func TestStateManager_Snapshots(t *testing.T) {
	config := &StateConfig{
		PersistenceEnabled: true,
		StorageBackend:     "memory",
		MaxSnapshots:       3,
	}
	manager, err := NewStateManager(config)
	require.NoError(t, err)
	defer manager.Close()

	runID := "test-run-789"

	// Create multiple snapshots
	for i := 0; i < 5; i++ {
		state := map[string]interface{}{
			"step":     i,
			"progress": float64(i) / 5.0,
		}

		snapshot, err := manager.CreateSnapshot(runID, i, fmt.Sprintf("step-%d", i), state)
		assert.NoError(t, err)
		assert.NotNil(t, snapshot)
		assert.Equal(t, runID, snapshot.RunID)
		assert.Equal(t, i, snapshot.StepIndex)

		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List snapshots
	snapshots, err := manager.ListSnapshots(runID)
	assert.NoError(t, err)
	assert.NotEmpty(t, snapshots)

	// Restore from snapshot
	if len(snapshots) > 0 {
		restoredState, err := manager.RestoreSnapshot(runID, snapshots[0].ID)
		assert.NoError(t, err)
		assert.NotNil(t, restoredState)

		// Verify it's a copy (modifications don't affect original)
		restoredState["modified"] = true
		originalSnapshot, _ := manager.store.LoadSnapshot(runID, snapshots[0].ID)
		_, hasModified := originalSnapshot.State["modified"]
		assert.False(t, hasModified)
	}
}

func TestStateManager_PersistenceDisabled(t *testing.T) {
	config := &StateConfig{
		PersistenceEnabled: false,
		StorageBackend:     "memory",
	}
	manager, err := NewStateManager(config)
	require.NoError(t, err)
	defer manager.Close()

	runID := "test-run-disabled"
	state := map[string]interface{}{
		"data": "value",
	}

	// Save should be no-op
	err = manager.SaveState(runID, state)
	assert.NoError(t, err)

	// Load should return empty
	loadedState, err := manager.LoadState(runID)
	assert.NoError(t, err)
	assert.Empty(t, loadedState)

	// Snapshot should return nil
	snapshot, err := manager.CreateSnapshot(runID, 0, "step-0", state)
	assert.NoError(t, err)
	assert.Nil(t, snapshot)

	// Restore should fail
	_, err = manager.RestoreSnapshot(runID, "any-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "persistence is disabled")
}

func TestMemoryStateStore(t *testing.T) {
	store := NewMemoryStateStore()

	runID := "mem-test-123"
	state := map[string]interface{}{
		"key": "value",
	}

	// Test Set and Get
	err := store.Set(runID, state)
	assert.NoError(t, err)

	loaded, err := store.Get(runID)
	assert.NoError(t, err)
	assert.Equal(t, "value", loaded["key"])

	// Test List
	runIDs, err := store.List()
	assert.NoError(t, err)
	assert.Contains(t, runIDs, runID)

	// Test Delete
	err = store.Delete(runID)
	assert.NoError(t, err)

	loaded, err = store.Get(runID)
	assert.NoError(t, err)
	assert.Nil(t, loaded)

	// Test snapshots
	snapshot := &StateSnapshot{
		ID:        "snap-1",
		RunID:     runID,
		Timestamp: time.Now(),
		StepIndex: 0,
		StepID:    "step-0",
		State:     state,
	}

	err = store.SaveSnapshot(runID, snapshot)
	assert.NoError(t, err)

	loadedSnap, err := store.LoadSnapshot(runID, "snap-1")
	assert.NoError(t, err)
	assert.Equal(t, snapshot.ID, loadedSnap.ID)

	snapshots, err := store.ListSnapshots(runID)
	assert.NoError(t, err)
	assert.Len(t, snapshots, 1)

	// Test Close
	err = store.Close()
	assert.NoError(t, err)
}

func TestFileStateStore(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewFileStateStore(tempDir)
	require.NoError(t, err)
	defer store.Close()

	runID := "file-test-123"
	state := map[string]interface{}{
		"key":    "value",
		"number": 42,
		"nested": map[string]interface{}{
			"inner": true,
		},
	}

	// Test Set and Get
	err = store.Set(runID, state)
	assert.NoError(t, err)

	// Verify file was created
	statePath := filepath.Join(tempDir, runID+".json")
	assert.FileExists(t, statePath)

	loaded, err := store.Get(runID)
	assert.NoError(t, err)
	assert.Equal(t, "value", loaded["key"])
	assert.Equal(t, float64(42), loaded["number"]) // JSON numbers decode as float64

	// Test List
	runIDs, err := store.List()
	assert.NoError(t, err)
	assert.Contains(t, runIDs, runID)

	// Test snapshots
	snapshot := &StateSnapshot{
		ID:        "snap-1",
		RunID:     runID,
		Timestamp: time.Now(),
		StepIndex: 0,
		StepID:    "step-0",
		State:     state,
	}

	err = store.SaveSnapshot(runID, snapshot)
	assert.NoError(t, err)

	// Verify snapshot file was created
	snapshotPath := filepath.Join(tempDir, "snapshots", runID, "snap-1.json")
	assert.FileExists(t, snapshotPath)

	loadedSnap, err := store.LoadSnapshot(runID, "snap-1")
	assert.NoError(t, err)
	assert.Equal(t, snapshot.ID, loadedSnap.ID)

	snapshots, err := store.ListSnapshots(runID)
	assert.NoError(t, err)
	assert.Len(t, snapshots, 1)

	// Test Delete
	err = store.Delete(runID)
	assert.NoError(t, err)
	assert.NoFileExists(t, statePath)
	assert.NoDirExists(t, filepath.Join(tempDir, "snapshots", runID))

	loaded, err = store.Get(runID)
	assert.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestFileStateStore_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewFileStateStore(tempDir)
	require.NoError(t, err)
	defer store.Close()

	// Test concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			runID := fmt.Sprintf("concurrent-%d", id)
			state := map[string]interface{}{
				"id": id,
			}
			err := store.Set(runID, state)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all writes succeeded
	runIDs, err := store.List()
	assert.NoError(t, err)
	assert.Len(t, runIDs, 10)
}

func TestStateOperations(t *testing.T) {
	config := &StateConfig{
		PersistenceEnabled: true,
		StorageBackend:     "memory",
	}
	manager, err := NewStateManager(config)
	require.NoError(t, err)
	defer manager.Close()

	ops := NewStateOperations(manager)
	runID := "ops-test-123"

	// Test Merge
	initialState := map[string]interface{}{
		"a": 1,
		"b": 2,
	}
	err = manager.SaveState(runID, initialState)
	assert.NoError(t, err)

	updates := map[string]interface{}{
		"b": 3,
		"c": 4,
	}
	err = ops.Merge(runID, updates)
	assert.NoError(t, err)

	state, err := manager.LoadState(runID)
	assert.NoError(t, err)
	assert.Equal(t, 1, state["a"])
	assert.Equal(t, 3, state["b"])
	assert.Equal(t, 4, state["c"])

	// Test Transform
	err = ops.Transform(runID, func(s map[string]interface{}) error {
		if val, ok := s["a"].(int); ok {
			s["a"] = val * 10
		}
		return nil
	})
	assert.NoError(t, err)

	state, err = manager.LoadState(runID)
	assert.NoError(t, err)
	assert.Equal(t, 10, state["a"])

	// Test Query
	result, err := ops.Query(runID, func(s map[string]interface{}) interface{} {
		return s["b"]
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, result)

	// Test Export
	var buf bytes.Buffer
	err = ops.Export(runID, &buf)
	assert.NoError(t, err)

	var exported map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &exported)
	assert.NoError(t, err)
	assert.Equal(t, float64(10), exported["a"]) // JSON numbers decode as float64

	// Test Import
	importData := `{"x": 100, "y": "imported"}`
	err = ops.Import("new-run", strings.NewReader(importData))
	assert.NoError(t, err)

	imported, err := manager.LoadState("new-run")
	assert.NoError(t, err)
	assert.Equal(t, float64(100), imported["x"])
	assert.Equal(t, "imported", imported["y"])
}

func TestGenerateSnapshotID(t *testing.T) {
	id1 := generateSnapshotID()
	time.Sleep(1 * time.Nanosecond)
	id2 := generateSnapshotID()

	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "snapshot-")
	assert.Contains(t, id2, "snapshot-")
}

func TestStateSnapshot_Structure(t *testing.T) {
	snapshot := &StateSnapshot{
		ID:        "test-snap",
		RunID:     "test-run",
		Timestamp: time.Now(),
		StepIndex: 5,
		StepID:    "step-5",
		State: map[string]interface{}{
			"progress": 0.5,
		},
		Metadata: map[string]interface{}{
			"created_by": "test",
		},
	}

	assert.Equal(t, "test-snap", snapshot.ID)
	assert.Equal(t, "test-run", snapshot.RunID)
	assert.Equal(t, 5, snapshot.StepIndex)
	assert.Equal(t, "step-5", snapshot.StepID)
	assert.Equal(t, 0.5, snapshot.State["progress"])
	assert.Equal(t, "test", snapshot.Metadata["created_by"])
}

func TestStateConfig_Structure(t *testing.T) {
	config := &StateConfig{
		PersistenceEnabled: true,
		StorageBackend:     "file",
		StoragePath:        "/tmp/state",
		SnapshotInterval:   10 * time.Minute,
		MaxSnapshots:       20,
		CompressSnapshots:  true,
	}

	assert.True(t, config.PersistenceEnabled)
	assert.Equal(t, "file", config.StorageBackend)
	assert.Equal(t, "/tmp/state", config.StoragePath)
	assert.Equal(t, 10*time.Minute, config.SnapshotInterval)
	assert.Equal(t, 20, config.MaxSnapshots)
	assert.True(t, config.CompressSnapshots)
}

// Integration test with ExecutionContext
func TestStateManager_IntegrationWithExecutionContext(t *testing.T) {
	config := &StateConfig{
		PersistenceEnabled: true,
		StorageBackend:     "memory",
	}
	manager, err := NewStateManager(config)
	require.NoError(t, err)
	defer manager.Close()

	// Create a simple workflow
	workflow := &ast.Workflow{
		Version: "1.0",
		Workflow: &ast.WorkflowDef{
			State: map[string]interface{}{
				"initial": "value",
			},
		},
	}

	// Create execution context
	ctx := context.Background()
	execCtx := NewExecutionContext(ctx, workflow, nil)

	// Update state in execution context
	execCtx.SetState("counter", 1)
	execCtx.SetState("status", "running")

	// Save execution context state
	err = manager.SaveState(execCtx.RunID, execCtx.GetAllState())
	assert.NoError(t, err)

	// Create snapshot at step completion
	snapshot, err := manager.CreateSnapshot(execCtx.RunID, 0, "step-1", execCtx.GetAllState())
	assert.NoError(t, err)
	assert.NotNil(t, snapshot)

	// Simulate step failure - restore from snapshot
	execCtx.SetState("status", "failed")
	execCtx.SetState("error", "simulated error")

	// Restore state
	restoredState, err := manager.RestoreSnapshot(execCtx.RunID, snapshot.ID)
	assert.NoError(t, err)
	assert.Equal(t, "running", restoredState["status"])
	assert.Nil(t, restoredState["error"])
}

// Benchmark tests
func BenchmarkStateManager_SaveState(b *testing.B) {
	config := &StateConfig{
		PersistenceEnabled: true,
		StorageBackend:     "memory",
	}
	manager, _ := NewStateManager(config)
	defer manager.Close()

	state := map[string]interface{}{
		"counter": 0,
		"data":    "test data",
		"nested": map[string]interface{}{
			"value": "nested value",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state["counter"] = i
		manager.SaveState("bench-run", state)
	}
}

func BenchmarkFileStateStore_Set(b *testing.B) {
	tempDir := b.TempDir()
	store, _ := NewFileStateStore(tempDir)
	defer store.Close()

	state := map[string]interface{}{
		"counter": 0,
		"data":    "test data",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state["counter"] = i
		store.Set("bench-run", state)
	}
}
