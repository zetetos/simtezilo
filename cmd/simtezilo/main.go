package main

import (
	"flag"
	"fmt"
	_ "image/png"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/zetetos/simtezilo/app"
	"github.com/zetetos/simtezilo/app/crashlog"
	"github.com/zetetos/simtezilo/app/exitcode"
	"github.com/zetetos/simtezilo/app/logstore"
	"github.com/zetetos/simtezilo/app/profiler"
)

type cliFlags struct {
	configFile       string
	crashLogDir      string
	logLevel         string
	profilerEndpoint string
	version          bool
}

func main() {
	exitSignal := make(chan exitcode.Code, 1)
	setupSignalHandler(exitSignal)

	flags := parseFlags()

	if flags.version {
		printVersion()
		os.Exit(0)
	}

	crashLogger := initCrashLogger(flags.crashLogDir)
	defer crashLogger.Close()

	defer handlePanic(crashLogger)

	loggerWithStore := initLogger(flags.logLevel)
	logger := loggerWithStore.Logger

	logStartupInfo(&logger)

	prof, err := startPyroscope(flags.profilerEndpoint, &logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to setup Pyroscope profiler")
	}

	exitCode := runApp(flags.configFile, exitSignal, &logger, loggerWithStore.Store, crashLogger)

	shutdownProfiler(prof, &logger)

	logger.Info().Int("exitCode", int(exitCode)).Msg("Exiting app")

	os.Exit(int(exitCode)) //nolint:gocritic // defers run before this exit at end of main
}

func initCrashLogger(crashLogDir string) *crashlog.CrashLogger {
	logDir := crashLogDir
	if logDir == "" {
		logDir = filepath.Join(os.TempDir(), "simtezilo")
	}

	err := crashlog.EnsureLogDir(logDir)
	if err != nil {
		log.Printf("Warning: Failed to create crash log directory: %v", err)
	}

	return crashlog.New(crashlog.Options{
		LogDir:     logDir,
		MaxSize:    10,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   true,
	})
}

func handlePanic(crashLogger *crashlog.CrashLogger) {
	if r := recover(); r != nil {
		crashLogger.WritePanic(r, app.Version, app.CommitHash, app.BuildTime, app.Platform)
		_ = crashLogger.Close()

		os.Exit(1)
	}
}

func setupSignalHandler(exitCodeChan chan exitcode.Code) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Printf("Received %v signal, shutting down\n", sig)

		exitCodeChan <- exitcode.Success

		close(exitCodeChan)
	}()
}

func parseFlags() cliFlags {
	var flags cliFlags

	flag.StringVar(&flags.configFile, "c", "", "Configuration file to load")
	flag.StringVar(&flags.crashLogDir, "d", "", "Directory for crash logs. Default is $TMPDIR/simtezilo")
	flag.StringVar(&flags.logLevel, "l", "info", "Log level. Default is 'info'")
	flag.StringVar(&flags.profilerEndpoint, "p", "", "Send profiles to this Pyroscope endpoint (http://host:port). Default is off")
	flag.BoolVar(&flags.version, "v", false, "Print version information and exit")

	flag.Parse()

	return flags
}

func printVersion() {
	fmt.Fprintf(
		os.Stdout,
		"Version: %s  Commit Hash: %s  Build Time: %s  Platform: %s\n",
		app.Version,
		app.CommitHash,
		app.BuildTime,
		app.Platform,
	)
}

func initLogger(logLevelArg string) *logstore.LoggerWithStore {
	logLevel, err := zerolog.ParseLevel(logLevelArg)
	if err != nil {
		log.Fatalf("Invalid log level: %s", err)
	}

	return logstore.NewLoggerWithStore(logLevel, 5000)
}

func logStartupInfo(logger *zerolog.Logger) {
	if app.BuildTime == "" {
		app.BuildTime = time.Now().Format("2006-01-02_15:04:05")
	}

	logger.Info().
		Str("Version", app.Version).
		Str("CommitHash", app.CommitHash).
		Str("BuildTime", app.BuildTime).
		Str("Platform", app.Platform).
		Msg("Starting Simtezilo")
}

func runApp(
	configFile string,
	exitCodeChan chan exitcode.Code,
	logger *zerolog.Logger,
	logStore *logstore.Store,
	crashLogger *crashlog.CrashLogger,
) exitcode.Code {
	application, err := app.New(app.Options{
		ConfigFile:   configFile,
		ExitCodeChan: exitCodeChan,
		Logger:       logger,
		LogStore:     logStore,
		CrashLogger:  crashLogger,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Error creating app")
	}

	go application.Start()

	exitCode := <-exitCodeChan

	application.Close()

	if exitCode == exitcode.RestartApp {
		logger.Info().Msg("Restarting application - exiting to allow process restart")

		return exitcode.Success
	}

	return exitCode
}

func shutdownProfiler(prof *profiler.PyroscopeProfiler, logger *zerolog.Logger) {
	if prof == nil {
		return
	}

	err := prof.Shutdown()
	if err != nil {
		logger.Fatal().Err(err).Msg("Error shutting down Pyroscope profiler")
	}
}

func startPyroscope(endpoint string, logger *zerolog.Logger) (*profiler.PyroscopeProfiler, error) {
	if endpoint == "" {
		return nil, nil
	}

	profiler, err := profiler.NewPyroscopeProfiler(
		endpoint,
		map[string]string{
			"app":       "simtezilo",
			"version":   app.Version,
			"buildTime": app.BuildTime,
			"hostname":  os.Getenv("HOSTNAME"),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create profiler: %w", err)
	}

	err = profiler.Start()
	if err != nil {
		return nil, fmt.Errorf("start profiler: %w", err)
	}

	logger.Info().Str("Component", "pyroscope").Str("endpoint", profiler.Endpoint()).Msg("profiler started")

	return profiler, nil
}
