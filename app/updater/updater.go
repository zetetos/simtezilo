package updater

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Config holds the updater configuration.
type Config struct {
	Enabled         bool
	BaseURL         string
	Channel         string
	CheckInterval   time.Duration
	HTTPTimeout     time.Duration
	DownloadTimeout time.Duration
	AutoInstall     bool
	InstallDir      string
	InitDir         string
	DataDir         string
	BinaryName      string
	ServiceName     string
	UseSystemd      bool
}

// Updater is the main entry point for the update system.
type Updater struct {
	log zerolog.Logger
	cfg *Config

	checker    *Checker
	downloader *Downloader
	installer  *Installer

	currentVersion string
}

// New creates a new Updater instance.
func New(cfg *Config, currentVersion string, log zerolog.Logger) (*Updater, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	if cfg.BaseURL == "" {
		return nil, errors.New("manifest URL is required")
	}

	logger := log.With().Str("component", "updater").Logger()

	checker, err := NewChecker(CheckerConfig{
		BaseURL:        cfg.BaseURL,
		CheckInterval:  cfg.CheckInterval,
		HTTPTimeout:    cfg.HTTPTimeout,
		Channel:        cfg.Channel,
		CurrentVersion: currentVersion,
		DataDir:        cfg.DataDir,
	}, logger)
	if err != nil {
		return nil, err
	}

	downloadDir := filepath.Join(cfg.DataDir, "downloads")
	downloader := NewDownloader(downloadDir, cfg.DownloadTimeout, logger)

	installer := NewInstaller(cfg.InstallDir, cfg.InitDir, cfg.DataDir, cfg.BinaryName, cfg.UseSystemd, logger)

	return &Updater{
		log:            logger,
		cfg:            cfg,
		checker:        checker,
		downloader:     downloader,
		installer:      installer,
		currentVersion: currentVersion,
	}, nil
}

// Start begins the update checker if updates are enabled.
func (u *Updater) Start(lifecycleCTX context.Context) {
	if !u.cfg.Enabled {
		u.log.Debug().Msg("Updates disabled")

		return
	}

	// Check if we should auto-rollback due to previous failures
	if u.installer.ShouldAutoRollback(DefaultMaxFailures) {
		u.log.Warn().Msg("Too many failed starts, initiating auto-rollback")

		err := u.installer.Rollback()
		if err != nil {
			u.log.Error().Err(err).Msg("Auto-rollback failed")
		}
	}

	// Confirm successful start after an update
	err := u.installer.ConfirmSuccess()
	if err != nil {
		u.log.Warn().Err(err).Msg("Failed to confirm successful update")
	}

	// Check for existing downloads that might be ready to install
	u.CheckExistingDownloads() //nolint:contextcheck // context is for managing lifecycle only

	u.log.Info().
		Str("manifestURL", u.cfg.BaseURL).
		Str("channel", u.cfg.Channel).
		Dur("checkInterval", u.cfg.CheckInterval).
		Msg("Starting update checker")

	u.checker.Start(lifecycleCTX)
}

// Stop gracefully stops the updater.
func (u *Updater) Stop() {
	if u.checker != nil {
		u.checker.Stop()
	}
}

// CheckNow performs an immediate update check.
func (u *Updater) CheckNow() (*UpdateInfo, error) {
	return u.checker.CheckNow()
}

// Status returns the current update status.
func (u *Updater) Status() UpdateStatus {
	return u.checker.Status()
}

// SetStatus updates the current update status.
func (u *Updater) SetStatus(status UpdateStatus) {
	u.checker.SetStatus(status)
}

// AvailableUpdate returns information about an available update.
func (u *Updater) AvailableUpdate() *UpdateInfo {
	return u.checker.AvailableUpdate()
}

// DownloadUpdate downloads the available update.
func (u *Updater) DownloadUpdate(ctx context.Context, progressCb ProgressCallback) (string, error) {
	info := u.checker.AvailableUpdate()
	if info == nil {
		return "", errors.New("no update available")
	}

	u.checker.SetStatus(UpdateStatusDownloading)

	path, err := u.downloader.Download(ctx, info, progressCb)
	if err != nil {
		u.checker.SetStatus(UpdateStatusError)

		return "", err
	}

	u.checker.SetStatus(UpdateStatusReadyToInstall)

	return path, nil
}

// PrepareInstall stages the downloaded update for installation on next restart.
func (u *Updater) PrepareInstall(downloadPath string) error {
	info := u.checker.AvailableUpdate()
	if info == nil {
		return errors.New("no update available")
	}

	return u.installer.Prepare(downloadPath, info, u.currentVersion)
}

// DownloadAndPrepare is a convenience method that downloads and prepares an update.
func (u *Updater) DownloadAndPrepare(ctx context.Context, progressCb ProgressCallback) error {
	path, err := u.DownloadUpdate(ctx, progressCb)
	if err != nil {
		return err
	}

	return u.PrepareInstall(path)
}

// Rollback reverts to the previous version.
func (u *Updater) Rollback() error {
	return u.installer.Rollback()
}

// RollbackAvailable returns true if a previous version is available for rollback.
func (u *Updater) RollbackAvailable() bool {
	return u.installer.RollbackAvailable()
}

// RollbackVersion returns the version that would be rolled back to.
func (u *Updater) RollbackVersion() string {
	return u.installer.RollbackVersion()
}

// Installer returns the installer for use by external scripts.
func (u *Updater) Installer() *Installer {
	return u.installer
}

// Checker returns the update checker.
func (u *Updater) Checker() *Checker {
	return u.checker
}

// Downloader returns the downloader for checking download status.
func (u *Updater) Downloader() *Downloader {
	return u.downloader
}

// SetChannel updates the channel for update checking.
func (u *Updater) SetChannel(channel string) {
	if u.checker != nil {
		u.checker.SetChannel(channel)
	}
}

// CheckExistingDownloads checks if there are any valid downloaded updates in the download directory.
// If a valid newer version is found, it sets the status to ReadyToInstall.
func (u *Updater) CheckExistingDownloads() {
	// First, check the installer state for a pending update
	// This handles the case where a download completed and was prepared but the app restarted
	state, err := u.installer.LoadState()
	if err != nil {
		u.log.Warn().Err(err).Msg("Could not load installer state")
	}

	currentChannel := u.checker.Channel()

	if state != nil && state.Status == InstallStatusPending {
		// Determine the channel of the pending update
		// Use explicit channel if available, otherwise infer from version or filename
		pendingChannel := state.Channel
		if pendingChannel == "" {
			// Check if this is a custom upload (filename starts with "custom-")
			if strings.Contains(state.DownloadPath, "/custom-") {
				pendingChannel = "custom"
			} else {
				pendingVer, parseErr := ParseVersion(state.PendingVersion)
				if parseErr == nil {
					pendingChannel = pendingVer.InferredChannel()
				}
			}
		}

		// Only restore if the channel matches
		if pendingChannel != currentChannel {
			u.log.Info().
				Str("version", state.PendingVersion).
				Str("pendingChannel", pendingChannel).
				Str("currentChannel", currentChannel).
				Msg("Found pending update from previous session but channel doesn't match, ignoring")

			return
		}

		u.log.Info().
			Str("version", state.PendingVersion).
			Str("channel", pendingChannel).
			Str("path", state.DownloadPath).
			Msg("Found pending update from previous session")

		// Set the available update info from the state
		u.checker.SetAvailableUpdate(&UpdateInfo{
			CurrentVersion:   state.CurrentVersion,
			AvailableVersion: state.PendingVersion,
			Channel:          pendingChannel,
			SHA256:           state.SHA256,
		})
		u.checker.SetStatus(UpdateStatusReadyToInstall)

		return
	}

	// No pending state, check for updates to get the latest version info
	updateInfo, err := u.checker.CheckNow()
	if err != nil {
		u.log.Debug().Err(err).Msg("Could not check for updates during startup")

		return
	}

	// If no update is available (already on latest or newer), nothing to check
	if updateInfo == nil {
		u.log.Debug().Msg("No updates available, skipping download check")

		return
	}

	// Check if the download already exists
	exists, downloadPath := u.downloader.DownloadExists(updateInfo)
	if !exists {
		u.log.Debug().Msg("No existing download found")

		return
	}

	u.log.Info().
		Str("path", downloadPath).
		Str("version", updateInfo.AvailableVersion).
		Msg("Found existing download")

	// Verify the downloaded file matches the expected checksum
	if updateInfo.SHA256 != "" {
		valid, err := u.downloader.VerifyFile(downloadPath, updateInfo.SHA256)
		if err != nil {
			u.log.Warn().Err(err).Str("path", downloadPath).Msg("Could not verify existing download")

			return
		}

		if !valid {
			u.log.Warn().Str("path", downloadPath).Msg("Existing download checksum mismatch, ignoring")

			return
		}
	}

	// Set status to ready to install (this preserves the availableInfo from CheckNow)
	u.checker.SetStatus(UpdateStatusReadyToInstall)
	u.log.Info().
		Str("version", updateInfo.AvailableVersion).
		Msg("Existing download is valid and ready to install")
}
