package app

import (
	"context"
	"errors"
	"time"
)

// startBackgroundTasks launches all necessary background goroutines for the application.
func (a *App) startBackgroundTasks() {
	a.startHIDEventHandler()
	a.startNewLapHandler()
	a.startPitRadioTask()
	a.startLogStatsBroadcaster()
	a.startIPAddressUpdater()
	a.startGTClient()
	a.startStartupSignaler()
	a.startCrashLogManager()
}

// startHIDEventHandler starts the HID event handler goroutine.
func (a *App) startHIDEventHandler() {
	a.wg.Add(1)

	go func() {
		defer func() {
			a.log.Debug().Msg("HIDEventHandler goroutine exiting")
			a.wg.Done()
		}()

		a.ui.HIDEventHandler(a.ctx)
	}()
}

// startNewLapHandler starts the new lap handler goroutine.
func (a *App) startNewLapHandler() {
	a.wg.Add(1)

	go func() {
		defer func() {
			a.log.Debug().Msg("newLapHandler goroutine exiting")
			a.wg.Done()
		}()

		a.newLapHandler()
	}()
}

// startPitRadioTask starts the pit radio background task if pit radio is enabled.
func (a *App) startPitRadioTask() {
	if a.pitRadio == nil {
		return
	}

	a.wg.Add(1)

	go func() {
		defer func() {
			a.log.Debug().Msg("pitRadio.BackgroundTask goroutine exiting")
			a.wg.Done()
		}()

		a.pitRadio.BackgroundTask()
	}()
}

// startLogStatsBroadcaster starts the log stats broadcaster for WebUI if WebUI is enabled.
func (a *App) startLogStatsBroadcaster() {
	if !a.config.GetAppWebUIEnabled() || a.logStore == nil {
		return
	}

	a.wg.Add(1)

	go func() {
		defer func() {
			a.log.Debug().Msg("logStatsBroadcaster goroutine exiting")
			a.wg.Done()
		}()

		a.logStatsBroadcaster()
	}()
}

// startIPAddressUpdater starts the IP address updater goroutine.
func (a *App) startIPAddressUpdater() {
	a.wg.Add(1)

	go func() {
		defer func() {
			a.log.Debug().Msg("updateIPAddress goroutine exiting")
			a.wg.Done()
		}()

		a.updateIPAddress()
	}()
}

// startGTClient starts the GT telemetry client with context support and automatic recovery.
func (a *App) startGTClient() {
	a.wg.Add(1)

	go func() {
		defer func() {
			a.log.Debug().Msg("GT client goroutine exiting")
			a.wg.Done()
		}()

		for {
			select {
			case <-a.ctx.Done():
				return
			default:
				recoverable, err := a.gtClient.Run(a.ctx)
				if err != nil {
					// Check if error is due to context cancellation
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						a.log.Debug().Msg("GT client stopped due to context cancellation")

						return
					}

					_ = a.ui.Screen.RenderSplashScreen("GT client error")

					if recoverable {
						a.log.Error().
							Err(err).
							Str("component", "gt client").
							Str("result", "failure").
							Msg("run")

						continue
					}

					a.log.Fatal().
						Err(err).
						Str("component", "gt client").
						Str("result", "failure").
						Msg("run")
				}
			}
		}
	}()
}

// startStartupSignaler signals successful startup after 10 seconds.
func (a *App) startStartupSignaler() {
	a.wg.Add(1)

	go func() {
		defer func() {
			a.log.Debug().Msg("signalStartupSuccess goroutine exiting")
			a.wg.Done()
		}()

		select {
		case <-time.After(10 * time.Second):
			a.signalStartupSuccess()
		case <-a.ctx.Done():
			a.log.Debug().Msg("Startup signal cancelled due to shutdown")

			return
		}
	}()
}

// startCrashLogManager performs a single crash log rotation on startup.
// This ensures any pending rotation/compression happens outside of panic handling.
func (a *App) startCrashLogManager() {
	if a.crashLogger == nil {
		return
	}

	a.log.Info().
		Str("component", "crashlog").
		Str("path", a.crashLogger.LogPath()).
		Msg("Crash log manager started")

	err := a.crashLogger.Rotate()
	if err != nil {
		a.log.Warn().
			Err(err).
			Str("component", "crashlog").
			Msg("Failed to rotate crash log")
	}
}
