package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v1.0.0", "1.0.0"},
		{"1.0.0", "1.0.0"},
		{"v2.1.3", "2.1.3"},
		{"dev", "dev"},
	}

	for _, test := range tests {
		result := normalizeVersion(test.input)
		assert.Equal(t, test.expected, result)
	}
}

func TestUpdateCacheOperations(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "lacquer_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Temporarily change the home directory for testing
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tempDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Test data
	updateInfo := &UpdateInfo{
		LastChecked:   time.Now(),
		LatestVersion: "v1.2.3",
		CurrentIsOld:  true,
		DownloadURL:   "https://example.com/download",
	}

	// Test saving cache
	saveUpdateCache(updateInfo)

	// Verify cache file exists
	cacheFile := filepath.Join(tempDir, updateCacheFile)
	assert.FileExists(t, cacheFile)

	// Test loading cache
	loadedInfo := loadUpdateCache()
	require.NotNil(t, loadedInfo)
	assert.Equal(t, updateInfo.LatestVersion, loadedInfo.LatestVersion)
	assert.Equal(t, updateInfo.CurrentIsOld, loadedInfo.CurrentIsOld)
	assert.Equal(t, updateInfo.DownloadURL, loadedInfo.DownloadURL)
	assert.WithinDuration(t, updateInfo.LastChecked, loadedInfo.LastChecked, time.Second)
}

func TestUpdateCacheExpiry(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "lacquer_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Temporarily change the home directory for testing
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tempDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Create an expired cache entry
	expiredInfo := &UpdateInfo{
		LastChecked:   time.Now().Add(-3 * time.Hour), // 3 hours ago (expired)
		LatestVersion: "v1.0.0",
		CurrentIsOld:  false,
		DownloadURL:   "https://example.com/old",
	}

	saveUpdateCache(expiredInfo)

	// Test ShouldShowUpdateNotification with expired cache
	notification := ShouldShowUpdateNotification()
	assert.Nil(t, notification, "Should not show notification for expired cache")

	// Create a fresh cache entry
	freshInfo := &UpdateInfo{
		LastChecked:   time.Now().Add(-30 * time.Minute), // 30 minutes ago (fresh)
		LatestVersion: "v1.2.0",
		CurrentIsOld:  true,
		DownloadURL:   "https://example.com/new",
	}

	saveUpdateCache(freshInfo)

	// Test ShouldShowUpdateNotification with fresh cache
	notification = ShouldShowUpdateNotification()
	require.NotNil(t, notification)
	assert.Equal(t, "v1.2.0", notification.LatestVersion)
	assert.True(t, notification.CurrentIsOld)
}

func TestLoadUpdateCacheWithInvalidJSON(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "lacquer_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Temporarily change the home directory for testing
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tempDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Create .lacquer directory
	lacquerDir := filepath.Join(tempDir, ".lacquer")
	_ = os.MkdirAll(lacquerDir, 0755)

	// Write invalid JSON to cache file
	cacheFile := filepath.Join(tempDir, updateCacheFile)
	_ = os.WriteFile(cacheFile, []byte("invalid json"), 0644)

	// Test loading invalid cache
	loadedInfo := loadUpdateCache()
	assert.Nil(t, loadedInfo, "Should return nil for invalid JSON")
}

func TestLoadUpdateCacheWithNonexistentFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "lacquer_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Temporarily change the home directory for testing
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tempDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Test loading nonexistent cache
	loadedInfo := loadUpdateCache()
	assert.Nil(t, loadedInfo, "Should return nil for nonexistent cache file")
}

func TestSaveUpdateCacheCreatesDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "lacquer_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Temporarily change the home directory for testing
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tempDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Test data
	updateInfo := &UpdateInfo{
		LastChecked:   time.Now(),
		LatestVersion: "v1.0.0",
		CurrentIsOld:  false,
		DownloadURL:   "https://example.com/download",
	}

	// Save cache (should create directory)
	saveUpdateCache(updateInfo)

	// Verify .lacquer directory was created
	lacquerDir := filepath.Join(tempDir, ".lacquer")
	assert.DirExists(t, lacquerDir)

	// Verify cache file was created
	cacheFile := filepath.Join(tempDir, updateCacheFile)
	assert.FileExists(t, cacheFile)

	// Verify content is correct JSON
	data, err := os.ReadFile(cacheFile)
	require.NoError(t, err)

	var savedInfo UpdateInfo
	err = json.Unmarshal(data, &savedInfo)
	require.NoError(t, err)
	assert.Equal(t, updateInfo.LatestVersion, savedInfo.LatestVersion)
}
