package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"fyne.io/fyne/v2/app"
	"github.com/tartampluch/go-birthday/internal/config"
	"github.com/tartampluch/go-birthday/internal/engine"
	"github.com/tartampluch/go-birthday/internal/server"
	"github.com/tartampluch/go-birthday/internal/ui"
)

// main is the application entry point.
// It delegates execution to runMain to ensure that deferred function calls
// (like closing log files) are executed before the process terminates.
// os.Exit() does not run defers, so we must return an integer code first.
func main() {
	os.Exit(runMain())
}

// runMain manages the application lifecycle, argument parsing, and exit codes.
// Returns config.ExitCodeSuccess on success, config.ExitCodeError on failure.
func runMain() int {
	// -------------------------------------------------------------------------
	// 1. CLI Argument Parsing
	// -------------------------------------------------------------------------
	showVersion := flag.Bool(config.FlagVersion, false, config.FlagDescVersion)
	debugMode := flag.Bool(config.FlagDebug, false, config.FlagDescDebug)
	flag.Parse()

	if *showVersion {
		printVersion()
		return config.ExitCodeSuccess
	}

	// -------------------------------------------------------------------------
	// 2. Logging Initialization
	// -------------------------------------------------------------------------
	// We configure structured logging (slog) early to capture startup issues.
	logCloser := setupLogging(*debugMode)
	if logCloser != nil {
		defer func() {
			_ = logCloser.Close() // Best effort close
		}()
	}

	// -------------------------------------------------------------------------
	// 3. Context & Signal Handling
	// -------------------------------------------------------------------------
	// Create a root context that cancels on SIGINT (Ctrl+C) or SIGTERM.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logStartupInfo()

	// -------------------------------------------------------------------------
	// 4. Application Logic
	// -------------------------------------------------------------------------
	if err := run(ctx); err != nil {
		slog.Error(config.ErrAppFailed,
			config.LogKeyComponent, config.CompMain,
			config.LogKeyError, err,
		)
		return config.ExitCodeError
	}

	slog.Info(config.MsgAppStop, config.LogKeyComponent, config.CompMain)
	return config.ExitCodeSuccess
}

// run initializes the Fyne application, wires dependencies, and starts the UI loop.
func run(ctx context.Context) error {
	// Initialize Fyne App.
	a := app.NewWithID(config.AppID)

	// Record the version for potential migration logic in future updates.
	a.Preferences().SetString(config.PrefLastRun, config.Version)

	// Dependency Injection.
	port := a.Preferences().StringWithFallback(config.PrefServerPort, config.DefaultPort)
	srv := server.NewCalendarServer(port)
	fetcher := engine.NewHTTPFetcher()

	// Initialize the UI Controller (MVC pattern).
	gui := ui.NewGoBirthdayApp(a, ctx, srv, fetcher)

	// Lifecycle Bridge:
	// Watch for context cancellation to quit the UI gracefully.
	go func() {
		<-ctx.Done()
		slog.Info(config.MsgCtxCancel, config.LogKeyComponent, config.CompMain)
		a.Quit()
	}()

	// Start the Application (blocks until main window closes).
	gui.Run()

	return nil
}

// printVersion outputs the build information to stdout and exits.
func printVersion() {
	fmt.Printf(config.MsgVersionOutput,
		config.AppName,
		config.Version,
		runtime.GOOS,
		runtime.GOARCH,
	)
}

// logStartupInfo logs environment details useful for debugging.
func logStartupInfo() {
	slog.Info(config.MsgAppStarting,
		config.LogKeyComponent, config.CompMain,
		slog.Group(config.LogKeyBuild,
			slog.String(config.LogKeyApp, config.AppName),
			slog.String(config.LogKeyVersion, config.Version),
			slog.String(config.LogKeyGoVer, runtime.Version()),
		),
		slog.Group(config.LogKeyEnv,
			slog.String(config.LogKeyOS, runtime.GOOS),
			slog.String(config.LogKeyArch, runtime.GOARCH),
			slog.Int(config.LogKeyPID, os.Getpid()),
		),
	)
}

// setupLogging configures the default slog logger.
func setupLogging(debugMode bool) io.Closer {
	var writers []io.Writer
	var logFile *os.File

	// 1. Always write to Stdout.
	writers = append(writers, os.Stdout)

	// 2. Attempt to set up a file writer in the user's cache directory.
	if logPath, err := getLogFilePath(); err == nil {
		// O_TRUNC resets logs on restart to prevent indefinite growth.
		// Use centralized permission constants for security.
		f, err := os.OpenFile(logPath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, config.FilePermUserRW)
		if err == nil {
			writers = append(writers, f)
			logFile = f
		} else {
			fmt.Fprintf(os.Stderr, config.MsgLogWarning, config.ErrLogFile, logPath, err)
		}
	}

	level := slog.LevelInfo
	if debugMode {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: debugMode,
	}

	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(writers...), opts))
	slog.SetDefault(logger)

	if logFile == nil {
		return nil
	}
	return logFile
}

// getLogFilePath determines the platform-specific cache directory for logs.
func getLogFilePath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("%s: %w", config.ErrCacheDir, err)
	}

	appDir := filepath.Join(cacheDir, config.AppID)

	// Ensure the directory exists with restricted permissions (700).
	if err := os.MkdirAll(appDir, config.DirPermUserRWX); err != nil {
		return "", fmt.Errorf("%s: %w", config.ErrCreateDir, err)
	}

	return filepath.Join(appDir, config.LogFileName), nil
}
