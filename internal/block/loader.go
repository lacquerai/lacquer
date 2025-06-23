package block

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const blockConfigFile = "block.laq.yaml"

// FileLoader loads blocks from the filesystem
type FileLoader struct {
	cache   map[string]*cacheEntry
	cacheMu sync.RWMutex
}

type cacheEntry struct {
	block   *Block
	modTime time.Time
}

// NewFileLoader creates a new file-based block loader
func NewFileLoader() *FileLoader {
	return &FileLoader{
		cache: make(map[string]*cacheEntry),
	}
}

// Load loads a block from the given path
func (l *FileLoader) Load(ctx context.Context, path string) (*Block, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve block path: %w", err)
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("block directory not found: %s", absPath)
		}
		return nil, fmt.Errorf("failed to access block directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("block path is not a directory: %s", absPath)
	}

	// Check cache
	configPath := filepath.Join(absPath, blockConfigFile)
	if cached, ok := l.getFromCache(configPath); ok {
		return cached, nil
	}

	// Load block configuration
	configData, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("block.laq.yaml not found in %s", absPath)
		}
		return nil, fmt.Errorf("failed to read block.laq.yaml: %w", err)
	}

	// Parse block metadata
	var block Block
	if err := yaml.Unmarshal(configData, &block); err != nil {
		return nil, fmt.Errorf("failed to parse block.laq.yaml: %w", err)
	}

	// Set defaults and validate
	block.Path = absPath
	if block.Runtime == "" {
		block.Runtime = RuntimeNative
	}

	// Validate runtime type
	switch block.Runtime {
	case RuntimeNative, RuntimeGo, RuntimeDocker:
		// Valid runtime types
	default:
		return nil, fmt.Errorf("unsupported runtime type: %s", block.Runtime)
	}

	// Validate required fields
	if block.Name == "" {
		return nil, fmt.Errorf("block name is required in block.laq.yaml")
	}

	// Runtime-specific validation
	if err := l.validateBlock(&block); err != nil {
		return nil, err
	}

	// Get file mod time for caching
	fileInfo, err := os.Stat(configPath)
	if err == nil {
		block.ModTime = fileInfo.ModTime()
	}

	// Update cache
	l.updateCache(configPath, &block)

	return &block, nil
}

// GetFromCache returns a cached block if available
func (l *FileLoader) GetFromCache(path string) (*Block, bool) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, false
	}
	configPath := filepath.Join(absPath, blockConfigFile)
	return l.getFromCache(configPath)
}

// InvalidateCache removes a block from the cache
func (l *FileLoader) InvalidateCache(path string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}
	configPath := filepath.Join(absPath, blockConfigFile)
	
	l.cacheMu.Lock()
	delete(l.cache, configPath)
	l.cacheMu.Unlock()
}

func (l *FileLoader) getFromCache(configPath string) (*Block, bool) {
	l.cacheMu.RLock()
	entry, ok := l.cache[configPath]
	l.cacheMu.RUnlock()

	if !ok {
		return nil, false
	}

	// Check if file has been modified
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		return nil, false
	}

	if fileInfo.ModTime().After(entry.modTime) {
		// File has been modified, invalidate cache
		l.cacheMu.Lock()
		delete(l.cache, configPath)
		l.cacheMu.Unlock()
		return nil, false
	}

	return entry.block, true
}

func (l *FileLoader) updateCache(configPath string, block *Block) {
	l.cacheMu.Lock()
	l.cache[configPath] = &cacheEntry{
		block:   block,
		modTime: block.ModTime,
	}
	l.cacheMu.Unlock()
}

func (l *FileLoader) validateBlock(block *Block) error {
	switch block.Runtime {
	case RuntimeNative:
		if block.Workflow == nil {
			return fmt.Errorf("native block requires 'workflow' field")
		}
	case RuntimeGo:
		if block.Script == "" {
			return fmt.Errorf("go block requires 'script' field")
		}
	case RuntimeDocker:
		if block.Image == "" {
			return fmt.Errorf("docker block requires 'image' field")
		}
	}

	// Validate input schemas
	for name, input := range block.Inputs {
		if err := validateSchema(name, input.Type, "input"); err != nil {
			return err
		}
	}

	// Validate output schemas
	for name, output := range block.Outputs {
		if err := validateSchema(name, output.Type, "output"); err != nil {
			return err
		}
	}

	return nil
}

func validateSchema(name, schemaType, kind string) error {
	if schemaType == "" {
		return fmt.Errorf("%s '%s' requires a type", kind, name)
	}

	validTypes := map[string]bool{
		"string":  true,
		"number":  true,
		"integer": true,
		"boolean": true,
		"array":   true,
		"object":  true,
	}

	if !validTypes[schemaType] {
		return fmt.Errorf("%s '%s' has invalid type: %s", kind, name, schemaType)
	}

	return nil
}