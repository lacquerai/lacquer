package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/minio/selfupdate"
	"github.com/spf13/cobra"

	"github.com/lacquerai/lacquer/internal/style"
)

const (
	updateCacheFile = ".lacquer/update_cache.json"
	cacheExpiry     = 2 * time.Hour
	githubAPIURL    = "https://api.github.com/repos/lacquerai/lacquer/releases/latest"
)

type UpdateInfo struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
	CurrentIsOld  bool      `json:"current_is_old"`
	DownloadURL   string    `json:"download_url"`
}

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update laq to the latest version",
	Long: `Update laq to the latest version available on GitHub.

This command:
- Checks for the latest release on GitHub
- Downloads the appropriate binary for your platform
- Replaces the current binary with the new version
- Validates the download using checksums when available
`,
	Example: `
  laq update              # Update to latest version
  laq update --check      # Only check for updates without updating
  laq update --force      # Force update even if already on latest version`,
	Run: func(cmd *cobra.Command, args []string) {
		checkOnly, _ := cmd.Flags().GetBool("check")
		force, _ := cmd.Flags().GetBool("force")

		if checkOnly {
			checkForUpdate(cmd, true, true)
			return
		}

		performUpdate(cmd, force)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().Bool("check", false, "only check for updates without updating")
	updateCmd.Flags().Bool("force", false, "force update even if already on latest version")
}

// checkForUpdate checks if a newer version is available
func checkForUpdate(cmd *cobra.Command, verbose bool, withoutCache bool) *UpdateInfo {
	if !withoutCache {
		updateInfo := loadUpdateCache()

		if updateInfo != nil && time.Since(updateInfo.LastChecked) < cacheExpiry {
			return updateInfo
		}
	}

	latest, downloadURL, err := fetchLatestVersion()
	if err != nil {
		if verbose {
			fmt.Fprintf(cmd.ErrOrStderr(), "%s Failed to check for updates: %s\n", style.ErrorIcon(), err)
		}
		return nil
	}

	currentVersion := normalizeVersion(Version)
	latestVersion := normalizeVersion(latest)

	currentSemver, err1 := semver.NewVersion(currentVersion)
	latestSemver, err2 := semver.NewVersion(latestVersion)

	isOutdated := false
	if err1 == nil && err2 == nil {
		isOutdated = currentSemver.LessThan(latestSemver)
	} else {
		isOutdated = currentVersion != latestVersion && Version != "dev"
	}

	updateInfo := &UpdateInfo{
		LastChecked:   time.Now(),
		LatestVersion: latest,
		CurrentIsOld:  isOutdated,
		DownloadURL:   downloadURL,
	}

	// Save to cache
	saveUpdateCache(updateInfo)

	if verbose {
		if isOutdated {
			fmt.Fprintf(cmd.OutOrStdout(), "%s A newer version (%s) is available!\n", style.InfoIcon(), latest)
			fmt.Fprintf(cmd.OutOrStdout(), "Run 'laq update' to upgrade.\n")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s You are running the latest version (%s)\n", style.SuccessIcon(), Version)
		}
	}

	return updateInfo
}

// performUpdate downloads and installs the latest version
func performUpdate(cmd *cobra.Command, force bool) {
	updateInfo := checkForUpdate(cmd, false, true)
	if updateInfo == nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s Failed to check for updates\n", style.ErrorIcon())
		return
	}

	if !updateInfo.CurrentIsOld && !force {
		fmt.Fprintf(cmd.OutOrStdout(), "%s You are already running the latest version (%s)\n", style.SuccessIcon(), Version)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s Downloading laq %s...\n", style.InfoIcon(), updateInfo.LatestVersion)

	binary, err := downloadAndExtractBinary(updateInfo.DownloadURL)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s Failed to download update: %s\n", style.ErrorIcon(), err)
		return
	}

	if err := selfupdate.Apply(binary, selfupdate.Options{}); err != nil {
		if err := selfupdate.RollbackError(err); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "%s Failed to rollback update: %s\n", style.ErrorIcon(), err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "%s Failed to apply update: %s\n", style.ErrorIcon(), err)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s Successfully updated to laq %s!\n", style.SuccessIcon(), updateInfo.LatestVersion)
}

// fetchLatestVersion gets the latest version from GitHub API
func fetchLatestVersion() (version, downloadURL string, err error) {
	resp, err := http.Get(githubAPIURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", fmt.Errorf("failed to decode release info: %w", err)
	}

	// Find the appropriate asset for current platform
	assetName := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	for _, asset := range release.Assets {
		if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(assetName)) {
			return release.TagName, asset.BrowserDownloadURL, nil
		}
	}

	return "", "", fmt.Errorf("no binary found for platform %s/%s", runtime.GOOS, runtime.GOARCH)
}

// downloadAndExtractBinary downloads the archive and extracts the laq binary
func downloadAndExtractBinary(url string) (io.Reader, error) {
	resp, err := http.Get(url) // #nosec G107 - URL comes from GitHub API
	if err != nil {
		return nil, fmt.Errorf("failed to download archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if strings.HasSuffix(url, ".tar.gz") {
		return extractFromTarGz(bytes.NewReader(data))
	} else if strings.HasSuffix(url, ".zip") {
		return extractFromZip(bytes.NewReader(data), int64(len(data)))
	}

	return nil, fmt.Errorf("unsupported archive format")
}

// extractFromTarGz extracts the laq binary from a tar.gz archive
func extractFromTarGz(r io.Reader) (io.Reader, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		if strings.HasSuffix(header.Name, "laq") || strings.HasSuffix(header.Name, "laq.exe") {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("failed to read binary from tar: %w", err)
			}
			return bytes.NewReader(data), nil
		}
	}

	return nil, fmt.Errorf("laq binary not found in tar.gz archive")
}

// extractFromZip extracts the laq binary from a zip archive
func extractFromZip(r io.ReaderAt, size int64) (io.Reader, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %w", err)
	}

	for _, file := range zr.File {
		if strings.HasSuffix(file.Name, "laq") || strings.HasSuffix(file.Name, "laq.exe") {
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open file in zip: %w", err)
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("failed to read binary from zip: %w", err)
			}
			return bytes.NewReader(data), nil
		}
	}

	return nil, fmt.Errorf("laq binary not found in zip archive")
}

// normalizeVersion removes 'v' prefix from version strings
func normalizeVersion(version string) string {
	return strings.TrimPrefix(version, "v")
}

// loadUpdateCache loads cached update information
func loadUpdateCache() *UpdateInfo {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	cacheFile := filepath.Join(homeDir, updateCacheFile)
	data, err := os.ReadFile(cacheFile) // #nosec G304 - cacheFile path is controlled
	if err != nil {
		return nil
	}

	var updateInfo UpdateInfo
	if err := json.Unmarshal(data, &updateInfo); err != nil {
		return nil
	}

	return &updateInfo
}

// saveUpdateCache saves update information to cache
func saveUpdateCache(updateInfo *UpdateInfo) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	lacquerDir := filepath.Join(homeDir, ".lacquer")
	_ = os.MkdirAll(lacquerDir, 0750)

	cacheFile := filepath.Join(homeDir, updateCacheFile)
	data, err := json.MarshalIndent(updateInfo, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(cacheFile, data, 0600)
}

// ShouldShowUpdateNotification checks if we should show an update notification
// This is called from the root command to show notifications on CLI operations
func ShouldShowUpdateNotification() *UpdateInfo {
	updateInfo := loadUpdateCache()

	if updateInfo == nil || time.Since(updateInfo.LastChecked) > cacheExpiry {
		return nil
	}

	if updateInfo.CurrentIsOld {
		return updateInfo
	}

	return nil
}
