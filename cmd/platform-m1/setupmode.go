package main

import (
	"maps"
	"os"
	"slices"
	"time"

	"github.com/zetetos/simtezilo/app/exitcode"
	"github.com/zetetos/simtezilo/app/platform"
)

// init initializes the setup mode network connection if it does not already exist.
// This is typically called during first boot or after a factory reset.
func (p *manager) init() exitcode.Code {
	if ok := p.waitForNetworkManager(); !ok {
		return exitcode.GeneralErr
	}

	connections, err := p.getConnections()
	if err != nil {
		errMsg := "failed to get network connections"
		p.log.Debug().Err(err).Msg(errMsg)

		outputJSON(map[string]any{
			"error":  errMsg,
			"result": platform.Failure,
		})

		return exitcode.GeneralErr
	}

	p.log.Debug().Strs("connections", slices.Collect(maps.Keys(connections))).Msg("Checking for SetupMode connection")

	if _, exists := connections[setupModeProfile]; !exists {
		p.log.Info().Msg("SetupMode connection not found, provisioning")

		err := p.provisionSetupModeConnection()
		if err != nil {
			errMsg := "failed to provision SetupMode connection"
			p.log.Debug().Err(err).Msg(errMsg)

			outputJSON(map[string]any{
				"error":  errMsg,
				"result": platform.Failure,
			})

			return exitcode.GeneralErr
		}

		p.log.Debug().Msg("SetupMode connection provisioned successfully")

		outputJSON(map[string]any{
			"result": platform.Success,
			"action": "create",
		})

		return exitcode.Success
	}

	p.log.Debug().Msg("SetupMode connection already exists")
	outputJSON(map[string]any{
		"result": platform.Success,
		"action": "none",
	})

	return exitcode.Success
}

// reset deletes all network connection profiles and reinitializes the setup mode
// connection, effectively performing a factory reset of network configuration.
func (p *manager) reset() exitcode.Code {
	if ok := p.waitForNetworkManager(); !ok {
		return exitcode.GeneralErr
	}

	err := p.deleteConnectionProfile(setupModeProfile)
	if err != nil {
		p.log.Debug().Err(err).Msgf("Failed to delete %s connection", setupModeProfile)
	}

	err = p.deleteConnectionProfile(runModeProfile)
	if err != nil {
		p.log.Debug().Err(err).Msgf("Failed to delete %s connection", runModeProfile)
	}

	p.log.Debug().Msgf("Connections deleted, reinitializing %s connection", setupModeProfile)

	return p.init()
}

// disableSetupModeFlag removes the setup mode flag file, indicating that initial
// setup has been completed and the device should boot into run mode.
func (p *manager) disableSetupModeFlag() exitcode.Code {
	err := os.Remove(setupModeFlag)
	if err != nil && !os.IsNotExist(err) {
		errMsg := "failed to remove setup mode flag"
		p.log.Debug().Err(err).Msg(errMsg)

		outputJSON(map[string]any{
			"error":  errMsg,
			"result": platform.Failure,
		})

		return exitcode.GeneralErr
	}

	outputJSON(map[string]any{"result": platform.Success})

	return exitcode.Success
}

// enableSetupModeFlag creates the setup mode flag file, which will cause the
// device to enter setup mode on the next boot.
func (p *manager) enableSetupModeFlag() exitcode.Code {
	file, err := os.Create(setupModeFlag)
	if err != nil {
		errMsg := "failed to create setup mode flag"
		p.log.Debug().Err(err).Msg(errMsg)

		outputJSON(map[string]any{
			"error":  errMsg,
			"result": platform.Failure,
		})

		return exitcode.GeneralErr
	}

	err = file.Close()
	if err != nil {
		errMsg := "failed to close setup mode flag file"
		p.log.Debug().Err(err).Msg(errMsg)

		outputJSON(map[string]any{
			"error":  errMsg,
			"result": platform.Failure,
		})

		return exitcode.GeneralErr
	}

	outputJSON(map[string]any{"result": platform.Success})

	return exitcode.Success
}

// enterRunMode activates the run mode network connection and stops the dnsmasq
// service, switching the device from setup mode to normal operation.
func (p *manager) enterRunMode() exitcode.Code {
	if ok := p.waitForNetworkManager(); !ok {
		return exitcode.GeneralErr
	}

	err := p.activateConnection(runModeProfile)
	if err != nil {
		errMsg := "failed to activate RunMode connection"
		p.log.Debug().Err(err).Msg(errMsg)

		outputJSON(map[string]any{
			"error":  errMsg,
			"result": platform.Failure,
		})

		return exitcode.GeneralErr
	}

	// Stop dnsmasq service when entering run mode
	_, err = p.controlSystemd("dnsmasq.service", sysctlStop)
	if err != nil {
		p.log.Debug().Err(err).Msg("failed to stop dnsmasq service")
	}

	outputJSON(map[string]any{"result": platform.Success})

	return exitcode.Success
}

// enterSetupMode activates the setup mode network connection (access point) and
// starts the dnsmasq service to provide DHCP and DNS for connected clients.
func (p *manager) enterSetupMode() exitcode.Code {
	if ok := p.waitForNetworkManager(); !ok {
		return exitcode.GeneralErr
	}

	err := p.activateConnection(setupModeProfile)
	if err != nil {
		errMsg := "failed to activate SetupMode connection"
		p.log.Debug().Err(err).Msg(errMsg)

		outputJSON(map[string]any{
			"error":  errMsg,
			"result": platform.Failure,
		})

		return exitcode.GeneralErr
	}

	// Give some time for the new connection to stabilize
	time.Sleep(2 * time.Second)

	// Start dnsmasq service when entering setup mode
	_, err = p.controlSystemd("dnsmasq.service", sysctlStart)
	if err != nil {
		p.log.Debug().Err(err).Msg("failed to start dnsmasq service")
	}

	outputJSON(map[string]any{"result": platform.Success})

	return exitcode.Success
}
