// Package cache provides functionality for caching generated data to the local filesystem.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/rs/zerolog"
)

// TODO: add cache eviction policy (LRU, TTL, max size, etc).

// Cache manages the storage of generated data to the local filesystem.
type Cache struct {
	log       zerolog.Logger // Logger instance
	directory string         // Directory for caching TTS audio files
	mu        sync.RWMutex   // Mutex to protect concurrent access
}

// New creates a new Cache instance storing data in the specified directory.
func New(dir string, log zerolog.Logger) Cache {
	return Cache{
		log:       log.With().Str("package", "cache").Logger(),
		directory: dir,
	}
}

// Read attempts to read cached DCA data from the filesystem.
func (c *Cache) Read(identifier string) ([]byte, error) {
	if c.directory == "" {
		return nil, errors.New("cache directory not configured")
	}

	filename := c.generateFilename(identifier)
	filePath := c.directory + "/" + filename

	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		c.log.Debug().Str("identifier", identifier).Str("file", filePath).Str("result", "miss").Msg("cache read")

		return nil, fmt.Errorf("read cache file: %w", err)
	}

	c.log.Debug().Str("identifier", identifier).Str("file", filePath).Str("result", "hit").Msg("cache read")

	return data, nil
}

// Write writes the given data to the cache filesystem with a filename based on the identifier.
func (c *Cache) Write(identifier string, data []byte) error {
	if c.directory == "" {
		return errors.New("cache directory not configured")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	err := os.MkdirAll(c.directory, 0o750)
	if err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	filename := c.generateFilename(identifier)
	filePath := c.directory + "/" + filename

	err = os.WriteFile(filePath, data, 0o600)
	if err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}

	return nil
}

// generateFilename creates a hashed filename for a given identifier.
func (c *Cache) generateFilename(identifier string) string {
	hash := sha256.Sum256([]byte(identifier))

	hashedIdentifier := hex.EncodeToString(hash[:])

	return hashedIdentifier
}
