package updater_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/zetetos/simtezilo/app/updater"
)

// TestFullUpdateFlowFromCheckToInstall tests the complete update flow from check to install.
func TestFullUpdateFlowFromCheckToInstall(t *testing.T) {
	t.Parallel()

	// Create test binary content
	testBinary := []byte("#!/bin/bash\necho 'new version'\n")
	hasher := sha256.New()
	hasher.Write(testBinary)
	binaryHash := hex.EncodeToString(hasher.Sum(nil))

	// Create test server
	mux := http.NewServeMux()

	wantVersion := "2.0.0" //nolint:goconst // test constant
	releaseURL := fmt.Sprintf("/releases/%s/simtezilo", wantVersion)

	// Manifest endpoint
	mux.HandleFunc("/releases/stable/latest.json", func(resp http.ResponseWriter, _ *http.Request) {
		manifest := updater.Manifest{
			Version:     wantVersion,
			ReleaseDate: time.Now().UTC(),
			Channel:     "stable",
			Changelog:   []string{"- Feature 1", "- Feature 2"},
			Platforms: map[string]updater.Platform{
				updater.GetPlatformKey(): {
					URL:    releaseURL,
					SHA256: binaryHash,
					Size:   int64(len(testBinary)),
				},
			},
		}

		resp.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(resp).Encode(manifest)
	})

	// Binary download endpoint
	mux.HandleFunc(releaseURL, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(testBinary)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Setup test directories
	installDir := t.TempDir()
	dataDir := t.TempDir()

	// Create current binary
	currentBinaryPath := filepath.Join(installDir, "simtezilo")

	err := os.WriteFile(currentBinaryPath, []byte("old version"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create current binary: %v", err)
	}

	log := zerolog.Nop()

	// Create updater config
	cfg := &updater.Config{
		Enabled:         true,
		BaseURL:         server.URL + "/releases",
		Channel:         "stable",
		CheckInterval:   1 * time.Hour,
		HTTPTimeout:     30 * time.Second,
		DownloadTimeout: 5 * time.Minute,
		AutoInstall:     false,
		InstallDir:      installDir,
		InitDir:         installDir,
		DataDir:         dataDir,
		BinaryName:      "simtezilo",
		ServiceName:     "simtezilo",
		UseSystemd:      false, // Disable for testing
	}

	// Create updater
	updateManager, err := updater.New(cfg, "1.0.0", log)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Step 1: Check for updates
	info, err := updateManager.CheckNow()
	if err != nil {
		t.Fatalf("CheckNow() error = %v", err)
	}

	if info == nil {
		t.Fatal("CheckNow() returned nil, expected update info")
	}

	if info.AvailableVersion != "v"+wantVersion {
		t.Errorf("AvailableVersion = %v, want v%s", info.AvailableVersion, wantVersion)
	}

	if updateManager.Status() != updater.UpdateStatusUpdateAvailable {
		t.Errorf("Status() = %v, want StatusUpdateAvailable", updateManager.Status())
	}

	// Step 2: Download update
	// Update the download URL to use full server URL
	info.DownloadURL = server.URL + info.DownloadURL

	var progressUpdates []updater.DownloadProgress

	progressCb := func(p updater.DownloadProgress) {
		progressUpdates = append(progressUpdates, p)
	}

	downloadPath, err := updateManager.DownloadUpdate(context.Background(), progressCb)
	if err != nil {
		t.Fatalf("DownloadUpdate() error = %v", err)
	}

	if downloadPath == "" {
		t.Error("DownloadUpdate() returned empty path")
	}

	// Verify downloaded file exists
	_, statErr := os.Stat(downloadPath)
	if statErr != nil {
		t.Errorf("Downloaded file does not exist: %v", statErr)
	}

	if updateManager.Status() != updater.UpdateStatusReadyToInstall {
		t.Errorf("Status() = %v, want StatusReadyToInstall", updateManager.Status())
	}

	// Step 3: Prepare installation
	err = updateManager.PrepareInstall(downloadPath)
	if err != nil {
		t.Fatalf("PrepareInstall() error = %v", err)
	}

	// Verify state was saved
	state, err := updateManager.Installer().LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if state == nil {
		t.Fatal("State should not be nil after PrepareInstall()")
	}

	if state.PendingVersion != "v2.0.0" {
		t.Errorf("PendingVersion = %v, want v2.0.0", state.PendingVersion)
	}

	// Step 4: Apply update (simulating what the startup script would do)
	err = updateManager.Installer().ApplyPendingUpdate()
	if err != nil {
		t.Fatalf("ApplyPendingUpdate() error = %v", err)
	}

	// Verify new binary is installed
	content, err := os.ReadFile(currentBinaryPath)
	if err != nil {
		t.Fatalf("Failed to read installed binary: %v", err)
	}

	if string(content) != string(testBinary) {
		t.Error("Installed binary content does not match expected")
	}

	// Verify rollback binary exists
	rollbackPath := filepath.Join(installDir, "simtezilo.rollback")

	rollbackContent, err := os.ReadFile(rollbackPath)
	if err != nil {
		t.Fatalf("Failed to read rollback binary: %v", err)
	}

	if string(rollbackContent) != "old version" {
		t.Error("Rollback binary content does not match old version")
	}
}

// TestDownloadAndPrepareStagesUpdate tests the convenience method.
func TestDownloadAndPrepareStagesUpdate(t *testing.T) {
	t.Parallel()

	testBinary := []byte("binary content")
	hasher := sha256.New()
	hasher.Write(testBinary)
	binaryHash := hex.EncodeToString(hasher.Sum(nil))

	mux := http.NewServeMux()

	manifestHandler := func(resp http.ResponseWriter, _ *http.Request) {
		manifest := updater.Manifest{
			Version:     "2.0.0",
			ReleaseDate: time.Now().UTC(),
			Channel:     "stable",
			Platforms: map[string]updater.Platform{
				updater.GetPlatformKey(): {
					URL:    "/binary",
					SHA256: binaryHash,
					Size:   int64(len(testBinary)),
				},
			},
		}

		resp.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(resp).Encode(manifest) //nolint:errchkjson // ignore error in test
	}

	mux.HandleFunc("/manifest.json", manifestHandler)
	mux.HandleFunc("/stable/latest.json", manifestHandler)

	mux.HandleFunc("/binary", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(testBinary)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	installDir := t.TempDir()
	dataDir := t.TempDir()

	log := zerolog.Nop()

	cfg := &updater.Config{
		Enabled:         true,
		BaseURL:         server.URL,
		Channel:         "stable",
		CheckInterval:   1 * time.Hour,
		HTTPTimeout:     30 * time.Second,
		DownloadTimeout: 5 * time.Minute,
		AutoInstall:     false,
		InstallDir:      installDir,
		InitDir:         installDir,
		DataDir:         dataDir,
		BinaryName:      "simtezilo",
		UseSystemd:      false,
	}

	updaterManager, err := updater.New(cfg, "1.0.0", log)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Check for update first
	info, err := updaterManager.CheckNow()
	if err != nil {
		t.Fatalf("CheckNow() error = %v", err)
	}

	if info == nil {
		t.Fatal("Expected update to be available")
	}

	// Update URL to use server
	info.DownloadURL = server.URL + info.DownloadURL

	updaterManager.Checker().SetAvailableDownloadURL(info.DownloadURL)

	// Use DownloadAndPrepare convenience method
	err = updaterManager.DownloadAndPrepare(context.Background(), nil)
	if err != nil {
		t.Fatalf("DownloadAndPrepare() error = %v", err)
	}

	// Verify state
	state, err := updaterManager.Installer().LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if state == nil || state.Status != updater.InstallStatusPending {
		t.Error("State should be pending after DownloadAndPrepare()")
	}
}

// TestRollbackRestoresWorkingVersionAfterFailure tests the rollback functionality.
func TestRollbackRestoresWorkingVersionAfterFailure(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	dataDir := t.TempDir()

	// Create current binary (the failed new version)
	currentBinaryPath := filepath.Join(installDir, "simtezilo")

	err := os.WriteFile(currentBinaryPath, []byte("failed new version"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create current binary: %v", err)
	}

	// Create rollback binary (the old working version)
	rollbackPath := filepath.Join(installDir, "simtezilo.rollback")

	err = os.WriteFile(rollbackPath, []byte("old working version"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create rollback binary: %v", err)
	}

	log := zerolog.Nop()

	cfg := &updater.Config{
		Enabled:         true,
		BaseURL:         "https://example.com/manifest.json",
		Channel:         "stable",
		CheckInterval:   1 * time.Hour,
		HTTPTimeout:     30 * time.Second,
		DownloadTimeout: 5 * time.Minute,
		InstallDir:      installDir,
		InitDir:         installDir,
		DataDir:         dataDir,
		BinaryName:      "simtezilo",
		UseSystemd:      false,
	}

	updateManager, err := updater.New(cfg, "2.0.0", log)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Save state indicating an update was applied
	state := &updater.InstallState{
		PendingVersion: "2.0.0",
		CurrentVersion: "1.0.0",
		Status:         updater.InstallStatusComplete,
	}

	err = updateManager.Installer().SaveState(state)
	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Verify rollback is available
	if !updateManager.RollbackAvailable() {
		t.Error("RollbackAvailable() should be true")
	}

	if updateManager.RollbackVersion() != state.CurrentVersion {
		t.Errorf("RollbackVersion() = %v, want %v", updateManager.RollbackVersion(), state.CurrentVersion)
	}

	// Perform rollback
	err = updateManager.Rollback()
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	// Verify old version is restored
	content, err := os.ReadFile(currentBinaryPath)
	if err != nil {
		t.Fatalf("Failed to read current binary: %v", err)
	}

	if string(content) != "old working version" {
		t.Errorf("Current binary = %v, want 'old working version'", string(content))
	}

	// Verify state was updated
	state, err = updateManager.Installer().LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if state == nil || state.Status != updater.InstallStatusRolledBack {
		t.Error("State should be rolled_back after Rollback()")
	}
}

// TestAutoRollbackTriggersAfterMultipleFailures tests automatic rollback after failures.
func TestAutoRollbackTriggersAfterMultipleFailures(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	dataDir := t.TempDir()

	// Create current binary
	currentBinaryPath := filepath.Join(installDir, "simtezilo")

	err := os.WriteFile(currentBinaryPath, []byte("failed version"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create current binary: %v", err)
	}

	// Create rollback binary
	rollbackPath := filepath.Join(installDir, "simtezilo.rollback")

	err = os.WriteFile(rollbackPath, []byte("stable version"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create rollback binary: %v", err)
	}

	log := zerolog.Nop()

	installer := updater.NewInstaller(installDir, installDir, dataDir, "simtezilo", false, log)

	// Save state with enough failures to trigger auto-rollback
	state := &updater.InstallState{
		PendingVersion: "2.0.0",
		CurrentVersion: "1.0.0",
		Status:         "failed",
		FailCount:      3,
		LastError:      "service failed to start",
	}

	err = installer.SaveState(state)
	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Check should indicate auto-rollback needed
	if !installer.ShouldAutoRollback(3) {
		t.Error("ShouldAutoRollback() should be true with 3 failures")
	}

	// Perform rollback
	err = installer.Rollback()
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	// Verify stable version is restored
	content, err := os.ReadFile(currentBinaryPath)
	if err != nil {
		t.Fatalf("Failed to read current binary: %v", err)
	}

	if string(content) != "stable version" {
		t.Errorf("Current binary = %v, want 'stable version'", string(content))
	}
}

// TestSuccessConfirmationCleansUpAfterUpdate tests the success confirmation flow.
func TestSuccessConfirmationCleansUpAfterUpdate(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	dataDir := t.TempDir()

	// Create current binary (successfully updated)
	currentBinaryPath := filepath.Join(installDir, "simtezilo")

	err := os.WriteFile(currentBinaryPath, []byte("new version"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create current binary: %v", err)
	}

	// Create rollback binary (from before update)
	rollbackPath := filepath.Join(installDir, "simtezilo.rollback")

	err = os.WriteFile(rollbackPath, []byte("old version"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create rollback binary: %v", err)
	}

	log := zerolog.Nop()

	installer := updater.NewInstaller(installDir, installDir, dataDir, "simtezilo", false, log)

	// Save state indicating complete update
	state := &updater.InstallState{
		PendingVersion: "2.0.0",
		CurrentVersion: "1.0.0",
		Status:         updater.InstallStatusComplete,
	}

	err = installer.SaveState(state)
	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Confirm success (called on successful app startup)
	err = installer.ConfirmSuccess()
	if err != nil {
		t.Fatalf("ConfirmSuccess() error = %v", err)
	}

	// Verify rollback was cleaned up
	_, statErr := os.Stat(rollbackPath)
	if statErr == nil {
		t.Error("Rollback binary should be removed after ConfirmSuccess()")
	}

	// Verify state was cleared
	state, err = installer.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if state != nil {
		t.Error("State should be nil after ConfirmSuccess()")
	}
}

// TestCheckNowReturnsNilWhenNoUpdateAvailable tests when no update is available.
func TestCheckNowReturnsNilWhenNoUpdateAvailable(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()

	manifestHandler := func(resp http.ResponseWriter, _ *http.Request) {
		manifest := updater.Manifest{
			Version:     "1.0.0",
			ReleaseDate: time.Now().UTC(),
			Channel:     "stable",
			Platforms: map[string]updater.Platform{
				updater.GetPlatformKey(): {
					URL:    "/binary",
					SHA256: "abc123",
					Size:   1024,
				},
			},
		}

		resp.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(resp).Encode(manifest) //nolint:errchkjson // ignore error in test
	}

	mux.HandleFunc("/manifest.json", manifestHandler)
	mux.HandleFunc("/stable/latest.json", manifestHandler)

	server := httptest.NewServer(mux)
	defer server.Close()

	log := zerolog.Nop()

	cfg := &updater.Config{
		Enabled:         true,
		BaseURL:         server.URL,
		Channel:         "stable",
		CheckInterval:   1 * time.Hour,
		HTTPTimeout:     30 * time.Second,
		DownloadTimeout: 5 * time.Minute,
		InstallDir:      t.TempDir(),
		InitDir:         t.TempDir(),
		DataDir:         t.TempDir(),
		BinaryName:      "simtezilo",
		UseSystemd:      false,
	}

	updateManager, err := updater.New(cfg, "1.0.0", log)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	info, err := updateManager.CheckNow()
	if err != nil {
		t.Fatalf("CheckNow() error = %v", err)
	}

	if info != nil {
		t.Error("CheckNow() should return nil when already on latest")
	}

	if updateManager.Status() != updater.UpdateStatusIdle {
		t.Errorf("Status() = %v, want StatusIdle", updateManager.Status())
	}

	if updateManager.AvailableUpdate() != nil {
		t.Error("AvailableUpdate() should be nil when no update available")
	}
}
