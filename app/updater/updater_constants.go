package updater

import "time"

const (
	// DefaultCheckInterval is the default time between update checks.
	DefaultCheckInterval = 1 * time.Hour

	// DefaultHTTPTimeout is the default timeout for HTTP operations.
	DefaultHTTPTimeout = 30 * time.Second

	// DefaultDownloadTimeout is the timeout for downloading binaries.
	DefaultDownloadTimeout = 10 * time.Minute

	// DefaultChannel is the default update channel.
	DefaultChannel = "stable"

	// DefaultMaxFailures is the maximum number of consecutive failures before auto-rollback.
	DefaultMaxFailures = 3

	// initialCheckDelay is the delay before the first update check after startup.
	initialCheckDelay = 10 * time.Second
)

const (
	// InstallStatusPending is the status value for pending installations.
	InstallStatusPending = "pending"

	// InstallStatusComplete is the status value for completed installations.
	InstallStatusComplete = "complete"

	// InstallStatusRolledBack is the status value for rolled back installations.
	InstallStatusRolledBack = "rolled_back"

	// InstallStatusFailed is the status value for failed installations.
	InstallStatusFailed = "failed"

	// channelCustom is the name of the custom update channel.
	channelCustom = "custom"
)
