package golang

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/lacquerai/lacquer/internal/runtime/types"
	"github.com/lacquerai/lacquer/internal/runtime/utils"
	"github.com/rs/zerolog/log"
)

const (
	goDownloadBaseURL = "https://go.dev/dl/"
	goVersionsAPI     = "https://go.dev/dl/?mode=json&include=all"
)

// GoRuntime implements the Runtime interface for Go
type GoRuntime struct {
	cache      types.Cache
	downloader types.Downloader
	platform   types.Platform
}

// New creates a new Go runtime manager
func New(cache types.Cache, downloader types.Downloader) *GoRuntime {
	return &GoRuntime{
		cache:      cache,
		downloader: downloader,
		platform:   types.GetPlatform(),
	}
}

// Name returns the runtime name
func (g *GoRuntime) Name() string {
	return "go"
}

// Get downloads and installs the specified Go version
func (g *GoRuntime) Get(ctx context.Context, version string) (string, error) {
	if !strings.HasPrefix(version, "go") {
		version = "go" + version
	}

	if path, exists := g.checkInstalled(ctx, version); exists {
		return path, nil
	}

	if path, exists := g.cache.Get(g.Name(), version); exists {
		return path, nil
	}

	// Get download URL
	downloadURL, err := g.getDownloadURL(ctx, version)
	if err != nil {
		return "", fmt.Errorf("getting download URL: %w", err)
	}

	// Create temporary directory for download
	tempDir, err := os.MkdirTemp("", "go-download-*")
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

	if err := g.downloader.Download(ctx, downloadURL, file); err != nil {
		file.Close()
		return "", fmt.Errorf("downloading Go: %w", err)
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

	// Move to cache
	goRoot := filepath.Join(extractDir, "go")
	cachePath := g.cache.Path(g.Name(), version)
	if err := g.cache.Set(g.Name(), version, goRoot); err != nil {
		return "", fmt.Errorf("caching runtime: %w", err)
	}

	return cachePath, nil
}

// GetLatest downloads the latest stable Go version
func (g *GoRuntime) GetLatest(ctx context.Context) (string, error) {
	versions, err := g.List(ctx)
	if err != nil {
		return "", err
	}

	for _, v := range versions {
		if v.Stable {
			return g.Get(ctx, v.Version)
		}
	}

	return "", fmt.Errorf("no stable version found")
}

// List returns available Go versions
func (g *GoRuntime) List(ctx context.Context) ([]types.Version, error) {
	cachedVersions, err := g.cache.GetManifest(g.Name())
	if err == nil && len(cachedVersions) > 0 {
		return cachedVersions, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", goVersionsAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching versions: %w", err)
	}
	defer resp.Body.Close()

	var releases []goRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	versions := make([]types.Version, 0, len(releases))
	platformKey := g.getPlatformKey()

	for _, release := range releases {
		v := types.Version{
			Version:      release.Version,
			Stable:       release.Stable,
			DownloadURLs: make(map[string]string),
		}

		for _, file := range release.Files {
			if file.Kind == "archive" {
				v.DownloadURLs[file.OS+"-"+file.Arch] = goDownloadBaseURL + file.Filename
			}
		}

		// Only include if available for current platform
		if _, ok := v.DownloadURLs[platformKey]; ok {
			versions = append(versions, v)
		}
	}

	// Sort by version (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].Version, versions[j].Version) > 0
	})

	if err := g.cache.SetManifest(g.Name(), versions); err != nil {
		log.Debug().Err(err).Msg("failed to set manifest")
	}

	return versions, nil
}

func (g *GoRuntime) getDownloadURL(ctx context.Context, version string) (string, error) {
	versions, err := g.List(ctx)
	if err != nil {
		return "", err
	}

	platformKey := g.getPlatformKey()

	// try a naive and fast matching first
	for _, v := range versions {
		if v.Version == version {
			if url, ok := v.DownloadURLs[platformKey]; ok {
				return url, nil
			}
			return "", fmt.Errorf("version %s not available for platform %s", version, platformKey)
		}
	}

	cv, err := semver.NewVersion(strings.TrimPrefix(version, "go"))
	if err != nil {
		return "", fmt.Errorf("invalid semver version: %w", err)
	}

	for _, v := range versions {
		sv, err := semver.NewVersion(strings.TrimPrefix(v.Version, "go"))
		if err != nil {
			continue
		}

		if sv.Compare(cv) == 0 {
			return v.DownloadURLs[platformKey], nil
		}
	}

	return "", fmt.Errorf("version %s not found", version)
}

func (g *GoRuntime) getPlatformKey() string {
	return g.platform.OS + "-" + g.platform.Arch
}

func (g *GoRuntime) checkInstalled(ctx context.Context, version string) (string, bool) {
	out := bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "go", "version")
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run()

	output := out.String()
	if strings.Contains(output, version) {
		out := bytes.Buffer{}
		cmd := exec.CommandContext(ctx, "which", "go")
		cmd.Stdout = &out
		cmd.Stderr = &out
		_ = cmd.Run()

		return strings.Trim(out.String(), "\n"), true
	}

	return "", false
}

// goRelease represents a Go release from the API
type goRelease struct {
	Version string   `json:"version"`
	Stable  bool     `json:"stable"`
	Files   []goFile `json:"files"`
}

// goFile represents a downloadable file in a Go release
type goFile struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Kind     string `json:"kind"`
	SHA256   string `json:"sha256"`
}

// compareVersions compares two version strings
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	// Extract version numbers
	re := regexp.MustCompile(`go(\d+)\.(\d+)(?:\.(\d+))?`)

	parts1 := re.FindStringSubmatch(v1)
	parts2 := re.FindStringSubmatch(v2)

	if len(parts1) < 3 || len(parts2) < 3 {
		return strings.Compare(v1, v2)
	}

	for i := 1; i < 4; i++ {
		var p1, p2 int
		if i < len(parts1) && parts1[i] != "" {
			_, _ = fmt.Sscanf(parts1[i], "%d", &p1)
		}
		if i < len(parts2) && parts2[i] != "" {
			_, _ = fmt.Sscanf(parts2[i], "%d", &p2)
		}

		if p1 > p2 {
			return 1
		} else if p1 < p2 {
			return -1
		}
	}

	return 0
}
