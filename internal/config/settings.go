package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/qdm12/gosettings"
	"github.com/qdm12/gosettings/reader"
	"github.com/qdm12/gosettings/validate"
	"github.com/qdm12/gotree"
	"github.com/qdm12/log"
)

type Settings struct {
	LogLevel        string
	UIListenAddress *string
	PlexURL         string
	PlexToken       string
	PlexLibraries   []string
	PlexSendTokenTo *string
	JellyfinURL     string
	JellyfinAPIKey  string
	JellyfinUser    string
	DryRun          *bool
}

// OverrideWith sets fields in the receiving settings
// from non-zero fields from the other settings given.
func (s *Settings) OverrideWith(other Settings) {
	s.LogLevel = gosettings.OverrideWithComparable(s.LogLevel, other.LogLevel)
	s.UIListenAddress = gosettings.OverrideWithPointer(s.UIListenAddress, other.UIListenAddress)
	s.PlexURL = gosettings.OverrideWithComparable(s.PlexURL, other.PlexURL)
	s.PlexToken = gosettings.OverrideWithComparable(s.PlexToken, other.PlexToken)
	s.PlexLibraries = gosettings.OverrideWithSlice(s.PlexLibraries, other.PlexLibraries)
	s.PlexSendTokenTo = gosettings.OverrideWithComparable(s.PlexSendTokenTo, other.PlexSendTokenTo)
	s.JellyfinURL = gosettings.OverrideWithComparable(s.JellyfinURL, other.JellyfinURL)
	s.JellyfinAPIKey = gosettings.OverrideWithComparable(s.JellyfinAPIKey, other.JellyfinAPIKey)
	s.JellyfinUser = gosettings.OverrideWithComparable(s.JellyfinUser, other.JellyfinUser)
	s.DryRun = gosettings.OverrideWithPointer(s.DryRun, other.DryRun)
}

// SetDefaults sets the defaults to all the zero-ed fields
// in the receiving settings.
func (s *Settings) SetDefaults() {
	s.LogLevel = gosettings.DefaultComparable(s.LogLevel, "info")
	s.UIListenAddress = gosettings.DefaultPointer(s.UIListenAddress, ":8080")
	s.PlexLibraries = gosettings.DefaultSlice(s.PlexLibraries, []string{})
	s.PlexSendTokenTo = gosettings.DefaultPointer(s.PlexSendTokenTo, "")
	s.DryRun = gosettings.DefaultPointer(s.DryRun, false)
}

// Validate validates all the settings are correct.
// Note `.SetDefaults()` must be called to ensure all
// the fields are not their zeroed value such as `nil`.
func (s *Settings) Validate(webUI bool) (err error) {
	_, err = log.ParseLevel(s.LogLevel)
	if err != nil {
		return fmt.Errorf("log level is not valid: %w", err)
	}

	if s.PlexURL != "" {
		if err = validateURL(s.PlexURL); err != nil {
			return fmt.Errorf("plex URL is not valid: %w", err)
		}
	}

	if s.JellyfinURL != "" {
		if err = validateURL(s.JellyfinURL); err != nil {
			return fmt.Errorf("jellyfin URL is not valid: %w", err)
		}
	}

	if webUI {
		// allow empty values if using the web ui
		err = s.validateForWebUI()
		if err != nil {
			return err
		}
	} else {
		err = s.validateForCLI()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s Settings) validateForCLI() error {
	switch {
	case s.PlexURL == "":
		return errors.New("plex URL is not set")
	case s.PlexToken == "":
		return errors.New("plex token is not set")
	case s.JellyfinURL == "":
		return errors.New("jellyfin URL is not set")
	case s.JellyfinAPIKey == "":
		return errors.New("jellyfin API key is not set")
	case s.JellyfinUser == "":
		return errors.New("jellyfin user is not set")
	}
	return nil
}

func (s Settings) validateForWebUI() error {
	err := validate.ListeningAddress(*s.UIListenAddress, os.Getuid())
	if err != nil {
		return fmt.Errorf("web ui listening address is not valid: %w", err)
	}
	return nil
}

func (s *Settings) Read(r *reader.Reader) (err error) {
	s.LogLevel = r.String("LOG_LEVEL")
	s.UIListenAddress = r.Get("WEBUI_LISTENING_ADDRESS")
	s.PlexURL = r.String("PLEX_URL", reader.ForceLowercase(false))
	s.PlexToken = r.String("PLEX_TOKEN", reader.ForceLowercase(false))
	s.PlexLibraries = r.CSV("PLEX_LIBRARIES", reader.ForceLowercase(false))
	s.PlexSendTokenTo = r.Get("PLEX_SEND_TOKEN_TO", reader.ForceLowercase(false))
	s.JellyfinURL = r.String("JELLYFIN_URL", reader.ForceLowercase(false))
	s.JellyfinAPIKey = r.String("JELLYFIN_API_KEY", reader.ForceLowercase(false))
	s.JellyfinUser = r.String("JELLYFIN_USER", reader.ForceLowercase(false))

	s.DryRun, err = r.BoolPtr("DRY_RUN")
	if err != nil {
		return err
	}

	return nil
}

// toLinesNode returns a gotree.Node with the settings
// as a formatted tree node.
func (s *Settings) toLinesNode() *gotree.Node {
	node := gotree.New("Settings:")
	node.Appendf("Log level: %s", s.LogLevel)
	node.Appendf("Web UI listening address (if in web ui mode): %s", *s.UIListenAddress)
	node.Appendf("Plex URL: %s", s.PlexURL)
	node.Appendf("Plex token: %s", gosettings.ObfuscateKey(s.PlexToken))
	node.Appendf("Plex Libraries: %s", strings.Join(s.PlexLibraries, ","))
	node.Appendf("Jellyfin URL: %s", s.JellyfinURL)
	node.Appendf("Jellyfin user: %s", s.JellyfinUser)
	node.Appendf("Jellyfin API key: %s", gosettings.ObfuscateKey(s.JellyfinAPIKey))
	node.Appendf("Dry run: %s", gosettings.BoolToYesNo(s.DryRun))
	return node
}

func (s Settings) String() string {
	return s.toLinesNode().String()
}

func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return fmt.Errorf("URL %q has invalid URL scheme which must be http or https", rawURL)
	case u.Host == "":
		return fmt.Errorf("URL %q has no host", rawURL)
	}
	return nil
}
