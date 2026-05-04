package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/qdm12/goservices/httpserver"
	"github.com/qdm12/gosettings/reader"
	"github.com/qdm12/log"
	"github.com/qdm12/plex-to-jellyfin/internal/config"
	"github.com/qdm12/plex-to-jellyfin/internal/jellyfin"
	"github.com/qdm12/plex-to-jellyfin/internal/migrate"
	"github.com/qdm12/plex-to-jellyfin/internal/plex"
	"github.com/qdm12/plex-to-jellyfin/internal/webui"
)

//nolint:gochecknoglobals
var (
	version = "unknown"
	commit  = "unknown"
	date    = "an unknown date"
)

type buildInfo struct {
	Version string
	Commit  string
	Date    string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	buildInfo := buildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}

	errorCh := make(chan error)
	go func() {
		errorCh <- _main(ctx, buildInfo, os.Stdout)
	}()

	select {
	case err := <-errorCh:
		close(errorCh)
		if err == nil { // expected exit
			os.Exit(0)
		}
		fmt.Println("Fatal error:", err)
		os.Exit(1)
	case <-ctx.Done():
		stop()
	}

	const shutdownGracePeriod = time.Second
	timer := time.NewTimer(shutdownGracePeriod)
	select {
	case <-errorCh:
		if !timer.Stop() {
			<-timer.C
		}
	case <-timer.C:
		fmt.Println("Shutdown timed out")
	}

	os.Exit(1)
}

func _main(ctx context.Context, buildInfo buildInfo, stdout io.Writer) (err error) {
	logger := log.New(log.SetWriters(stdout))

	logger.Infof("Version %s (commit %s built on %s)",
		buildInfo.Version, buildInfo.Commit, buildInfo.Date)

	command := detectSubcommand(os.Args[1:])
	switch command {
	case "": // web UI
	case "migrate":
	default:
		return fmt.Errorf("unknown subcommand %q", command)
	}

	reader := reader.New(reader.Settings{})
	var settings config.Settings
	err = settings.Read(reader)
	if err != nil {
		return fmt.Errorf("reading settings: %w", err)
	}
	settings.SetDefaults()

	logLevel, err := log.ParseLevel(settings.LogLevel)
	if err != nil {
		return fmt.Errorf("parsing log level: %w", err)
	}
	logger.Patch(log.SetLevel(logLevel))

	webUI := command == ""
	err = settings.Validate(webUI)
	if err != nil {
		return fmt.Errorf("validating settings: %w", err)
	}

	logger.Infof("%s", settings)

	const timeout = 15 * time.Second
	client := &http.Client{
		Timeout: timeout,
	}

	if webUI {
		err = runWebUI(ctx, client, logger, logLevel, settings)
		if err != nil {
			return fmt.Errorf("running web UI: %w", err)
		}
		return nil
	}

	plexClient := plex.New(plex.Settings{
		Client:  client,
		BaseURL: settings.PlexURL,
		Token:   settings.PlexToken,
	})

	jellyfinClient := jellyfin.New(jellyfin.Settings{
		Client:  client,
		BaseURL: settings.JellyfinURL,
		APIKey:  settings.JellyfinAPIKey,
	})

	migrator := migrate.New(plexClient, jellyfinClient, logger,
		settings.JellyfinUser, settings.PlexLibraries, *settings.DryRun)

	err = migrator.Run(ctx)
	if err != nil {
		return fmt.Errorf("running migration: %w", err)
	}
	return nil
}

func detectSubcommand(arguments []string) (subcommand string) {
	if len(arguments) == 0 {
		return ""
	}
	firstArgument := strings.TrimSpace(arguments[0])
	if firstArgument == "" || strings.HasPrefix(firstArgument, "-") {
		return ""
	}
	return firstArgument
}

func runWebUI(ctx context.Context, client *http.Client, logger migrate.Logger,
	logLevel log.Level, settings config.Settings,
) error {
	userInterface, err := webui.New(webui.Settings{
		Ctx: ctx,
		InitialSettings: webui.MigrationSettings{
			PlexURL:        settings.PlexURL,
			PlexToken:      settings.PlexToken,
			PlexLibraries:  settings.PlexLibraries,
			JellyfinURL:    settings.JellyfinURL,
			JellyfinUser:   settings.JellyfinUser,
			JellyfinAPIKey: settings.JellyfinAPIKey,
			DryRun:         *settings.DryRun,
		},
		Logger:          logger,
		LogLevel:        logLevel,
		Client:          client,
		PlexSendTokenTo: *settings.PlexSendTokenTo,
	})
	if err != nil {
		return fmt.Errorf("creating web ui: %w", err)
	}

	httpServer, err := httpserver.New(httpserver.Settings{
		Handler: userInterface.Handler(),
		Address: new(*settings.UIListenAddress),
		Logger:  logger,
	})
	if err != nil {
		return fmt.Errorf("creating HTTP server: %w", err)
	}

	runErrCh, err := httpServer.Start(ctx)
	if err != nil {
		return fmt.Errorf("starting HTTP server: %w", err)
	}

	httpAddress := "http://" + httpServer.GetAddress()
	logger.Infof("Web UI available at %s", httpAddress)

	err = openBrowser(ctx, httpAddress)
	if err != nil {
		logger.Warnf("opening browser at %s: %v", httpAddress, err)
	}

	select {
	case err := <-runErrCh:
		if err != nil {
			return fmt.Errorf("running HTTP server: %w", err)
		}
		return nil
	case <-ctx.Done():
		err := httpServer.Stop()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("stopping HTTP server: %w", err)
		}
		return nil
	}
}

func openBrowser(ctx context.Context, address string) error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.CommandContext(ctx, "cmd", "/c", "start", "", address)
		err := cmd.Start()
		if err != nil {
			return fmt.Errorf("starting browser command for windows: %w", err)
		}
		return nil
	case "darwin":
		cmd := exec.CommandContext(ctx, "open", address)
		err := cmd.Start()
		if err == nil {
			return nil
		}
		return fmt.Errorf("starting browser command for macos: %w", err)
	default:
		cmd := exec.CommandContext(ctx, "xdg-open", address)
		err := cmd.Start()
		if err == nil {
			return nil
		}
		return fmt.Errorf("starting browser command for unix: %w", err)
	}
}
