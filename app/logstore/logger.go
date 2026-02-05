package logstore

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

// LoggerWithStore wraps a zerolog.Logger with an in-memory store.
type LoggerWithStore struct {
	Logger zerolog.Logger
	Store  *Store
}

// NewLoggerWithStore creates a new logger that writes to both stderr and an in-memory store.
func NewLoggerWithStore(level zerolog.Level, maxEntries int) *LoggerWithStore {
	store := New(maxEntries)

	// Create a multi-level writer that writes to both stderr and the store
	multiWriter := zerolog.MultiLevelWriter(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02 15:04:05"},
		store,
	)

	logger := zerolog.New(multiWriter).With().Timestamp().Logger().Level(level)

	return &LoggerWithStore{
		Logger: logger,
		Store:  store,
	}
}

// NewLoggerWithStoreAndWriter creates a new logger with a custom writer along with the store.
func NewLoggerWithStoreAndWriter(writer io.Writer, level zerolog.Level, maxEntries int) *LoggerWithStore {
	store := New(maxEntries)

	// Create a multi-level writer that writes to the provided writer and the store
	multiWriter := zerolog.MultiLevelWriter(writer, store)

	logger := zerolog.New(multiWriter).With().Timestamp().Logger().Level(level)

	return &LoggerWithStore{
		Logger: logger,
		Store:  store,
	}
}
