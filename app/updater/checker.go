package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// UpdateStatus represents the current state of the updater.
type UpdateStatus int

const (
	UpdateStatusIdle UpdateStatus = iota
	UpdateStatusChecking
	UpdateStatusUpdateAvailable
	UpdateStatusDownloading
	UpdateStatusReadyToInstall
	UpdateStatusInstalling
	UpdateStatusError
)

func (s UpdateStatus) String() string {
	return [...]string{
		"idle",
		"checking",
		"update_available",
		"downloading",
		"ready_to_install",
		"installing",
		"error",
	}[s]
}

// UpdateInfo contains information about an available update.
type UpdateInfo struct {
	CurrentVersion   string
	AvailableVersion string
	Channel          string
	Changelog        []string
	DownloadURL      string
	DownloadSize     int64
	SHA256           string
	ReleaseDate      time.Time
}

// Checker handles update checking and orchestration.
type Checker struct {
	log zerolog.Logger

	baseURL       string // Base URL without channel-specific path
	checkInterval time.Duration
	httpTimeout   time.Duration
	channel       string
	dataDir       string

	currentVersion *Version

	mu               sync.RWMutex
	status           UpdateStatus
	downloadProgress float64 // Download progress percentage (0-100)
	lastCheck        time.Time
	lastError        error
	availableInfo    *UpdateInfo
	cachedManifest   *Manifest

	// Lifecycle management using context and WaitGroup pattern
	// - cancel is called to signal shutdown to the background goroutine
	// - wg tracks when the goroutine has finished cleanup
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// CheckerConfig holds configuration for the update checker.
type CheckerConfig struct {
	BaseURL        string
	CheckInterval  time.Duration
	HTTPTimeout    time.Duration
	Channel        string
	CurrentVersion string
	DataDir        string
}

// Validate checks the configuration for required fields and sensible defaults.
func (cfg CheckerConfig) Validate() error {
	if cfg.BaseURL == "" {
		return errors.New("BaseURL is required")
	}

	if cfg.CheckInterval <= 0 {
		return errors.New("CheckInterval must be positive")
	}

	if cfg.HTTPTimeout <= 0 {
		return errors.New("HTTPTimeout must be positive")
	}

	return nil
}

// NewChecker creates a new update checker.
func NewChecker(cfg CheckerConfig, log zerolog.Logger) (*Checker, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid checker config: %w", err)
	}

	currentVer, err := ParseVersion(cfg.CurrentVersion)
	if err != nil {
		// If version is "dev" or unparseable, create a zero version
		currentVer = &Version{Major: 0, Minor: 0, Patch: 0, Raw: cfg.CurrentVersion}
	}

	return &Checker{
		log:            log.With().Str("component", "updater").Logger(),
		baseURL:        cfg.BaseURL,
		checkInterval:  cfg.CheckInterval,
		httpTimeout:    cfg.HTTPTimeout,
		channel:        cfg.Channel,
		dataDir:        cfg.DataDir,
		currentVersion: currentVer,
		status:         UpdateStatusIdle,
	}, nil
}

// Start begins periodic update checking.
func (c *Checker) Start(ctx context.Context) {
	// Create a cancellable context for this checker
	childCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)

	go c.runPeriodicCheck(childCtx)
}

// Stop gracefully stops the update checker.
func (c *Checker) Stop() {
	if c.cancel != nil {
		c.cancel()
	}

	c.wg.Wait()
}

// CheckNow performs an immediate update check.
func (c *Checker) CheckNow() (*UpdateInfo, error) {
	c.mu.Lock()
	c.status = UpdateStatusChecking
	baseURL := c.baseURL
	channel := c.channel
	dataDir := c.dataDir
	c.mu.Unlock()

	if channel == channelCustom {
		return c.checkCustomUpdate(dataDir)
	}

	manifest, err := c.fetchUpdateManifest(baseURL, channel)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil {
		c.setError(err)
		c.availableInfo = nil
		c.log.Warn().Err(err).Msg("Failed to check for updates")

		return nil, err
	}

	if !c.manifestChannelIsValid(manifest) {
		c.status = UpdateStatusIdle

		return nil, nil
	}

	c.log.Debug().
		Str("version", manifest.Version).
		Str("channel", manifest.Channel).
		Time("releaseDate", manifest.ReleaseDate).
		Int("platforms", len(manifest.Platforms)).
		Msg("Manifest fetched successfully")

	return c.processManifestUpdate(manifest), nil
}

// Status returns the current update status.
func (c *Checker) Status() UpdateStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.status
}

// SetStatus updates the current status (used by Installer).
func (c *Checker) SetStatus(status UpdateStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status = status

	// Reset download progress when status changes away from downloading
	if status != UpdateStatusDownloading {
		c.downloadProgress = 0
	}
}

// DownloadProgress returns the current download progress percentage (0-100).
func (c *Checker) DownloadProgress() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.downloadProgress
}

// SetDownloadProgress updates the current download progress percentage.
func (c *Checker) SetDownloadProgress(percent float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.downloadProgress = percent
}

// SetAvailableUpdate sets the available update information (for custom uploads).
func (c *Checker) SetAvailableUpdate(info *UpdateInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.availableInfo = info
}

// LastCheck returns the time of the last update check.
func (c *Checker) LastCheck() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastCheck
}

// LastError returns the last error encountered during update checking.
func (c *Checker) LastError() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastError
}

// AvailableUpdate returns information about an available update, or nil if none.
func (c *Checker) AvailableUpdate() *UpdateInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.availableInfo
}

// SetAvailableDownloadURL updates the download URL for the available update.
func (c *Checker) SetAvailableDownloadURL(url string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.availableInfo != nil {
		c.availableInfo.DownloadURL = url
	}
}

// CurrentVersion returns the current version string.
func (c *Checker) CurrentVersion() string {
	return c.currentVersion.String()
}

// BaseURL returns the base URL for update checking.
func (c *Checker) BaseURL() string {
	return c.baseURL
}

// Channel returns the current update channel.
func (c *Checker) Channel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.channel
}

// SetChannel updates the channel for update checking.
// This clears any cached update information since it may not apply to the new channel.
func (c *Checker) SetChannel(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel == channel {
		return
	}

	c.channel = channel
	c.availableInfo = nil
	c.lastError = nil
	c.status = UpdateStatusIdle
	c.log.Info().Str("channel", channel).Msg("Update channel changed")
}

// setError sets the checker to error state with the given error.
// Caller must hold the mutex.
func (c *Checker) setError(err error) {
	c.status = UpdateStatusError
	c.lastError = err
}

func (c *Checker) fetchUpdateManifest(baseURL, channel string) (*Manifest, error) {
	manifestURL := fmt.Sprintf("%s/%s/latest.json", baseURL, channel)

	c.log.Debug().
		Str("url", manifestURL).
		Dur("timeout", c.httpTimeout).
		Msg("Fetching update manifest")

	ctx, cancel := context.WithTimeout(context.Background(), c.httpTimeout)
	defer cancel()

	manifest, err := FetchManifest(ctx, manifestURL)
	if err != nil {
		return nil, fmt.Errorf("fetch update manifest: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastCheck = time.Now()
	c.cachedManifest = manifest

	return manifest, nil
}

func (c *Checker) manifestChannelIsValid(manifest *Manifest) bool {
	if manifest.Channel != c.channel && c.channel != "" {
		c.log.Debug().
			Str("expected", c.channel).
			Str("got", manifest.Channel).
			Msg("Manifest channel mismatch, skipping")

		return false
	}

	return true
}

// parseAndValidateVersion parses the manifest version and checks if it's applicable.
// Returns the parsed version, or nil if not applicable (with reason logged).
// This function allows updates in these scenarios:
// - The available version is newer than the current version (normal upgrade)
// - The target channel differs from the inferred channel of the current version
// This is a pure function that does not modify checker state.
func (c *Checker) parseAndValidateVersion(manifest *Manifest) (*Version, error) {
	availableVer, err := ParseVersion(manifest.Version)
	if err != nil {
		return nil, fmt.Errorf("parse manifest version %q: %w", manifest.Version, err)
	}

	if manifest.MinUpgradeVersion != "" {
		minVer, err := ParseVersion(manifest.MinUpgradeVersion)
		if err == nil && c.currentVersion.LessThan(minVer) {
			c.log.Warn().
				Str("current", c.currentVersion.String()).
				Str("minimum", minVer.String()).
				Msg("Current version is below minimum upgrade version")
		}
	}

	// Allow update if the available version is newer
	if availableVer.GreaterThan(c.currentVersion) {
		return availableVer, nil
	}

	// Allow cross-channel updates when the target channel differs from the
	// inferred channel of the currently installed version.
	// This enables scenarios like:
	// - beta v0.8.0-beta.1 → stable v0.7.5
	// - stable v0.7.5 → dev v0.7.0-dev.10
	// - beta v0.8.0-beta.1 → dev v0.7.0-dev.5
	currentInferredChannel := c.currentVersion.InferredChannel()
	if manifest.Channel != currentInferredChannel && !availableVer.Equal(c.currentVersion) {
		c.log.Info().
			Str("current", c.currentVersion.String()).
			Str("available", availableVer.String()).
			Str("currentChannel", currentInferredChannel).
			Str("targetChannel", manifest.Channel).
			Msg("Cross-channel update available")

		return availableVer, nil
	}

	c.log.Debug().
		Str("current", c.currentVersion.String()).
		Str("available", availableVer.String()).
		Msg("Already running latest version")

	return nil, nil
}

// processManifestUpdate processes a manifest and updates checker state accordingly.
// Caller must hold the mutex.
func (c *Checker) processManifestUpdate(manifest *Manifest) *UpdateInfo {
	availableVer, err := c.parseAndValidateVersion(manifest)
	if err != nil {
		c.setError(err)
		c.log.Warn().Err(err).Str("version", manifest.Version).Msg("Failed to parse manifest version")

		return nil
	}

	if availableVer == nil {
		// Already on latest version
		c.status = UpdateStatusIdle

		if c.availableInfo == nil || c.availableInfo.Channel != channelCustom {
			c.availableInfo = nil
		}

		return nil
	}

	platform := manifest.GetPlatform()
	if platform == nil {
		c.log.Warn().
			Str("platform", GetPlatformKey()).
			Msg("No binary available for current platform")

		c.status = UpdateStatusIdle

		return nil
	}

	return c.setUpdateAvailable(manifest, availableVer, platform)
}

func (c *Checker) setUpdateAvailable(manifest *Manifest, availableVer *Version, platform *Platform) *UpdateInfo {
	c.availableInfo = &UpdateInfo{
		CurrentVersion:   c.currentVersion.String(),
		AvailableVersion: availableVer.String(),
		Channel:          manifest.Channel,
		Changelog:        manifest.Changelog,
		DownloadURL:      platform.URL,
		DownloadSize:     platform.Size,
		SHA256:           platform.SHA256,
		ReleaseDate:      manifest.ReleaseDate,
	}

	if c.status != UpdateStatusReadyToInstall && c.status != UpdateStatusDownloading && c.status != UpdateStatusInstalling {
		c.status = UpdateStatusUpdateAvailable
	}

	c.lastError = nil

	c.log.Info().
		Str("current", c.currentVersion.String()).
		Str("available", availableVer.String()).
		Str("channel", manifest.Channel).
		Msg("Update available")

	return c.availableInfo
}

// runPeriodicCheck runs the periodic update check loop.
func (c *Checker) runPeriodicCheck(ctx context.Context) {
	defer c.wg.Done()

	// Initial check after a short delay
	initialDelay := time.NewTimer(initialCheckDelay)
	select {
	case <-initialDelay.C:
		_, _ = c.CheckNow() //nolint:contextcheck // CheckNow creates its own timeout context
	case <-ctx.Done():
		initialDelay.Stop()

		return
	}

	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, _ = c.CheckNow() //nolint:contextcheck // CheckNow creates its own timeout context
		case <-ctx.Done():
			return
		}
	}
}

// checkCustomUpdate checks for custom update files in the data directory.
func (c *Checker) checkCustomUpdate(dataDir string) (*UpdateInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastCheck = time.Now()

	if dataDir == "" {
		c.status = UpdateStatusIdle
		c.lastError = errors.New("data directory not configured")

		return nil, c.lastError
	}

	downloadsDir := filepath.Join(dataDir, "downloads")

	customFile, err := findCustomUpdateFile(downloadsDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.log.Debug().Msg("Downloads directory does not exist, no custom update found")
			c.status = UpdateStatusIdle

			return nil, nil
		}

		c.setError(err)

		return nil, err
	}

	if customFile == "" {
		c.log.Debug().Msg("No custom update file found")
		c.status = UpdateStatusIdle

		return nil, nil
	}

	c.log.Debug().Str("file", customFile).Msg("Found custom update file")

	// Extract and parse manifest.json
	manifest, err := extractManifest(customFile)
	if err != nil {
		c.setError(fmt.Errorf("failed to extract manifest: %w", err))
		c.log.Warn().Err(err).Str("file", customFile).Msg("Failed to extract manifest from custom update")

		return nil, c.lastError
	}

	// Get file size
	fileInfo, err := os.Stat(customFile)
	if err != nil {
		c.log.Warn().Err(err).Msg("Failed to get custom file size")
	}

	c.availableInfo = buildCustomUpdateInfo(c.currentVersion.String(), manifest, fileInfo.Size())
	c.status = UpdateStatusReadyToInstall
	c.lastError = nil

	c.log.Info().
		Str("version", manifest.Version).
		Str("file", customFile).
		Msg("Custom update detected")

	return c.availableInfo, nil
}

// findCustomUpdateFile searches the downloads directory for a custom update file.
// Returns the full path to the custom file, or empty string if not found.
// Returns an error if the directory cannot be read.
func findCustomUpdateFile(downloadsDir string) (string, error) {
	files, err := os.ReadDir(downloadsDir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if strings.HasPrefix(file.Name(), "custom-") {
			return filepath.Join(downloadsDir, file.Name()), nil
		}
	}

	return "", nil
}

// buildCustomUpdateInfo creates an UpdateInfo struct for a custom update.
func buildCustomUpdateInfo(currentVersion string, manifest *Manifest, fileSize int64) *UpdateInfo {
	return &UpdateInfo{
		CurrentVersion:   currentVersion,
		AvailableVersion: manifest.Version,
		Channel:          channelCustom,
		Changelog:        manifest.Changelog,
		DownloadURL:      "",
		DownloadSize:     fileSize,
		SHA256:           "",
		ReleaseDate:      manifest.ReleaseDate,
	}
}

// extractManifest extracts manifest.json from a zip or tar.gz archive.
func extractManifest(archivePath string) (*Manifest, error) {
	ext := filepath.Ext(archivePath)

	if ext == ".zip" {
		return extractManifestFromZip(archivePath)
	}

	if ext == ".gz" || strings.HasSuffix(archivePath, ".tar.gz") {
		return extractManifestFromTarGz(archivePath)
	}

	return nil, fmt.Errorf("unsupported archive format: %s", ext)
}

// extractManifestFromZip extracts manifest.json from a zip archive.
func extractManifestFromZip(zipPath string) (*Manifest, error) {
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer zipReader.Close()

	for _, innerFile := range zipReader.File {
		if filepath.Base(innerFile.Name) == "manifest.json" {
			manifest, err := readManifestFromZipFile(innerFile)
			if err != nil {
				return nil, err
			}

			return manifest, nil
		}
	}

	return nil, errors.New("manifest.json not found in archive")
}

// readManifestFromZipFile reads and parses a manifest from a zip file entry.
func readManifestFromZipFile(file *zip.File) (*Manifest, error) {
	fileHandle, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest.json: %w", err)
	}
	defer fileHandle.Close()

	data, err := io.ReadAll(fileHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest.json: %w", err)
	}

	var manifest Manifest

	err = json.Unmarshal(data, &manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	return &manifest, nil
}

// extractManifestFromTarGz extracts manifest.json from a tar.gz archive.
func extractManifestFromTarGz(tarGzPath string) (*Manifest, error) {
	tgzFile, err := os.Open(tarGzPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tar.gz: %w", err)
	}
	defer tgzFile.Close()

	gzReader, err := gzip.NewReader(tgzFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}

		if filepath.Base(header.Name) == "manifest.json" {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read manifest.json: %w", err)
			}

			var manifest Manifest

			err = json.Unmarshal(data, &manifest)
			if err != nil {
				return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
			}

			return &manifest, nil
		}
	}

	return nil, errors.New("manifest.json not found in archive")
}
