package updater //nolint:testpackage // testing internal functions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewDownloaderCreatesValidInstance(t *testing.T) {
	t.Parallel()

	log := zerolog.Nop()
	downloadDir := "/tmp/test-downloads"
	timeout := 30 * time.Second

	downloader := NewDownloader(downloadDir, timeout, log)

	if downloader == nil {
		t.Fatal("NewDownloader() returned nil")
	}

	if downloader.directory != downloadDir {
		t.Errorf("downloadDir = %v, want %v", downloader.directory, downloadDir)
	}

	if downloader.httpTimeout != timeout {
		t.Errorf("httpTimeout = %v, want %v", downloader.httpTimeout, timeout)
	}
}

func TestDownloadSucceedsWithValidChecksumAndReportsProgress(t *testing.T) {
	t.Parallel()

	// Create test content with known hash
	testContent := []byte("test binary content for download")
	hasher := sha256.New()
	hasher.Write(testContent)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "32")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(testContent)
	}))
	defer server.Close()

	// Create temp download directory
	downloadDir := t.TempDir()
	log := zerolog.Nop()

	downloader := NewDownloader(downloadDir, 30*time.Second, log)

	info := &UpdateInfo{
		DownloadURL:      server.URL,
		SHA256:           expectedHash,
		AvailableVersion: "1.0.0",
		DownloadSize:     int64(len(testContent)),
	}

	var progressUpdates []DownloadProgress

	progressCb := func(p DownloadProgress) {
		progressUpdates = append(progressUpdates, p)
	}

	path, err := downloader.Download(context.Background(), info, progressCb)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	if path == "" {
		t.Error("Download() returned empty path")
	}

	// Verify file exists and contains correct content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != string(testContent) {
		t.Errorf("Downloaded content = %v, want %v", string(content), string(testContent))
	}

	// Verify progress was reported
	if len(progressUpdates) == 0 {
		t.Error("No progress updates received")
	}
}

func TestDownloadFailsWhenUpdateInfoIsNil(t *testing.T) {
	t.Parallel()

	downloadDir := t.TempDir()
	log := zerolog.Nop()

	d := NewDownloader(downloadDir, 30*time.Second, log)

	_, err := d.Download(context.Background(), nil, nil)
	if err == nil {
		t.Error("Download() with nil info should return error")
	}
}

func TestDownloadFailsWhenServerReturnsError(t *testing.T) {
	t.Parallel()

	// Create server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	downloadDir := t.TempDir()
	log := zerolog.Nop()

	downloader := NewDownloader(downloadDir, 30*time.Second, log)

	info := &UpdateInfo{
		DownloadURL: server.URL,
		SHA256:      "somehash",
	}

	_, err := downloader.Download(context.Background(), info, nil)
	if err == nil {
		t.Error("Download() should return error on HTTP error")
	}
}

func TestDownloadFailsWhenChecksumDoesNotMatch(t *testing.T) {
	t.Parallel()

	testContent := []byte("test content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(testContent)
	}))
	defer server.Close()

	downloadDir := t.TempDir()
	log := zerolog.Nop()

	downloader := NewDownloader(downloadDir, 30*time.Second, log)

	info := &UpdateInfo{
		DownloadURL:  server.URL,
		SHA256:       "invalid-checksum-that-wont-match",
		DownloadSize: int64(len(testContent)),
	}

	_, err := downloader.Download(context.Background(), info, nil)
	if err == nil {
		t.Error("Download() should return error on checksum mismatch")
	}
}

func TestVerifyFileReturnsTrueForMatchingHash(t *testing.T) {
	t.Parallel()

	// Create a test file with known content
	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "test-file")
	testContent := []byte("test content for verification")

	err := os.WriteFile(testFile, testContent, 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate expected hash
	hasher := sha256.New()
	hasher.Write(testContent)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	log := zerolog.Nop()
	downloader := NewDownloader(testDir, 30*time.Second, log)

	// Test with correct hash
	valid, err := downloader.VerifyFile(testFile, expectedHash)
	if err != nil {
		t.Fatalf("VerifyFile() error = %v", err)
	}

	if !valid {
		t.Error("VerifyFile() = false, want true for matching hash")
	}

	// Test with wrong hash
	valid, err = downloader.VerifyFile(testFile, "wrong-hash")
	if err != nil {
		t.Fatalf("VerifyFile() error = %v", err)
	}

	if valid {
		t.Error("VerifyFile() = true, want false for non-matching hash")
	}
}

func TestVerifyFileFailsForNonExistentFile(t *testing.T) {
	t.Parallel()

	log := zerolog.Nop()
	d := NewDownloader(t.TempDir(), 30*time.Second, log)

	_, err := d.VerifyFile("/nonexistent/file", "somehash")
	if err == nil {
		t.Error("VerifyFile() should return error for non-existent file")
	}
}

func TestCleanupDownloadsRemovesOldFiles(t *testing.T) {
	t.Parallel()

	downloadDir := t.TempDir()
	log := zerolog.Nop()

	// Create test files to clean up
	files := []string{
		filepath.Join(downloadDir, "simtezilo.new"),
		filepath.Join(downloadDir, "simtezilo.new.tmp"),
	}

	for _, f := range files {
		err := os.WriteFile(f, []byte("test"), 0o600)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	downloader := NewDownloader(downloadDir, 30*time.Second, log)

	err := downloader.CleanupDownloads()
	if err != nil {
		t.Fatalf("CleanupDownloads() error = %v", err)
	}

	// Verify files were cleaned up
	for _, f := range files {
		_, statErr := os.Stat(f)
		if statErr == nil {
			t.Errorf("File %s should have been cleaned up", f)
		}
	}
}

func TestDownloadSucceedsWithoutProgressCallback(t *testing.T) {
	t.Parallel()

	testContent := []byte("test binary content")
	hasher := sha256.New()
	hasher.Write(testContent)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(testContent)
	}))
	defer server.Close()

	downloadDir := t.TempDir()
	log := zerolog.Nop()

	downloader := NewDownloader(downloadDir, 30*time.Second, log)

	info := &UpdateInfo{
		DownloadURL:  server.URL,
		SHA256:       expectedHash,
		DownloadSize: int64(len(testContent)),
	}

	// Test with nil progress callback
	path, err := downloader.Download(context.Background(), info, nil)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	if path == "" {
		t.Error("Download() returned empty path")
	}
}

func TestDownloadProgressStoresAllFields(t *testing.T) {
	t.Parallel()

	progress := DownloadProgress{
		TotalBytes:      1000,
		DownloadedBytes: 500,
		Percent:         50.0,
	}

	if progress.TotalBytes != 1000 {
		t.Errorf("TotalBytes = %v, want 1000", progress.TotalBytes)
	}

	if progress.DownloadedBytes != 500 {
		t.Errorf("DownloadedBytes = %v, want 500", progress.DownloadedBytes)
	}

	if progress.Percent != 50.0 {
		t.Errorf("Percent = %v, want 50.0", progress.Percent)
	}
}
