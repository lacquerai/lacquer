package provider

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lacquerai/lacquer/internal/utils"
	"github.com/rs/zerolog/log"
)

// ModelCache handles caching of model lists from providers
type ModelCache struct {
	cacheDir string
	mu       *sync.RWMutex
	disable  bool
}

// CachedModels represents cached model data
type CachedModels struct {
	Models    []Info    `json:"models"`
	Provider  string    `json:"provider"`
	CachedAt  time.Time `json:"cached_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewModelCache creates a new model cache
func NewModelCache(disable bool) *ModelCache {
	cacheDir := filepath.Join(utils.LacquerCacheDir, "models")
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		log.Warn().Err(err).Str("dir", cacheDir).Msg("Could not create cache directory")
	}

	if flag.Lookup("test.v") != nil {
		disable = true
	}

	return &ModelCache{
		cacheDir: cacheDir,
		mu:       &sync.RWMutex{},
		disable:  disable,
	}
}

// GetModels retrieves cached models or fetches them from the provider
func (mc *ModelCache) GetModels(ctx context.Context, provider Provider) ([]Info, error) {
	mc.mu.RLock()
	cached := mc.loadFromCache(provider.GetName())
	mc.mu.RUnlock()

	// Check if cache is valid and not expired
	if cached != nil && time.Now().Before(cached.ExpiresAt) && !mc.disable {
		log.Debug().
			Str("provider", provider.GetName()).
			Time("expires_at", cached.ExpiresAt).
			Int("models_count", len(cached.Models)).
			Msg("Using cached models")
		return cached.Models, nil
	}

	// Cache is expired or doesn't exist, fetch fresh data
	log.Debug().
		Str("provider", provider.GetName()).
		Msg("Fetching models from provider API")

	models, err := provider.ListModels(ctx)
	if err != nil {
		// If we have expired cache, return it as fallback
		if cached != nil {
			log.Warn().
				Err(err).
				Str("provider", provider.GetName()).
				Msg("Failed to fetch models, using expired cache")
			return cached.Models, nil
		}

		return nil, err
	}

	// Cache the new data
	mc.mu.Lock()
	mc.saveToCache(provider.GetName(), models)
	mc.mu.Unlock()

	log.Debug().
		Str("provider", provider.GetName()).
		Int("models_count", len(models)).
		Msg("Successfully fetched and cached models")

	return models, nil
}

// InvalidateCache removes cached data for a provider
func (mc *ModelCache) InvalidateCache(providerName string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	cacheFile := mc.getCacheFilePath(providerName)
	if err := os.Remove(cacheFile); err != nil && !os.IsNotExist(err) {
		log.Warn().
			Err(err).
			Str("provider", providerName).
			Str("file", cacheFile).
			Msg("Failed to remove cache file")
	}
}

// loadFromCache loads cached models from disk
func (mc *ModelCache) loadFromCache(providerName string) *CachedModels {
	cacheFile := mc.getCacheFilePath(providerName)

	data, err := os.ReadFile(cacheFile) // #nosec G304 - cacheFile path is controlled
	if err != nil {
		if !os.IsNotExist(err) {
			log.Debug().
				Err(err).
				Str("provider", providerName).
				Str("file", cacheFile).
				Msg("Failed to read cache file")
		}
		return nil
	}

	var cached CachedModels
	if err := json.Unmarshal(data, &cached); err != nil {
		log.Warn().
			Err(err).
			Str("provider", providerName).
			Str("file", cacheFile).
			Msg("Failed to unmarshal cache file")
		return nil
	}

	return &cached
}

// saveToCache saves models to disk cache
func (mc *ModelCache) saveToCache(providerName string, models []Info) {
	cached := CachedModels{
		Models:    models,
		Provider:  providerName,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24-hour TTL
	}

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		log.Warn().
			Err(err).
			Str("provider", providerName).
			Msg("Failed to marshal cache data")
		return
	}

	cacheFile := mc.getCacheFilePath(providerName)
	if err := os.WriteFile(cacheFile, data, 0600); err != nil {
		log.Warn().
			Err(err).
			Str("provider", providerName).
			Str("file", cacheFile).
			Msg("Failed to write cache file")
	}
}

// getCacheFilePath returns the cache file path for a provider
func (mc *ModelCache) getCacheFilePath(providerName string) string {
	return filepath.Join(mc.cacheDir, fmt.Sprintf("%s_models.json", providerName))
}
