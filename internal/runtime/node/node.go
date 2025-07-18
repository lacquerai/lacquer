// runtime/node/node.go

package node

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/lacquerai/lacquer/internal/runtime/types"
	"github.com/lacquerai/lacquer/internal/runtime/utils"
	"github.com/rs/zerolog/log"
)

const (
	nodeReleasesAPI  = "https://nodejs.org/dist/index.json"
	nodeDownloadBase = "https://nodejs.org/dist/"
)

// NodeRuntime implements the Runtime interface for Node.js
type NodeRuntime struct {
	cache      types.Cache
	downloader types.Downloader
	platform   types.Platform
}

// New creates a new Node.js runtime manager
func New(cache types.Cache, downloader types.Downloader) *NodeRuntime {
	return &NodeRuntime{
		cache:      cache,
		downloader: downloader,
		platform:   types.GetPlatform(),
	}
}

// Name returns the runtime name
func (n *NodeRuntime) Name() string {
	return "node"
}

// Get downloads and installs the specified Node.js version
func (n *NodeRuntime) Get(ctx context.Context, version string) (string, error) {
	// Normalize version (add 'v' prefix if not present)
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	// Check cache first
	if path, exists := n.cache.Get(n.Name(), version); exists {
		return path, nil
	}

	// Get download URL
	downloadURL, err := n.getDownloadURL(ctx, version)
	if err != nil {
		return "", fmt.Errorf("getting download URL: %w", err)
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "node-download-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download archive
	archivePath := filepath.Join(tempDir, filepath.Base(downloadURL))
	file, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("creating archive file: %w", err)
	}

	if err := n.downloader.Download(ctx, downloadURL, file); err != nil {
		file.Close()
		return "", fmt.Errorf("downloading Node.js: %w", err)
	}
	file.Close()

	// Extract archive
	extractor, err := utils.GetExtractor(archivePath)
	if err != nil {
		return "", err
	}

	extractDir := filepath.Join(tempDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", fmt.Errorf("creating extract dir: %w", err)
	}

	if err := extractor.Extract(archivePath, extractDir); err != nil {
		return "", fmt.Errorf("extracting archive: %w", err)
	}

	// Find Node.js root directory
	nodeRoot, err := findNodeRoot(extractDir)
	if err != nil {
		return "", fmt.Errorf("finding Node.js root: %w", err)
	}

	// Move to cache
	cachePath := n.cache.Path(n.Name(), version)
	if err := n.cache.Set(n.Name(), version, nodeRoot); err != nil {
		return "", fmt.Errorf("caching runtime: %w", err)
	}

	return cachePath, nil
}

// GetLatest downloads the latest LTS Node.js version
func (n *NodeRuntime) GetLatest(ctx context.Context) (string, error) {
	versions, err := n.List(ctx)
	if err != nil {
		return "", err
	}

	// Find latest LTS version
	for _, v := range versions {
		if v.Stable {
			return n.Get(ctx, v.Version)
		}
	}

	// If no LTS found, use latest version
	if len(versions) > 0 {
		return n.Get(ctx, versions[0].Version)
	}

	return "", fmt.Errorf("no versions found")
}

// List returns available Node.js versions
func (n *NodeRuntime) List(ctx context.Context) ([]types.Version, error) {
	cachedVersions, err := n.cache.GetManifest(n.Name())
	if err == nil && len(cachedVersions) > 0 {
		return cachedVersions, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", nodeReleasesAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching versions: %w", err)
	}
	defer resp.Body.Close()

	var releases []nodeRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	versions := make([]types.Version, 0, len(releases))
	platformKey := n.getPlatformKey()

	for _, release := range releases {
		// Skip versions without the required files
		if !n.hasRequiredFiles(release) {
			continue
		}

		v := types.Version{
			Version:      release.Version,
			Stable:       release.LTS != false, // LTS versions are considered stable
			ReleaseDate:  release.Date,
			DownloadURLs: make(map[string]string),
		}

		// Build download URL for platform
		filename := n.getFilename(release.Version)
		if filename != "" {
			v.DownloadURLs[platformKey] = nodeDownloadBase + release.Version + "/" + filename
		}

		versions = append(versions, v)
	}

	// Sort by version (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].Version, versions[j].Version) > 0
	})

	if err := n.cache.SetManifest(n.Name(), versions); err != nil {
		log.Debug().Err(err).Msg("failed to set manifest")
	}

	return versions, nil
}

func (n *NodeRuntime) getDownloadURL(ctx context.Context, version string) (string, error) {
	versions, err := n.List(ctx)
	if err != nil {
		return "", err
	}

	for _, v := range versions {
		if v.Version == version {
			if url, ok := v.DownloadURLs[n.getPlatformKey()]; ok {
				return url, nil
			}
			return "", fmt.Errorf("version %s not available for platform %s", version, n.getPlatformKey())
		}
	}

	cv, err := semver.NewVersion(version)
	if err != nil {
		return "", fmt.Errorf("invalid semver version: %w", err)
	}

	for _, v := range versions {
		sv, err := semver.NewVersion(v.Version)
		if err != nil {
			continue
		}

		if sv.Compare(cv) == 0 {
			return v.DownloadURLs[n.getPlatformKey()], nil
		}
	}

	return "", fmt.Errorf("version %s not found", version)
}

func (n *NodeRuntime) getPlatformKey() string {
	return n.platform.OS + "-" + n.platform.Arch
}

func (n *NodeRuntime) getFilename(version string) string {
	osMap := map[string]string{
		"darwin":  "darwin",
		"linux":   "linux",
		"windows": "win",
	}

	archMap := map[string]string{
		"amd64": "x64",
		"arm64": "arm64",
		"386":   "x86",
	}

	os, ok := osMap[n.platform.OS]
	if !ok {
		return ""
	}

	arch, ok := archMap[n.platform.Arch]
	if !ok {
		return ""
	}

	ext := ".tar.gz"
	if n.platform.OS == "windows" {
		ext = ".zip"
	}

	return fmt.Sprintf("node-%s-%s-%s%s", version, os, arch, ext)
}

func (n *NodeRuntime) hasRequiredFiles(release nodeRelease) bool {
	// Check if the release has files for the current platform
	filename := n.getFilename(release.Version)
	if filename == "" {
		return false
	}

	ext := ".tar.gz"
	if n.platform.OS == "windows" {
		ext = ".zip"
	}

	// Check in Files array
	for _, file := range release.Files {
		if strings.HasSuffix(file, ext) && strings.Contains(file, n.platform.OS) {
			return true
		}
	}

	// For newer releases, assume files are available
	// (the API doesn't always list all files)
	return true
}

// nodeRelease represents a Node.js release
type nodeRelease struct {
	Version string      `json:"version"`
	Date    string      `json:"date"`
	Files   []string    `json:"files"`
	LTS     interface{} `json:"lts"` // Can be false or string (LTS codename)
}

func findNodeRoot(dir string) (string, error) {
	// Look for node executable
	nodePaths := []string{
		filepath.Join(dir, "bin", "node"),
		filepath.Join(dir, "node.exe"),
	}

	// Check direct paths first
	for _, path := range nodePaths {
		if _, err := os.Stat(path); err == nil {
			if strings.HasSuffix(path, ".exe") {
				return dir, nil
			}
			return filepath.Dir(filepath.Dir(path)), nil
		}
	}

	// Check subdirectories (Node.js archives usually have a versioned directory)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "node-") {
			subDir := filepath.Join(dir, entry.Name())
			for _, path := range nodePaths {
				checkPath := filepath.Join(subDir, strings.TrimPrefix(path, dir+string(filepath.Separator)))
				if _, err := os.Stat(checkPath); err == nil {
					return subDir, nil
				}
			}
		}
	}

	return "", fmt.Errorf("Node.js executable not found")
}

func compareVersions(v1, v2 string) int {
	// Remove 'v' prefix
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int

		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &p1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &p2)
		}

		if p1 > p2 {
			return 1
		} else if p1 < p2 {
			return -1
		}
	}

	return 0
}
