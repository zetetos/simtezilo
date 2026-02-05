package updater //nolint:testpackage // testing internal functions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseManifest(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		"version":           "1.2.3",
		"channel":           "stable",
		"minUpgradeVersion": "1.0.0",
		"sha256":            "abc123",
		"size":              "15728640",
	}

	manifestJSON := fmt.Sprintf(`{
		"version": "%s",
		"releaseDate": "2026-01-07T10:00:00Z",
		"channel": "%s",
		"minUpgradeVersion": "%s",
		"changelog": ["- Bug fixes", "- New features"],
		"platforms": {
			"linux-arm64": {
				"url": "https://example.com/releases/v1.2.3/simtezilo-linux-arm64",
				"sha256": "%s",
				"size": %s
			},
			"linux-amd64": {
				"url": "https://example.com/releases/v1.2.3/simtezilo-linux-amd64",
				"sha256": "def456",
				"size": 16252928
			}
		}
	}`, want["version"], want["channel"], want["minUpgradeVersion"], want["sha256"], want["size"])

	manifest, err := ParseManifest([]byte(manifestJSON))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}

	if manifest.Version != want["version"] {
		t.Errorf("Version = %v, want %v", manifest.Version, want["version"])
	}

	if manifest.Channel != want["channel"] {
		t.Errorf("Channel = %v, want %v", manifest.Channel, want["channel"])
	}

	if manifest.MinUpgradeVersion != want["minUpgradeVersion"] {
		t.Errorf("MinUpgradeVersion = %v, want %v", manifest.MinUpgradeVersion, want["minUpgradeVersion"])
	}

	if len(manifest.Platforms) != 2 {
		t.Errorf("len(Platforms) = %v, want 2", len(manifest.Platforms))
	}

	linuxArm64 := manifest.Platforms["linux-arm64"]
	if linuxArm64.SHA256 != want["sha256"] {
		t.Errorf("linux-arm64 SHA256 = %v, want %v", linuxArm64.SHA256, want["sha256"])
	}

	wantSize, err := strconv.ParseInt(want["size"], 10, 64)
	require.NoError(t, err)

	if linuxArm64.Size != wantSize {
		t.Errorf("linux-arm64 Size = %v, want %v", linuxArm64.Size, wantSize)
	}
}

func TestFetchManifest(t *testing.T) {
	t.Parallel()

	manifest := Manifest{
		Version:     "2.0.0",
		ReleaseDate: time.Now().UTC(),
		Channel:     "stable",
		Platforms: map[string]Platform{
			"linux-arm64": {
				URL:    "https://example.com/binary",
				SHA256: "abc123",
				Size:   1024,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(respWriter http.ResponseWriter, _ *http.Request) {
		respWriter.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(respWriter).Encode(manifest)
		if err != nil {
			t.Errorf("Failed to encode manifest: %v", err)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fetched, err := FetchManifest(ctx, server.URL)
	if err != nil {
		t.Fatalf("FetchManifest() error = %v", err)
	}

	if fetched.Version != manifest.Version {
		t.Errorf("Version = %v, want %v", fetched.Version, manifest.Version)
	}

	if fetched.Channel != manifest.Channel {
		t.Errorf("Channel = %v, want %v", fetched.Channel, manifest.Channel)
	}
}

func TestFetchManifest_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(respWriter http.ResponseWriter, _ *http.Request) {
		respWriter.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := FetchManifest(ctx, server.URL)
	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestFetchManifest_InvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(respWriter http.ResponseWriter, _ *http.Request) {
		_, err := respWriter.Write([]byte("not json"))
		if err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := FetchManifest(ctx, server.URL)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestGetPlatformKey(t *testing.T) {
	t.Parallel()

	key := GetPlatformKey()
	if key == "" {
		t.Error("GetPlatformKey() returned empty string")
	}
	// Should contain a hyphen (os-arch format)
	if len(key) < 3 {
		t.Errorf("GetPlatformKey() = %v, expected longer format", key)
	}
}

func TestParseManifest_ChangelogStringFormat(t *testing.T) {
	t.Parallel()

	// Test backward compatibility with old string format for changelog
	manifestJSON := `{
		"version": "1.2.3",
		"releaseDate": "2026-01-07T10:00:00Z",
		"channel": "stable",
		"changelog": "- Bug fixes\n- New features\n- Performance improvements",
		"platforms": {}
	}`

	manifest, err := ParseManifest([]byte(manifestJSON))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}

	expectedChangelog := []string{"- Bug fixes", "- New features", "- Performance improvements"}
	if len(manifest.Changelog) != len(expectedChangelog) {
		t.Fatalf("Changelog length = %v, want %v", len(manifest.Changelog), len(expectedChangelog))
	}

	for i, line := range expectedChangelog {
		if manifest.Changelog[i] != line {
			t.Errorf("Changelog[%d] = %q, want %q", i, manifest.Changelog[i], line)
		}
	}
}

func TestParseManifest_ChangelogNullOrMissing(t *testing.T) {
	t.Parallel()

	// Test with null changelog
	manifestJSON := `{
		"version": "1.2.3",
		"releaseDate": "2026-01-07T10:00:00Z",
		"channel": "stable",
		"changelog": null,
		"platforms": {}
	}`

	manifest, err := ParseManifest([]byte(manifestJSON))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}

	if manifest.Changelog == nil {
		t.Error("Changelog should not be nil")
	}

	if len(manifest.Changelog) != 0 {
		t.Errorf("Changelog length = %v, want 0", len(manifest.Changelog))
	}
}
