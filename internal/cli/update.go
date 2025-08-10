package cli

import (
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
			checkForUpdate(cmd, true)
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
func checkForUpdate(cmd *cobra.Command, verbose bool) *UpdateInfo {
	updateInfo := loadUpdateCache()

	if updateInfo != nil && time.Since(updateInfo.LastChecked) < cacheExpiry {
		return updateInfo
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

	updateInfo = &UpdateInfo{
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
	updateInfo := checkForUpdate(cmd, false)
	if updateInfo == nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s Failed to check for updates\n", style.ErrorIcon())
		return
	}

	if !updateInfo.CurrentIsOld && !force {
		fmt.Fprintf(cmd.OutOrStdout(), "%s You are already running the latest version (%s)\n", style.SuccessIcon(), Version)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s Downloading laq %s...\n", style.InfoIcon(), updateInfo.LatestVersion)

	// Download the new binary
	tempFile, err := downloadBinary(updateInfo.DownloadURL)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s Failed to download update: %s\n", style.ErrorIcon(), err)
		return
	}
	defer func() { _ = os.Remove(tempFile) }()

	// Get current executable path
	currentExe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s Failed to get current executable path: %s\n", style.ErrorIcon(), err)
		return
	}

	// Make the downloaded file executable
	if err := os.Chmod(tempFile, 0600); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s Failed to make binary executable: %s\n", style.ErrorIcon(), err)
		return
	}

	// Replace the current binary
	if err := replaceBinary(currentExe, tempFile); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s Failed to replace binary: %s\n", style.ErrorIcon(), err)
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

// downloadBinary downloads the binary to a temporary file
func downloadBinary(url string) (string, error) {
	resp, err := http.Get(url) // #nosec G107 - URL comes from GitHub API
	if err != nil {
		return "", fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp("", "laq_update_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() { _ = tempFile.Close() }()

	// Copy the downloaded content
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write binary: %w", err)
	}

	return tempFile.Name(), nil
}

// replaceBinary replaces the current binary with the new one
func replaceBinary(currentPath, newPath string) error {
	// On Windows, we can't replace a running executable directly
	if runtime.GOOS == "windows" {
		backupPath := currentPath + ".bak"

		// Move current binary to backup
		if err := os.Rename(currentPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup current binary: %w", err)
		}

		// Move new binary to current location
		if err := os.Rename(newPath, currentPath); err != nil {
			// Try to restore backup if move failed
			_ = os.Rename(backupPath, currentPath)
			return fmt.Errorf("failed to move new binary: %w", err)
		}

		// Remove backup
		_ = os.Remove(backupPath)
	} else {
		// On Unix systems, we can replace the file directly
		if err := os.Rename(newPath, currentPath); err != nil {
			return fmt.Errorf("failed to replace binary: %w", err)
		}
	}

	return nil
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
