// Package app implements the main application logic for Simtezilo, a racing simulator telemetry and haptics engine.
package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	gttelemetry "github.com/zetetos/gt-telemetry/v2"
	gtmodels "github.com/zetetos/gt-telemetry/v2/pkg/models"
	"github.com/zetetos/simtezilo/app/cache"
	"github.com/zetetos/simtezilo/app/calibrator"
	"github.com/zetetos/simtezilo/app/circuit"
	"github.com/zetetos/simtezilo/app/config"
	"github.com/zetetos/simtezilo/app/crashlog"
	"github.com/zetetos/simtezilo/app/exitcode"
	"github.com/zetetos/simtezilo/app/fuelrange"
	"github.com/zetetos/simtezilo/app/hardware"
	"github.com/zetetos/simtezilo/app/hardware/console"
	"github.com/zetetos/simtezilo/app/hardware/display"
	"github.com/zetetos/simtezilo/app/hardware/pirateaudio"
	"github.com/zetetos/simtezilo/app/hardware/spotpear"
	"github.com/zetetos/simtezilo/app/hardware/waveshare"
	"github.com/zetetos/simtezilo/app/i18n"
	"github.com/zetetos/simtezilo/app/i18n/languagedb"
	"github.com/zetetos/simtezilo/app/kinematics"
	"github.com/zetetos/simtezilo/app/logstore"
	"github.com/zetetos/simtezilo/app/odometer"
	"github.com/zetetos/simtezilo/app/pitradio"
	"github.com/zetetos/simtezilo/app/pitradio/discord"
	"github.com/zetetos/simtezilo/app/platform"
	"github.com/zetetos/simtezilo/app/setupmode"
	"github.com/zetetos/simtezilo/app/synthesizer"
	"github.com/zetetos/simtezilo/app/tyres"
	"github.com/zetetos/simtezilo/app/ui"
	"github.com/zetetos/simtezilo/app/ui/webui"
	"github.com/zetetos/simtezilo/app/updater"
	"github.com/zetetos/simtezilo/app/vehicle"
)

// lapEvent represents a single lap completion event.
type lapEvent struct {
	Lap      int16
	LapTime  time.Duration
	Delta    time.Duration
	Position int16
	HasDelta bool
}

// App is the main application struct holding all components and state.
type App struct {
	log          zerolog.Logger     // Application logger
	logStore     *logstore.Store    // In-memory log storage
	config       *config.Config     // Application configuration
	exitCodeChan chan exitcode.Code // Channel to signal application shutdown with exit code

	// Lifecycle management for background goroutines
	ctx    context.Context //nolint:containedctx // Context for managing lifecycle
	cancel context.CancelFunc
	wg     sync.WaitGroup

	cache cache.Cache // Cache manager

	setupMode *setupmode.SetupMode // Setup mode manager

	ui *ui.UserInterface // User interface manager

	i18n    *i18n.I18n       // Language translations
	display hardware.Display // Hardware display interface

	gtClient   *gttelemetry.Client       // GT telemetry client
	pitRadio   pitradio.PitRadio         // Pit radio notification service
	kinematics kinematics.State          // Vehicle kinematics tracker
	synth      *synthesizer.Synthesizer  // Audio synthesizer for haptic feedback
	calibrator *calibrator.ToneGenerator // Calibration mode manager

	odometer  *odometer.Odometer  // Odometer for distance tracking
	fuelRange fuelrange.Estimator // Fuel range estimator
	circuit   circuit.Manager     // Circuit information and tracking

	transmissionGainMin float64 // Minimum transmission gain based on vehicle type

	state         gameState               // Application state tracker
	pitRadioState *pitRadioState          // Current pit radio state
	vehicle       vehicle.Characteristics // Current vehicle information
	tyres         *tyres.Tyre             // Tyre monitoring

	telemetryChartFeed chan map[string]float32 // Channel for sending telemetry data to web UI
	vehicleInfoFeed    chan map[string]any     // Channel for sending vehicle info to web UI
	circuitInfoFeed    chan map[string]string  // Channel for sending circuit info to web UI
	raceInfoFeed       chan map[string]any     // Channel for sending race info to web UI
	gameStateFeed      chan string             // Channel for sending game state to web UI
	logStatsFeed       chan map[string]any     // Channel for sending log stats to web UI
	webUI              *webui.WebUI            // Web UI server and handler
	webSequenceID      uint32                  // Last sequence ID sent to the web UI
	ipAddress          string                  // Local IP address for web UI access

	lapEvents      []lapEvent    // History of lap events
	lapEventsMutex sync.Mutex    // Mutex for lap events slice
	bestLapTime    time.Duration // Best lap time for delta calculation

	lapStartEvents chan uint32 // Channel for notifying new lap starts

	httpServer        *http.Server // Shared HTTP server for both modes
	activeHTTPHandler http.Handler // Current handler (setup mode or run mode)

	updater *updater.Updater // Self-update manager

	crashLogger *crashlog.CrashLogger // Crash log manager for panic capture

	// Chassis haptics state
	jerkPeakHold         float64       // Peak hold value for jerk to prevent cancellation
	jerkPeakHoldTime     time.Time     // Time when peak hold was last updated
	jerkPeakHoldDuration time.Duration // Duration to hold peak based on pulse length

	// Telemetry source management
	customTelemetrySource string // Stores custom telemetry source when user switches to auto/demo

	// TODO: fix menu nav and remove this
	activeBuildInfoItem int // Active build info item index
}

// Options holds configuration options for initializing the App.
type Options struct {
	ConfigFile   string                // Path to configuration file
	ExitCodeChan chan exitcode.Code    // Channel to signal application shutdown with exit code
	Logger       *zerolog.Logger       // Logger instance for application logging
	LogStore     *logstore.Store       // In-memory log storage
	CrashLogger  *crashlog.CrashLogger // Crash log manager for panic capture
}

// New creates a new App instance and sets up all components based on the provided options.
func New(opts Options) (*App, error) {
	// Create cancellable context for managing background goroutines
	ctx, cancel := context.WithCancel(context.Background())

	newApp := &App{
		log:                opts.Logger.With().Str("package", "app").Logger(),
		logStore:           opts.LogStore,
		exitCodeChan:       opts.ExitCodeChan,
		ctx:                ctx,
		cancel:             cancel,
		state:              NewGameState(opts.Logger),
		kinematics:         kinematics.NewKinematicsState(),
		telemetryChartFeed: make(chan map[string]float32, 600),
		vehicleInfoFeed:    make(chan map[string]any, 10),
		circuitInfoFeed:    make(chan map[string]string, 10),
		raceInfoFeed:       make(chan map[string]any, 10),
		gameStateFeed:      make(chan string, 10),
		logStatsFeed:       make(chan map[string]any, 10),
		lapStartEvents:     make(chan uint32),
		crashLogger:        opts.CrashLogger,
	}

	newApp.initializeConfig(opts)

	// Initialize calibrator with config reference
	var err error

	newApp.calibrator, err = calibrator.NewToneGenerator(newApp.config)
	if err != nil {
		return nil, err
	}

	err = newApp.initializeI18n(opts)
	if err != nil {
		return nil, err
	}

	hidEvents := make(chan ui.HIDInputEvent, 10)

	err = newApp.initializeHardware(hidEvents)
	if err != nil {
		newApp.log.Error().
			Err(err).
			Msg("Initialize hardware failed, fallback to console display")
	}

	// Initialize setupMode after display is created
	newApp.setupMode = setupmode.New(setupmode.Options{
		Config:       newApp.config,
		ExitCodeChan: newApp.exitCodeChan,
		Logger:       &newApp.log,
		Display:      newApp.getDisplayLCD(),
	})

	err = newApp.initializeUI(opts, hidEvents)
	if err != nil {
		return nil, err
	}

	err = newApp.initializeComponents(opts)
	if err != nil {
		return nil, err
	}

	newApp.initializePitRadio(opts)

	newApp.log.Debug().
		Str("component", "app").
		Str("result", "success").
		Msg("init")

	return newApp, nil
}

// RunResult is the result of a mode function.
type RunResult int

const (
	RunResultContinue RunResult = iota
	RunResultSwitchMode
	RunResultExit
)

// Start launches the application and switches between setup and run modes as needed.
func (a *App) Start() {
	for {
		status := a.setupMode.Status(context.Background())
		setupModeActive := status.Available && status.FlagEnabled

		a.log.Debug().Bool("available", status.Available).Bool("flagEnabled", status.FlagEnabled).Msg("Setup mode status")

		if setupModeActive {
			result := a.runSetupMode()
			if result == RunResultExit {
				return
			}
		} else {
			result := a.runAppMode()
			if result == RunResultExit {
				return
			}
		}
	}
}

// Close performs cleanup and resource deallocation before application exit.
func (a *App) Close() {
	a.log.Info().Msg("Shutting down app")

	// Cancel context to signal all background goroutines to stop
	if a.cancel != nil {
		a.log.Debug().Msg("Cancelling context")
		a.cancel()
		a.log.Debug().Msg("Context cancelled")
	}

	// Close WebUI first to stop broadcaster and WebSocket clients
	if a.webUI != nil {
		a.log.Debug().Msg("Closing WebUI")
		a.webUI.Close()
		a.log.Debug().Msg("WebUI closed")
	}

	// Stop HTTP server to prevent new requests
	a.log.Debug().Msg("Stopping HTTP server")
	a.stopHTTPServer()
	a.log.Debug().Msg("HTTP server stopped")

	// Stop the updater if running
	if a.updater != nil {
		a.updater.Stop()
	}

	err := a.synth.Close()
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "audio output device").
			Str("result", "failure").
			Msg("close")
	}

	if a.pitRadio != nil {
		err = a.pitRadio.Close()
		if err != nil {
			a.log.Error().
				Err(err).
				Str("component", "discord").
				Str("result", "failure").
				Msg("close")
		}
	} else {
		a.log.Info().
			Str("component", "discord").
			Str("result", "skipped").
			Str("reason", "object is nil").
			Msg("close")
	}

	err = a.ui.Screen.RenderSplashScreen(a.i18n.GetString(languagedb.UIQuit))
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "ui").
			Str("sub", "screen").
			Str("result", "failure").
			Msg("render splash screen")
	}

	// Wait for all background goroutines to finish
	a.log.Debug().Msg("Waiting for goroutines")
	a.wg.Wait()
	a.log.Debug().Msg("All goroutines finished")

	// Give time for cleanup to complete before closing display
	time.Sleep(1 * time.Second)
	a.display.Close()
}

// runSetupMode runs setup mode and returns ModeSwitchResult.
func (a *App) runSetupMode() RunResult {
	a.log.Info().Msg("Launching setup mode")

	if a.httpServer == nil {
		a.startHTTPServer()
	}

	a.activeHTTPHandler = a.setupMode.GetHTTPHandler()

	// Run() blocks until setup completes or is cancelled (via shutdown channel)
	a.setupMode.Run()

	// Check if we received an exit code while setup was running
	select {
	case exitCode := <-a.exitCodeChan:
		if exitCode == exitcode.Success || exitCode == exitcode.SetupMode {
			a.log.Info().Msg("Setup mode signaled switch to run mode")

			return RunResultSwitchMode
		}

		// Shutdown requested
		return RunResultExit
	default:
		// No exit code - setup completed normally
		a.log.Info().Msg("Setup mode completed, switching to run mode")

		return RunResultSwitchMode
	}
}

// runAppMode runs the main app logic and returns ModeSwitchResult.
func (a *App) runAppMode() RunResult {
	a.log.Info().Msg("Launching run mode")

	readyMessage := a.i18n.GetString(languagedb.UIReady)

	err := a.ui.Screen.RenderSplashScreen(readyMessage)
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "ui").
			Str("sub", "screen").
			Str("result", "failure").
			Msg("render splash screen")
	}

	// Ensure webUI is initialized before setting handler
	if a.config.GetAppWebUIEnabled() && a.webUI == nil {
		a.webUI = webui.New(webui.Config{
			Log:                a.log,
			Port:               a.config.GetAppWebUIPort(),
			TelemetryChartFeed: a.telemetryChartFeed,
			VehicleInfoFeed:    a.vehicleInfoFeed,
			CircuitInfoFeed:    a.circuitInfoFeed,
			RaceInfoFeed:       a.raceInfoFeed,
			GameStateFeed:      a.gameStateFeed,
			LogStatsFeed:       a.logStatsFeed,
			Config:             a.config,
			Calibrator:         a.calibrator,
			ShutdownChan:       a.exitCodeChan,
			SetupMode:          a.setupMode,
			LogStore:           a.logStore,
			BuildVersion:       Version,
			BuildCommitHash:    CommitHash,
			BuildTime:          BuildTime,
			BuildPlatform:      Platform,
			Updater:            a.updater,
		})
	}

	if a.config.GetAppWebUIEnabled() {
		if a.httpServer == nil {
			a.startHTTPServer()
		}

		if a.webUI != nil {
			a.activeHTTPHandler = a.webUI.GetHTTPHandler()
		}
	} else {
		if a.httpServer != nil {
			a.stopHTTPServer()
		}

		a.activeHTTPHandler = nil
	}

	// Start the updater if initialized (pass parent context for lifecycle management)
	if a.updater != nil {
		a.updater.Start(context.Background())
	}

	a.run()

	// run() returned because mainLoop exited due to exit code signal
	code := <-a.exitCodeChan

	// Check for special exit codes
	switch code { //nolint:exhaustive // other codes mean normal exit
	case exitcode.SetupMode:
		a.log.Info().Msg("Setup mode requested from run mode")

		return RunResultSwitchMode
	case exitcode.RestartGTClient:
		a.log.Info().Msg("GT client restart requested, reinitializing")

		err := a.reinitializeGTClient()
		if err != nil {
			a.log.Error().Err(err).Msg("Failed to reinitialize GT client")

			return RunResultExit
		}

		return RunResultContinue
	default:
		// Normal shutdown
		return RunResultExit
	}
}

// run starts the main application loop and associated goroutines.
func (a *App) run() {
	a.startBackgroundTasks()

	a.startAudioOutput()

	a.mainLoop()

	a.log.Debug().Msg("run() completed, mainLoop has exited")
}

// initializeConfig sets up configuration and logging.
func (a *App) initializeConfig(opts Options) {
	// load config from file
	a.config = config.New(config.Options{
		ConfigFile: opts.ConfigFile,
		Logger:     *opts.Logger,
	})

	zerolog.FloatingPointPrecision = 5

	// update to configured log level when greater than current
	configLogLevel, err := zerolog.ParseLevel(a.config.GetAppLogLevel())
	if err != nil {
		a.log.Error().Int("config value", int(configLogLevel)).Msg("invalid log level")
	}

	if configLogLevel < a.log.GetLevel() || configLogLevel >= zerolog.NoLevel {
		a.log = a.log.Level(configLogLevel).With().Logger()
		a.log.Debug().Str("level", configLogLevel.String()).Str("source", "config").Msg("log level update")
	}

	cacheDir := filepath.Join(a.config.GetAppBaseDir(), "data", "cache")
	a.cache = cache.New(cacheDir, *opts.Logger)
}

// initializeI18n sets up language translations.
func (a *App) initializeI18n(opts Options) error {
	var err error

	a.i18n, err = i18n.New(
		a.config.GetAppLanguage(),
		*opts.Logger,
	)
	if err != nil {
		a.log.Error().Err(err).Str("component", "i18n").Str("result", "failure").Msg("init")

		return err
	}

	a.config.SetI18n(a.i18n)
	a.log.Debug().Str("language", a.i18n.LanguageCode()).Str("result", "success").Msg("init language")

	return nil
}

// initializeHardware sets up display and HID hardware based on configuration.
func (a *App) initializeHardware(hidEvents chan ui.HIDInputEvent) error {
	var err error

	// initialise display and button hardware
	switch a.config.GetHardwareModel() {
	case "pirateaudio":
		err = a.initializePirateAudio(hidEvents)
	case "spotpear":
		err = a.initializeSpotpear(hidEvents)
	case "waveshare":
		err = a.initializeWaveshare(hidEvents)
	default:
		a.initializeConsole()
	}

	// fallback to console display on error
	if err != nil {
		a.initializeConsole()
	}

	return err
}

// initializePirateAudio sets up Pirate Audio hardware.
func (a *App) initializePirateAudio(hidEvents chan ui.HIDInputEvent) error {
	hardware.Init()

	orientation := a.config.GetDisplayOrientation()

	var err error

	a.display, err = pirateaudio.NewDisplay(pirateaudio.DisplayOptions{
		Orientation: orientation,
		I18n:        a.i18n,
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "pirate audio").
			Str("sub", "display").
			Str("result", "failure").
			Msg("init")

		return err
	}

	a.log.Debug().
		Str("component", "pirate audio").
		Str("sub", "display").
		Str("result", "success").
		Msg("init")

	pirateaudio.SetupHID(orientation, hidEvents)
	a.log.Debug().
		Str("component", "pirate audio").
		Str("sub", "hid").
		Msg("init")

	return nil
}

// initializeSpotpear sets up Spotpear hardware.
func (a *App) initializeSpotpear(hidEvents chan ui.HIDInputEvent) error {
	hardware.Init()

	var err error

	a.display, err = spotpear.NewDisplay(spotpear.DisplayOptions{
		Orientation: a.config.GetDisplayOrientation(),
		I18n:        a.i18n,
	})
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "spotpear game 1.3").
			Str("sub", "display").
			Str("result", "failure").
			Msg("init")

		return err
	}

	a.log.Debug().
		Str("component", "spotpear game 1.3").
		Str("sub", "display").
		Str("result", "success").
		Msg("init")

	spotpear.SetupHID(hidEvents)
	log.Debug().
		Str("component", "spotpear game 1.3").
		Str("sub", "hid").
		Msg("init")

	return nil
}

// initializeWaveshare sets up Waveshare hardware.
func (a *App) initializeWaveshare(hidEvents chan ui.HIDInputEvent) error {
	hardware.Init()

	orientation := a.config.GetDisplayOrientation()

	var err error

	a.display, err = waveshare.NewDisplay(waveshare.DisplayOptions{
		Orientation: orientation,
		I18n:        a.i18n,
	})
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "waveshare 14972").
			Str("sub", "display").
			Str("result", "failure").
			Msg("init")

		return err
	}

	a.log.Debug().
		Str("component", "waveshare 14972").
		Str("sub", "display").
		Str("result", "success").
		Msg("init")

	waveshare.SetupHID(orientation, hidEvents)
	log.Debug().
		Str("component", "waveshare 14972").
		Str("sub", "hid").
		Msg("init")

	return nil
}

// initializeConsole sets up console display.
func (a *App) initializeConsole() {
	a.display = console.New()
	a.log.Debug().
		Str("component", "console").
		Str("sub", "display").
		Str("result", "success").
		Msg("init")
}

// initializeUI sets up the user interface.
func (a *App) initializeUI(opts Options, hidEvents chan ui.HIDInputEvent) error {
	a.ui = ui.NewUserInterface(&ui.Config{
		I18n:             a.i18n,
		HIDEvents:        hidEvents,
		Display:          a.display,
		LiveData:         &ui.LiveData{Gear: kinematics.NullGear},
		Log:              *opts.Logger,
		SettingsCallback: a.settingAction,
		DevToolsEnabled:  a.config.GetDevToolsEnabled,
		ExitCodeChan:     a.exitCodeChan,
	})

	startingMessage := a.i18n.GetString(languagedb.UIStarting)

	err := a.ui.Screen.RenderSplashScreen(startingMessage)
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "ui").
			Str("sub", "screen").
			Str("result", "failure").
			Msg("render splash screen")

		return fmt.Errorf("render splash screen: %w", err)
	}

	// Watch for display orientation changes and update display hardware
	a.wg.Add(1)

	go func() {
		defer func() {
			a.log.Debug().Msg("watchDisplayOrientation goroutine exiting")
			a.wg.Done()
		}()

		a.watchDisplayOrientation()
	}()

	return nil
}

// getIPAddress retrieves the local IP address of the host machine.
// Currently returns the first non-loopback IPv4 address found.
func getIPAddress() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		// Check if the address is an IP address (not a network address)
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			// Check if it's an IPv4 address
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}

	return ""
}

// updateIPAddress periodically updates the application's IP address.
// This handles cases where the IP is not available on startup or changes via DHCP.
func (a *App) updateIPAddress() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			newIP := getIPAddress()
			if newIP != "" && newIP != a.ipAddress {
				a.ipAddress = newIP
				a.log.Info().
					Str("ip", newIP).
					Msg("IP address updated")
			}
		}
	}
}

// watchDisplayOrientation monitors for display orientation changes and updates the display hardware.
func (a *App) watchDisplayOrientation() {
	currentOrientation := a.config.GetDisplayOrientation()
	a.display.SetOrientation(currentOrientation)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			newOrientation := a.config.GetDisplayOrientation()
			if newOrientation != currentOrientation {
				currentOrientation = newOrientation
				a.display.SetOrientation(currentOrientation)

				// Update HID button mappings based on hardware model
				switch a.config.GetHardwareModel() {
				case "pirateaudio":
					pirateaudio.UpdateOrientation(currentOrientation)
				case "waveshare":
					waveshare.UpdateOrientation(currentOrientation)
				}

				a.ui.ForceRedraw()
				a.log.Debug().Int("orientation", currentOrientation).Msg("display orientation changed")
			}
		}
	}
}

// initializeComponents sets up synthesizer, GT client, and other core components.
func (a *App) initializeComponents(opts Options) error {
	var err error

	// initialise synthesizer
	a.synth, err = synthesizer.New(&synthesizer.SynthOpts{
		Config:     a.config.GetSynthesizer(),
		BaseConfig: a.config,
		Logger:     *opts.Logger,
		Kinematics: &a.kinematics,
		Calibrator: a.calibrator,
	})
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "synth").
			Str("result", "failure").
			Msg("init")

		_ = a.ui.Screen.RenderErrorScreen("Synth init")

		return err
	}

	// initialise GT telemetry client
	gtClientLogger := opts.Logger.With().Str("component", "gt client").Logger()

	a.gtClient, err = gttelemetry.New(gttelemetry.Options{
		Source:    a.config.GetTelemetrySource(),
		Logger:    &gtClientLogger,
		LogLevel:  a.config.GetAppLogLevel(),
		VehicleDB: a.config.GetAppVehicleDBFile(),
	})
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "gt client").
			Str("result", "failure").
			Msg("init")

		_ = a.ui.Screen.RenderErrorScreen("GT client init")

		return err
	}

	a.odometer = odometer.New(*opts.Logger)

	a.fuelRange = fuelrange.New(*opts.Logger)

	a.tyres = tyres.New(
		a.config.GetPitRadioTyreTemperatureOptimalCelsius(),
		a.config.GetPitRadioTyreTemperatureOperatingWindow(),
		a.config.GetPitRadioTyreTemperatureMarginCelsius(),
		gtmodels.CornerSet{},
	)

	a.circuit, err = circuit.New(*a.gtClient.CircuitDB, *opts.Logger)
	if err != nil {
		// TODO: fatal error?
		a.log.Error().
			Err(err).
			Str("package", "circuit").
			Str("result", "failure").
			Msg("init")
	}

	// Initialize updater (always available for manual checks)
	// Auto-check setting controls whether periodic checks are scheduled
	updateBaseURL := a.config.GetAppUpdateBaseURL()
	if updateBaseURL != "" {
		baseDir := a.config.GetAppBaseDir()

		a.updater, err = updater.New(&updater.Config{
			Enabled:         a.config.GetAppUpdateAutoCheck(),
			BaseURL:         updateBaseURL,
			Channel:         a.config.GetAppUpdateChannel(),
			CheckInterval:   time.Duration(a.config.GetAppUpdateCheckIntervalMinutes()) * time.Minute,
			HTTPTimeout:     updater.DefaultHTTPTimeout,
			DownloadTimeout: updater.DefaultDownloadTimeout,
			AutoInstall:     a.config.GetAppUpdateAutoInstall(),
			InstallDir:      filepath.Join(baseDir, "bin"),
			InitDir:         filepath.Join(baseDir, "init"),
			DataDir:         filepath.Join(baseDir, "data", "update"),
			BinaryName:      "simtezilo",
			ServiceName:     "simtezilo",
			UseSystemd:      true,
		}, Version, a.log)
		if err != nil {
			a.log.Warn().
				Err(err).
				Str("component", "updater").
				Str("result", "failure").
				Msg("init (continuing without updates)")
		} else {
			// Check for existing downloads immediately (works even if auto-check is disabled)
			a.updater.CheckExistingDownloads()

			a.log.Debug().
				Str("component", "updater").
				Str("result", "success").
				Bool("autoCheckEnabled", a.config.GetAppUpdateAutoCheck()).
				Msg("init")
		}
	}

	return nil
}

// initializePitRadio sets up pit radio notification service based on configuration.
func (a *App) initializePitRadio(opts Options) {
	if !a.config.PitRadioEnabled() {
		return
	}

	pitRadioOutput := a.config.GetPitRadioOutput()
	switch pitRadioOutput {
	case "discord":
		a.initialiseDiscord(opts)
	case "log":
		var err error

		a.pitRadio, err = pitradio.NewLogOutput(&a.log)
		if err != nil {
			a.log.Error().
				Err(err).
				Str("component", "pit radio log output").
				Str("result", "failure").
				Msg("init")
		}
	default:
		a.config.SetPitRadioEnabled(false)

		a.log.Warn().
			Str("component", "pit radio").
			Str("output", pitRadioOutput).
			Str("state", "disabled").
			Msg("Invalid output type")

		return
	}

	a.resetPitRadioState()
}

// initialiseDiscord sets up Discord pit radio bot.
func (a *App) initialiseDiscord(opts Options) {
	token := a.config.GetDiscordToken()
	guildID := a.config.GetDiscordGuildID()
	channelID := a.config.GetDiscordChannelID()
	voiceChannelID := a.config.GetDiscordVoiceChannelID()

	hasTextConfig := token != "" && channelID != ""
	hasVoiceConfig := token != "" && guildID != "" && voiceChannelID != ""

	if !hasTextConfig && !hasVoiceConfig {
		a.log.Warn().
			Str("component", "discord").
			Msg("Pit Radio is enabled but Discord configuration is incomplete - Discord bot will not function")

		if token == "" {
			a.log.Warn().
				Str("component", "discord").
				Msg("Missing Discord bot token")
		}

		if channelID == "" && voiceChannelID == "" {
			a.log.Warn().
				Str("component", "discord").
				Msg("Missing Discord channel ID and voice channel ID")
		}

		if guildID == "" && voiceChannelID != "" {
			a.log.Warn().
				Str("component", "discord").
				Msg("Missing Discord guild ID (required for voice)")
		}
	}

	discordBotConfig := discord.Config{
		Enabled:        a.config.PitRadioEnabled(),
		Token:          token,
		ChannelID:      channelID,
		VoiceChannelID: voiceChannelID,
		GuildID:        guildID,
		MessageGap:     time.Duration(a.config.GetPitRadioMessageSendIntervalMs()) * time.Millisecond,
		Cache:          &a.cache,
		SampleBank:     a.synth.EffectSampleBank(),
		Logger:         *opts.Logger,
	}

	var err error

	a.pitRadio, err = discord.New(discordBotConfig)
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "discord").
			Str("result", "failure").
			Msg("init")
	}
}

// reinitializeGTClient reinitializes the GT telemetry client with current config settings.
func (a *App) reinitializeGTClient() error {
	gtClientLogger := a.log.With().Str("component", "gt client").Logger()

	var err error

	a.gtClient, err = gttelemetry.New(gttelemetry.Options{
		Source:    a.config.GetTelemetrySource(),
		Logger:    &gtClientLogger,
		LogLevel:  a.config.GetAppLogLevel(),
		VehicleDB: a.config.GetAppVehicleDBFile(),
	})
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "gt client").
			Str("result", "failure").
			Msg("reinit")

		_ = a.ui.Screen.RenderErrorScreen("GT client reinit")

		return err
	}

	// Reinitialize circuit with new GT client
	a.circuit, err = circuit.New(*a.gtClient.CircuitDB, a.log)
	if err != nil {
		a.log.Error().
			Err(err).
			Str("package", "circuit").
			Str("result", "failure").
			Msg("reinit")
	}

	a.log.Info().
		Str("source", a.config.GetTelemetrySource()).
		Msg("GT client reinitialized")

	return nil
}

// startHTTPServer creates and starts the HTTP server.
func (a *App) startHTTPServer() {
	port := a.config.GetAppWebUIPort()
	a.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           http.HandlerFunc(a.handleHTTP),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	go func() {
		a.log.Info().Msgf("Starting HTTP server on port %d", port)

		err := a.httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			a.log.Error().Err(err).Msg("HTTP server error")
		}
	}()
}

// stopHTTPServer gracefully shuts down the HTTP server.
func (a *App) stopHTTPServer() {
	if a.httpServer == nil {
		return
	}

	a.log.Info().Msg("Stopping HTTP server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := a.httpServer.Shutdown(ctx)
	if err != nil {
		a.log.Error().Err(err).Msg("HTTP server shutdown error")
	}

	a.httpServer = nil
}

// handleHTTP delegates requests to the current handler based on mode.
func (a *App) handleHTTP(writer http.ResponseWriter, request *http.Request) {
	if a.activeHTTPHandler == nil {
		http.NotFound(writer, request)

		return
	}

	a.activeHTTPHandler.ServeHTTP(writer, request)
}

// getDisplayLCD returns the underlying ST7789LCD display if available, otherwise returns nil.
func (a *App) getDisplayLCD() *display.ST7789LCD {
	if lcd, ok := a.display.(*display.ST7789LCD); ok {
		return lcd
	}

	return nil
}

func (a *App) startAudioOutput() {
	outputSampleRate := beep.SampleRate(a.config.GetSynthOutputSampleRateHz())

	hapticStreamer := synthesizer.NewHapticStream(a.synth, outputSampleRate)

	err := speaker.Init(
		outputSampleRate,
		outputSampleRate.N(time.Second/15),
	)
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "audio output device").
			Str("result", "failure").
			Msg("init")
		_ = a.ui.Screen.RenderErrorScreen("Audio output init")

		return
	}

	go speaker.Play(hapticStreamer)
}

// switchToSetupMode creates the setup mode flag file and signals the app to exit for restart in setup mode.
func (a *App) switchToSetupMode() {
	a.log.Info().Msg("Setup mode requested")

	// Run the setup command to enable setup mode
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := a.setupMode.PlatformAction(ctx, platform.SetupEnable, nil)
	if err != nil {
		_ = a.ui.Screen.RenderErrorScreen("Setup enable")

		a.log.Error().
			Err(err).
			Msg("Failed to enable setup mode")

		return
	}

	// Display message and trigger exit for restart
	_ = a.ui.Screen.RenderSplashScreen("Restarting")

	a.log.Info().Msg("Setup mode enabled, exiting for restart")

	time.Sleep(2 * time.Second)

	// Signal exit with setup mode code
	a.exitCodeChan <- exitcode.SetupMode
}

// signalStartupSuccess calls the platform command to signal successful application startup.
// This resets the failed start counter to prevent automatic rollback.
func (a *App) signalStartupSuccess() {
	platformCommand := filepath.Join(a.config.GetAppBaseDir(), "bin", "platform")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := platform.RunCommand(ctx, platformCommand, a.log, platform.SignalStart, nil)
	if err != nil {
		a.log.Warn().
			Err(err).
			Msg("Failed to signal startup success to platform command")

		return
	}

	a.log.Info().Msg("Successfully signaled startup to platform, failed start counter reset")
}

// mainLoop is the primary application loop handling telemetry updates, haptics, UI updates, and pit radio
// notifications.
func (a *App) mainLoop() { //nolint:cyclop // compact and simple enough
	tickerHaptics := time.NewTicker((1000 / hapticFrameRate) * time.Millisecond)
	tickerGeneral := time.NewTicker((1000 / telemetryFrameRate) * time.Millisecond)
	tickerEngineHaptics := time.NewTicker((1000 / engineHapticFrameRate) * time.Millisecond)
	tickerDisplay := time.NewTicker((1000 / displayFrameRate) * time.Millisecond)
	tickerPitRadio := time.NewTicker((1000 / pitRadioFrameRate) * time.Millisecond)
	tickerRaceData := time.NewTicker(500 * time.Millisecond)
	tickerDebug := time.NewTicker(30 * time.Second)

	a.log.Debug().Str("component", "app").Str("result", "success").Msg("main loop started")

	for {
		select {
		case <-a.ctx.Done():
			a.log.Debug().Msg("mainLoop exiting due to context cancellation")

			return
		case <-a.exitCodeChan:
			return
		case <-tickerHaptics.C:
			a.handleHapticsTick()
		case <-tickerGeneral.C:
			a.handleGeneralTick()
		case <-tickerEngineHaptics.C:
			a.handleEngineHapticsTick()
		case <-tickerDisplay.C:
			a.handleDisplayTick()
		case <-tickerPitRadio.C:
			a.handlePitRadioTick()
		case <-tickerRaceData.C:
			a.handleRaceDataTick()
		case <-tickerDebug.C:
			a.handleDebugTick()
		}
	}
}

// handleHapticsTick processes haptics-related updates.
func (a *App) handleHapticsTick() {
	// Skip telemetry-dependent processing if no telemetry available
	if !a.gtClient.Telemetry.TelemetryStarted() {
		return
	}

	_ = a.handleGameStateChange()

	stateChanged := a.updateState()

	if a.gtClient.Telemetry.IsInMainMenu() {
		return
	}

	if stateChanged {
		a.handleVehicleChange()
		a.generateForceHaptics()
		a.manageRecordingState()
	}

	a.checkRaceComplete()
	a.checkInPostRaceMenu()
	a.checkForNewLap()
}

// handleGeneralTick processes general telemetry updates.
func (a *App) handleGeneralTick() {
	// Update calibrator state - this works independently of telemetry
	a.synth.UpdateCalibrator()

	// Skip if no telemetry available
	if !a.gtClient.Telemetry.TelemetryStarted() {
		return
	}

	if a.gtClient.Telemetry.IsOnCircuit() {
		a.updateFuelRange()
		a.updateTyreTemperature()
	}

	a.updateCircuit()
	a.sendTelemetryChartData()
}

// handleEngineHapticsTick processes engine haptics updates.
func (a *App) handleEngineHapticsTick() {
	// Skip if no telemetry available
	if !a.gtClient.Telemetry.TelemetryStarted() {
		return
	}

	if a.gtClient.Telemetry.IsOnCircuit() {
		a.generateEngineHaptic()
	}
}

// handleDisplayTick processes display updates.
func (a *App) handleDisplayTick() {
	a.ui.UpdateDisplay(ui.LiveData{
		Gear:            a.kinematics.Current.TransmissionGear,
		TelemetryActive: a.state.telemetryActive,
		Calibrating:     a.calibrator.IsEnabled(),
	})
}

// handlePitRadioTick processes pit radio updates.
func (a *App) handlePitRadioTick() {
	// Skip if no telemetry available
	if !a.gtClient.Telemetry.TelemetryStarted() {
		return
	}

	if !a.gtClient.Telemetry.IsOnCircuit() {
		return
	}

	a.sendPitRadioMessage()
}

// handleRaceDataTick periodically pushes current game state to ensure UI stays updated.
func (a *App) handleRaceDataTick() {
	if a.webUI == nil {
		return
	}

	// Skip if no telemetry available
	if !a.gtClient.Telemetry.TelemetryStarted() {
		return
	}

	a.pushGameState()

	if a.gtClient.Telemetry.IsOnCircuit() {
		a.pushRaceInfo()
	}

	if a.vehicle.ID > 0 {
		a.pushVehicleInfo()
	}

	// Push circuit info if we have valid data OR if telemetry is active (to show "Analyzing...")
	if a.circuit.Name() != "" || a.circuit.Variation() != "" || (a.state.telemetryActive && a.gtClient.Telemetry.IsOnCircuit()) {
		a.pushCircuitInfo()
	}
}

// handleDebugTick processes debug logging.
func (a *App) handleDebugTick() {
	if a.log.GetLevel() > zerolog.DebugLevel {
		return
	}

	// Skip if no telemetry available
	if !a.gtClient.Telemetry.TelemetryStarted() {
		return
	}

	if !a.gtClient.Telemetry.IsOnCircuit() {
		return
	}

	a.log.Debug().
		Int("lap", int(a.state.current.lapNumber)).
		Float32("percent", a.gtClient.Telemetry.FuelLevelPercent()).
		Int("odometer", int(a.odometer.Read())).
		Float64("rate", a.fuelRange.UsageRatePerKm()).
		Int("range_metres", int(a.fuelRange.DistanceMetres())).
		Int("lap_remaining_pc", int(a.circuit.LapProgressRemaining()*100)).
		Int("circuit_length", int(a.circuit.LengthMetres())).
		Msg("debug fuel range")

	averageTemp := (a.gtClient.Telemetry.TyreTemperatureCelsius().FrontLeft +
		a.gtClient.Telemetry.TyreTemperatureCelsius().FrontRight +
		a.gtClient.Telemetry.TyreTemperatureCelsius().RearLeft +
		a.gtClient.Telemetry.TyreTemperatureCelsius().RearRight) / 4

	a.log.Debug().
		Float32("temp_avg", averageTemp).
		Float32("temp_fl", a.gtClient.Telemetry.TyreTemperatureCelsius().FrontLeft).
		Float32("temp_fr", a.gtClient.Telemetry.TyreTemperatureCelsius().FrontRight).
		Float32("temp_rl", a.gtClient.Telemetry.TyreTemperatureCelsius().RearLeft).
		Float32("temp_rr", a.gtClient.Telemetry.TyreTemperatureCelsius().RearRight).
		Msg("debug tyre temp")
}

func (a *App) newLapHandler() bool {
	a.log.Debug().
		Str("handler", "new lap events").
		Msg("Start")

	for {
		select {
		case <-a.ctx.Done():
			return false
		case <-a.lapStartEvents:
			lap := a.state.current.lapNumber
			lapTime := a.state.current.lastLapTime
			position := a.gtClient.Telemetry.GridPosition()
			coordinate := a.gtClient.Telemetry.PositionalMapCoordinates()
			odometerReading := a.odometer.Add(coordinate)

			// Track lap event if valid lap time
			// When we cross the line to start lap N, lastLapTime is for lap N-1
			if lapTime > 0 && lap > 1 {
				completedLap := lap - 1
				a.addLapEvent(completedLap, lapTime, position)
			}

			didUpdate := a.circuit.UpdateCircuit(odometerReading, lap, lapTime, coordinate, gtmodels.CoordinateTypeStartLine)
			if didUpdate {
				a.log.Debug().Msg("lapStartEvents: circuit was updated, calling pushCircuitInfo")
				a.state.last.lastLapTime = 0
				a.pushCircuitInfo()
			} else {
				a.log.Debug().Msg("lapStartEvents: circuit was NOT updated")
			}

			a.notifyLapTime()
			a.notifyLapNumber()
		default:
			time.Sleep(8 * time.Millisecond)
		}
	}
}

// resetAppState resets the application state.
func (a *App) resetAppState() {
	a.state.last = raceState{
		transmissionGear: kinematics.NullGear,
		isLive:           true,
		gameState:        a.state.current.gameState,
	}

	a.state.current = raceState{
		transmissionGear: kinematics.NullGear,
		isLive:           true,
		gameState:        a.gtClient.Telemetry.GameState(),
	}

	a.synth.Silence()

	a.kinematics = kinematics.NewKinematicsState()

	// Reset engine haptic state to prevent polarity misalignment
	a.state.engine = engineState{}

	a.state.sessionEnded = false
	a.state.mainMenuFrameCount = 0

	a.log.Debug().Msg("App state reset")
}

// getGameStateString returns the current game state as a string for the web UI.
func (a *App) getGameStateString() string {
	state := a.state.current.gameState

	switch state { //nolint:exhaustive // handled by default
	case gtmodels.GameStateMainMenu:
		return "main_menu"
	case gtmodels.GameStateRaceMenu:
		return "race_menu"
	case gtmodels.GameStateLive:
		if a.gtClient.Telemetry.Flags().GamePaused {
			return "paused"
		}

		return "on_circuit"
	case gtmodels.GameStateReplay:
		return "replay"
	default:
		return "unknown"
	}
}

func (a *App) handleGameStateChange() bool {
	if a.sessionFinished() {
		return false
	}

	current := a.state.current.gameState
	stateChanged := a.gameStateHasChanged() && current != gtmodels.GameStateUnknown
	isOnCircuit := current == gtmodels.GameStateLive || current == gtmodels.GameStateReplay

	if isOnCircuit {
		a.handleContinuityFlags()
	}

	if !stateChanged {
		return false
	}

	switch current {
	case gtmodels.GameStateMainMenu:
		a.resetAllState("entered main menu")
	case gtmodels.GameStateRaceMenu:
		a.resetRaceState("entered race menu")
	case gtmodels.GameStateLive, gtmodels.GameStateReplay:
		// Toggle fuel range estimation when switching between live and replay modes
		if a.liveFlagHasChanged() {
			a.fuelRange.SetLive(a.state.current.isLive)
		}
	case gtmodels.GameStateUnknown:
		// do nothing
	default:
		a.log.Warn().
			Int("game_state", int(current)).
			Msg("unhandled game state change")
	}

	return true
}

// sessionFinished checks if the telemetry session has finished and resets state if so.
func (a *App) sessionFinished() bool {
	if !a.gtClient.Finished {
		return false
	}

	if a.state.sessionEnded {
		return true
	}

	a.resetAllState("telemetry stream ended")
	a.state.sessionEnded = true

	return true
}

// resetAllState resets all application, vehicle, and race states.
func (a *App) resetAllState(reason string) {
	// App states
	a.disableHaptics(reason)
	a.stopRecording()
	a.resetAppState()

	//	Vehicle states
	a.vehicle = vehicle.Characteristics{}
	a.clearVehicleInfo()
	a.odometer.Reset()
	a.fuelRange.Reset()

	// Race states
	a.resetPitRadioState()
	a.state.ResetRaceComplete()
	a.circuit.Reset()
	a.clearCircuitInfo()
	a.clearRaceInfo()

	a.log.Info().Str("reason", reason).Msg("Reset all state")
}

// resetRaceState resets race-specific states.
func (a *App) resetRaceState(reason string) {
	// App states
	a.disableHaptics(reason)
	a.stopRecording()

	// Vehicle states
	a.odometer.Reset()
	a.fuelRange.ResetEstimate()

	// Race states
	a.resetPitRadioState()
	a.state.ResetRaceComplete()
	a.circuit.ResetLapProgress()

	a.log.Debug().Str("reason", reason).Msg("Reset race state")
}

// handleContinuityFlags processes continuity-related flags such as time of day reset and loading.
func (a *App) handleContinuityFlags() {
	if a.timeOfDayHasReset() {
		// App state
		a.disableHaptics("time of day reset")

		// Vehicle state
		a.fuelRange.ResetEstimate()

		// Race state
		a.resetPitRadioState()
		a.circuit.ResetLapProgress()
		a.state.ResetRaceComplete()

		a.log.Debug().Msg("Time of day reset")
	}

	if a.gtClient.Telemetry.Flags().Loading {
		// Vehicle state
		a.fuelRange.ResetEstimate()

		a.log.Debug().Msg("Loading flag")
	}
}

// handleVehicleChange checks for changes in the vehicle and updates vehicle data accordingly.
func (a *App) handleVehicleChange() {
	if a.vehicleHasChanged() {
		a.disableHaptics("vehicle changed")

		a.updateVehicle()
	}
}

// updateVehicle updates the vehicle characteristics from the current telemetry vehicle ID.
func (a *App) updateVehicle() {
	vehicleType := vehicle.DetermineVehicleType(a.gtClient.Telemetry.VehicleType())
	engine := a.getEngineData()
	revLimit := a.gtClient.Telemetry.EngineRPMLight().Max

	a.adjustEngineHaptics(&engine, revLimit)

	var wheelbaseMetres float32

	var trackFrontMetres float32

	var trackRearMetres float32

	wheelbase := a.gtClient.Telemetry.VehicleWheelbaseMillimetres()

	if wheelbase > 0 {
		wheelbaseMetres = float32(wheelbase) / 1000
	} else {
		wheelbaseMetres = (float32(a.gtClient.Telemetry.VehicleLengthMillimetres()) / 1000) * 0.55
	}

	trackFront := a.gtClient.Telemetry.VehicleTrackFrontMillimetres()

	trackRear := a.gtClient.Telemetry.VehicleTrackRearMillimetres()
	if trackFront > 0 || trackRear > 0 {
		trackFrontMetres = float32(trackFront) / 1000
		trackRearMetres = float32(trackRear) / 1000
	} else {
		trackFrontMetres = (float32(a.gtClient.Telemetry.VehicleWidthMillimetres()) / 1000) * 0.85
		trackRearMetres = trackFrontMetres
	}

	trackWidthMetres := (trackFrontMetres + trackRearMetres) / 2

	a.vehicle = vehicle.Characteristics{
		ID:          a.gtClient.Telemetry.VehicleID(),
		VehicleType: vehicleType,
		Engine:      engine,
		RevLimit:    a.normalizeRevLimit(revLimit),
		Dimensions: vehicle.Dimensions{
			WheelbaseMetres:    wheelbaseMetres,
			TrackWidthMetres:   trackWidthMetres,
			LongitudinalRadius: wheelbaseMetres / 2,
			TransverseRadius:   trackWidthMetres / 2,
		},
	}

	a.setTransmissionGain(vehicleType)
	a.resetVehicleState()
	a.logVehicleUpdate(engine, revLimit)
	a.pushVehicleInfo()
}

// getEngineData retrieves and processes engine characteristics.
func (a *App) getEngineData() vehicle.EngineCharacteristics {
	engineLayout := a.gtClient.Telemetry.VehicleEngineLayout()
	bankAngle := a.gtClient.Telemetry.VehicleEngineBankAngle()
	crankPlaneAngle := a.gtClient.Telemetry.VehicleEngineCrankPlaneAngle()
	revLimit := a.gtClient.Telemetry.EngineRPMLight().Max

	engine, err := a.getEngineCharacteristics(engineLayout, bankAngle, crankPlaneAngle, revLimit)
	if err != nil {
		a.log.Error().
			Err(err).
			Str("engine_layout", engineLayout).
			Float32("cylinder_angle", bankAngle).
			Float32("crank_plane_angle", crankPlaneAngle).
			Msg("failed to get engine characteristics")
	}

	return engine
}

// adjustEngineHaptics adjusts engine haptics based on pulse rate limits.
func (a *App) adjustEngineHaptics(engine *vehicle.EngineCharacteristics, revLimit uint16) {
	peakNaturalPulseRate := float64(revLimit) * engine.FiringFrequency
	peakPulseRate := peakNaturalPulseRate * engine.Haptics.PulseScale

	if peakPulseRate > maxPulseRate {
		engine.Haptics.PulseScale = (maxPulseRate / peakPulseRate) * engine.Haptics.PulseScale
	}
}

// normalizeRevLimit sets a default rev limit if not available.
func (a *App) normalizeRevLimit(revLimit uint16) uint16 {
	if revLimit == 0 {
		return 8000
	}

	return revLimit
}

// setTransmissionGain sets the transmission gain based on vehicle type.
func (a *App) setTransmissionGain(vehicleType vehicle.TypeName) {
	switch vehicleType {
	case vehicle.TypeRace:
		a.transmissionGainMin = a.config.GetSynthTransmissionGain() + a.config.GetSynthTransmissionGainMinRace()
	case vehicle.TypeTuned:
		minGain := (a.config.GetSynthTransmissionGainMinStreet() + a.config.GetSynthTransmissionGainMinRace()) / 2
		a.transmissionGainMin = a.config.GetSynthTransmissionGain() + minGain
	case vehicle.TypeStreet:
		fallthrough
	default:
		a.transmissionGainMin = a.config.GetSynthTransmissionGain() + a.config.GetSynthTransmissionGainMinStreet()
	}
}

// resetVehicleState resets vehicle-related state.
func (a *App) resetVehicleState() {
	a.state.last.transmissionGear = a.state.current.transmissionGear
	a.odometer.Reset()
	a.fuelRange.Reset()
}

// logVehicleUpdate logs vehicle update information.
func (a *App) logVehicleUpdate(engine vehicle.EngineCharacteristics, revLimit uint16) {
	bankAngle := a.gtClient.Telemetry.VehicleEngineBankAngle()
	crankPlaneAngle := a.gtClient.Telemetry.VehicleEngineCrankPlaneAngle()
	peakNaturalPulseRate := float64(revLimit) * engine.FiringFrequency
	peakPulseRate := peakNaturalPulseRate * engine.Haptics.PulseScale
	a.log.Debug().
		Str("engine_layout", a.vehicle.Engine.Layout).
		Str("resolved_engine", a.vehicle.Engine.DBEntry).
		Float32("crank_plane_angle", crankPlaneAngle).
		Float32("cylinder_bank_angle", bankAngle).
		Uint16("rev_limit", a.vehicle.RevLimit).
		Float64("peak_natural_pulse_rate", peakNaturalPulseRate).
		Float64("pulse_scale", a.vehicle.Engine.Haptics.PulseScale).
		Float64("peak_pulse_rate", peakPulseRate).
		Float64("pulse_overlap", a.vehicle.Engine.PulseOverlap).
		Msg("engine characteristics")

	a.log.Info().
		Uint32("ID", a.vehicle.ID).
		Str("manufacturer", a.gtClient.Telemetry.VehicleManufacturer()).
		Str("model", a.gtClient.Telemetry.VehicleModel()).
		Str("type", string(a.vehicle.VehicleType)).
		Str("engine", a.vehicle.Engine.DBEntry).
		Str("wheelbase", fmt.Sprintf("%.2f m", a.vehicle.Dimensions.WheelbaseMetres)).
		Str("track_width", fmt.Sprintf("%.2f m", a.vehicle.Dimensions.TrackWidthMetres)).
		Msg("vehicle update")
}

// gearHasChanged checks if the gear has changed based on telemetry data.
func (a *App) gearHasChanged() bool {
	// ignore gear change events from initial unset state
	if a.kinematics.Current.TransmissionGear == kinematics.NullGear ||
		a.kinematics.Last.TransmissionGear == kinematics.NullGear {
		return false
	}

	if a.kinematics.Current.TransmissionGear == a.kinematics.Last.TransmissionGear {
		return false
	}

	return true
}

// raceHasFinished returns true 5 seconds after the race has completed.
func (a *App) raceHasFinished() bool {
	if a.state.isInPostRaceMenu {
		return true
	}

	if a.state.raceComplete {
		return true
	}

	return false
}

// pushVehicleInfo sends the current vehicle manufacturer and model to the web UI.
func (a *App) pushVehicleInfo() {
	if a.webUI == nil {
		return
	}

	manufacturer := a.gtClient.Telemetry.VehicleManufacturer()
	model := a.gtClient.Telemetry.VehicleModel()
	carID := a.gtClient.Telemetry.VehicleID()

	if manufacturer == "" && model == "" {
		return
	}

	vehicleInfo := map[string]any{
		"manufacturer": manufacturer,
		"model":        model,
		"carID":        carID,
	}

	select {
	case a.vehicleInfoFeed <- vehicleInfo:
		a.log.Debug().
			Str("manufacturer", manufacturer).
			Str("model", model).
			Uint32("carID", carID).
			Msg("pushed vehicle info to web UI")
	default:
		a.log.Debug().Msg("vehicle info feed channel full, skipping push")
	}
}

// pushGameState sends the current game state to the web UI.
func (a *App) pushGameState() {
	if a.webUI == nil {
		return
	}

	gameStateStr := a.getGameStateString()

	select {
	case a.gameStateFeed <- gameStateStr:
		a.log.Debug().
			Str("gameState", gameStateStr).
			Msg("pushed game state to web UI")
	default:
		a.log.Debug().Msg("game state feed channel full, skipping push")
	}
}

// clearVehicleInfo sends empty vehicle info to the web UI to clear the display.
func (a *App) clearVehicleInfo() {
	if a.webUI == nil {
		return
	}

	vehicleInfo := map[string]any{
		"manufacturer": "",
		"model":        "",
		"carID":        uint32(0),
	}

	select {
	case a.vehicleInfoFeed <- vehicleInfo:
		a.log.Debug().Msg("cleared vehicle info in web UI")
	default:
		a.log.Debug().Msg("vehicle info feed channel full, skipping clear")
	}
}

// pushCircuitInfo sends the current circuit details to the web UI.
func (a *App) pushCircuitInfo() {
	if a.webUI == nil {
		a.log.Debug().Msg("pushCircuitInfo: webUI is nil")

		return
	}

	circuitName := a.circuit.Name()
	circuitVariation := a.circuit.Variation()
	circuitCountry := a.circuit.Country()
	circuitLength := a.circuit.LengthMetres()
	candidateCount := a.circuit.CandidateCount()

	a.log.Debug().
		Str("name", circuitName).
		Str("variation", circuitVariation).
		Str("country", circuitCountry).
		Float64("length", circuitLength).
		Int("candidates", candidateCount).
		Msg("push circuit data to web UI")

	// Send circuit info even if empty to indicate we're analyzing
	circuitInfo := map[string]string{
		"name":       circuitName,
		"variation":  circuitVariation,
		"country":    circuitCountry,
		"length":     fmt.Sprintf("%.2f", circuitLength/1000.0), // Convert metres to km
		"candidates": strconv.Itoa(candidateCount),
	}

	select {
	case a.circuitInfoFeed <- circuitInfo:
		a.log.Debug().Str("circuit", circuitName).Str("variation", circuitVariation).Msg("pushed circuit info to channel")
	default:
		a.log.Debug().Msg("circuit info feed channel full, skipping push")
	}
}

// clearCircuitInfo sends empty circuit info to the web UI to clear the display.
func (a *App) clearCircuitInfo() {
	if a.webUI == nil {
		return
	}

	circuitInfo := map[string]string{
		"name":      "",
		"variation": "",
		"country":   "",
		"length":    "",
	}

	select {
	case a.circuitInfoFeed <- circuitInfo:
		a.log.Debug().Msg("cleared circuit info in web UI")
	default:
		a.log.Debug().Msg("circuit info feed channel full, skipping clear")
	}
}

// addLapEvent adds a new lap event to the history.
func (a *App) addLapEvent(lap int16, lapTime time.Duration, position int16) {
	a.lapEventsMutex.Lock()
	defer a.lapEventsMutex.Unlock()

	// Check if this lap already exists (prevent duplicates)
	for _, existingEvent := range a.lapEvents {
		if existingEvent.Lap == lap {
			a.log.Debug().
				Int16("lap", lap).
				Msg("lap event already exists, skipping duplicate")

			return
		}
	}

	// Calculate delta from previous lap
	var delta time.Duration

	hasDelta := false
	// Lap 1 has no delta
	if lap > 1 {
		// Find the previous lap to calculate delta
		previousLapTime := a.getPreviousLapTime(lap)

		if previousLapTime > 0 {
			hasDelta = true
			delta = lapTime - previousLapTime
		}
	}

	// Update best lap time
	if a.bestLapTime == 0 || lapTime < a.bestLapTime {
		a.bestLapTime = lapTime
	}

	// Add new event
	event := lapEvent{
		Lap:      lap,
		LapTime:  lapTime,
		Delta:    delta,
		Position: position,
		HasDelta: hasDelta,
	}

	a.lapEvents = append(a.lapEvents, event)

	// Keep only last 10 laps
	if len(a.lapEvents) > 10 {
		a.lapEvents = a.lapEvents[len(a.lapEvents)-10:]
	}

	a.log.Debug().
		Int16("lap", lap).
		Dur("lapTime", lapTime).
		Dur("delta", delta).
		Int16("position", position).
		Msg("added lap event")
}

// pushRaceInfo sends the current race details to the web UI.
func (a *App) pushRaceInfo() {
	if a.webUI == nil {
		return
	}

	// Get race info from telemetry
	timeOfDay := a.gtClient.Telemetry.TimeOfDay()
	currentLap := a.state.current.lapNumber
	raceLaps := a.gtClient.Telemetry.RaceLaps()
	position := a.gtClient.Telemetry.GridPosition()
	gridSize := a.gtClient.Telemetry.RaceEntrants()

	// Format time of day as HH:MM:SS
	hours := int(timeOfDay.Hours())
	minutes := int(timeOfDay.Minutes()) % 60
	seconds := int(timeOfDay.Seconds()) % 60
	timeOfDayStr := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

	// Get lap events
	a.lapEventsMutex.Lock()

	lapEventsData := make([]map[string]any, 0, len(a.lapEvents))
	for _, event := range a.lapEvents {
		// Format lap time as MM:SS.mmm
		lapTimeStr := formatLapTime(event.LapTime)

		// Format delta as +/-SS.mmm or show as best lap
		deltaStr := "-"

		if event.HasDelta {
			switch {
			case event.Delta > 0:
				deltaStr = fmt.Sprintf("+%.3f", event.Delta.Seconds())
			case event.Delta < 0:
				deltaStr = fmt.Sprintf("-%.3f", -event.Delta.Seconds())
			default:
				// Delta == 0 means this is the best lap
				deltaStr = "0.000"
			}
		}

		lapEventsData = append(lapEventsData, map[string]any{
			"lap":      event.Lap,
			"laptime":  lapTimeStr,
			"delta":    deltaStr,
			"position": event.Position,
		})
	}

	a.lapEventsMutex.Unlock()

	raceInfo := map[string]any{
		"timeofday":  timeOfDayStr,
		"currentlap": strconv.Itoa(int(currentLap)),
		"racelaps":   strconv.Itoa(int(raceLaps)),
		"position":   strconv.Itoa(int(position)),
		"gridsize":   strconv.Itoa(int(gridSize)),
		"lapevents":  lapEventsData,
	}

	select {
	case a.raceInfoFeed <- raceInfo:
		a.log.Debug().
			Str("timeofday", timeOfDayStr).
			Int16("lap", currentLap).
			Int16("race_laps", raceLaps).
			Int("lap_events", len(lapEventsData)).
			Msg("pushed race info to web UI")
	default:
		a.log.Debug().Msg("race info feed channel full, skipping push")
	}
}

// formatLapTime formats a duration as MM:SS.mmm.
func formatLapTime(d time.Duration) string {
	totalSeconds := d.Seconds()
	minutes := int(totalSeconds / 60)
	seconds := totalSeconds - float64(minutes*60)

	return fmt.Sprintf("%d:%06.3f", minutes, seconds)
}

// clearRaceInfo sends empty race info to the web UI to clear the display.
func (a *App) clearRaceInfo() {
	if a.webUI == nil {
		return
	}

	// Clear lap events
	a.lapEventsMutex.Lock()
	a.lapEvents = nil
	a.bestLapTime = 0
	a.lapEventsMutex.Unlock()

	raceInfo := map[string]any{
		"timeofday":  "",
		"currentlap": "",
		"racelaps":   "",
		"position":   "",
		"gridsize":   "",
		"lapevents":  []map[string]any{},
	}

	select {
	case a.raceInfoFeed <- raceInfo:
		a.log.Debug().Msg("cleared race info in web UI")
	default:
		a.log.Debug().Msg("race info feed channel full, skipping clear")
	}
}

// logStatsBroadcaster periodically sends log statistics to the WebUI.
func (a *App) logStatsBroadcaster() {
	ticker := time.NewTicker(2 * time.Second) // Update stats every 2 seconds
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if a.webUI == nil || !a.webUI.HasActiveClients() || a.logStore == nil {
				continue
			}

			stats := a.logStore.GetStats()
			allLogs := a.logStore.GetAll()
			totalCount := len(allLogs)

			// Calculate total pages based on default page size of 100
			pageSize := 100

			totalPages := max((totalCount+pageSize-1)/pageSize, 1)

			logStats := map[string]any{
				"stats":      stats,
				"totalCount": totalCount,
				"totalPages": totalPages,
			}

			select {
			case a.logStatsFeed <- logStats:
				a.log.Debug().Int("totalCount", totalCount).Msg("broadcast log stats to web UI")
			default:
				a.log.Debug().Msg("log stats feed channel full; log stats not sent to web UI")
			}
		}
	}
}
