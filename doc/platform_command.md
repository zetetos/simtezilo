# Platform Command API Reference

The `platform` command is a privileged CLI tool for managing platform-specific system operations on the Simtezilo device. It handles network configuration, setup mode transitions, SSH provisioning, and software updates.

## Synopsis

```
platform [options] <command>
```

## Options

| Option       | Description                                               |
|--------------|-----------------------------------------------------------|
| `-b <dir>`   | Base directory for installation (default: /opt/simtezilo) |
| `-h`         | Show help message                                         |
| `-l <level>` | Set log level (`debug`, `info`, `warn`, `error`)          |
| `-v`         | Show version information                                  |

## Commands

The following commands are required and **must** be implemented for the platform integration to work correctly.

| Command         | Description                                                   |
|-----------------|---------------------------------------------------------------|
| init            | Initialize setup mode connection if not present               |
| mode-run        | Enter run mode                                                |
| mode-setup      | Enter setup mode                                              |
| reset           | Delete all connections and reinitialize setup mode            |
| setup-disable   | Disable setup mode flag                                       |
| setup-enable    | Enable setup mode flag                                        |
| signal-start    | Signal successful startup                                     |
| ssh-enable      | Enable SSH service                                            |
| ssh-disable     | Disable SSH service                                           |
| ssh-provision   | Provision SSH access                                          |
| status          | Check current environment status                              |
| update-apply    | Apply a pending update (extracts, installs, swaps binaries)   |
| update-rollback | Rollback to the previous version                              |
| version         | Print version information                                     |
| wifi-access     | Provide the network access details for the setup mode network |
| wifi-provision  | Provision network connection                                  |
| wifi-scan       | Scan for available WiFi networks                              |

## Exit Codes

| Code |      Meaning       |                           Description                              |
|------|--------------------|--------------------------------------------------------------------|
|    0 | `Success`          | Command completed successfully                                     |
|    1 | `GeneralErr`       | General error occurred                                             |
|   31 | `SetupRequired`    | Device requires setup (returned by `status` when setup incomplete) |
|   64 | `CommandUsageErr`  | Invalid command or usage                                           |
|   65 | `DataFormatErr`    | Invalid input data format                                          |
|   70 | `InternalErr`      | Internal error                                                     |
|   71 | `SystemErr`        | System-level error                                                 |
|   77 | `PermissionDenied` | Permission denied                                                  |
|   78 | `ConfigErr`        | Configuration error                                                |

## Command specifications

### Initialization & Reset

#### `init`

Initialize the setup mode network connection if not present. Typically called during first boot or after a factory reset.

**Output:**
```json
{
  "result": "success",
  "action": "create"  // or "none" if already exists
}
```

#### `reset`

Delete all network connection profiles and reinitialize setup mode. Effectively performs a factory reset of network configuration.

**Output:**
```json
{
  "result": "success",
  "action": "create"
}
```

---

### Mode Switching

#### `mode-run`

Enter run mode by activating the run mode network connection and stopping the dnsmasq service.

**Output:**
```json
{
  "result": "success"
}
```

#### `mode-setup`

Enter setup mode by activating the setup mode access point and starting the dnsmasq service for DHCP/DNS.

**Output:**
```json
{
  "result": "success"
}
```

---

### Setup Flag Management

#### `setup-enable`

Create the setup mode flag file, causing the device to enter setup mode on next boot.

**Output:**
```json
{
  "result": "success"
}
```

#### `setup-disable`

Remove the setup mode flag file, indicating initial setup is complete and the device should boot into run mode.

**Output:**
```json
{
  "result": "success"
}
```

---

### Status

#### `status`

Check and report the current environment status including setup mode flag, network connection profiles, SSH state, and whether setup is required.

**Output:**
```json
{
  "result": "success",
  "status": {
    "available": true,
    "activeConn": "RunMode",
    "flagEnabled": false,
    "runModePresent": true,
    "setupModePresent": true,
    "setupRequired": false,
    "ready": true,
    "lcdPresent": true,
    "sshEnabled": false
  }
}
```

**Status Fields:**

| Field              | Type    | Description                                      |
|--------------------|---------|--------------------------------------------------|
| `available`        | bool    | Whether the platform command is available        |
| `activeConn`       | string  | Name of the active network connection            |
| `flagEnabled`      | bool    | Whether setup mode flag file exists              |
| `runModePresent`   | bool    | Whether RunMode connection profile exists        |
| `setupModePresent` | bool    | Whether SetupMode connection profile exists      |
| `setupRequired`    | bool    | Whether initial setup is incomplete              |
| `ready`            | bool    | Whether NetworkManager is ready                  |
| `lcdPresent`       | bool    | Whether LCD display is detected                  |
| `sshEnabled`       | bool    | Whether SSH service is enabled                   |

**Exit Codes:**
- `0` (Success): All setup complete
- `31` (SetupRequired): Setup is incomplete

---

### Startup Signaling

#### `signal-start`

Signal that the application has started successfully. This resets the failed start counter to prevent unnecessary recovery actions when the service next starts.

**Output:**
```json
{
  "result": "success",
  "action": "failed_start_counter_reset"
}
```

---

### WiFi Management

#### `wifi-scan`

Scan for available WiFi networks and return the list as JSON.

**Output:**
```json
{
  "result": "success",
  "networks": [
    {
      "ssid": "MyNetwork",
      "security": "wpa2"
    }
  ]
}
```

#### `wifi-access`

Return the network access details for the setup mode access point. Only works when setup mode is active.

**Output:**
```json
{
  "result": "success",
  "wifi": {
    "ssid": "Simtezilo-XXXX",
    "psk": "5imtezil0",
    "security": "wpa2"
  }
}
```

#### `wifi-provision`

Provision a run mode network connection. Reads configuration from stdin as JSON.

**Input (stdin):**
```json
[{
  "ssid": "<string>",
  "psk": "<string>",
  "security": "<wpa2|wpa3>",
  "method": "<dhcp|static>",
  "ip": "<address>",
  "prefix": "<bits>",
  "gateway": "<address>",
  "dns": "<address>"
}]
```

|   Field    |  Required  |                    Description                    |
|------------|------------|---------------------------------------------------|
| `ssid`     |    Yes     | Network SSID                                      |
| `psk`      |    Yes     | Network password/pre-shared key                   |
| `security` |    Yes     | Security type: `wpa2` or `wpa3`                   |
| `method`   |    Yes     | IP configuration: `dhcp` or `static`              |
| `ip`       | For static | IP address (when method is `static`)              |
| `prefix`   | For static | Subnet prefix bits (e.g., `24`)                   |
| `gateway`  | For static | Gateway address                                   |
| `dns`      | For static | DNS server address (comma-separated for multiple) |

**Output:**
```json
{
  "result": "success"
}
```

**Example:**
```bash
echo '[{"ssid":"MyNetwork","psk":"mypassword","security":"wpa2","method":"dhcp"}]' | platform wifi-provision
```

---

### SSH Management

#### `ssh-enable`

Enable and start the SSH service.

**Output:**
```json
{
  "result": "success"
}
```

#### `ssh-disable`

Stop and disable the SSH service.

**Output:**
```json
{
  "result": "success"
}
```

#### `ssh-provision`

Install an SSH public key for the admin user. Reads the public key from stdin.

**Input (stdin):**
```
ssh-ed25519 AAAA... user@host
```

**Output:**
```json
{
  "result": "success"
}
```

**Example:**
```bash
cat ~/.ssh/id_ed25519.pub | platform ssh-provision
```

---

### Update Management

#### `update-apply`

Apply a pending software update. Processes updates based on the state file at `/opt/simtezilo/data/update/update-state.json`.

**States Handled:**
- `pending`:     Verifies download, extracts archive, installs new binary
- `complete`:    Cleans up rollback binary and state file
- `failed`:      Reports failure details
- `rolled_back`: No action needed
- `installing`:  No action needed

**Output (installed):**
```json
{
  "result": "success",
  "action": "installed",
  "version": "1.2.3"
}
```

**Output (no action needed):**
```json
{
  "result": "success",
  "action": "none"
}
```

**Output (failure):**
```json
{
  "result": "failure",
  "error": "reason for failure",
  "failCount": 1
}
```

#### `update-rollback`

Rollback to the previous software version by restoring the backup binary.

**Output:**
```json
{
  "result": "success",
  "action": "rolled_back"
}
```

---

### Informational

#### `version`

Print version information including version number, commit hash, build time, and platform.

**Output (stdout):**
```
Version: 1.2.3  Commit Hash: abc123  Build Time: 2024-01-01T00:00:00Z  Platform: linux/arm64
```

#### `help`

Display usage information for all commands.

---

## Update State File

The update system uses a state file at `<base_dir>/data/update/update-state.json` (default: `/opt/simtezilo/data/update/update-state.json`):

```json
{
  "pendingVersion": "1.2.3",
  "currentVersion": "1.2.2",
  "downloadPath": "/opt/simtezilo/data/update/simtezilo-1.2.3.tar.gz",
  "extractDir": "/opt/simtezilo/data/update/extract",
  "sha256": "abc123...",
  "timestamp": "2024-01-01T00:00:00Z",
  "status": "pending",
  "failCount": 0,
  "lastError": ""
}
```

**Status Values:**

| Status       | Description                            |
|--------------|----------------------------------------|
| `pending`    | Update downloaded and ready to install |
| `installing` | Installation in progress               |
| `complete`   | Installation completed successfully    |
| `failed`     | Installation failed                    |
| `rolled_back`| Previous version restored              |

---

## Directory Structure

The platform command uses the following directory structure under the base directory (configurable via `-b`, default `/opt/simtezilo`):

```
/opt/simtezilo/
├── bin/
│   └── simtezilo                # Main application binary
├── data/
│   └── update/
│       ├── update-state.json    # Update state tracking
│       ├── extract/             # Temporary extraction directory
│       └── *.tar.gz             # Downloaded update archives
└── failed-starts                # Failed start counter file
```

---

## Network Profiles

The platform manages two NetworkManager connection profiles:

|   Profile   |                     Purpose                   |        Mode       |
|-------------|-----------------------------------------------|-------------------|
| `SetupMode` | Access point for initial device configuration | AP (10.33.0.1/24) |
| `RunMode`   | Client connection to user's WiFi network      | Infrastructure    |

---

## Error Response Format

All commands return JSON with an `error` field on failure:

```json
{
  "result": "failure",
  "error": "descriptive error message"
}
```

---

## Security Considerations

- The `platform` command requires root privileges for most operations
- SSH keys are validated before installation
- Update archives are verified against SHA256 checksums when provided
- The setup mode access point uses a device-specific SSID based on serial number
- Default setup mode password is `5imtezil0`

---

## Implementation Notes

The platform command is implemented in Go and uses:

- **NetworkManager** via D-Bus for network configuration
- **systemd** for service management (SSH, dnsmasq)
- **JSON** for all structured input/output

### Source Files

| File           | Purpose                                |
|----------------|----------------------------------------|
| `main.go`      | Command dispatch and initialization    |
| `archive.go`   | Archive extraction utilities           |
| `filesystem.go`| File system operations                 |
| `network.go`   | NetworkManager integration             |
| `setupmode.go` | Setup mode flag management             |
| `ssh.go`       | SSH service and key provisioning       |
| `status.go`    | Status reporting and startup signaling |
| `update.go`    | Update installation and rollback       |
| `utils.go`     | Helper utilities and JSON output       |
