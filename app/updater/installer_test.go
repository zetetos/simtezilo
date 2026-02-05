package updater_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/zetetos/simtezilo/app/updater"
)

func TestNewInstallerCreatesValidInstance(t *testing.T) {
	t.Parallel()

	log := zerolog.Nop()
	installDir := "/opt/simtezilo/bin"
	initDir := "/opt/simtezilo/init"
	dataDir := "/opt/simtezilo/data"
	binaryName := "simtezilo"

	installer := updater.NewInstaller(installDir, initDir, dataDir, binaryName, true, log)

	if installer == nil {
		t.Fatal("NewInstaller() returned nil")
	}

	if installer.InstallDir() != installDir {
		t.Errorf("installDir = %v, want %v", installer.InstallDir(), installDir)
	}

	if installer.InitDir() != initDir {
		t.Errorf("initDir = %v, want %v", installer.InitDir(), initDir)
	}

	if installer.DataDir() != dataDir {
		t.Errorf("dataDir = %v, want %v", installer.DataDir(), dataDir)
	}

	if installer.BinaryName() != binaryName {
		t.Errorf("binaryName = %v, want %v", installer.BinaryName(), binaryName)
	}

	if !installer.UseSystemd() {
		t.Error("useSystemd should be true")
	}
}

func TestSaveAndLoadStatePreservesAllFields(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(dataDir, dataDir, dataDir, "simtezilo", false, log)

	state := &updater.InstallState{
		PendingVersion: "2.0.0",
		CurrentVersion: "1.0.0",
		DownloadPath:   "/tmp/simtezilo.new",
		SHA256:         "abc123",
		Timestamp:      time.Now(),
		Status:         updater.InstallStatusPending,
		FailCount:      0,
	}

	// Save state
	err := installer.SaveState(state)
	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Load state
	loaded, err := installer.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if loaded == nil {
		t.Fatal("LoadState() returned nil")
	}

	if loaded.PendingVersion != state.PendingVersion {
		t.Errorf("PendingVersion = %v, want %v", loaded.PendingVersion, state.PendingVersion)
	}

	if loaded.CurrentVersion != state.CurrentVersion {
		t.Errorf("CurrentVersion = %v, want %v", loaded.CurrentVersion, state.CurrentVersion)
	}

	if loaded.Status != state.Status {
		t.Errorf("Status = %v, want %v", loaded.Status, state.Status)
	}
}

func TestLoadStateReturnsNilForMissingFile(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(dataDir, dataDir, dataDir, "simtezilo", false, log)

	state, err := installer.LoadState()
	if err != nil {
		t.Fatalf("LoadState() should not error for missing file: %v", err)
	}

	if state != nil {
		t.Error("LoadState() should return nil for non-existent state")
	}
}

func TestLoadStateFailsForInvalidJSON(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(dataDir, dataDir, dataDir, "simtezilo", false, log)

	// Write invalid JSON to state file
	statePath := filepath.Join(dataDir, "update-state.json")

	err := os.WriteFile(statePath, []byte("invalid json"), 0o600)
	if err != nil {
		t.Fatalf("Failed to write invalid state file: %v", err)
	}

	_, err = installer.LoadState()
	if err == nil {
		t.Error("LoadState() should error for invalid JSON")
	}
}

func TestClearStateRemovesStateFile(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(dataDir, dataDir, dataDir, "simtezilo", false, log)

	// First save a state
	state := &updater.InstallState{
		PendingVersion: "2.0.0",
		Status:         updater.InstallStatusPending,
	}

	err := installer.SaveState(state)
	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Clear the state
	err = installer.ClearState()
	if err != nil {
		t.Fatalf("ClearState() error = %v", err)
	}

	// Verify state is gone
	loaded, err := installer.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if loaded != nil {
		t.Error("State should be nil after ClearState()")
	}
}

func TestClearStateSucceedsWhenNoStateExists(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(dataDir, dataDir, dataDir, "simtezilo", false, log)

	// Should not error when state doesn't exist
	err := installer.ClearState()
	if err != nil {
		t.Fatalf("ClearState() should not error for non-existent state: %v", err)
	}
}

func TestPrepareStagesUpdateForInstallation(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(dataDir, dataDir, dataDir, "simtezilo", false, log)

	// Create a fake download file
	downloadPath := filepath.Join(dataDir, "simtezilo.new")

	err := os.WriteFile(downloadPath, []byte("binary content"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create download file: %v", err)
	}

	info := &updater.UpdateInfo{
		AvailableVersion: "2.0.0",
		SHA256:           "abc123",
	}

	err = installer.Prepare(downloadPath, info, "1.0.0")
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	// Verify state was saved
	state, err := installer.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if state == nil {
		t.Fatal("State should not be nil after Prepare()")
	}

	if state.PendingVersion != "2.0.0" {
		t.Errorf("PendingVersion = %v, want 2.0.0", state.PendingVersion)
	}

	if state.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %v, want 1.0.0", state.CurrentVersion)
	}

	if state.Status != updater.InstallStatusPending {
		t.Errorf("Status = %v, want pending", state.Status)
	}
}

func TestPrepareFailsWhenDownloadFileMissing(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(dataDir, dataDir, dataDir, "simtezilo", false, log)

	info := &updater.UpdateInfo{
		AvailableVersion: "2.0.0",
	}

	err := installer.Prepare("/nonexistent/file", info, "1.0.0")
	if err == nil {
		t.Error("Prepare() should error for missing download file")
	}
}

func TestApplyPendingUpdateInstallsNewBinaryAndCreatesRollback(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(installDir, installDir, dataDir, "simtezilo", false, log)

	// Create current binary
	currentBinaryPath := filepath.Join(installDir, "simtezilo")

	err := os.WriteFile(currentBinaryPath, []byte("old binary"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create current binary: %v", err)
	}

	// Create new binary in download location
	downloadPath := filepath.Join(dataDir, "simtezilo.new")

	err = os.WriteFile(downloadPath, []byte("new binary"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create new binary: %v", err)
	}

	// Save pending state
	state := &updater.InstallState{
		PendingVersion: "2.0.0",
		CurrentVersion: "1.0.0",
		DownloadPath:   downloadPath,
		SHA256:         "abc123",
		Status:         updater.InstallStatusPending,
	}

	err = installer.SaveState(state)
	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Apply update
	err = installer.ApplyPendingUpdate()
	if err != nil {
		t.Fatalf("ApplyPendingUpdate() error = %v", err)
	}

	// Verify new binary is installed
	content, err := os.ReadFile(currentBinaryPath)
	if err != nil {
		t.Fatalf("Failed to read installed binary: %v", err)
	}

	if string(content) != "new binary" {
		t.Errorf("Installed binary content = %v, want 'new binary'", string(content))
	}

	// Verify rollback exists
	rollbackPath := filepath.Join(installDir, "simtezilo.rollback")

	content, err = os.ReadFile(rollbackPath)
	if err != nil {
		t.Fatalf("Failed to read rollback binary: %v", err)
	}

	if string(content) != "old binary" {
		t.Errorf("Rollback binary content = %v, want 'old binary'", string(content))
	}

	// Verify state was updated
	state, err = installer.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if state.Status != updater.InstallStatusComplete {
		t.Errorf("Status = %v, want complete", state.Status)
	}
}

func TestApplyPendingUpdateSucceedsWhenNoPendingUpdate(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(dataDir, dataDir, dataDir, "simtezilo", false, log)

	// Should not error when no pending update
	err := installer.ApplyPendingUpdate()
	if err != nil {
		t.Fatalf("ApplyPendingUpdate() should not error when no pending: %v", err)
	}
}

func TestRollbackRestoresPreviousVersion(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(installDir, installDir, dataDir, "simtezilo", false, log)

	// Create current binary
	currentBinaryPath := filepath.Join(installDir, "simtezilo")

	err := os.WriteFile(currentBinaryPath, []byte("new binary"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create current binary: %v", err)
	}

	// Create rollback binary
	rollbackPath := filepath.Join(installDir, "simtezilo.rollback")

	err = os.WriteFile(rollbackPath, []byte("old binary"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create rollback binary: %v", err)
	}

	// Perform rollback
	err = installer.Rollback()
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	// Verify rollback binary is now current
	content, err := os.ReadFile(currentBinaryPath)
	if err != nil {
		t.Fatalf("Failed to read current binary: %v", err)
	}

	if string(content) != "old binary" {
		t.Errorf("Current binary content = %v, want 'old binary'", string(content))
	}
}

func TestRollbackFailsWhenNoRollbackBinaryExists(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(installDir, installDir, dataDir, "simtezilo", false, log)

	err := installer.Rollback()
	if err == nil {
		t.Error("Rollback() should error when no rollback binary exists")
	}
}

func TestRollbackAvailableReturnsTrueWhenRollbackExists(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(installDir, installDir, dataDir, "simtezilo", false, log)

	// Should be false when no rollback binary
	if installer.RollbackAvailable() {
		t.Error("RollbackAvailable() should be false when no rollback binary")
	}

	// Create rollback binary
	rollbackPath := filepath.Join(installDir, "simtezilo.rollback")

	err := os.WriteFile(rollbackPath, []byte("old binary"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create rollback binary: %v", err)
	}

	// Should now be true
	if !installer.RollbackAvailable() {
		t.Error("RollbackAvailable() should be true when rollback binary exists")
	}
}

func TestRollbackVersionReturnsVersionFromState(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(installDir, installDir, dataDir, "simtezilo", false, log)

	// Should return empty when no state
	version := installer.RollbackVersion()
	if version != "" {
		t.Errorf("RollbackVersion() = %v, want empty string", version)
	}

	// Save state with current version
	state := &updater.InstallState{
		PendingVersion: "2.0.0",
		CurrentVersion: "1.5.0",
		Status:         updater.InstallStatusComplete,
	}

	err := installer.SaveState(state)
	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Should return current version from state
	version = installer.RollbackVersion()
	if version != "1.5.0" {
		t.Errorf("RollbackVersion() = %v, want 1.5.0", version)
	}
}

func TestShouldAutoRollbackReturnsTrueWhenFailCountExceedsMax(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(installDir, installDir, dataDir, "simtezilo", false, log)

	// Should be false when no state
	if installer.ShouldAutoRollback(3) {
		t.Error("ShouldAutoRollback() should be false with no state")
	}

	// Save state with fail count below threshold
	state := &updater.InstallState{
		Status:    "failed",
		FailCount: 2,
	}

	err := installer.SaveState(state)
	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	if installer.ShouldAutoRollback(3) {
		t.Error("ShouldAutoRollback() should be false when fail count < max")
	}

	// Update to exceed threshold
	state.FailCount = 3

	err = installer.SaveState(state)
	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	if !installer.ShouldAutoRollback(3) {
		t.Error("ShouldAutoRollback() should be true when fail count >= max")
	}
}

func TestConfirmSuccessRemovesRollbackAndClearsState(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(installDir, installDir, dataDir, "simtezilo", false, log)

	// Create rollback binary
	rollbackPath := filepath.Join(installDir, "simtezilo.rollback")

	err := os.WriteFile(rollbackPath, []byte("old binary"), 0o755) //nolint:gosec // binary executable file
	if err != nil {
		t.Fatalf("Failed to create rollback binary: %v", err)
	}

	// Save complete state
	state := &updater.InstallState{
		PendingVersion: "2.0.0",
		Status:         updater.InstallStatusComplete,
	}

	err = installer.SaveState(state)
	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Confirm success
	err = installer.ConfirmSuccess()
	if err != nil {
		t.Fatalf("ConfirmSuccess() error = %v", err)
	}

	// Verify rollback binary was removed
	_, err = os.Stat(rollbackPath)
	if err == nil {
		t.Error("Rollback binary should be removed after ConfirmSuccess()")
	}

	// Verify state was cleared
	loadedState, err := installer.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if loadedState != nil {
		t.Error("State should be cleared after ConfirmSuccess()")
	}
}

func TestConfirmSuccessDoesNothingWhenStatusNotComplete(t *testing.T) {
	t.Parallel()

	installDir := t.TempDir()
	dataDir := t.TempDir()
	log := zerolog.Nop()

	installer := updater.NewInstaller(installDir, installDir, dataDir, "simtezilo", false, log)

	// Save pending state (not complete)
	state := &updater.InstallState{
		PendingVersion: "2.0.0",
		Status:         updater.InstallStatusPending,
	}

	err := installer.SaveState(state)
	if err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// ConfirmSuccess should do nothing when not complete
	err = installer.ConfirmSuccess()
	if err != nil {
		t.Fatalf("ConfirmSuccess() error = %v", err)
	}

	// Verify state was NOT cleared
	loadedState, err := installer.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if loadedState == nil {
		t.Error("State should not be cleared when status != complete")
	}
}

func TestInstallStateSerializesToAndFromJSON(t *testing.T) {
	t.Parallel()

	state := &updater.InstallState{
		PendingVersion: "2.0.0",
		CurrentVersion: "1.0.0",
		DownloadPath:   "/path/to/binary",
		SHA256:         "abc123",
		Timestamp:      time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		Status:         updater.InstallStatusPending,
		FailCount:      1,
		LastError:      "some error",
	}

	// Marshal to JSON
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Failed to marshal state: %v", err)
	}

	// Unmarshal back
	var loaded updater.InstallState

	err = json.Unmarshal(data, &loaded)
	if err != nil {
		t.Fatalf("Failed to unmarshal state: %v", err)
	}

	if loaded.PendingVersion != state.PendingVersion {
		t.Errorf("PendingVersion = %v, want %v", loaded.PendingVersion, state.PendingVersion)
	}

	if loaded.LastError != state.LastError {
		t.Errorf("LastError = %v, want %v", loaded.LastError, state.LastError)
	}
}
