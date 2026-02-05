package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zetetos/simtezilo/app/exitcode"
	"github.com/zetetos/simtezilo/app/platform"
	"golang.org/x/crypto/ssh"
)

// isSSHEnabled checks whether the SSH service is both enabled and currently active.
func (p *manager) isSSHEnabled() bool {
	enabled, _ := p.controlSystemd("ssh.service", sysctlIsEnabled)
	active, _ := p.controlSystemd("ssh.service", sysctlIsActive)

	return enabled == "enabled" && active == "active"
}

// enableSSH enables and starts the SSH service using systemctl.
func (p *manager) enableSSH() exitcode.Code {
	return p.controlSSH([]systemctlCmd{sysctlEnable, sysctlStart})
}

// disableSSH stops and disables the SSH service using systemctl.
func (p *manager) disableSSH() exitcode.Code {
	return p.controlSSH([]systemctlCmd{sysctlStop, sysctlDisable})
}

// controlSSH executes a sequence of systemctl actions on the SSH service.
func (p *manager) controlSSH(actions []systemctlCmd) exitcode.Code {
	for _, action := range actions {
		_, err := p.controlSystemd("ssh.service", action)
		if err != nil {
			p.log.Debug().Err(err).Msgf("failed to %s sshd service", action)

			outputJSON(map[string]any{
				"result": platform.Failure,
				"error":  fmt.Errorf("failed to %s sshd service", action),
			})

			return exitcode.GeneralErr
		}
	}

	outputJSON(map[string]any{"result": platform.Success})

	return exitcode.Success
}

// provisionSSH reads an SSH public key from stdin and installs it to the admin
// user's authorized_keys file, creating the .ssh directory if needed.
func (p *manager) provisionSSH() exitcode.Code {
	// Read public key from stdin
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		errMsg := "failed to read SSH key from stdin"
		p.log.Debug().Err(err).Msg(errMsg)

		outputJSON(map[string]any{
			"error":  errMsg,
			"result": platform.Failure,
		})

		return exitcode.DataFormatErr
	}

	sshKey := strings.TrimSpace(string(data))
	if sshKey == "" {
		errMsg := "no SSH key provided"
		p.log.Debug().Msg(errMsg)

		outputJSON(map[string]any{
			"error":  errMsg,
			"result": platform.Failure,
		})

		return exitcode.DataFormatErr
	}

	// Validate the SSH public key
	_, _, _, _, err = ssh.ParseAuthorizedKey([]byte(sshKey)) //nolint:dogsled // validation only
	if err != nil {
		errMsg := "invalid SSH public key format"
		p.log.Debug().Err(err).Msg(errMsg)

		outputJSON(map[string]any{
			"error":  errMsg,
			"result": platform.Failure,
		})

		return exitcode.DataFormatErr
	}

	// Ensure the .ssh directory exists
	sshDir := "/home/" + sshUser + "/.ssh"

	err = os.MkdirAll(sshDir, 0o700)
	if err != nil {
		p.log.Debug().
			Err(err).
			Str("path", sshDir).
			Msg("Create .ssh directory")

		outputJSON(map[string]any{
			"error":  "failed to create .ssh directory",
			"result": platform.Failure,
		})

		return exitcode.GeneralErr
	}

	// Write the authorized_keys file
	authorizedKeysPath := "/home/" + sshUser + "/.ssh/authorized_keys"

	err = os.WriteFile(authorizedKeysPath, []byte(sshKey+"\n"), 0o600)
	if err != nil {
		p.log.Debug().
			Err(err).
			Str("path", authorizedKeysPath).
			Msg("Write authorized_keys file")

		outputJSON(map[string]any{
			"error":  "failed to write authorized_keys file",
			"result": platform.Failure,
		})

		return exitcode.GeneralErr
	}

	p.log.Debug().Str("path", authorizedKeysPath).Msg("SSH key provisioned successfully")

	outputJSON(map[string]any{"result": platform.Success})

	return exitcode.Success
}
