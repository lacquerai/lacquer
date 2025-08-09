// runtime/utils.go

package utils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lacquerai/lacquer/internal/runtime/types"
)

// DefaultDownloader implements the Downloader interface using HTTP
type DefaultDownloader struct {
	Client *http.Client
}

// NewDefaultDownloader creates a new HTTP downloader
func NewDefaultDownloader() *DefaultDownloader {
	return &DefaultDownloader{
		Client: &http.Client{},
	}
}

// Download downloads a file from the given URL
func (d *DefaultDownloader) Download(ctx context.Context, url string, writer io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := d.Client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	_, err = io.Copy(writer, resp.Body)
	return err
}

// DownloadWithChecksum downloads a file and verifies its checksum
func DownloadWithChecksum(ctx context.Context, d types.Downloader, url, expectedChecksum string) ([]byte, error) {
	var buf strings.Builder
	if err := d.Download(ctx, url, &buf); err != nil {
		return nil, err
	}

	data := []byte(buf.String())

	if expectedChecksum != "" {
		h := sha256.New()
		h.Write(data)
		checksum := hex.EncodeToString(h.Sum(nil))

		if checksum != expectedChecksum {
			return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, checksum)
		}
	}

	return data, nil
}

// TarGzExtractor extracts tar.gz archives
type TarGzExtractor struct{}

// Extract extracts a tar.gz archive
func (e *TarGzExtractor) Extract(src, dest string) error {
	file, err := os.Open(src) // #nosec G304 - src path is controlled by runtime
	if err != nil {
		return fmt.Errorf("opening archive: %w", err)
	}
	defer func() { _ = file.Close() }()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar header: %w", err)
		}

		target := filepath.Join(dest, header.Name) // #nosec G305 - path traversal prevention is implemented below
		// Validate path to prevent path traversal
		if !strings.HasPrefix(target, dest) {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0750); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
		case tar.TypeReg:
			// #nosec G115 - tar header mode conversion is safe within this context
			if err := extractFile(tr, target, os.FileMode(uint32(header.Mode))); err != nil {
				return err
			}
		}
	}

	return nil
}

// ZipExtractor extracts zip archives
type ZipExtractor struct{}

// Extract extracts a zip archive
func (e *ZipExtractor) Extract(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		target := filepath.Join(dest, f.Name) // #nosec G305 - path traversal prevention is implemented below
		// Validate path to prevent path traversal
		if !strings.HasPrefix(target, dest) {
			return fmt.Errorf("invalid path in archive: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return fmt.Errorf("creating parent directory: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("opening file in archive: %w", err)
		}

		if err := extractFile(rc, target, f.Mode()); err != nil {
			_ = rc.Close()
			return err
		}
		_ = rc.Close()
	}

	return nil
}

func extractFile(src io.Reader, dest string, mode os.FileMode) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	file, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode) // #nosec G304 - dest path is controlled by runtime
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, src)
	return err
}

// GetExtractor returns the appropriate extractor based on file extension
func GetExtractor(filename string) (types.Extractor, error) {
	switch {
	case strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".tgz"):
		return &TarGzExtractor{}, nil
	case strings.HasSuffix(filename, ".zip"):
		return &ZipExtractor{}, nil
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", filename)
	}
}
