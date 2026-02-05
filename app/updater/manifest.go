// Package updater provides self-update functionality for the application.
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// Manifest represents the release manifest JSON structure hosted on the update server.
type Manifest struct {
	Version           string              `json:"version"`
	ReleaseDate       time.Time           `json:"releaseDate"`
	Channel           string              `json:"channel"`
	MinUpgradeVersion string              `json:"minUpgradeVersion"`
	Changelog         []string            `json:"changelog"`
	Platforms         map[string]Platform `json:"platforms"`
}

// Platform contains platform-specific binary information.
type Platform struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// UnmarshalJSON implements custom JSON unmarshaling for Manifest to handle
// both string and array formats for the changelog field (backward compatibility).
func (m *Manifest) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion
	type ManifestAlias Manifest

	// First try unmarshaling with []string changelog (new format)
	var aux struct {
		ManifestAlias

		Changelog json.RawMessage `json:"changelog"`
	}

	err := json.Unmarshal(data, &aux)
	if err != nil {
		return err
	}

	*m = Manifest(aux.ManifestAlias)

	// Handle null or missing changelog
	if len(aux.Changelog) == 0 || string(aux.Changelog) == "null" {
		m.Changelog = []string{}

		return nil
	}

	// Try to unmarshal changelog as []string first
	var changelogArray []string

	err = json.Unmarshal(aux.Changelog, &changelogArray)
	if err == nil {
		if changelogArray == nil {
			changelogArray = []string{}
		}

		m.Changelog = changelogArray

		return nil
	}

	// Fall back to string format (old format)
	var changelogString string

	err = json.Unmarshal(aux.Changelog, &changelogString)
	if err == nil {
		if changelogString != "" {
			m.Changelog = strings.Split(changelogString, "\n")
		} else {
			m.Changelog = []string{}
		}

		return nil
	}

	// If changelog is null or missing, use empty slice
	m.Changelog = []string{}

	return nil
}

// GetPlatformKey returns the platform key for the current OS/architecture.
func GetPlatformKey() string {
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}

// GetPlatform returns the platform information for the current OS/architecture.
// Returns nil if the current platform is not available in the manifest.
func (m *Manifest) GetPlatform() *Platform {
	key := GetPlatformKey()
	if platform, ok := m.Platforms[key]; ok {
		return &platform
	}

	return nil
}

// FetchManifest retrieves and parses the release manifest from the given URL.
func FetchManifest(ctx context.Context, manifestURL string) (*Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest body: %w", err)
	}

	var manifest Manifest

	err = json.Unmarshal(body, &manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// ParseManifest parses a manifest from raw JSON bytes.
func ParseManifest(data []byte) (*Manifest, error) {
	var manifest Manifest

	err := json.Unmarshal(data, &manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}
