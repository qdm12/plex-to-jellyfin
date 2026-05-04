package webui

import (
	"context"
	"errors"
	"net/http"

	"github.com/qdm12/log"
)

type Settings struct {
	Ctx             context.Context //nolint:containedctx
	InitialSettings MigrationSettings
	Logger          Logger
	LogLevel        log.Level
	Client          *http.Client
	PlexSendTokenTo string
}

type MigrationSettings struct {
	PlexURL        string
	PlexToken      string
	PlexLibraries  []string
	JellyfinURL    string
	JellyfinUser   string
	JellyfinAPIKey string
	DryRun         bool
}

func (s *Settings) Validate() error {
	switch {
	case s.Ctx == nil:
		return errors.New("context is nil")
	case s.Logger == nil:
		return errors.New("logger is nil")
	case s.Client == nil:
		return errors.New("HTTP client is nil")
	}
	return nil
}

func (s *MigrationSettings) validate() error {
	switch {
	case s.PlexURL == "":
		return errors.New("plex URL is empty")
	case s.PlexToken == "":
		return errors.New("plex token is empty")
	case s.JellyfinURL == "":
		return errors.New("jellyfin URL is empty")
	case s.JellyfinUser == "":
		return errors.New("jellyfin user is empty")
	case s.JellyfinAPIKey == "":
		return errors.New("jellyfin API key is empty")
	}
	return nil
}
