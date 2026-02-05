package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// DownloadProgress represents the progress of a download operation.
type DownloadProgress struct {
	TotalBytes      int64
	DownloadedBytes int64
	Percent         float64
}

// ProgressCallback is called periodically during download with progress information.
type ProgressCallback func(progress DownloadProgress)

// Downloader handles downloading update binaries.
type Downloader struct {
	log         zerolog.Logger
	httpTimeout time.Duration
	directory   string
}

// NewDownloader creates a new Downloader instance.
func NewDownloader(downloadDir string, httpTimeout time.Duration, log zerolog.Logger) *Downloader {
	return &Downloader{
		log:         log.With().Str("component", "downloader").Logger(),
		httpTimeout: httpTimeout,
		directory:   downloadDir,
	}
}

// DownloadExists checks if a file has already been downloaded.
func (d *Downloader) DownloadExists(info *UpdateInfo) (bool, string) {
	if info == nil {
		return false, ""
	}

	filename := d.extractFilenameFromURL(info.DownloadURL)
	if filename == "" {
		return false, ""
	}

	destPath := filepath.Join(d.directory, filename)
	_, err := os.Stat(destPath)

	return err == nil, destPath
}

// Download fetches the update binary and verifies its checksum.
// Returns the path to the downloaded file.
func (d *Downloader) Download(ctx context.Context, info *UpdateInfo, progressCb ProgressCallback) (string, error) {
	if info == nil {
		return "", errors.New("no update info provided")
	}

	// Ensure download directory exists
	err := os.MkdirAll(d.directory, 0o755)
	if err != nil {
		return "", fmt.Errorf("failed to create download directory: %w", err)
	}

	// Clean up any old downloads before starting new one
	d.log.Debug().Msg("Cleaning up old downloads before starting new download")
	_ = d.CleanupDownloads()

	// Extract filename from URL
	filename := d.extractFilenameFromURL(info.DownloadURL)
	if filename == "" {
		filename = "simtezilo.new"
	}

	// Create temporary file for download
	destPath := filepath.Join(d.directory, filename)
	tmpPath := destPath + ".tmp"

	d.log.Info().
		Str("url", info.DownloadURL).
		Str("dest", destPath).
		Int64("size", info.DownloadSize).
		Msg("Starting download")

	// Create HTTP client with proper timeouts
	// Use shorter timeout for connection, but allow the download itself to take longer
	client := &http.Client{
		Timeout: d.httpTimeout, // Overall timeout for the entire request
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second, // Connection timeout
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second, // Time to receive response headers
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	d.log.Debug().Msg("Creating HTTP request")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.DownloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}

	d.log.Debug().Msg("Sending HTTP request")

	resp, err := client.Do(req)
	if err != nil {
		d.log.Error().Err(err).Str("url", info.DownloadURL).Msg("HTTP request failed")

		return "", fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	d.log.Debug().Int("status", resp.StatusCode).Msg("Received HTTP response")

	if resp.StatusCode != http.StatusOK {
		d.log.Error().
			Int("status", resp.StatusCode).
			Str("status_text", resp.Status).
			Str("url", info.DownloadURL).
			Msg("Download failed with non-OK HTTP status")

		return "", fmt.Errorf("download failed with HTTP status %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return d.downloadToFile(resp, tmpPath, destPath, info, progressCb)
}

// VerifyFile checks if a file matches the expected SHA256 hash.
func (d *Downloader) VerifyFile(path string, expectedSHA256 string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()

	_, err = io.Copy(hasher, file)
	if err != nil {
		return false, fmt.Errorf("failed to hash file: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))

	return actualHash == expectedSHA256, nil
}

// CleanupDownloads removes old downloaded files from the download directory.
func (d *Downloader) CleanupDownloads() error {
	patterns := []string{"*.zip", "*.tar.gz", "*.tmp", "simtezilo.new", "simtezilo.rollback"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(d.directory, pattern))
		if err != nil {
			d.log.Warn().Err(err).Str("pattern", pattern).Msg("Failed to glob pattern")

			continue
		}

		for _, path := range matches {
			_, statErr := os.Stat(path)
			if statErr == nil {
				removeErr := os.Remove(path)
				if removeErr != nil {
					d.log.Warn().Err(removeErr).Str("path", path).Msg("Failed to remove old download")
				}
			}
		}
	}

	return nil
}

// downloadToFile handles the actual file writing and verification.
func (d *Downloader) downloadToFile(
	resp *http.Response,
	tmpPath, destPath string,
	info *UpdateInfo,
	progressCb ProgressCallback,
) (string, error) {
	// Create temporary file
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	defer func() {
		tmpFile.Close()
		// Clean up temp file on error
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	downloaded, actualHash, err := d.writeAndHash(resp, tmpFile, info, progressCb)
	if err != nil {
		return "", err
	}

	// Close file before rename
	err = tmpFile.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Verify checksum
	if info.SHA256 != "" && actualHash != info.SHA256 {
		os.Remove(tmpPath)

		return "", fmt.Errorf("checksum mismatch: expected %s, got %s", info.SHA256, actualHash)
	}

	d.log.Debug().
		Str("expected", info.SHA256).
		Str("actual", actualHash).
		Msg("Checksum verified")

	// Rename temp file to final destination
	err = os.Rename(tmpPath, destPath)
	if err != nil {
		return "", fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Ensure executable permissions
	err = os.Chmod(destPath, 0o755)
	if err != nil {
		return "", fmt.Errorf("failed to set executable permissions: %w", err)
	}

	d.log.Info().
		Str("path", destPath).
		Int64("bytes", downloaded).
		Str("sha256", actualHash).
		Msg("Download completed")

	return destPath, nil
}

// writeAndHash reads from the response body, writes to file, and computes the hash.
func (d *Downloader) writeAndHash(
	resp *http.Response,
	tmpFile *os.File,
	info *UpdateInfo,
	progressCb ProgressCallback,
) (int64, string, error) {
	totalSize := resp.ContentLength
	if totalSize <= 0 && info.DownloadSize > 0 {
		totalSize = info.DownloadSize
	}

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	var downloaded int64

	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		bytesRead, readErr := resp.Body.Read(buf)
		if bytesRead > 0 {
			_, writeErr := writer.Write(buf[:bytesRead])
			if writeErr != nil {
				return 0, "", fmt.Errorf("failed to write to file: %w", writeErr)
			}

			downloaded += int64(bytesRead)

			// Report progress
			if progressCb != nil && totalSize > 0 {
				progressCb(DownloadProgress{
					TotalBytes:      totalSize,
					DownloadedBytes: downloaded,
					Percent:         float64(downloaded) / float64(totalSize) * 100,
				})
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}

			return 0, "", fmt.Errorf("failed to read response: %w", readErr)
		}
	}

	return downloaded, hex.EncodeToString(hasher.Sum(nil)), nil
}

// extractFilenameFromURL extracts the filename from a download URL.
func (d *Downloader) extractFilenameFromURL(downloadURL string) string {
	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		d.log.Warn().Err(err).Str("url", downloadURL).Msg("Failed to parse URL")

		return ""
	}

	// Get the last segment of the path
	filename := filepath.Base(parsedURL.Path)

	// Clean up the filename
	filename = strings.TrimSpace(filename)

	// Validate filename
	if filename == "" || filename == "." || filename == "/" {
		return ""
	}

	return filename
}
