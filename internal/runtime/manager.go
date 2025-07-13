package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/lacquerai/lacquer/internal/runtime/cache"
	"github.com/lacquerai/lacquer/internal/runtime/golang"
	"github.com/lacquerai/lacquer/internal/runtime/node"
	"github.com/lacquerai/lacquer/internal/runtime/python"
	"github.com/lacquerai/lacquer/internal/runtime/types"
	"github.com/lacquerai/lacquer/internal/runtime/utils"
)

// Manager manages multiple runtime types
type Manager struct {
	runtimes   map[string]types.Runtime
	cache      types.Cache
	downloader types.Downloader
	mu         sync.RWMutex
}

// NewManager creates a new runtime manager
func NewManager(cacheDir string) (*Manager, error) {
	cache, err := cache.NewFileCache(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("creating cache: %w", err)
	}

	downloader := utils.NewDefaultDownloader()

	m := &Manager{
		runtimes:   make(map[string]types.Runtime),
		cache:      cache,
		downloader: downloader,
	}

	// Register default runtimes
	m.Register(golang.New(cache, downloader))
	m.Register(node.New(cache, downloader))
	m.Register(python.New(cache, downloader))

	return m, nil
}

// Register adds a new runtime to the manager
func (m *Manager) Register(runtime types.Runtime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runtimes[runtime.Name()] = runtime
}

// Get downloads and installs a specific runtime version
func (m *Manager) Get(ctx context.Context, runtime, version string) (string, error) {
	r, err := m.getRuntime(runtime)
	if err != nil {
		return "", err
	}
	return r.Get(ctx, version)
}

// GetLatest downloads and installs the latest version of a runtime
func (m *Manager) GetLatest(ctx context.Context, runtime string) (string, error) {
	r, err := m.getRuntime(runtime)
	if err != nil {
		return "", err
	}
	return r.GetLatest(ctx)
}

// List returns available versions for a runtime
func (m *Manager) List(ctx context.Context, runtime string) ([]types.Version, error) {
	r, err := m.getRuntime(runtime)
	if err != nil {
		return nil, err
	}
	return r.List(ctx)
}

// ListRuntimes returns all registered runtime names
func (m *Manager) ListRuntimes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.runtimes))
	for name := range m.runtimes {
		names = append(names, name)
	}
	return names
}

// Clean removes all cached versions of a runtime
func (m *Manager) Clean(runtime string) error {
	if fc, ok := m.cache.(*cache.FileCache); ok {
		return fc.Clean(runtime)
	}
	return fmt.Errorf("cache does not support cleaning")
}

// CleanAll removes all cached runtimes
func (m *Manager) CleanAll() error {
	for _, runtime := range m.ListRuntimes() {
		if err := m.Clean(runtime); err != nil {
			return fmt.Errorf("cleaning %s: %w", runtime, err)
		}
	}
	return nil
}

func (m *Manager) getRuntime(name string) (types.Runtime, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	runtime, ok := m.runtimes[name]
	if !ok {
		return nil, fmt.Errorf("runtime %s not found", name)
	}
	return runtime, nil
}

// RuntimeInfo provides information about an installed runtime
type RuntimeInfo struct {
	Name    string
	Version string
	Path    string
}

// GetInstalled returns all installed runtime versions
func (m *Manager) GetInstalled() ([]RuntimeInfo, error) {
	var infos []RuntimeInfo

	for _, runtimeName := range m.ListRuntimes() {
		versions, err := m.List(context.Background(), runtimeName)
		if err != nil {
			continue
		}

		for _, v := range versions {
			if path, exists := m.cache.Get(runtimeName, v.Version); exists {
				infos = append(infos, RuntimeInfo{
					Name:    runtimeName,
					Version: v.Version,
					Path:    path,
				})
			}
		}
	}

	return infos, nil
}
