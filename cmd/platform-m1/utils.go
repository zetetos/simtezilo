package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

// controlSystemd executes a systemctl command on the specified service with
// automatic retry logic. Returns the command output and any error.
func (p *manager) controlSystemd(service string, command systemctlCmd) (stdout string, err error) {
	p.log.Debug().
		Str("service", service).
		Str("action", string(command)).
		Msg("Controlling systemd service")

	const maxAttempts = 5

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		cmd := exec.CommandContext(context.Background(), "systemctl", string(command), service) //nolint:gosec // action is controlled internally

		output, err := cmd.Output()
		if err == nil {
			p.log.Debug().
				Str("service", service).
				Str("action", string(command)).
				Msg("Successfully controlled systemd service")

			return strings.TrimSpace(string(output)), nil
		}

		lastErr = err
		stdout = strings.TrimSpace(string(output))

		p.log.Debug().
			Err(err).
			Str("service", service).
			Str("action", string(command)).
			Int("attempt", attempt).
			Int("max_attempts", maxAttempts).
			Msg("Failed to control service, retrying...")

		if attempt < maxAttempts {
			time.Sleep(1 * time.Second)
		}
	}

	return stdout, fmt.Errorf("failed to %s %s after %d attempts: %w", string(command), service, maxAttempts, lastErr)
}

// outputJSON marshals the given value to JSON and writes it to stdout.
// Errors during marshaling are output as a JSON error message.
func outputJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("{\"error\":\"%s\"}\n", err.Error()) //nolint:forbidigo // Allow for error output

		return
	}

	fmt.Fprintln(os.Stdout, string(data))
}

// getSerial reads the device serial number from /proc/cpuinfo and returns it
// truncated to 8 characters. Returns "00000000" if the serial cannot be read.
func (p *manager) getSerial() string {
	const defaultSerial = "00000000"

	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		p.log.Debug().Err(err).Msg("Failed to read /proc/cpuinfo")

		return defaultSerial
	}

	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Serial") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				serial := strings.TrimSpace(parts[1])

				// Strip leading zeros until serial is 8 characters long
				for len(serial) > 8 && serial[0] == '0' {
					serial = serial[1:]
				}

				p.log.Debug().Str("serial", serial).Msg("Retrieved device serial number")

				return serial
			}
		}
	}

	p.log.Debug().Msg("Serial number not found in /proc/cpuinfo")

	return defaultSerial
}

// validateIPConfiguration validates static IP configuration parameters including
// CIDR format, gateway address, and DNS server addresses.
func (p *manager) validateIPConfiguration(config networkConfig) error {
	p.log.Debug().Str("method", config.method).Msg("Validating IP configuration")

	if config.method != "static" {
		return nil
	}

	_, _, err := net.ParseCIDR(config.ipAddr + "/" + config.prefix)
	if err != nil {
		return errors.New("invalid CIDR format")
	}

	if config.gateway != "" && net.ParseIP(config.gateway) == nil {
		return errors.New("invalid gateway")
	}

	for _, server := range config.dns {
		if net.ParseIP(server) == nil {
			return fmt.Errorf("invalid DNS server: %s", server)
		}
	}

	return nil
}
