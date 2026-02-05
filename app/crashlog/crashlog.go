// Package crashlog provides panic capture and crash log management.
package crashlog

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// DefaultMaxSize is the default maximum size in megabytes before rotation.
	DefaultMaxSize = 10
	// DefaultMaxBackups is the default number of old files to retain.
	DefaultMaxBackups = 5
	// DefaultMaxAge is the default maximum number of days to retain old files.
	DefaultMaxAge = 30
)

// CrashLogger handles writing panic information to a rotating log file.
type CrashLogger struct {
	logger *lumberjack.Logger
}

// Options configures the crash logger.
type Options struct {
	// LogDir is the directory where crash logs will be written.
	LogDir string
	// MaxSize is the maximum size in megabytes before the log file is rotated.
	MaxSize int
	// MaxBackups is the maximum number of old log files to retain.
	MaxBackups int
	// MaxAge is the maximum number of days to retain old log files.
	MaxAge int
	// Compress determines if rotated files should be compressed.
	Compress bool
}

// New creates a new CrashLogger with the given options.
func New(opts Options) *CrashLogger {
	if opts.MaxSize == 0 {
		opts.MaxSize = DefaultMaxSize
	}

	if opts.MaxBackups == 0 {
		opts.MaxBackups = DefaultMaxBackups
	}

	if opts.MaxAge == 0 {
		opts.MaxAge = DefaultMaxAge
	}

	logPath := filepath.Join(opts.LogDir, "crash.log")

	return &CrashLogger{
		logger: &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    opts.MaxSize,
			MaxBackups: opts.MaxBackups,
			MaxAge:     opts.MaxAge,
			Compress:   opts.Compress,
			LocalTime:  true,
		},
	}
}

// WritePanic writes panic information to the crash log.
// This method is designed to be called from a deferred recover() and
// intentionally avoids any operations that could fail or panic.
func (c *CrashLogger) WritePanic(panicValue any, version, commitHash, buildTime, platform string) {
	stackTrace := debug.Stack()
	timestamp := time.Now().Format(time.RFC3339)

	entry := fmt.Sprintf(
		"================================================================================\n"+
			"PANIC at %s\n"+
			"--------------------------------------------------------------------------------\n"+
			"Version: %s  Commit: %s  Build: %s  Platform: %s\n"+
			"--------------------------------------------------------------------------------\n"+
			"Panic: %v\n"+
			"--------------------------------------------------------------------------------\n"+
			"Stack Trace:\n%s"+
			"================================================================================\n\n",
		timestamp,
		version, commitHash, buildTime, platform,
		panicValue,
		stackTrace,
	)

	// Write directly to the underlying logger
	// We don't check for errors here as we're in a panic handler
	_, _ = c.logger.Write([]byte(entry))
}

// Rotate triggers log rotation. This should be called from a background task,
// not during panic handling.
func (c *CrashLogger) Rotate() error {
	return c.logger.Rotate()
}

// Close closes the crash logger.
func (c *CrashLogger) Close() error {
	return c.logger.Close()
}

// LogPath returns the path to the crash log file.
func (c *CrashLogger) LogPath() string {
	return c.logger.Filename
}

// EnsureLogDir creates the log directory if it doesn't exist.
func EnsureLogDir(logDir string) error {
	return os.MkdirAll(logDir, 0o755)
}
