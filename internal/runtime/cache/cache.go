package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/runtime/types"
)

// FileCache implements a file-based cache for runtimes
type FileCache struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileCache creates a new file-based cache
func NewFileCache(baseDir string) (*FileCache, error) {
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home directory: %w", err)
		}
		baseDir = filepath.Join(home, ".lacquer", "runtimes")
	}

	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}

	return &FileCache{
		baseDir: baseDir,
	}, nil
}

// Get retrieves a cached runtime path
func (c *FileCache) Get(runtime, version string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	path := c.Path(runtime, version)
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	return "", false
}

// Set stores a runtime path in the cache
func (c *FileCache) Set(runtime, version, path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cachePath := c.Path(runtime, version)
	cacheDir := filepath.Dir(cachePath)

	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	// If the path is different from cache path, we need to move it
	if path != cachePath {
		if err := os.Rename(path, cachePath); err != nil {
			// If rename fails (e.g., across filesystems), copy instead
			if err := copyDir(path, cachePath); err != nil {
				return fmt.Errorf("moving to cache: %w", err)
			}
			_ = os.RemoveAll(path)
		}
	}

	return nil
}

func (c *FileCache) SetManifest(runtime string, manifest []types.Version) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cachePath := c.Path(runtime, "manifest.json")
	cacheDir := filepath.Dir(cachePath)

	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	json, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshalling manifest: %w", err)
	}

	return os.WriteFile(cachePath, json, 0600)
}

func (c *FileCache) GetManifest(runtime string) ([]types.Version, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cachePath := c.Path(runtime, "manifest.json")

	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	if time.Since(info.ModTime()) > 24*time.Hour {
		return nil, types.ErrManifestExpired
	}

	data, err := os.ReadFile(cachePath) // #nosec G304 - cachePath is controlled
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest []types.Version
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshalling manifest: %w", err)
	}

	return manifest, nil
}

// Path returns the cache path for a runtime
func (c *FileCache) Path(runtime, version string) string {
	return filepath.Join(c.baseDir, runtime, version)
}

// Clean removes all cached versions of a runtime
func (c *FileCache) Clean(runtime string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := filepath.Join(c.baseDir, runtime)
	return os.RemoveAll(path)
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath, info.Mode())
	})
}

// copyFile copies a single file
func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src) // #nosec G304 - src path is controlled
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode) // #nosec G304 - dst path is controlled
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
