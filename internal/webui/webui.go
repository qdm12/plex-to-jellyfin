package webui

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/qdm12/log"
	"github.com/qdm12/plex-to-jellyfin/internal/jellyfin"
	"github.com/qdm12/plex-to-jellyfin/internal/migrate"
	"github.com/qdm12/plex-to-jellyfin/internal/plex"
)

type UI struct {
	ctx             context.Context //nolint:containedctx
	initialSettings MigrationSettings
	logger          Logger
	logLevel        log.Level
	httpClient      *http.Client
	plexSendTokenTo string
	jobs            *jobs
}

func New(settings Settings) (*UI, error) {
	return &UI{
		ctx:             settings.Ctx,
		initialSettings: settings.InitialSettings,
		logger:          settings.Logger,
		logLevel:        settings.LogLevel,
		httpClient:      settings.Client,
		plexSendTokenTo: settings.PlexSendTokenTo,
		jobs:            newJobs(),
	}, nil
}

func (u *UI) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", u.handleIndex)
	mux.HandleFunc("GET /step/get-plex-token/settings", u.handleGetPlexTokenSettings)
	mux.HandleFunc("POST /step/get-plex-token/fetch", u.handleGetPlexTokenFetch)
	mux.HandleFunc("GET /step/migrate/settings", u.handleMigrateSettings)
	mux.HandleFunc("POST /step/migrate/start", u.handleMigrateStart)
	mux.HandleFunc("GET /step/migrate/logs", u.handleMigrateLogs)
	return mux
}

func (u *UI) handleIndex(responseWriter http.ResponseWriter, _ *http.Request) {
	data := struct {
		SelectStepHTML template.HTML
	}{
		SelectStepHTML: selectStepHTML,
	}
	err := indexTemplate.Execute(responseWriter, data)
	if err != nil {
		http.Error(responseWriter, fmt.Sprintf("executing index template: %s", err), http.StatusInternalServerError)
	}
}

func (u *UI) handleGetPlexTokenSettings(responseWriter http.ResponseWriter, _ *http.Request) {
	err := getPlexTokenTemplate.Execute(responseWriter, getPlexTokenData{})
	if err != nil {
		http.Error(responseWriter, fmt.Sprintf("executing get plex token template: %s", err), http.StatusInternalServerError)
	}
}

func (u *UI) handleGetPlexTokenFetch(responseWriter http.ResponseWriter, request *http.Request) {
	const maxFormSizeBytes = 1024 * 1024
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxFormSizeBytes)

	err := request.ParseMultipartForm(maxFormSizeBytes)
	if err != nil {
		http.Error(responseWriter, fmt.Sprintf("parsing form: %s", err), http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(request.FormValue("plex_username"))
	password := strings.TrimSpace(request.FormValue("plex_password"))
	oneTimeCode := strings.TrimSpace(request.FormValue("plex_otp"))

	data := getPlexTokenData{
		PlexUsername: username,
		PlexOTP:      oneTimeCode,
	}

	if username == "" || password == "" {
		data.ErrorMessage = "Plex username and password are required"
		err = getPlexTokenTemplate.Execute(responseWriter, data)
		if err != nil {
			http.Error(responseWriter, fmt.Sprintf("executing get plex token template: %s", err), http.StatusInternalServerError)
		}
		return
	}

	token, err := plex.FetchToken(request.Context(), u.httpClient, username, password, oneTimeCode)
	if err != nil {
		data.ErrorMessage = err.Error()
		err = getPlexTokenTemplate.Execute(responseWriter, data)
		if err != nil {
			http.Error(responseWriter, fmt.Sprintf("executing get plex token template: %s", err), http.StatusInternalServerError)
		}
		return
	}

	resultData := struct {
		PlexToken string
		SendTo    string
	}{
		PlexToken: token,
		SendTo:    u.plexSendTokenTo,
	}
	err = getPlexTokenResultTemplate.Execute(responseWriter, resultData)
	if err != nil {
		message := fmt.Sprintf("executing get plex token result template: %s", err)
		http.Error(responseWriter, message, http.StatusInternalServerError)
	}
}

func (u *UI) handleMigrateSettings(responseWriter http.ResponseWriter, request *http.Request) {
	data := migrateSettingsDataFromSettings(u.initialSettings)
	prefilledToken := strings.TrimSpace(request.URL.Query().Get("plex_token"))
	if prefilledToken != "" {
		data.PlexToken = prefilledToken
	}
	err := migrateSettingsTemplate.Execute(responseWriter, data)
	if err != nil {
		http.Error(responseWriter, fmt.Sprintf("executing settings template: %s", err), http.StatusInternalServerError)
	}
}

func (u *UI) handleMigrateStart(responseWriter http.ResponseWriter, request *http.Request) {
	const maxFormSizeBytes = 10 * 1024 * 1024 // 10 MiB
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxFormSizeBytes)

	err := request.ParseForm()
	if err != nil {
		http.Error(responseWriter, fmt.Sprintf("parsing form: %s", err), http.StatusBadRequest)
		return
	}

	settings := MigrationSettings{
		PlexURL:        strings.TrimSpace(request.FormValue("plex_url")),
		PlexToken:      strings.TrimSpace(request.FormValue("plex_token")),
		JellyfinURL:    strings.TrimSpace(request.FormValue("jellyfin_url")),
		JellyfinAPIKey: strings.TrimSpace(request.FormValue("jellyfin_api_key")),
		JellyfinUser:   strings.TrimSpace(request.FormValue("jellyfin_user")),
		PlexLibraries:  parseLibraries(request.FormValue("plex_libraries")),
		DryRun:         request.FormValue("dry_run") == "on",
	}
	err = settings.validate()
	if err != nil {
		formData := migrateSettingsDataFromSettings(settings)
		formData.ErrorMessage = err.Error()
		templateErr := migrateSettingsTemplate.Execute(responseWriter, formData)
		if templateErr != nil {
			message := fmt.Sprintf("executing settings template: %s", templateErr)
			http.Error(responseWriter, message, http.StatusInternalServerError)
		}
		return
	}

	job := u.jobs.create()

	ctx := context.WithoutCancel(request.Context())
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer cancel()
		select {
		case <-done:
		case <-u.ctx.Done():
			job.markDone(errors.New("operation aborted by main program"))
		}
	}()

	go u.runMigrationJob(ctx, done, job, settings)

	progressData := struct {
		JobID uint32
	}{
		JobID: job.id,
	}
	err = migrateProgressTemplate.Execute(responseWriter, progressData)
	if err != nil {
		http.Error(responseWriter, fmt.Sprintf("executing progress template: %s", err), http.StatusInternalServerError)
	}
}

func (u *UI) runMigrationJob(ctx context.Context, done chan<- struct{}, job *job, settings MigrationSettings) {
	defer close(done)
	logger := newStreamLogger(u.logger, u.logLevel, job)

	plexClient := plex.New(plex.Settings{
		Client:  u.httpClient,
		BaseURL: settings.PlexURL,
		Token:   settings.PlexToken,
	})

	jellyfinClient := jellyfin.New(jellyfin.Settings{
		Client:  u.httpClient,
		BaseURL: settings.JellyfinURL,
		APIKey:  settings.JellyfinAPIKey,
	})

	migrator := migrate.New(plexClient, jellyfinClient, logger,
		settings.JellyfinUser, settings.PlexLibraries, settings.DryRun)

	err := migrator.Run(ctx)
	job.markDone(err)
	if err != nil {
		u.logger.Errorf("web migration %d failed: %v", job.id, err)
		return
	}
	u.logger.Infof("web migration %d completed", job.id)
}

func (u *UI) handleMigrateLogs(responseWriter http.ResponseWriter, request *http.Request) {
	job := u.getJob(request, responseWriter)
	if job == nil {
		return
	}

	header := responseWriter.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")

	_, ok := responseWriter.(http.Flusher)
	if !ok {
		http.Error(responseWriter, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	go func() {
		select {
		case <-ctx.Done(): // completed
		case <-u.ctx.Done():
			cancel()
		}
	}()

	streamLogs(ctx, job, responseWriter)
}

func (u *UI) getJob(request *http.Request, responseWriter http.ResponseWriter) *job {
	jobIDString := request.URL.Query().Get("id")
	if jobIDString == "" {
		http.Error(responseWriter, "missing migration job ID", http.StatusBadRequest)
		return nil
	}
	const base, bitSize = 10, 32
	jobIDUint64, err := strconv.ParseUint(jobIDString, base, bitSize)
	if err != nil {
		http.Error(responseWriter, "invalid migration job ID", http.StatusBadRequest)
		return nil
	}

	job, found := u.jobs.get(uint32(jobIDUint64))
	if !found {
		http.Error(responseWriter, "migration job not found", http.StatusNotFound)
		return nil
	}

	return job
}

func streamLogs(ctx context.Context, job *job, responseWriter http.ResponseWriter) {
	// type already checked in [UI.handleMigrateLogs]
	flusher := responseWriter.(http.Flusher) //nolint:forcetypeassert

	nextLineIndex := 0
	const pollInterval = 50 * time.Millisecond
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		lines, completed, err := job.snapshot()
		startLineIndex := nextLineIndex
		const maxLinesInBuffer = 10
		for nextLineIndex < len(lines) {
			writeLogSSE(responseWriter, lines[nextLineIndex])
			nextLineIndex++
			if (nextLineIndex-startLineIndex)%maxLinesInBuffer == 0 {
				flusher.Flush()
			}
		}

		if completed {
			if err != nil {
				writeLogSSE(responseWriter, err.Error())
			}
			flusher.Flush()
			break
		}

		if (nextLineIndex-startLineIndex)%maxLinesInBuffer != 0 {
			// Only flush if we did not flush in the loop above.
			flusher.Flush()
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func parseLibraries(raw string) (libraries []string) {
	for value := range strings.SplitSeq(raw, ",") {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		libraries = append(libraries, trimmed)
	}
	return libraries
}

func writeLogSSE(responseWriter http.ResponseWriter, data string) {
	const eventName = "log"
	for line := range strings.SplitSeq(data, "\n") {
		_, _ = fmt.Fprintf(responseWriter, "event: %s\ndata: %s\n\n", eventName, line)
	}
}

type streamLogger struct {
	baseLogger Logger
	level      log.Level
	job        *job
}

func newStreamLogger(baseLogger Logger, level log.Level, job *job) *streamLogger {
	return &streamLogger{
		baseLogger: baseLogger,
		level:      level,
		job:        job,
	}
}

func (l *streamLogger) Debugf(format string, args ...any) {
	if l.level < log.LevelDebug {
		return
	}
	l.baseLogger.Debugf(format, args...)
	message := fmt.Sprintf(format, args...)
	l.job.addLine(formatWebUILogLine("DEBUG", message))
}

func (l *streamLogger) Info(message string) {
	if l.level < log.LevelInfo {
		return
	}
	l.baseLogger.Info(message)
	l.job.addLine(formatWebUILogLine("INFO", message))
}

func (l *streamLogger) Infof(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	l.Info(message)
}

func (l *streamLogger) Warnf(format string, args ...any) {
	if l.level < log.LevelWarn {
		return
	}
	l.baseLogger.Warnf(format, args...)
	message := fmt.Sprintf(format, args...)
	l.job.addLine(formatWebUILogLine("WARN", message))
}

func (l *streamLogger) Errorf(format string, args ...any) {
	if l.level < log.LevelError {
		return
	}
	l.baseLogger.Errorf(format, args...)
	message := fmt.Sprintf(format, args...)
	l.job.addLine(formatWebUILogLine("ERROR", message))
}

func formatWebUILogLine(level, message string) string {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	return timestamp + " " + level + " " + strings.TrimSpace(message)
}
