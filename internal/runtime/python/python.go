package python

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	pythonDownloadBaseURL = "https://www.python.org/ftp/python/"
	pythonVersionsAPI     = "https://www.python.org/api/v2/downloads/release/?is_published=true&pre_release=false"
)

// PythonRuntime implements the Runtime interface for Python
type PythonRuntime struct {
	cache      types.Cache
	downloader types.Downloader
	platform   types.Platform
}

// New creates a new Python runtime manager
func New(cache types.Cache, downloader types.Downloader) *PythonRuntime {
	return &PythonRuntime{
		cache:      cache,
		downloader: downloader,
		platform:   types.GetPlatform(),
	}
}

// Name returns the runtime name
func (p *PythonRuntime) Name() string {
	return "python"
}

// Get downloads and installs the specified Python version
func (p *PythonRuntime) Get(ctx context.Context, version string) (string, error) {
	// Normalize version (remove 'python' prefix if present)
	version = strings.TrimPrefix(strings.ToLower(version), "python")
	version = strings.TrimSpace(version)

	if path, exists := p.checkInstalled(ctx, version); exists {
		return path, nil
	}

	if path, exists := p.cache.Get(p.Name(), version); exists {
		return path, nil
	}

	// Try Homebrew on macOS if available
	if p.platform.OS == "darwin" && p.isHomebrewAvailable(ctx) {
		if path, err := p.installWithHomebrew(ctx, version); err == nil {
			return path, nil
		} else {
			log.Debug().Err(err).Msg("homebrew installation failed, falling back to download")
		}
	}

	// Get download URL
	downloadURL, err := p.getDownloadURL(ctx, version)
	if err != nil {
		return "", fmt.Errorf("getting download URL: %w", err)
	}

	// Create temporary directory for download
	tempDir, err := os.MkdirTemp("", "python-download-*")
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

	if err := p.downloader.Download(ctx, downloadURL, file); err != nil {
		file.Close()
		return "", fmt.Errorf("downloading Python: %w", err)
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

	// Find Python root directory (it's usually Python-X.Y.Z)
	pythonRoot, err := p.findPythonRoot(extractDir)
	if err != nil {
		return "", fmt.Errorf("finding Python root: %w", err)
	}

	// For source distributions, we need to build Python
	if p.isSourceDistribution(downloadURL) {
		if err := p.buildPython(ctx, pythonRoot); err != nil {
			return "", fmt.Errorf("building Python: %w", err)
		}
	}

	// Move to cache
	cachePath := p.cache.Path(p.Name(), version)
	if err := p.cache.Set(p.Name(), version, pythonRoot); err != nil {
		return "", fmt.Errorf("caching runtime: %w", err)
	}

	return cachePath, nil
}

// GetLatest downloads the latest stable Python version
func (p *PythonRuntime) GetLatest(ctx context.Context) (string, error) {
	versions, err := p.List(ctx)
	if err != nil {
		return "", err
	}

	for _, v := range versions {
		if v.Stable {
			return p.Get(ctx, v.Version)
		}
	}

	return "", fmt.Errorf("no stable version found")
}

// List returns available Python versions
func (p *PythonRuntime) List(ctx context.Context) ([]types.Version, error) {
	cachedVersions, err := p.cache.GetManifest(p.Name())
	if err == nil && len(cachedVersions) > 0 {
		return cachedVersions, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", pythonVersionsAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching versions: %w", err)
	}
	defer resp.Body.Close()

	var apiResponse []pythonRelease
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("decoding response: %w: %s", err, string(body))
	}

	versions := make([]types.Version, 0, len(apiResponse))
	platformKey := p.getPlatformKey()

	for _, release := range apiResponse {
		// Extract version from name (e.g., "Python 3.11.5" -> "3.11.5")
		versionMatch := regexp.MustCompile(`Python\s+(\d+\.\d+\.\d+)`).FindStringSubmatch(release.Name)
		if len(versionMatch) < 2 {
			continue
		}

		version := versionMatch[1]
		v := types.Version{
			Version:      version,
			Stable:       !release.IsPreRelease,
			DownloadURLs: make(map[string]string),
		}

		// Build download URLs based on version
		v.DownloadURLs = p.buildDownloadURLs(version)

		// Only include if available for current platform
		if _, ok := v.DownloadURLs[platformKey]; ok {
			versions = append(versions, v)
		}
	}

	// Sort by version (newest first)
	sort.Slice(versions, func(i, j int) bool {
		v1, err1 := semver.NewVersion(versions[i].Version)
		v2, err2 := semver.NewVersion(versions[j].Version)
		if err1 != nil || err2 != nil {
			return versions[i].Version > versions[j].Version
		}
		return v1.GreaterThan(v2)
	})

	if err := p.cache.SetManifest(p.Name(), versions); err != nil {
		log.Debug().Err(err).Msg("failed to set manifest")
	}

	return versions, nil
}

func (p *PythonRuntime) getDownloadURL(ctx context.Context, version string) (string, error) {
	versions, err := p.List(ctx)
	if err != nil {
		return "", err
	}

	platformKey := p.getPlatformKey()

	// Try exact match first
	for _, v := range versions {
		if v.Version == version {
			if url, ok := v.DownloadURLs[platformKey]; ok {
				return url, nil
			}
			return "", fmt.Errorf("version %s not available for platform %s", version, platformKey)
		}
	}

	// Try semver matching
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
			if url, ok := v.DownloadURLs[platformKey]; ok {
				return url, nil
			}
		}
	}

	return "", fmt.Errorf("version %s not found", version)
}

func (p *PythonRuntime) getPlatformKey() string {
	return p.platform.OS + "-" + p.platform.Arch
}

func (p *PythonRuntime) buildDownloadURLs(version string) map[string]string {
	urls := make(map[string]string)
	baseURL := pythonDownloadBaseURL + version + "/"

	// Source distributions (for all platforms)
	urls["linux-amd64"] = baseURL + "Python-" + version + ".tgz"
	urls["linux-arm64"] = baseURL + "Python-" + version + ".tgz"
	urls["darwin-amd64"] = baseURL + "Python-" + version + ".tgz"
	urls["darwin-arm64"] = baseURL + "Python-" + version + ".tgz"

	// Windows binaries
	urls["windows-amd64"] = baseURL + "python-" + version + "-amd64.exe"
	urls["windows-386"] = baseURL + "python-" + version + ".exe"

	// macOS installers (for newer versions)
	if v, _ := semver.NewVersion(version); v != nil && v.GreaterThan(semver.MustParse("3.9.0")) {
		urls["darwin-amd64"] = baseURL + "python-" + version + "-macos11.pkg"
		urls["darwin-arm64"] = baseURL + "python-" + version + "-macos11.pkg"
	}

	return urls
}

func (p *PythonRuntime) checkInstalled(ctx context.Context, version string) (string, bool) {
	// Try python3 first, then python
	for _, cmd := range []string{"python3", "python"} {
		out := bytes.Buffer{}
		execCmd := exec.CommandContext(ctx, cmd, "--version")
		execCmd.Stdout = &out
		execCmd.Stderr = &out
		if err := execCmd.Run(); err != nil {
			continue
		}

		output := strings.TrimSpace(out.String())
		// Python version output format: "Python 3.11.5"
		if strings.Contains(output, version) {
			out := bytes.Buffer{}
			whichCmd := exec.CommandContext(ctx, "which", cmd)
			whichCmd.Stdout = &out
			whichCmd.Stderr = &out
			if err := whichCmd.Run(); err == nil {
				return strings.TrimSpace(out.String()), true
			}
		}
	}

	return "", false
}

func (p *PythonRuntime) findPythonRoot(extractDir string) (string, error) {
	// Look for Python-X.Y.Z directory
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "Python-") {
			return filepath.Join(extractDir, entry.Name()), nil
		}
	}

	// If not found, the extraction directory itself might be the root
	if _, err := os.Stat(filepath.Join(extractDir, "configure")); err == nil {
		return extractDir, nil
	}

	return "", fmt.Errorf("python root directory not found")
}

func (p *PythonRuntime) isSourceDistribution(downloadURL string) bool {
	return strings.HasSuffix(downloadURL, ".tgz") || strings.HasSuffix(downloadURL, ".tar.gz")
}

func (p *PythonRuntime) buildPython(ctx context.Context, sourceDir string) error {
	// Configure
	configureCmd := exec.CommandContext(ctx, "./configure", "--prefix="+sourceDir, "--enable-optimizations")
	configureCmd.Dir = sourceDir
	configureCmd.Stdout = os.Stdout
	configureCmd.Stderr = os.Stderr
	if err := configureCmd.Run(); err != nil {
		return fmt.Errorf("configure failed: %w", err)
	}

	// Make
	makeCmd := exec.CommandContext(ctx, "make", "-j4")
	makeCmd.Dir = sourceDir
	makeCmd.Stdout = os.Stdout
	makeCmd.Stderr = os.Stderr
	if err := makeCmd.Run(); err != nil {
		return fmt.Errorf("make failed: %w", err)
	}

	// Make install
	makeInstallCmd := exec.CommandContext(ctx, "make", "install")
	makeInstallCmd.Dir = sourceDir
	makeInstallCmd.Stdout = os.Stdout
	makeInstallCmd.Stderr = os.Stderr
	if err := makeInstallCmd.Run(); err != nil {
		return fmt.Errorf("make install failed: %w", err)
	}

	return nil
}

// isHomebrewAvailable checks if Homebrew is installed and available
func (p *PythonRuntime) isHomebrewAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "which", "brew")
	return cmd.Run() == nil
}

// installWithHomebrew installs Python using Homebrew and returns the Python executable path
func (p *PythonRuntime) installWithHomebrew(ctx context.Context, version string) (string, error) {
	// Homebrew formula name for specific Python versions
	formula := "python@" + version

	// Check if the formula exists and install it
	installCmd := exec.CommandContext(ctx, "brew", "install", formula)
	var stderr bytes.Buffer
	installCmd.Stderr = &stderr

	if err := installCmd.Run(); err != nil {
		// Try with just "python" if specific version formula doesn't exist
		if strings.Contains(stderr.String(), "No available formula") {
			formula = "python"
			installCmd = exec.CommandContext(ctx, "brew", "install", formula)
			installCmd.Stderr = &stderr
			if err := installCmd.Run(); err != nil {
				return "", fmt.Errorf("homebrew install failed: %w, stderr: %s", err, stderr.String())
			}
		} else {
			return "", fmt.Errorf("homebrew install failed: %w, stderr: %s", err, stderr.String())
		}
	}

	// Find the Python executable
	pythonPath, err := p.findHomebrewPythonPath(ctx, version)
	if err != nil {
		return "", fmt.Errorf("finding homebrew python path: %w", err)
	}

	// Cache the installation by creating a symlink in our cache
	cachePath := p.cache.Path(p.Name(), version)
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("creating cache dir: %w", err)
	}

	// Create a symbolic link to the Homebrew installation
	if err := os.Symlink(filepath.Dir(pythonPath), cachePath); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("creating cache symlink: %w", err)
	}

	return pythonPath, nil
}

// findHomebrewPythonPath finds the Python executable path from Homebrew installation
func (p *PythonRuntime) findHomebrewPythonPath(ctx context.Context, version string) (string, error) {
	// Try versioned python first (python3.11, python3.12, etc.)
	versionParts := strings.Split(version, ".")
	if len(versionParts) >= 2 {
		versionedName := fmt.Sprintf("python%s.%s", versionParts[0], versionParts[1])
		if path, err := exec.LookPath(versionedName); err == nil {
			return path, nil
		}
	}

	// Try with just major version (python3)
	if len(versionParts) >= 1 {
		majorVersionName := fmt.Sprintf("python%s", versionParts[0])
		if path, err := exec.LookPath(majorVersionName); err == nil {
			return path, nil
		}
	}

	// Fall back to "python3" or "python"
	for _, pythonCmd := range []string{"python3", "python"} {
		if path, err := exec.LookPath(pythonCmd); err == nil {
			// Verify this is the right version by checking its version output
			out := bytes.Buffer{}
			cmd := exec.CommandContext(ctx, path, "--version")
			cmd.Stdout = &out
			cmd.Stderr = &out
			if err := cmd.Run(); err == nil {
				output := strings.TrimSpace(out.String())
				if strings.Contains(output, version) {
					return path, nil
				}
			}
		}
	}

	return "", fmt.Errorf("python %s not found after homebrew installation", version)
}

// pythonRelease represents a Python release from the API
type pythonRelease struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Version      int    `json:"version"`
	IsPublished  bool   `json:"is_published"`
	IsPreRelease bool   `json:"is_pre_release"`
	ReleaseDate  string `json:"release_date"`
	ReleaseURL   string `json:"release_page_url"`
}
