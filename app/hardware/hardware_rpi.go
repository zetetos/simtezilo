package hardware

import (
	"bufio"
	"os"
	"strings"
)

type PlatformID string

const (
	PlatformUnknown PlatformID = "unknown"
	PlatformRpi     PlatformID = "rpi"
)

// platforms returns a map of supported hardware platforms.
func platforms() map[string]PlatformID {
	return map[string]PlatformID{
		"Unknown":      PlatformUnknown,
		"Raspberry Pi": PlatformRpi,
	}
}

// Platform detects and returns the hardware platform identifier.
func Platform() PlatformID {
	platforms := platforms()

	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return platforms["Unknown"]
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "Model") {
			continue
		}

		for name, id := range platforms {
			if strings.Contains(line, name) {
				return id
			}
		}
	}

	return platforms["Unknown"]
}

// String returns the string representation of the PlatformID.
func (p PlatformID) String() string {
	return string(p)
}
