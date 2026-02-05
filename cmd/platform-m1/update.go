package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/zetetos/simtezilo/app/exitcode"
)

const (
	// Update binary name.
	updateBinaryName = "simtezilo"

	// Update status constants.
	updateStatusPending    = "pending"
	updateStatusInstalling = "installing"
	updateStatusComplete   = "complete"
	updateStatusFailed     = "failed"
	updateStatusRolledBack = "rolled_back"
)

// updateState tracks the state of a pending installation (matches app/updater/installer.go).
type updateState struct {
	PendingVersion string    `json:"pendingVersion"`
	CurrentVersion string    `json:"currentVersion"`
	Channel        string    `json:"channel"`
	DownloadPath   string    `json:"downloadPath"`
	ExtractDir     string    `json:"extractDir"`
	SHA256         string    `json:"sha256"`
	Timestamp      time.Time `json:"timestamp"`
	Status         string    `json:"status"`
	FailCount      int       `json:"failCount"`
	LastError      string    `json:"lastError"`
}

// stagedFile tracks a file that has been moved to staging during installation.
type stagedFile struct {
	originalPath string
	stagingPath  string
}

// loadUpdateState reads and parses the update state file from disk.
// Returns nil with no error if the state file does not exist.
func (p *manager) loadUpdateState() (*updateState, error) {
	data, err := os.ReadFile(p.stateFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state updateState

	err = json.Unmarshal(data, &state)
	if err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// saveUpdateState persists the update state to disk as JSON, creating the
// data directory if it doesn't exist.
func (p *manager) saveUpdateState(state *updateState) error {
	err := os.MkdirAll(p.updateDir(), 0o755)
	if err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	err = os.WriteFile(p.stateFile(), data, 0o600)
	if err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// clearUpdateState removes the update state file from disk.
func (p *manager) clearUpdateState() error {
	err := os.Remove(p.stateFile())
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove state file: %w", err)
	}

	return nil
}

// updateApply processes a pending update based on the current update state.
// It handles pending, complete, failed, and rolled back states appropriately.
func (p *manager) updateApply() exitcode.Code {
	state, err := p.loadUpdateState()
	if err != nil {
		p.log.Error().Err(err).Msg("Failed to load update state")

		outputJSON(map[string]any{
			"result": "failure",
			"error":  err.Error(),
		})

		return exitcode.GeneralErr
	}

	if state == nil {
		p.log.Debug().Msg("No update state file found, nothing to do")

		outputJSON(map[string]any{
			"result": "success",
			"action": "none",
		})

		return exitcode.Success
	}

	p.log.Info().
		Str("status", state.Status).
		Str("pending", state.PendingVersion).
		Str("current", state.CurrentVersion).
		Int("failCount", state.FailCount).
		Msg("Update state loaded")

	switch state.Status {
	case updateStatusPending:
		return p.applyPendingUpdate(state)
	case updateStatusInstalling:
		return p.handleInstallingState(state)
	case updateStatusComplete:
		return p.handleCompleteState(state)
	case updateStatusFailed:
		return p.handleFailedState(state)
	case updateStatusRolledBack:
		p.log.Info().Str("status", state.Status).Msg("No action needed for current state")

		outputJSON(map[string]any{
			"result": "success",
			"action": "none",
			"status": state.Status,
		})

		return exitcode.Success
	default:
		p.log.Warn().Str("status", state.Status).Msg("Unknown update state")

		outputJSON(map[string]any{
			"result": "success",
			"action": "none",
			"status": state.Status,
		})

		return exitcode.Success
	}
}

// applyPendingUpdate processes an update in pending state by verifying the
// download, extracting the archive, and installing the new binary.
func (p *manager) applyPendingUpdate(state *updateState) exitcode.Code {
	p.log.Info().
		Str("from", state.CurrentVersion).
		Str("to", state.PendingVersion).
		Msg("Applying pending update")

	// Verify download exists and checksum
	if code := p.verifyUpdateDownload(state); code != exitcode.Success {
		return code
	}

	// Mark as installing
	state.Status = updateStatusInstalling

	err := p.saveUpdateState(state)
	if err != nil {
		p.log.Warn().Err(err).Msg("Failed to update state to installing")
	}

	// Extract and install update
	return p.extractAndInstallUpdate(state)
}

// verifyUpdateDownload checks that the downloaded update file exists and
// verifies its SHA256 checksum if one was provided in the state.
func (p *manager) verifyUpdateDownload(state *updateState) exitcode.Code {
	if state.DownloadPath == "" {
		return p.markUpdateFailed(state, "download path is empty")
	}

	_, err := os.Stat(state.DownloadPath)
	if err != nil {
		return p.markUpdateFailed(state, fmt.Sprintf("download file not found: %v", err))
	}

	if state.SHA256 != "" {
		checkErr := p.verifyChecksum(state.DownloadPath, state.SHA256)
		if checkErr != nil {
			return p.markUpdateFailed(state, fmt.Sprintf("checksum verification failed: %v", checkErr))
		}

		p.log.Debug().Msg("Checksum verified")
	}

	return exitcode.Success
}

// extractAndInstallUpdate extracts the downloaded archive to a temporary directory
// and proceeds with installation of the extracted files.
func (p *manager) extractAndInstallUpdate(state *updateState) exitcode.Code {
	extractDir := state.ExtractDir
	if extractDir == "" {
		extractDir = filepath.Join(p.updateDir(), "extract")
	}

	_ = os.RemoveAll(extractDir)

	err := os.MkdirAll(extractDir, 0o755)
	if err != nil {
		return p.markUpdateFailed(state, fmt.Sprintf("failed to create extract directory: %v", err))
	}

	p.log.Info().Str("archive", state.DownloadPath).Str("dest", extractDir).Msg("Extracting archive")

	err = p.extractArchive(state.DownloadPath, extractDir)
	if err != nil {
		_ = os.RemoveAll(extractDir)

		return p.markUpdateFailed(state, fmt.Sprintf("failed to extract archive: %v", err))
	}

	extractRoot := p.findExtractRoot(extractDir)

	return p.installExtractedUpdate(state, extractDir, extractRoot)
}

// installExtractedUpdate installs the extracted update using atomic moves with a staging directory.
// Original files are moved to staging first, then new files are moved to their destinations.
// If any step fails, files are restored from staging before marking the update as failed.
// Binary files are installed last to ensure a clean rollback is possible.
func (p *manager) installExtractedUpdate(state *updateState, extractDir, extractRoot string) exitcode.Code {
	extractedBinary := filepath.Join(extractRoot, "bin", updateBinaryName)

	_, err := os.Stat(extractedBinary)
	if err != nil {
		_ = os.RemoveAll(extractDir)

		return p.markUpdateFailed(state, "extracted binary not found at "+extractedBinary)
	}

	// Clean up and create staging directory
	stagingDir := p.stagingDir()
	_ = os.RemoveAll(stagingDir)

	err = os.MkdirAll(stagingDir, 0o755)
	if err != nil {
		_ = os.RemoveAll(extractDir)

		return p.markUpdateFailed(state, fmt.Sprintf("failed to create staging directory: %v", err))
	}

	// Track files that have been moved to staging for potential rollback
	var stagedFiles []stagedFile

	// Install init scripts first (non-critical, but install early)
	initSourceDir := filepath.Join(extractRoot, "init")
	stagedFiles = p.installInitScriptsFromExtract(initSourceDir, stagingDir, stagedFiles)

	// Install config files (only if they don't exist)
	etcSourceDir := filepath.Join(extractRoot, "etc")
	stagedFiles = p.installConfigFilesFromExtract(etcSourceDir, stagingDir, stagedFiles)

	// Install additional binaries (excluding main binary, which is installed last)
	binSourceDir := filepath.Join(extractRoot, "bin")
	stagedFiles = p.installAdditionalBinariesFromExtract(binSourceDir, stagingDir, stagedFiles)

	// Install main binary LAST (critical step)
	currentBinary := filepath.Join(p.installDir(), updateBinaryName)
	p.log.Debug().Str("from", extractedBinary).Str("to", currentBinary).Msg("Installing new binary")

	stagedFiles, err = p.installFileToStaging(extractedBinary, currentBinary, filepath.Join("bin", updateBinaryName), stagingDir, stagedFiles)
	if err != nil {
		p.restoreStagedFiles(stagedFiles, stagingDir)

		_ = os.RemoveAll(extractDir)

		return p.markUpdateFailed(state, fmt.Sprintf("failed to install new binary: %v", err))
	}

	err = os.Chmod(currentBinary, 0o755)
	if err != nil {
		p.log.Warn().Err(err).Msg("Failed to set executable permissions")
	}

	// Installation successful - create rollback archive from staging, then clean up
	err = p.createRollbackArchiveFromStaging(stagingDir)
	if err != nil {
		p.log.Warn().Err(err).Msg("Failed to create rollback archive from staging")
		// Continue anyway - rollback archive is a safety feature, not critical
	}

	// Clean up staging and extract directories
	_ = os.RemoveAll(stagingDir)
	_ = os.RemoveAll(extractDir)
	_ = os.Remove(state.DownloadPath)

	state.Status = updateStatusComplete

	err = p.saveUpdateState(state)
	if err != nil {
		p.log.Warn().Err(err).Msg("Failed to save completion state")
	}

	p.log.Info().Str("version", state.PendingVersion).Msg("Update installed successfully")

	outputJSON(map[string]any{
		"result":  "success",
		"action":  "installed",
		"version": state.PendingVersion,
	})

	return exitcode.Success
}

// installFileToStaging moves an original file to staging and installs a new file in its place.
// Returns the updated list of staged files for rollback tracking.
func (p *manager) installFileToStaging(sourcePath, destPath, stagingSubPath, stagingDir string, stagedFiles []stagedFile) ([]stagedFile, error) {
	_, statErr := os.Stat(sourcePath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return stagedFiles, nil // Source doesn't exist, nothing to install
		}

		return stagedFiles, fmt.Errorf("failed to stat source: %w", statErr)
	}

	// Create staging subdirectory if needed
	stagingPath := filepath.Join(stagingDir, stagingSubPath)
	stagingParent := filepath.Dir(stagingPath)

	mkErr := os.MkdirAll(stagingParent, 0o755)
	if mkErr != nil {
		return stagedFiles, fmt.Errorf("failed to create staging subdirectory: %w", mkErr)
	}

	// Move original to staging if it exists
	_, destStatErr := os.Stat(destPath)
	if destStatErr == nil {
		renameErr := os.Rename(destPath, stagingPath)
		if renameErr != nil {
			return stagedFiles, fmt.Errorf("failed to move original to staging: %w", renameErr)
		}

		stagedFiles = append(stagedFiles, stagedFile{originalPath: destPath, stagingPath: stagingPath})
	}

	// Move new file to destination
	destParent := filepath.Dir(destPath)

	mkDestErr := os.MkdirAll(destParent, 0o755)
	if mkDestErr != nil {
		return stagedFiles, fmt.Errorf("failed to create destination directory: %w", mkDestErr)
	}

	renameErr := os.Rename(sourcePath, destPath)
	if renameErr != nil {
		return stagedFiles, fmt.Errorf("failed to move new file to destination: %w", renameErr)
	}

	return stagedFiles, nil
}

// restoreStagedFiles restores all files from staging back to their original locations.
// Used to rollback a partial installation on failure.
func (p *manager) restoreStagedFiles(stagedFiles []stagedFile, stagingDir string) {
	p.log.Info().Int("count", len(stagedFiles)).Msg("Restoring files from staging")

	for idx := len(stagedFiles) - 1; idx >= 0; idx-- {
		staged := stagedFiles[idx]

		renameErr := os.Rename(staged.stagingPath, staged.originalPath)
		if renameErr != nil {
			p.log.Warn().
				Err(renameErr).
				Str("staging", staged.stagingPath).
				Str("original", staged.originalPath).
				Msg("Failed to restore file from staging")
		}
	}

	_ = os.RemoveAll(stagingDir)
}

// installInitScriptsFromExtract installs init scripts from the extract directory.
func (p *manager) installInitScriptsFromExtract(initSourceDir, stagingDir string, stagedFiles []stagedFile) []stagedFile {
	entries, readErr := os.ReadDir(initSourceDir)
	if readErr != nil {
		return stagedFiles
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		sourcePath := filepath.Join(initSourceDir, entry.Name())
		destPath := filepath.Join(p.initDir(), entry.Name())
		stagingSubPath := filepath.Join("init", entry.Name())

		var installErr error

		stagedFiles, installErr = p.installFileToStaging(sourcePath, destPath, stagingSubPath, stagingDir, stagedFiles)
		if installErr != nil {
			p.log.Warn().Err(installErr).Str("file", entry.Name()).Msg("Failed to install init script")
		}
	}

	return stagedFiles
}

// installConfigFilesFromExtract installs config files from the extract directory.
// Only installs files that don't already exist (won't overwrite user config).
func (p *manager) installConfigFilesFromExtract(etcSourceDir, stagingDir string, stagedFiles []stagedFile) []stagedFile {
	entries, readErr := os.ReadDir(etcSourceDir)
	if readErr != nil {
		return stagedFiles
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		destPath := filepath.Join(p.etcDir(), entry.Name())

		// Only install config if it doesn't exist (don't overwrite user config)
		_, destStatErr := os.Stat(destPath)
		if destStatErr == nil {
			continue
		}

		sourcePath := filepath.Join(etcSourceDir, entry.Name())
		stagingSubPath := filepath.Join("etc", entry.Name())

		var installErr error

		stagedFiles, installErr = p.installFileToStaging(sourcePath, destPath, stagingSubPath, stagingDir, stagedFiles)
		if installErr != nil {
			p.log.Warn().Err(installErr).Str("file", entry.Name()).Msg("Failed to install config file")
		}
	}

	return stagedFiles
}

// installAdditionalBinariesFromExtract installs additional binaries from the extract directory.
// Excludes the main binary which is installed separately.
func (p *manager) installAdditionalBinariesFromExtract(binSourceDir, stagingDir string, stagedFiles []stagedFile) []stagedFile {
	entries, readErr := os.ReadDir(binSourceDir)
	if readErr != nil {
		return stagedFiles
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == updateBinaryName {
			continue
		}

		sourcePath := filepath.Join(binSourceDir, entry.Name())
		destPath := filepath.Join(p.installDir(), entry.Name())
		stagingSubPath := filepath.Join("bin", entry.Name())

		var installErr error

		stagedFiles, installErr = p.installFileToStaging(sourcePath, destPath, stagingSubPath, stagingDir, stagedFiles)
		if installErr != nil {
			p.log.Warn().Err(installErr).Str("file", entry.Name()).Msg("Failed to install additional binary")
			// Non-critical, continue
		} else {
			// Set executable permissions
			_ = os.Chmod(destPath, 0o755)
		}
	}

	return stagedFiles
}

// verifyChecksum calculates the SHA256 hash of a file and compares it against
// the expected value, returning an error if they don't match.
func (p *manager) verifyChecksum(filePath, expected string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()

	_, err = io.Copy(hasher, file)
	if err != nil {
		return fmt.Errorf("failed to hash file: %w", err)
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}

	return nil
}

// findExtractRoot determines the root directory of the extracted archive contents,
// checking for a "simtezilo" subdirectory or returning the extract directory itself.
func (p *manager) findExtractRoot(extractDir string) string {
	simteziloDir := filepath.Join(extractDir, "simtezilo")

	_, err := os.Stat(simteziloDir)
	if err == nil {
		return simteziloDir
	}

	return extractDir
}

// handleCompleteState handles an update that has already completed successfully
// by cleaning up the rollback archive, failed start counter, and clearing the state file.
func (p *manager) handleCompleteState(_ *updateState) exitcode.Code {
	p.log.Info().Msg("Update already complete, cleaning up")

	// Remove rollback archive
	_ = os.Remove(p.rollbackArchive())

	// Failed start counter
	_ = os.Remove(p.failedStartCounter())

	// Clear state
	_ = p.clearUpdateState()

	outputJSON(map[string]any{
		"result": "success",
		"action": "cleanup",
	})

	return exitcode.Success
}

// handleFailedState reports the failed update status without retrying.
// The user must manually trigger a retry via the web UI or delete the failed update.
func (p *manager) handleFailedState(state *updateState) exitcode.Code {
	p.log.Warn().
		Str("lastError", state.LastError).
		Str("version", state.PendingVersion).
		Msg("Update in failed state, awaiting user action")

	outputJSON(map[string]any{
		"result":    "failure",
		"status":    state.Status,
		"version":   state.PendingVersion,
		"lastError": state.LastError,
	})

	return exitcode.Success // Return success to not block service startup
}

// handleInstallingState handles an interrupted installation by restoring files
// from the staging directory and marking the update as failed.
func (p *manager) handleInstallingState(state *updateState) exitcode.Code {
	p.log.Warn().
		Str("version", state.PendingVersion).
		Msg("Found interrupted installation, restoring from staging")

	stagingDir := p.stagingDir()

	// Check if staging directory exists
	_, statErr := os.Stat(stagingDir)
	if os.IsNotExist(statErr) {
		p.log.Warn().Msg("No staging directory found, cannot restore - marking as failed")

		return p.markUpdateFailed(state, "interrupted installation with no staging directory")
	}

	// Walk the staging directory and restore files to their original locations
	restoredCount := 0

	restoreErr := filepath.Walk(stagingDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if info.IsDir() {
			return nil
		}

		// Calculate the relative path from staging directory
		relPath, relErr := filepath.Rel(stagingDir, path)
		if relErr != nil {
			p.log.Warn().Err(relErr).Str("path", path).Msg("Failed to get relative path")

			return nil
		}

		// Calculate the original destination path
		destPath := filepath.Join(p.baseDir, relPath)

		// Ensure destination directory exists
		destDir := filepath.Dir(destPath)

		mkErr := os.MkdirAll(destDir, 0o755)
		if mkErr != nil {
			p.log.Warn().Err(mkErr).Str("dir", destDir).Msg("Failed to create destination directory")

			return nil
		}

		// Move file from staging back to original location
		renameErr := os.Rename(path, destPath)
		if renameErr != nil {
			p.log.Warn().
				Err(renameErr).
				Str("staging", path).
				Str("dest", destPath).
				Msg("Failed to restore file from staging")

			return nil
		}

		p.log.Debug().Str("file", relPath).Msg("Restored file from staging")

		restoredCount++

		return nil
	})
	if restoreErr != nil {
		p.log.Warn().Err(restoreErr).Msg("Error walking staging directory")
	}

	p.log.Info().Int("count", restoredCount).Msg("Files restored from staging")

	// Clean up staging directory and extract directory
	_ = os.RemoveAll(stagingDir)

	extractDir := state.ExtractDir
	if extractDir == "" {
		extractDir = filepath.Join(p.updateDir(), "extract")
	}

	_ = os.RemoveAll(extractDir)

	return p.markUpdateFailed(state, "interrupted installation, restored from staging")
}

// markUpdateFailed records a failure in the update state, persisting the
// error reason to the state file. The user must manually trigger a retry.
func (p *manager) markUpdateFailed(state *updateState, reason string) exitcode.Code {
	p.log.Error().Str("reason", reason).Msg("Update failed")

	state.Status = updateStatusFailed
	state.LastError = reason

	err := p.saveUpdateState(state)
	if err != nil {
		p.log.Warn().Err(err).Msg("Failed to save failed state")
	}

	outputJSON(map[string]any{
		"result": "failure",
		"error":  reason,
	})

	return exitcode.GeneralErr
}

// updateRollback restores the previous version from the rollback archive.
func (p *manager) updateRollback() exitcode.Code {
	p.log.Info().Msg("Rolling back from archive")

	// Extract archive directly to base directory, overwriting existing files
	err := p.extractArchive(p.rollbackArchive(), p.baseDir)
	if err != nil {
		p.log.Error().Err(err).Msg("Failed to extract rollback archive")

		outputJSON(map[string]any{
			"result": "failure",
			"error":  fmt.Sprintf("failed to extract rollback archive: %v", err),
		})

		return exitcode.GeneralErr
	}

	// Update state
	state, _ := p.loadUpdateState()
	if state != nil {
		state.Status = updateStatusRolledBack
		_ = p.saveUpdateState(state)
	}

	p.log.Info().Msg("Rollback complete")

	outputJSON(map[string]any{
		"result": "success",
		"action": "rolled_back",
	})

	return exitcode.Success
}
