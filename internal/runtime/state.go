package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// StateManager handles workflow state persistence and management
type StateManager struct {
	store       StateStore
	snapshots   map[string]*StateSnapshot
	snapshotMux sync.RWMutex
	config      *StateConfig
}

// StateConfig contains configuration for state management
type StateConfig struct {
	PersistenceEnabled bool          `yaml:"persistence_enabled"`
	StorageBackend     string        `yaml:"storage_backend"` // "memory", "file", "bolt"
	StoragePath        string        `yaml:"storage_path"`
	SnapshotInterval   time.Duration `yaml:"snapshot_interval"`
	MaxSnapshots       int           `yaml:"max_snapshots"`
	CompressSnapshots  bool          `yaml:"compress_snapshots"`
}

// StateStore defines the interface for state storage backends
type StateStore interface {
	// Get retrieves state data for a workflow run
	Get(runID string) (map[string]interface{}, error)

	// Set stores state data for a workflow run
	Set(runID string, state map[string]interface{}) error

	// Delete removes state data for a workflow run
	Delete(runID string) error

	// List returns all stored run IDs
	List() ([]string, error)

	// SaveSnapshot stores a state snapshot
	SaveSnapshot(runID string, snapshot *StateSnapshot) error

	// LoadSnapshot retrieves a state snapshot
	LoadSnapshot(runID string, snapshotID string) (*StateSnapshot, error)

	// ListSnapshots returns all snapshots for a run
	ListSnapshots(runID string) ([]*StateSnapshot, error)

	// Close closes the state store
	Close() error
}

// StateSnapshot represents a point-in-time state capture
type StateSnapshot struct {
	ID        string                 `json:"id"`
	RunID     string                 `json:"run_id"`
	Timestamp time.Time              `json:"timestamp"`
	StepIndex int                    `json:"step_index"`
	StepID    string                 `json:"step_id"`
	State     map[string]interface{} `json:"state"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// DefaultStateConfig returns default state management configuration
func DefaultStateConfig() *StateConfig {
	return &StateConfig{
		PersistenceEnabled: true,
		StorageBackend:     "file",
		StoragePath:        ".lacquer/state",
		SnapshotInterval:   5 * time.Minute,
		MaxSnapshots:       10,
		CompressSnapshots:  false,
	}
}

// NewStateManager creates a new state manager
func NewStateManager(config *StateConfig) (*StateManager, error) {
	if config == nil {
		config = DefaultStateConfig()
	}

	// Create appropriate state store based on backend
	var store StateStore
	var err error

	switch config.StorageBackend {
	case "memory":
		store = NewMemoryStateStore()
	case "file":
		store, err = NewFileStateStore(config.StoragePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file state store: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", config.StorageBackend)
	}

	manager := &StateManager{
		store:     store,
		snapshots: make(map[string]*StateSnapshot),
		config:    config,
	}

	log.Info().
		Bool("persistence_enabled", config.PersistenceEnabled).
		Str("storage_backend", config.StorageBackend).
		Str("storage_path", config.StoragePath).
		Msg("State manager initialized")

	return manager, nil
}

// SaveState persists the current workflow state
func (sm *StateManager) SaveState(runID string, state map[string]interface{}) error {
	if !sm.config.PersistenceEnabled {
		return nil
	}

	// Create a copy to avoid external modifications
	stateCopy := CopyMap(state)

	// Add timestamp
	stateCopy["_last_saved"] = time.Now().UTC().Format(time.RFC3339)

	// Save to store
	if err := sm.store.Set(runID, stateCopy); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	log.Debug().
		Str("run_id", runID).
		Int("state_size", len(stateCopy)).
		Msg("State saved")

	return nil
}

// LoadState retrieves the persisted workflow state
func (sm *StateManager) LoadState(runID string) (map[string]interface{}, error) {
	if !sm.config.PersistenceEnabled {
		return make(map[string]interface{}), nil
	}

	state, err := sm.store.Get(runID)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	if state == nil {
		return make(map[string]interface{}), nil
	}

	log.Debug().
		Str("run_id", runID).
		Int("state_size", len(state)).
		Msg("State loaded")

	return state, nil
}

// DeleteState removes the persisted workflow state
func (sm *StateManager) DeleteState(runID string) error {
	if !sm.config.PersistenceEnabled {
		return nil
	}

	if err := sm.store.Delete(runID); err != nil {
		return fmt.Errorf("failed to delete state: %w", err)
	}

	// Clean up in-memory snapshots
	sm.snapshotMux.Lock()
	delete(sm.snapshots, runID)
	sm.snapshotMux.Unlock()

	log.Debug().
		Str("run_id", runID).
		Msg("State deleted")

	return nil
}

// CreateSnapshot creates a point-in-time snapshot of the workflow state
func (sm *StateManager) CreateSnapshot(runID string, stepIndex int, stepID string, state map[string]interface{}) (*StateSnapshot, error) {
	if !sm.config.PersistenceEnabled {
		return nil, nil
	}

	snapshot := &StateSnapshot{
		ID:        generateSnapshotID(),
		RunID:     runID,
		Timestamp: time.Now().UTC(),
		StepIndex: stepIndex,
		StepID:    stepID,
		State:     CopyMap(state),
		Metadata: map[string]interface{}{
			"created_by": "state_manager",
		},
	}

	// Save snapshot to store
	if err := sm.store.SaveSnapshot(runID, snapshot); err != nil {
		return nil, fmt.Errorf("failed to save snapshot: %w", err)
	}

	// Cache in memory
	sm.snapshotMux.Lock()
	sm.snapshots[snapshot.ID] = snapshot
	sm.snapshotMux.Unlock()

	// Clean up old snapshots if needed
	if err := sm.cleanupOldSnapshots(runID); err != nil {
		log.Warn().Err(err).Msg("Failed to cleanup old snapshots")
	}

	log.Debug().
		Str("run_id", runID).
		Str("snapshot_id", snapshot.ID).
		Str("step_id", stepID).
		Int("step_index", stepIndex).
		Msg("Snapshot created")

	return snapshot, nil
}

// RestoreSnapshot restores workflow state from a snapshot
func (sm *StateManager) RestoreSnapshot(runID string, snapshotID string) (map[string]interface{}, error) {
	if !sm.config.PersistenceEnabled {
		return nil, fmt.Errorf("persistence is disabled")
	}

	// Check memory cache first
	sm.snapshotMux.RLock()
	snapshot, exists := sm.snapshots[snapshotID]
	sm.snapshotMux.RUnlock()

	if !exists {
		// Load from store
		var err error
		snapshot, err = sm.store.LoadSnapshot(runID, snapshotID)
		if err != nil {
			return nil, fmt.Errorf("failed to load snapshot: %w", err)
		}
		if snapshot == nil {
			return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
		}

		// Cache it
		sm.snapshotMux.Lock()
		sm.snapshots[snapshotID] = snapshot
		sm.snapshotMux.Unlock()
	}

	// Return a copy of the state
	restoredState := CopyMap(snapshot.State)

	log.Info().
		Str("run_id", runID).
		Str("snapshot_id", snapshotID).
		Str("step_id", snapshot.StepID).
		Int("step_index", snapshot.StepIndex).
		Msg("State restored from snapshot")

	return restoredState, nil
}

// ListSnapshots returns all snapshots for a workflow run
func (sm *StateManager) ListSnapshots(runID string) ([]*StateSnapshot, error) {
	if !sm.config.PersistenceEnabled {
		return []*StateSnapshot{}, nil
	}

	snapshots, err := sm.store.ListSnapshots(runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	return snapshots, nil
}

// cleanupOldSnapshots removes old snapshots beyond the configured limit
func (sm *StateManager) cleanupOldSnapshots(runID string) error {
	if sm.config.MaxSnapshots <= 0 {
		return nil
	}

	snapshots, err := sm.store.ListSnapshots(runID)
	if err != nil {
		return err
	}

	if len(snapshots) <= sm.config.MaxSnapshots {
		return nil
	}

	// Sort by timestamp (oldest first)
	// In production, we'd use a proper sorting algorithm
	// For now, we'll just remove the oldest ones
	toRemove := len(snapshots) - sm.config.MaxSnapshots

	for i := 0; i < toRemove && i < len(snapshots); i++ {
		// Remove from memory cache
		sm.snapshotMux.Lock()
		delete(sm.snapshots, snapshots[i].ID)
		sm.snapshotMux.Unlock()

		log.Debug().
			Str("snapshot_id", snapshots[i].ID).
			Msg("Removing old snapshot")
	}

	return nil
}

// Close closes the state manager and its underlying store
func (sm *StateManager) Close() error {
	log.Info().Msg("Closing state manager")
	return sm.store.Close()
}

// MemoryStateStore implements an in-memory state store
type MemoryStateStore struct {
	states    map[string]map[string]interface{}
	snapshots map[string]map[string]*StateSnapshot
	mu        sync.RWMutex
}

// NewMemoryStateStore creates a new in-memory state store
func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{
		states:    make(map[string]map[string]interface{}),
		snapshots: make(map[string]map[string]*StateSnapshot),
	}
}

// Get retrieves state from memory
func (m *MemoryStateStore) Get(runID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[runID]
	if !exists {
		return nil, nil
	}

	// Return a copy
	return CopyMap(state), nil
}

// Set stores state in memory
func (m *MemoryStateStore) Set(runID string, state map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.states[runID] = CopyMap(state)
	return nil
}

// Delete removes state from memory
func (m *MemoryStateStore) Delete(runID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.states, runID)
	delete(m.snapshots, runID)
	return nil
}

// List returns all stored run IDs
func (m *MemoryStateStore) List() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runIDs := make([]string, 0, len(m.states))
	for runID := range m.states {
		runIDs = append(runIDs, runID)
	}
	return runIDs, nil
}

// SaveSnapshot stores a snapshot in memory
func (m *MemoryStateStore) SaveSnapshot(runID string, snapshot *StateSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.snapshots[runID] == nil {
		m.snapshots[runID] = make(map[string]*StateSnapshot)
	}

	m.snapshots[runID][snapshot.ID] = snapshot
	return nil
}

// LoadSnapshot retrieves a snapshot from memory
func (m *MemoryStateStore) LoadSnapshot(runID string, snapshotID string) (*StateSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runSnapshots, exists := m.snapshots[runID]
	if !exists {
		return nil, nil
	}

	snapshot, exists := runSnapshots[snapshotID]
	if !exists {
		return nil, nil
	}

	return snapshot, nil
}

// ListSnapshots returns all snapshots for a run
func (m *MemoryStateStore) ListSnapshots(runID string) ([]*StateSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runSnapshots, exists := m.snapshots[runID]
	if !exists {
		return []*StateSnapshot{}, nil
	}

	snapshots := make([]*StateSnapshot, 0, len(runSnapshots))
	for _, snapshot := range runSnapshots {
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// Close is a no-op for memory store
func (m *MemoryStateStore) Close() error {
	return nil
}

// FileStateStore implements a file-based state store
type FileStateStore struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStateStore creates a new file-based state store
func NewFileStateStore(basePath string) (*FileStateStore, error) {
	// Ensure base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	return &FileStateStore{
		basePath: basePath,
	}, nil
}

// Get retrieves state from file
func (f *FileStateStore) Get(runID string) (map[string]interface{}, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	statePath := f.getStatePath(runID)

	file, err := os.Open(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open state file: %w", err)
	}
	defer file.Close()

	var state map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to decode state: %w", err)
	}

	return state, nil
}

// Set stores state to file
func (f *FileStateStore) Set(runID string, state map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	statePath := f.getStatePath(runID)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Write to temporary file first
	tmpPath := statePath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create state file: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(state); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to encode state: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close state file: %w", err)
	}

	// Atomically rename
	if err := os.Rename(tmpPath, statePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save state file: %w", err)
	}

	return nil
}

// Delete removes state file
func (f *FileStateStore) Delete(runID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Remove state file
	statePath := f.getStatePath(runID)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete state file: %w", err)
	}

	// Remove snapshots directory
	snapshotsDir := f.getSnapshotsDir(runID)
	if err := os.RemoveAll(snapshotsDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete snapshots: %w", err)
	}

	return nil
}

// List returns all stored run IDs
func (f *FileStateStore) List() ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	entries, err := os.ReadDir(f.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list state directory: %w", err)
	}

	var runIDs []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			runID := strings.TrimSuffix(entry.Name(), ".json")
			runIDs = append(runIDs, runID)
		}
	}

	return runIDs, nil
}

// SaveSnapshot stores a snapshot to file
func (f *FileStateStore) SaveSnapshot(runID string, snapshot *StateSnapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	snapshotPath := f.getSnapshotPath(runID, snapshot.ID)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Write to temporary file first
	tmpPath := snapshotPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close snapshot file: %w", err)
	}

	// Atomically rename
	if err := os.Rename(tmpPath, snapshotPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save snapshot file: %w", err)
	}

	return nil
}

// LoadSnapshot retrieves a snapshot from file
func (f *FileStateStore) LoadSnapshot(runID string, snapshotID string) (*StateSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	snapshotPath := f.getSnapshotPath(runID, snapshotID)

	file, err := os.Open(snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open snapshot file: %w", err)
	}
	defer file.Close()

	var snapshot StateSnapshot
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode snapshot: %w", err)
	}

	return &snapshot, nil
}

// ListSnapshots returns all snapshots for a run
func (f *FileStateStore) ListSnapshots(runID string) ([]*StateSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	snapshotsDir := f.getSnapshotsDir(runID)

	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*StateSnapshot{}, nil
		}
		return nil, fmt.Errorf("failed to list snapshots directory: %w", err)
	}

	var snapshots []*StateSnapshot
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			snapshotID := strings.TrimSuffix(entry.Name(), ".json")
			snapshot, err := f.LoadSnapshot(runID, snapshotID)
			if err != nil {
				log.Warn().Err(err).Str("snapshot_id", snapshotID).Msg("Failed to load snapshot")
				continue
			}
			if snapshot != nil {
				snapshots = append(snapshots, snapshot)
			}
		}
	}

	return snapshots, nil
}

// Close is a no-op for file store
func (f *FileStateStore) Close() error {
	return nil
}

// Helper methods for file paths
func (f *FileStateStore) getStatePath(runID string) string {
	return filepath.Join(f.basePath, runID+".json")
}

func (f *FileStateStore) getSnapshotsDir(runID string) string {
	return filepath.Join(f.basePath, "snapshots", runID)
}

func (f *FileStateStore) getSnapshotPath(runID, snapshotID string) string {
	return filepath.Join(f.getSnapshotsDir(runID), snapshotID+".json")
}

// generateSnapshotID generates a unique snapshot ID
func generateSnapshotID() string {
	return fmt.Sprintf("snapshot-%d", time.Now().UnixNano())
}

// StateOperations provides advanced state manipulation functions
type StateOperations struct {
	manager *StateManager
}

// NewStateOperations creates a new state operations helper
func NewStateOperations(manager *StateManager) *StateOperations {
	return &StateOperations{
		manager: manager,
	}
}

// Merge merges new state values into existing state
func (so *StateOperations) Merge(runID string, updates map[string]interface{}) error {
	state, err := so.manager.LoadState(runID)
	if err != nil {
		return err
	}

	// Merge updates
	MergeMap(state, updates)

	// Save merged state
	return so.manager.SaveState(runID, state)
}

// Transform applies a transformation function to the state
func (so *StateOperations) Transform(runID string, transformer func(map[string]interface{}) error) error {
	state, err := so.manager.LoadState(runID)
	if err != nil {
		return err
	}

	// Apply transformation
	if err := transformer(state); err != nil {
		return fmt.Errorf("state transformation failed: %w", err)
	}

	// Save transformed state
	return so.manager.SaveState(runID, state)
}

// Query provides read-only access to state with a query function
func (so *StateOperations) Query(runID string, query func(map[string]interface{}) interface{}) (interface{}, error) {
	state, err := so.manager.LoadState(runID)
	if err != nil {
		return nil, err
	}

	return query(state), nil
}

// Export exports state to a writer in JSON format
func (so *StateOperations) Export(runID string, w io.Writer) error {
	state, err := so.manager.LoadState(runID)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(state)
}

// Import imports state from a reader in JSON format
func (so *StateOperations) Import(runID string, r io.Reader) error {
	var state map[string]interface{}
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&state); err != nil {
		return fmt.Errorf("failed to decode state: %w", err)
	}

	return so.manager.SaveState(runID, state)
}
